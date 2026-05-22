# 构建阶段
FROM golang:1.25-alpine AS builder

# 可通过 --build-arg 覆盖：docker build --build-arg TARGETARCH=arm64
ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /app

# 安装 build dependencies
RUN apk add --no-cache git make

# 复制源码
COPY . .

# 构建二进制文件
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
	-ldflags="-s -w" -trimpath -a -installsuffix cgo \
	-o agent-kubectl-gateway-${TARGETOS}-${TARGETARCH} ./cmd/agent-kubectl-gateway

# 运行阶段
FROM alpine:latest

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN apk --no-cache add ca-certificates kubectl

WORKDIR /app

# 复制构建产物
COPY --from=builder /app/agent-kubectl-gateway-${TARGETOS}-${TARGETARCH} ./agent-kubectl-gateway
COPY --from=builder /app/configs ./configs

# 创建非 root 用户
RUN adduser -D -u 1000 appuser
USER appuser

# 暴露端口
EXPOSE 8080

# 启动命令
CMD ["./agent-kubectl-gateway"]
