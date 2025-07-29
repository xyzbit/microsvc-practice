.PHONY: help proto build run test clean up down logs install-deps

# 默认目标
help: ## 显示帮助信息
	@echo "可用的命令:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# 安装依赖
install-deps: ## 安装必要的依赖工具
	@echo "检查并安装依赖工具..."
	@if ! command -v protoc >/dev/null 2>&1; then \
		echo "安装 protoc..."; \
		if [[ "$$OSTYPE" == "darwin"* ]]; then \
			brew install protobuf; \
		elif [[ "$$OSTYPE" == "linux-gnu"* ]]; then \
			sudo apt-get update && sudo apt-get install -y protobuf-compiler; \
		else \
			echo "请手动安装 protoc"; \
			exit 1; \
		fi; \
	fi
	@echo "安装 Go proto 插件..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "依赖安装完成!"

# 生成 proto 文件
proto: ## 生成 proto Go 代码
	@echo "生成 proto Go 代码..."
	@cd proto && \
	if [ ! -f "go.mod" ]; then \
		go mod init proto; \
	fi && \
	protoc --go_out=. --go_opt=paths=source_relative \
		   --go-grpc_out=. --go-grpc_opt=paths=source_relative \
		   service.proto
	@echo "proto 代码生成完成!"

# 初始化模块依赖
tidy:
	@echo "初始化 proto 模块依赖..."
	@cd proto && go mod tidy
	@echo "初始化 svcb 模块依赖..."
	@cd svcb && go mod tidy
	@echo "初始化 svca 模块依赖..."
	@cd svca && go mod tidy
	@echo "依赖初始化完成!"

# 构建相关
build: ## 构建 svca 和 svcb 服务
	@echo "构建 svcb 服务..."
	@cd svcb && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../bin/svcb main.go
	@echo "构建 svca 服务..."
	@cd svca && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../bin/svca main.go
	@echo "构建完成!"

# Docker 相关
up: build ## 启动追踪基础设施
	@echo "启动 Docker Compose 服务..."
	@docker-compose up -d
	@echo "等待服务启动..."
	@sleep 10
	@echo "服务状态:"
	@docker-compose ps

up-force: build ## 强制重新构建并启动追踪基础设施(不使用缓存)
	@echo "强制重新构建并启动 Docker Compose 服务..."
	@docker-compose down
	@docker-compose build --no-cache
	@docker-compose up -d --force-recreate
	@echo "等待服务启动..."
	@sleep 10
	@echo "服务状态:"
	@docker-compose ps

rebuild: ## 重新构建Docker镜像(不使用缓存)
	@echo "重新构建 Docker 镜像(不使用缓存)..."
	@docker-compose build --no-cache

down: ## 停止所有服务
	@echo "停止 Docker Compose 服务..."
	@docker-compose down

clean: ## 清理所有服务和数据
	@echo "清理 Docker Compose 服务和数据..."
	@docker-compose down -v
	@docker-compose down --rmi all
	@rm -rf bin/
	@rm -f proto/*.pb.go
	@echo "清理完成!"

logs: ## 查看服务日志
	@docker-compose logs -f

# 运行服务
run-svcb: deps ## 运行 svcb gRPC 服务
	@echo "启动 svcb gRPC 服务..."
	@cd svcb && go run main.go

run-svca: deps ## 运行 svca HTTP 服务
	@echo "启动 svca HTTP 服务..."
	@cd svca && go run main.go

run-both: ## 同时运行两个服务 (后台运行 svcb，前台运行 svca)
	@echo "启动 svcb 服务 (后台)..."
	@cd svcb && go run main.go &
	@echo "等待 svcb 启动..."
	@sleep 3
	@echo "启动 svca 服务..."
	@cd svca && go run main.go

# 测试相关
test-health: ## 测试服务健康检查
	@echo "测试 svca 健康检查..."
	@curl -s http://localhost:8080/health | jq . || echo "svca 服务未启动"

test-get-user: ## 测试获取用户接口
	@echo "测试获取用户接口..."
	@curl -s "http://localhost:8080/users?user_id=123" | jq . || echo "服务未启动"

test-create-user: ## 测试创建用户接口
	@echo "测试创建用户接口..."
	@curl -s -X POST http://localhost:8080/users \
		-H "Content-Type: application/json" \
		-d '{"name":"张三","email":"zhangsan@example.com"}' | jq . || echo "服务未启动"

test: test-health test-get-user test-create-user ## 运行所有测试

# 监控相关
grafana: ## 打开 Grafana 界面
	@echo "打开 Grafana 界面: http://localhost:3000"
	@echo "用户名: admin, 密码: admin"
	@open http://localhost:3000 || xdg-open http://localhost:3000 || echo "请手动打开 http://localhost:3000"

tempo: ## 查看 Tempo 状态
	@echo "Tempo API 状态:"
	@curl -s http://localhost:3200/api/echo | jq . || echo "Tempo 未启动"

# 状态检查
status: ## 检查服务状态
	@echo "=== 服务状态检查 ==="
	@echo "Docker Compose 服务:"
	@docker-compose ps
	@echo "\n端口占用情况:"
	@echo "3000 (Grafana):"
	@nc -z localhost 3000 && echo "✓ 可用" || echo "✗ 不可用"
	@echo "3200 (Tempo):"
	@nc -z localhost 3200 && echo "✓ 可用" || echo "✗ 不可用"
	@echo "4317 (OTLP gRPC):"
	@nc -z localhost 4317 && echo "✓ 可用" || echo "✗ 不可用"
	@echo "8080 (svca HTTP):"
	@nc -z localhost 8080 && echo "✓ 可用" || echo "✗ 不可用"
	@echo "50051 (svcb gRPC):"
	@nc -z localhost 50051 && echo "✓ 可用" || echo "✗ 不可用"

# 生成测试数据
generate-traces: ## 生成测试追踪数据
	@echo "生成测试追踪数据..."
	@for i in {1..10}; do \
		echo "发送请求 $$i..."; \
		curl -s "http://localhost:8080/users?user_id=$$i" > /dev/null; \
		sleep 1; \
	done
	@echo "测试数据生成完成! 请在 Grafana 中查看追踪数据。"

# 完整演示流程
demo: ## 运行完整演示
	@echo "=== svca & svcb 微服务演示 ==="
	@echo "1. 安装依赖工具..."
	@make install-deps
	@echo "\n2. 生成 proto 代码..."
	@make proto
	@echo "\n3. 启动追踪基础设施..."
	@make up
	@echo "\n4. 等待服务就绪..."
	@sleep 15
	@echo "\n5. 测试服务..."
	@make test
	@echo "\n6. 演示完成!"
	@echo "   - Grafana: http://localhost:3000 (admin/admin)"
	@echo "   - svca HTTP: http://localhost:8080"
	@echo "   - svcb gRPC: localhost:50051"
	@echo "\n使用以下命令："
	@echo "   - 启动服务: make run-both"
	@echo "   - 测试服务: make test"
	@echo "   - 停止服务: make down" 