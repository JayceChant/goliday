# goliday-server 容器镜像：多阶段构建，运行层基于 distroless 静态镜像。
# 构建产物为 CGO_ENABLED=0 静态二进制；年份配置不打入镜像，运行时经
# volume 挂载并以 -config-dir 指定（配置与镜像解耦）。
#
# 构建：docker build -t goliday .
# 运行：docker run -p 8080:8080 -v $PWD/configs:/data:ro goliday -config-dir /data

# 构建阶段：先复制 go.mod/go.sum 下载依赖（层缓存友好），再复制源码编译。
FROM golang:1.27 AS build

# 二进制版本号：默认 dev，CI tag 构建经 --build-arg VERSION=<semver> 注入。
ARG VERSION=dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 静态编译：distroless/static 无动态 loader，必须 CGO_ENABLED=0；
# -trimpath 去除构建机路径，-ldflags="-s -w" 去除符号表减小体积，
# -X main.version 注入构建版本号（-v 与 /healthz 输出）。
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/goliday-server ./cmd/goliday-server

# 运行阶段：distroless 静态镜像，非 root 用户，无 shell、无包管理器。
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/goliday-server /goliday-server

# HTTP（默认 :8080）与 gRPC（默认 :50051）。
EXPOSE 8080 50051

USER nonroot:nonroot

ENTRYPOINT ["/goliday-server"]
