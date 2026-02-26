.PHONY: help build run test clean init-admin migrate

help: ## 显示帮助信息
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## 编译项目
	@echo "Building..."
	@go build -ldflags "-X main.GitCommit=$$(git rev-parse --short HEAD 2>/dev/null || echo dev)" -o bin/api cmd/api/main.go
	@echo "Build complete: bin/api"

run: ## 运行开发服务器
	@echo "Starting development server..."
	@go run cmd/api/main.go

test: ## 运行测试
	@echo "Running tests..."
	@go test -v ./...

test-cover: ## 运行测试并显示覆盖率
	@echo "Running tests with coverage..."
	@go test -cover -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

clean: ## 清理编译产物
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

init-admin: ## 初始化超级管理员
	@echo "Initializing super admin..."
	@go run scripts/init_admin.go

migrate: ## 执行数据库迁移（通过启动API服务自动执行）
	@echo "Database migration is automatic on server start"
	@go run cmd/api/main.go

deps: ## 安装依赖
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

fmt: ## 格式化代码
	@echo "Formatting code..."
	@go fmt ./...

lint: ## 代码检查
	@echo "Linting code..."
	@golangci-lint run

docker-build: ## 构建Docker镜像
	@echo "Building Docker image..."
	@docker build -t auralogic-backend .

docker-run: ## 运行Docker容器
	@echo "Running Docker container..."
	@docker run -d -p 8080:8080 --name auralogic-api \
		-e CONFIG_PATH=/app/config/config.json \
		-v "$(PWD)/config/docker-compose.json:/app/config/config.json:ro" \
		-v "$(PWD)/data:/app/data" \
		-v "$(PWD)/logs:/app/logs" \
		-v "$(PWD)/uploads:/app/uploads" \
		auralogic-backend

install-tools: ## 安装开发工具
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.DEFAULT_GOAL := help

