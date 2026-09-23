// goliday-server 基于 goliday 日历的 HTTP 查询服务。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
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

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	grpcAddr := flag.String("grpc-addr", ":50051", "gRPC 监听地址（空字符串禁用 gRPC）")
	configDir := flag.String("config-dir", "./configs", "年份配置目录")
	showVersion := flag.Bool("v", false, "打印版本信息后退出")
	flag.Parse()

	if *showVersion {
		fmt.Printf("goliday-server version %s\n", version)
		return
	}

	store, err := goliday.LoadDir(*configDir)
	if err != nil {
		log.Fatalf("加载配置目录 %s 失败: %v", *configDir, err)
	}
	calendar := goliday.NewCalendar(store)

	years := store.Years()
	names := make([]string, len(years))
	for i, y := range years {
		names[i] = strconv.Itoa(y)
	}

	srv := &http.Server{
		Addr:    *addr,
		Handler: newHandler(store, calendar),
	}

	// gRPC 与 HTTP 同进程：-grpc-addr 为空字符串时禁用。
	var grpcSrv *grpc.Server
	var grpcLis net.Listener
	errCh := make(chan error, 2)
	if *grpcAddr != "" {
		grpcLis, err = net.Listen("tcp", *grpcAddr)
		if err != nil {
			log.Fatalf("gRPC 监听 %s 失败: %v", *grpcAddr, err)
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

	// 监听 SIGINT/SIGTERM，收到信号后优雅关闭。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		stop()
		if err != nil {
			log.Fatalf("服务异常退出: %v", err)
		}
		return
	case <-ctx.Done():
		stop()
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
}
