# goliday 构建入口：二进制与容器镜像共用同一构建逻辑。
#
# Dockerfile 构建阶段调用 `make build-server`，保证 `docker build` 与本地
# `make build` 的产物出自同一命令（静态编译参数、版本注入只有这一份定义）。
#
# 常用目标：
#   make build                 构建 goliday-server 与 goliday-tool 到 bin/
#   make build-server          仅构建 goliday-server
#   make build-tool            仅构建 goliday-tool
#   make image VERSION=0.2.0   构建容器镜像（等价 docker build --build-arg VERSION=0.2.0）
#   make check                 提交门禁四件套：build/vet/test/gofmt 检查
#   make lint                  golangci-lint 零告警检查
#   make fix                   go fix ./...（重复执行至不动点）
#   make clean                 清理 bin/
#
# 版本号注入：VERSION 缺省 dev，经 -ldflags -X main.version 注入（-v 与
# /healthz 输出）；CI 的 tag 构建经 docker.yml --build-arg 传入。
#
# 交叉编译：GOOS/GOARCH 经环境变量透传（go build 原生识别），Windows 目标
# 自动追加 .exe 后缀，如
#   GOOS=windows GOARCH=amd64 make build-server   # → bin/goliday-server.exe

VERSION ?= dev
OUT_DIR ?= bin

# Windows 产物按平台惯例加 .exe 后缀。
EXE_SUFFIX :=
ifeq ($(GOOS),windows)
EXE_SUFFIX := .exe
endif

# 静态编译参数：CGO_ENABLED=0（distroless 无动态 loader，必须静态链接）；
# -trimpath 去构建机路径；-s -w 去符号表减小体积；-X 注入版本号。
LDFLAGS = -s -w -X main.version=$(VERSION)

.PHONY: all build build-server build-tool download-deps image check lint fix clean

all: build

build: build-server build-tool

# 依赖预下载（Dockerfile 依赖层调用；本地一般无需单独执行）。
download-deps:
	go mod download

# 二进制始终经 go build 重算（go 构建缓存兜底，改动版本号亦不会误用旧产物）。
build-server:
	mkdir -p $(OUT_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(OUT_DIR)/goliday-server$(EXE_SUFFIX) ./cmd/goliday-server

build-tool:
	mkdir -p $(OUT_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(OUT_DIR)/goliday-tool$(EXE_SUFFIX) ./cmd/goliday-tool

# 容器镜像：Dockerfile 内部同样执行 make build-server，构建逻辑唯一。
#
# 防递归约束：Dockerfile 只允许引用不触碰 docker 的目标（如
# download-deps / build-server），不得引用本目标——否则
# make image → docker build → 容器内 make image 将构成真实循环
# （docker-in-docker）。二者互引是「入口委托 + 构建同一性」的分层
# 委托，执行路径为 DAG，本约束是其成立的前提。
image:
	docker build --build-arg VERSION=$(VERSION) -t goliday:$(VERSION) .

# 提交门禁（AGENTS.md 引用的 agents/go.md 四件套；go fix 与 golangci-lint 见 fix/lint 目标）。
check:
	go build ./...
	go vet ./...
	go test -count=1 ./...
	@unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then \
		echo "以下文件未通过 gofmt："; echo "$$unformatted"; exit 1; \
	fi

lint:
	golangci-lint run ./...

fix:
	go fix ./...

clean:
	rm -rf $(OUT_DIR)
