// goliday-server 基于 goliday 日历的 HTTP 查询服务。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/JayceChant/goliday"
)

// version 服务版本号：缺省 dev（本地/go install 构建），正式版本经构建期
// 注入——docker.yml 的 tag 构建以 -ldflags "-X main.version=<semver>"
// 覆盖（release-please 的 simple 策略只维护 CHANGELOG/tag，不改代码常量，
// 硬编码会随每次发版漂移）。
var version = "dev"

// shutdownTimeout 优雅关闭时等待存量请求完成的超时时间。
const shutdownTimeout = 5 * time.Second

// HTTP 服务端超时（README 已声明服务应置于反代之后；直连暴露时这些
// 超时可阻断慢请求对连接的长期占用，如 slowloris 式读挂起）。
const (
	// readHeaderTimeout 读请求头超时：防止慢速发送头部的连接占坑。
	readHeaderTimeout = 10 * time.Second
	// idleTimeout keep-alive 空闲超时：回收长期无请求的连接。
	idleTimeout = 120 * time.Second
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("goliday-server: %v", err)
	}
}

// run 服务主流程（main 的可测形态）：解析参数 → 加载配置 → 启动
// HTTP/gRPC → 等待退出信号并优雅关闭。错误统一返回由 main 以退出码 1
// 终止；信号监听在启动监听之前安装，提前到达的退出信号也能安全走优雅
// 关闭路径。
func run(args []string) error {
	fs := flag.NewFlagSet("goliday-server", flag.ContinueOnError)
	// 解析错误的输出只经返回的 error 报告一次，丢弃 FlagSet 自带打印以免重复。
	fs.SetOutput(io.Discard)
	addr := fs.String("addr", ":8080", "HTTP 监听地址")
	grpcAddr := fs.String("grpc-addr", ":50051", "gRPC 监听地址（空字符串禁用 gRPC）")
	configDir := fs.String("config-dir", "./configs", "年份配置目录")
	showVersion := fs.Bool("v", false, "打印版本信息后退出")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *showVersion {
		fmt.Printf("goliday-server version %s\n", version)
		return nil
	}

	store, err := goliday.LoadDir(*configDir)
	if err != nil {
		return fmt.Errorf("加载配置目录 %s 失败: %w", *configDir, err)
	}
	calendar := goliday.NewCalendar(store)

	years := store.Years()
	names := make([]string, len(years))
	for i, y := range years {
		names[i] = strconv.Itoa(y)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           newHandler(store, calendar),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// gRPC 与 HTTP 同进程：-grpc-addr 为空字符串时禁用。
	var grpcSrv *grpc.Server
	errCh := make(chan error, 2)
	if *grpcAddr != "" {
		grpcLis, err := net.Listen("tcp", *grpcAddr)
		if err != nil {
			return fmt.Errorf("gRPC 监听 %s 失败: %w", *grpcAddr, err)
		}
		grpcSrv = newGRPCServer(calendar)
		go func() {
			log.Printf("goliday-server gRPC 监听 %s", *grpcAddr)
			if err := grpcSrv.Serve(grpcLis); err != nil {
				errCh <- err
				return
			}
			errCh <- nil
		}()
	} else {
		log.Println("gRPC 服务已禁用（-grpc-addr 为空）")
	}

	go func() {
		log.Printf("goliday-server 监听 %s，已加载年份：%s", *addr, strings.Join(names, ", "))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("服务异常退出: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	log.Println("收到退出信号，开始优雅关闭……")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP 优雅关闭失败: %v", err)
	} else {
		log.Println("HTTP 服务已关闭")
	}
	if grpcSrv != nil {
		grpcSrv.GracefulStop()
		log.Println("gRPC 服务已关闭")
	}
	log.Println("goliday-server 已退出")
	return nil
}
