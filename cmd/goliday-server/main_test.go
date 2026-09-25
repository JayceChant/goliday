// 白盒测试（package main）：驱动 run()（main 的可测形态）覆盖服务主流程——
// 版本打印、参数/配置/监听失败路径、双协议启动与 SIGTERM 优雅关闭。
package main

import (
	"io"
	"log"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

// silenceLog 静默 run() 的启动/关闭日志，测试结束后恢复。
func silenceLog(t *testing.T) {
	t.Helper()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
}

// freeAddr 返回一个当前空闲的 127.0.0.1 端口（先监听后释放；存在极小的
// 被第三方抢占窗口，仅用于测试）。
func freeAddr(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("获取空闲端口失败: %v", err)
	}
	addr := lis.Addr().String()
	if err := lis.Close(); err != nil {
		t.Fatalf("释放端口失败: %v", err)
	}
	return addr
}

// waitTCP 轮询拨号直到 addr 可连或超时（等待服务就绪，避免固定 sleep）。
func waitTCP(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("等待 %s 可连超时（%v）", addr, timeout)
}

// runAsync 在 goroutine 中启动 run，返回结果管道。
func runAsync(args ...string) <-chan error {
	done := make(chan error, 1)
	go func() { done <- run(args) }()
	return done
}

// waitDone 在超时内等待 run 返回，返回其错误。
func waitDone(t *testing.T, done <-chan error, timeout time.Duration) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		t.Fatal("run 未在超时内返回")
		return nil // 不可达，安抚编译器
	}
}

// sigterm 触发优雅关闭路径（NotifyContext 已在 run 内安装处理器）。
func sigterm(t *testing.T) {
	t.Helper()
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("发送 SIGTERM 失败: %v", err)
	}
}

func TestRunVersionFlag(t *testing.T) {
	silenceLog(t)

	// 捕获 stdout 断言版本输出（测试构建未注入版本号，应为缺省 dev）。
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("创建管道失败: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	gotErr := run([]string{"-v"})
	os.Stdout = orig
	_ = w.Close()
	buf, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("读取输出失败: %v", err)
	}
	if gotErr != nil {
		t.Fatalf("run(-v) = %v, want nil", gotErr)
	}
	if out := string(buf); out != "goliday-server version dev\n" {
		t.Errorf("run(-v) 输出 = %q, want %q", out, "goliday-server version dev\n")
	}
}

func TestRunFlagError(t *testing.T) {
	silenceLog(t)
	if err := run([]string{"-bogus"}); err == nil {
		t.Error("run(-bogus) 未报错，期望参数解析失败")
	}
}

func TestRunConfigDirFail(t *testing.T) {
	silenceLog(t)
	err := run([]string{"-config-dir", "./不存在-的-目录"})
	if err == nil || !strings.Contains(err.Error(), "加载配置目录") {
		t.Errorf("run(坏 config-dir) = %v, 期望包含「加载配置目录」的错误", err)
	}
}

func TestRunGRPCListenFail(t *testing.T) {
	silenceLog(t)
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("占用端口失败: %v", err)
	}
	defer func() { _ = occupied.Close() }()

	gotErr := waitDone(t, runAsync(
		"-addr", freeAddr(t),
		"-grpc-addr", occupied.Addr().String(),
		"-config-dir", "../../testdata",
	), 5*time.Second)
	if gotErr == nil || !strings.Contains(gotErr.Error(), "gRPC 监听") {
		t.Errorf("run(gRPC 端口被占) = %v, 期望包含「gRPC 监听」的错误", gotErr)
	}
}

func TestRunHTTPListenFail(t *testing.T) {
	silenceLog(t)
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("占用端口失败: %v", err)
	}
	defer func() { _ = occupied.Close() }()

	gotErr := waitDone(t, runAsync(
		"-addr", occupied.Addr().String(),
		"-grpc-addr", freeAddr(t),
		"-config-dir", "../../testdata",
	), 5*time.Second)
	if gotErr == nil || !strings.Contains(gotErr.Error(), "服务异常退出") {
		t.Errorf("run(HTTP 端口被占) = %v, 期望包含「服务异常退出」的错误", gotErr)
	}
}

func TestRunServeAndGracefulShutdown(t *testing.T) {
	silenceLog(t)
	httpAddr := freeAddr(t)
	done := runAsync(
		"-addr", httpAddr,
		"-grpc-addr", freeAddr(t),
		"-config-dir", "../../testdata",
	)

	waitTCP(t, httpAddr, 5*time.Second)
	sigterm(t)
	if err := waitDone(t, done, shutdownTimeout+5*time.Second); err != nil {
		t.Fatalf("优雅关闭后 run 返回错误: %v", err)
	}
}

func TestRunGRPCDisabled(t *testing.T) {
	silenceLog(t)
	httpAddr := freeAddr(t)
	done := runAsync("-addr", httpAddr, "-grpc-addr", "", "-config-dir", "../../testdata")

	waitTCP(t, httpAddr, 5*time.Second)
	sigterm(t)
	if err := waitDone(t, done, shutdownTimeout+5*time.Second); err != nil {
		t.Fatalf("禁用 gRPC 时 run 返回错误: %v", err)
	}
}
