# Grafana Tempo APM 监控系统示例

这是一个完整的 APM（Application Performance Monitoring）监控系统示例，基于 Grafana Tempo 分布式追踪系统构建，展示了现代微服务架构中的可观测性最佳实践。

## 系统架构

### 核心组件

- **Grafana Tempo**: 分布式追踪后端，专注于通过 TraceID 进行高效查找
- **Grafana**: 统一可视化界面，支持指标、日志、追踪的关联分析
- **Prometheus**: 指标收集和存储
- **Loki**: 日志聚合和查询
- **Promtail**: 日志收集器

### 微服务

- **API Gateway**: 统一入口，负责请求路由和服务编排
- **User Service**: 用户管理服务
- **Order Service**: 订单处理服务
- **Notification Service**: 通知发送服务

## 主要特性

### 📊 指标监控 (Metrics)
- HTTP 请求速率和延迟
- 业务指标（用户操作、订单创建、通知发送等）
- 系统资源监控

### 📋 日志聚合 (Logs)
- 结构化 JSON 日志
- 包含 TraceID 的日志关联
- 多服务日志统一收集

### 🔍 分布式追踪 (Traces)
- 完整的请求链路追踪
- 跨服务调用追踪
- 性能瓶颈识别

### 🔗 三种数据类型的关联
- 从指标图表跳转到相关日志
- 从日志跳转到完整的追踪链路
- 从追踪跳转到相关指标

## 快速开始

### 前置要求

- Docker
- Docker Compose

### 启动系统

1. 克隆项目并进入目录：
```bash
cd apm
```

2. 创建日志目录：
```bash
mkdir -p logs
```

3. 启动所有服务：
```bash
docker-compose up -d
```

4. 等待所有服务启动完成（约 1-2 分钟）。

### 访问界面

- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090
- **Tempo**: http://localhost:3200
- **Loki**: http://localhost:3100
- **API Gateway**: http://localhost:8080

### 服务端点

- **用户服务**: http://localhost:8081
- **订单服务**: http://localhost:8082
- **通知服务**: http://localhost:8083

## 示例操作

### 1. 生成测试数据

```bash
# 获取用户信息
curl http://localhost:8080/api/v1/users/1

# 创建订单（会调用用户服务验证，然后发送通知）
curl -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "product": "测试商品",
    "amount": 99.99
  }'

# 获取订单列表
curl http://localhost:8080/api/v1/orders
```

### 2. 查看监控数据

1. 打开 Grafana: http://localhost:3000
2. 进入 "APM Overview" 仪表盘
3. 观察请求速率、延迟分布
4. 点击追踪面板查看完整的调用链路

### 3. 日志分析

1. 在 Grafana 中打开 "Explore"
2. 选择 "Loki" 数据源
3. 使用查询：`{job="application-logs"}`
4. 在日志中找到 TraceID，点击跳转到追踪详情

### 4. 追踪分析

1. 在 Grafana 中选择 "Tempo" 数据源
2. 输入 TraceID 进行搜索
3. 查看完整的服务调用链路
4. 分析各个 Span 的耗时和错误信息

## 系统设计亮点

### Tempo 核心优势

1. **成本优化**: 专注于 TraceID 索引，避免复杂的二级索引，大幅降低存储成本
2. **简化运维**: 单组件部署，与对象存储深度集成
3. **生态集成**: 与 Grafana、Prometheus、Loki 无缝集成

### 工作流程

1. **发现问题**: 通过 Prometheus 指标发现服务异常（如延迟增加）
2. **定位原因**: 通过 Loki 日志查看具体错误信息和 TraceID
3. **深度分析**: 使用 TraceID 在 Tempo 中查看完整调用链路
4. **根因分析**: 分析具体哪个服务或操作导致了问题

### 追踪特性

- **自动注入**: 自动注入和提取追踪头信息
- **上下文传播**: 跨服务的追踪上下文传播
- **性能指标**: 每个 Span 包含详细的性能指标
- **错误追踪**: 自动记录错误和异常信息

## 配置说明

### Tempo 配置

- 接收器：支持 OTLP、Jaeger、Zipkin 协议
- 指标生成：自动生成服务图和 Span 指标
- 数据保留：1小时（演示用，生产环境建议更长）

### Prometheus 配置

- 抓取间隔：5-15秒
- 监控目标：所有微服务和基础设施组件
- 远程写入：接收 Tempo 生成的指标

### Grafana 配置

- 数据源：自动配置 Prometheus、Loki、Tempo
- 仪表盘：预配置 APM 概览仪表盘
- 关联：配置各数据源之间的跳转链接

## 生产部署建议

### 扩展性

1. **Tempo**: 使用对象存储（S3、GCS等）和分布式部署
2. **Prometheus**: 使用联邦或 Thanos 进行长期存储
3. **Loki**: 使用对象存储和多租户配置

### 安全性

1. 启用认证和授权
2. 使用 TLS 加密通信
3. 配置网络隔离

### 性能优化

1. 调整采样率以控制数据量
2. 配置合适的保留期
3. 优化查询性能

## 故障排除

### 常见问题

1. **服务无法启动**: 检查端口冲突，确保 Docker 有足够资源
2. **追踪数据缺失**: 检查服务间网络连通性和 Tempo 配置
3. **日志丢失**: 检查日志文件权限和 Promtail 配置
4. **指标缺失**: 检查 Prometheus 抓取配置和服务 `/metrics` 端点

### 日志查看

```bash
# 查看所有服务状态
docker-compose ps

# 查看特定服务日志
docker-compose logs -f tempo
docker-compose logs -f grafana
docker-compose logs -f api-gateway
```

### 重启服务

```bash
# 重启特定服务
docker-compose restart tempo

# 重建并重启
docker-compose up -d --build
```

## 学习资源

- [Grafana Tempo 官方文档](https://grafana.com/docs/tempo/)
- [OpenTelemetry 规范](https://opentelemetry.io/)
- [分布式追踪最佳实践](https://grafana.com/blog/2021/02/09/grafana-tempo-distributed-tracing-best-practices/)

## 许可证

本项目仅用于学习和演示目的。 