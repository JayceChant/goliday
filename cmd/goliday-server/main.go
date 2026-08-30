// goliday-server 基于 goliday 日历的 HTTP 查询服务。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"goliday"
)

// version 服务版本号。
const version = "0.1.0"

// shutdownTimeout 优雅关闭时等待存量请求完成的超时时间。
const shutdownTimeout = 5 * time.Second

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
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

	errCh := make(chan error, 1)
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
	defer stop()

	select {
	case err := <-errCh:
		if err != nil {
			log.Fatalf("HTTP 服务异常退出: %v", err)
		}
		return
	case <-ctx.Done():
	}

	log.Println("收到退出信号，开始优雅关闭……")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("优雅关闭失败: %v", err)
	}
	log.Println("goliday-server 已退出")
}
