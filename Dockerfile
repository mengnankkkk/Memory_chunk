# 构建阶段
FROM golang:1.25-alpine AS builder

WORKDIR /src

ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}

# 安装构建依赖
RUN apk add --no-cache git

# 复制 go mod 文件并下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/refiner ./cmd/refiner

# 运行阶段
FROM alpine:3.21

WORKDIR /app

# 安装运行时依赖并创建非 root 用户
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S refiner \
    && adduser -S -D -h /app -G refiner refiner

# 从构建阶段复制二进制文件和配置
COPY --from=builder --chown=refiner:refiner /out/refiner /app/refiner
COPY --from=builder --chown=refiner:refiner /src/config /app/config

ENV TZ=Asia/Shanghai
ENV CONFIG_FILE=/app/config/service.docker.yaml

USER refiner

# 暴露端口
EXPOSE 15051 18080 19091

HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=5 \
  CMD wget -q -O /dev/null http://127.0.0.1:18080/api/snapshot || exit 1

# 启动服务
ENTRYPOINT ["/app/refiner"]
