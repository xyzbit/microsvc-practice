# 📊 Grafana 可观测性三要素关联操作指南

## 🎯 概述

本指南详细介绍如何在 Grafana 中实现 **指标(Metrics)** → **日志(Logs)** → **追踪(Traces)** 的完整关联跳转，体现 Grafana Tempo 的核心设计理念。

## 🔗 关联链路配置

### 1. 数据源关联配置

我们已经配置了以下关联：

#### **Prometheus → Loki**
- 从指标图表点击数据点可查看相关日志
- 基于时间范围和标签进行关联

#### **Loki → Tempo** 
- 从日志中的 TraceID 直接跳转到完整追踪
- 正则表达式匹配：`"trace_id":"([a-f0-9]+)"`

#### **Tempo → Prometheus + Loki**
- 从追踪跳回到相关指标
- 从追踪跳转到相关日志

## 🚀 操作步骤

### 第一步：访问 Grafana
```bash
# 打开浏览器访问
http://localhost:3000
# 账号: admin
# 密码: admin
```

### 第二步：查看可观测性关联仪表盘

1. **进入仪表盘**
   - 点击左侧菜单 "Dashboards" 
   - 选择 "📊 可观测性关联演示"

2. **观察关联效果**
   - 🔗 HTTP Request Rate (点击查看日志)
   - 🔗 HTTP Request Duration (点击查看追踪) 
   - 🔗 Application Logs (点击 TraceID 查看追踪)

### 第三步：从指标跳转到日志

1. **选择指标图表**
   - 在 "HTTP Request Rate" 图表中点击任意数据点
   - 右键选择 "Explore" 或直接点击关联链接

2. **查看相关日志**
   - 系统会自动跳转到 Loki 日志视图
   - 时间范围会自动调整到所选时间点附近
   - 可以看到该时间段的所有请求日志

### 第四步：从日志跳转到追踪

1. **在日志面板中**
   - 找到包含 TraceID 的日志条目
   - 例如：`"trace_id":"07aef32d1bca39cc94a67a19d1db136e"`

2. **点击 TraceID**
   - 在日志详情中会显示 "View Trace" 链接
   - 点击链接自动跳转到 Tempo 追踪视图

3. **查看完整调用链**
   - 可以看到完整的服务调用链路
   - API Gateway → User Service → Order Service → Notification Service

### 第五步：从追踪跳转到指标/日志

1. **在 Tempo 追踪视图中**
   - 选择任意 Span（服务调用）
   - 点击 "Logs for this span" 查看相关日志
   - 点击 "Metrics for this span" 查看相关指标

## 🎪 实战演示

### 1. 生成测试数据
```bash
# 在 apm 目录下执行
make demo

# 或手动发送请求
curl -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"user_id": 1, "product": "测试商品", "amount": 99.99}'
```

### 2. 观察数据流

**指标层面**：
- HTTP 请求率增加
- 延迟分布变化
- 成功率统计

**日志层面**：
- 结构化 JSON 日志
- 包含完整的 TraceID
- 服务链路日志

**追踪层面**：
- 完整的调用链路
- 每个服务的耗时
- 错误和异常信息

## 🔍 深度分析场景

### 场景1：性能问题分析

1. **发现问题** - 在指标图表中看到延迟飙升
2. **定位时间** - 点击异常时间点查看日志
3. **找到TraceID** - 从日志中获取相关的 TraceID
4. **深度分析** - 用 TraceID 在 Tempo 中查看完整调用链
5. **确定根因** - 分析哪个服务或操作导致延迟

### 场景2：错误追踪

1. **错误告警** - 指标显示错误率增加
2. **查看错误日志** - 从指标跳转到日志查看错误详情
3. **获取 TraceID** - 从错误日志中提取 TraceID
4. **完整追踪** - 查看导致错误的完整调用链路
5. **修复问题** - 定位具体的错误原因

### 场景3：业务流程分析

1. **业务指标** - 观察订单创建、用户操作等业务指标
2. **流程日志** - 查看业务流程的详细日志
3. **端到端追踪** - 通过 TraceID 查看完整的业务流程追踪
4. **优化建议** - 基于追踪数据优化业务流程

## 🛠 高级配置

### 1. 自定义关联查询

在 `datasources.yaml` 中可以自定义关联查询：

```yaml
tracesToMetrics:
  datasourceUid: prometheus
  queries:
    - name: 'Request Rate'
      query: 'rate(http_requests_total{job="$__tags.service"}[$__rate_interval])'
    - name: 'Error Rate' 
      query: 'rate(http_requests_total{job="$__tags.service",status_code=~"5.."}[$__rate_interval])'
```

### 2. 日志格式优化

在日志面板中使用 LogQL 美化日志显示：

```logql
{job="application-logs"} 
| json 
| line_format "{{.time}} [{{.service}}] {{.method}} {{.uri}} ({{.status}}) - {{.latency}} - TraceID: {{.trace_id}}"
```

### 3. 标签映射

配置标签映射以改善关联效果：

```yaml
mappedTags: 
  - { key: 'service.name', value: 'service' }
  - { key: 'job', value: 'job' }
  - { key: 'instance', value: 'instance' }
```

## 🎯 最佳实践

### 1. TraceID 传播
- 确保所有服务都正确传播 TraceID
- 在日志中包含 TraceID
- 使用结构化日志格式

### 2. 标签一致性
- 在指标、日志、追踪中使用一致的标签
- 特别是 `service.name`、`job` 等关键标签

### 3. 时间同步
- 确保所有服务的时间同步
- 配置合适的时间窗口

### 4. 采样策略
- 在生产环境中使用合适的采样率
- 确保重要的追踪不被丢弃

## 🔧 故障排除

### 问题1：无法跳转到追踪
**解决方案**：
- 检查 TraceID 格式是否正确
- 确认 Tempo 数据源配置
- 验证正则表达式匹配

### 问题2：关联查询为空
**解决方案**：
- 检查标签映射配置
- 确认时间范围设置
- 验证数据源连接

### 问题3：TraceID 显示为全零
**解决方案**：
- 检查中间件顺序
- 确认 OTLP 端点配置
- 验证追踪初始化

## 📈 效果展示

成功配置后，您将看到：

1. **无缝跳转**：从任何数据点都能跳转到相关视图
2. **上下文保持**：跳转时保持时间范围和相关标签
3. **完整链路**：能够追踪完整的请求链路
4. **快速定位**：从问题发现到根因定位的完整流程

这就是 Grafana Tempo 的核心价值：**通过 TraceID 连接所有可观测性数据，实现从问题发现到根因分析的完整工作流**！ 