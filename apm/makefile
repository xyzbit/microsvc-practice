# Grafana Tempo APM 系统管理

.PHONY: help up down logs build clean test demo

# 默认目标
help:
	@echo "Grafana Tempo APM 监控系统"
	@echo ""
	@echo "可用命令:"
	@echo "  up      - 启动所有服务"
	@echo "  down    - 停止所有服务"
	@echo "  build   - 重新构建所有镜像"
	@echo "  logs    - 查看所有服务日志"
	@echo "  clean   - 清理数据和镜像"
	@echo "  test    - 运行测试请求"
	@echo "  demo    - 生成演示数据"
	@echo "  status  - 查看服务状态"

# 启动服务
up:
	@echo "启动 APM 监控系统..."
	@mkdir -p logs
	@docker-compose up -d
	@echo "系统启动中，请等待 1-2 分钟..."
	@echo "Grafana: http://localhost:3000 (admin/admin)"
	@echo "API Gateway: http://localhost:8080"

restart-svcs:
	@echo "启动服务..."
	@docker-compose down user-service order-service notification-service api-gateway
	@docker-compose up -d user-service order-service notification-service api-gateway
	@echo "服务启动完成，请等待 1-2 分钟..."

# 停止服务
down:
	@echo "停止所有服务..."
	@docker-compose down

# 重新构建
build:
	@echo "重新构建所有镜像..."
	@docker-compose build --no-cache

# 查看日志
logs:
	@docker-compose logs -f

# 查看特定服务日志
logs-tempo:
	@docker-compose logs -f tempo

logs-grafana:
	@docker-compose logs -f grafana

logs-api:
	@docker-compose logs -f api-gateway

# 清理
clean:
	@echo "清理数据和镜像..."
	@docker-compose down -v
	@docker system prune -f
	@rm -rf logs/*

# 查看状态
status:
	@docker-compose ps

# 重启服务
restart:
	@docker-compose restart

# 测试请求
test:
	@echo "发送测试请求..."
	@echo "1. 获取用户信息..."
	@curl -s http://localhost:8080/api/v1/users/1 | jq '.' || echo "请求失败"
	@echo "\n2. 创建订单..."
	@curl -s -X POST http://localhost:8080/api/v1/orders \
		-H "Content-Type: application/json" \
		-d '{"user_id": 1, "product": "测试商品", "amount": 99.99}' | jq '.' || echo "请求失败"
	@echo "\n3. 获取订单列表..."
	@curl -s http://localhost:8080/api/v1/orders | jq '.' || echo "请求失败"

# 生成演示数据
demo:
	@echo "生成演示数据..."
	@for i in $$(seq 1 10); do \
		echo "发送请求 $$i..."; \
		curl -s http://localhost:8080/api/v1/users/$$(($$i % 3 + 1)) > /dev/null; \
		curl -s -X POST http://localhost:8080/api/v1/orders \
			-H "Content-Type: application/json" \
			-d "{\"user_id\": $$(($$i % 3 + 1)), \"product\": \"商品$$i\", \"amount\": $$(($$i * 10 + 50))}" > /dev/null; \
		sleep 1; \
	done
	@echo "演示数据生成完成！"

# 健康检查
health:
	@echo "检查服务健康状态..."
	@echo "API Gateway:" && curl -s http://localhost:8080/health || echo "FAIL"
	@echo "User Service:" && curl -s http://localhost:8081/health || echo "FAIL"
	@echo "Order Service:" && curl -s http://localhost:8082/health || echo "FAIL"
	@echo "Notification Service:" && curl -s http://localhost:8083/health || echo "FAIL"
	@echo "Prometheus:" && curl -s http://localhost:9090/-/healthy || echo "FAIL"
	@echo "Grafana:" && curl -s http://localhost:3000/api/health || echo "FAIL"

# 快速部署
deploy: up
	@echo "等待服务启动..."
	@sleep 30
	@make health
	@make demo
	@echo "部署完成！访问 http://localhost:3000 查看监控界面" 