# 构建阶段
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 安装 build dependencies
RUN apk add --no-cache git make

# 复制源码
COPY . .

# 构建二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o agent-kubectl-gateway ./cmd/agent-kubectl-gateway

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates kubectl

WORKDIR /app

# 复制构建产物
COPY --from=builder /app/agent-kubectl-gateway .
COPY --from=builder /app/configs ./configs

# 创建非 root 用户
RUN adduser -D -u 1000 appuser
USER appuser

# 暴露端口
EXPOSE 8080

# 启动命令
CMD ["./agent-kubectl-gateway"]
