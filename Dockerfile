# goliday-server 容器镜像：多阶段构建，运行层基于 distroless 静态镜像。
# 构建产物为 CGO_ENABLED=0 静态二进制；年份配置不打入镜像，运行时经
# volume 挂载并以 -config-dir 指定（配置与镜像解耦）。
#
# 构建（与本地 make build 共用同一构建逻辑，见 Makefile）：
#   make image VERSION=0.2.0        # 等价下方 docker build
#   docker build --build-arg VERSION=0.2.0 -t goliday:0.2.0 .
#
# 运行：docker run -p 8080:8080 -v $PWD/configs:/data:ro goliday -config-dir /data

# 构建阶段：先复制 Makefile/go.mod/go.sum（构建脚本与依赖下载分层缓存友好），
# 再复制源码，经 make build-server 编译——与本地二进制构建同一命令同一参数。
# 防递归约束：本文件只调用不触碰 docker 的 Makefile 目标
# （download-deps/build-server），不得调用 make image——否则与 Makefile
# 的 image 目标互引构成真实构建循环。golang 镜像基于 buildpack-deps 的
# scm 变体，不含 make，须先安装。
FROM golang:1.27 AS build

RUN apt-get update \
    && apt-get install -y --no-install-recommends make \
    && rm -rf /var/lib/apt/lists/*

# 二进制版本号：默认 dev，CI tag 构建经 --build-arg VERSION=<semver> 注入。
ARG VERSION=dev

WORKDIR /src

COPY Makefile go.mod go.sum ./
RUN make download-deps

COPY . .

# make build-server 内部：CGO_ENABLED=0 go build -trimpath \
#   -ldflags "-s -w -X main.version=${VERSION}"（唯一定义，见 Makefile）。
RUN make build-server VERSION=${VERSION} OUT_DIR=/out

# 运行阶段：distroless 静态镜像，非 root 用户，无 shell、无包管理器。
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/goliday-server /goliday-server

# HTTP（默认 :8080）与 gRPC（默认 :50051）。
EXPOSE 8080 50051

USER nonroot:nonroot

ENTRYPOINT ["/goliday-server"]
