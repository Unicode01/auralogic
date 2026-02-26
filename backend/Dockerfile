# 构建阶段
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 安装依赖（SQLite 驱动依赖 CGO）
RUN apk add --no-cache gcc musl-dev sqlite-dev

# 安装依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 编译
ENV CGO_ENABLED=1
ARG BUILD_VERSION=dev
RUN GOOS=linux go build -ldflags "-X main.GitCommit=$BUILD_VERSION" -o /app/bin/api cmd/api/main.go

# 运行阶段
FROM alpine:latest

WORKDIR /app

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates tzdata sqlite-libs

# 从构建阶段复制编译好的二进制文件
COPY --from=builder /app/bin/api /app/api

# 复制邮件模板
COPY --from=builder /app/templates /app/templates

# 复制配置文件目录（需要挂载实际配置）
RUN mkdir -p /app/config /app/data /app/logs /app/uploads

# 暴露端口
EXPOSE 8080

# 运行
CMD ["/app/api"]

