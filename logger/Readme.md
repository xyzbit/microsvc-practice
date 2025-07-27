## 日志收集核心链路

在微服务架构中，日志收集是监控和排障的关键环节。本项目实现了一个完整的日志收集链路：

1. **应用生成结构化日志**：Go应用使用`slog`库生成JSON格式的结构化日志
2. **日志持久化**：日志同时输出到标准输出和本地文件`app_logs/app.log`
3. **日志收集**：Filebeat监控并读取日志文件
4. **日志解析和处理**：Filebeat对JSON日志进行解析和字段处理
5. **日志存储**：解析后的日志发送到VictoriaLogs存储
6. **日志可视化**：通过Grafana连接VictoriaLogs进行查询和可视化

整个链路通过Docker Compose编排，确保各组件协同工作。

## fileBeat 配置文件解析

用如下日志讲解解析流程

```json
{
  "http_method": "GET",
  "http_path": "/hello",
  "level": "INFO",
  "msg": "Successfully processed /hello request.",
  "processing_time_ms": 78000000,
  "request_id": "req-10362a7ab007475e",
  "response_payload": "Hello Gopher!",
  "service_name": "my-go-filebeat-app",
  "service_version": "1.0.0",
  "source": {
    "file": "/Users/litao/code/microsvc-practice/logger/main.go",
    "function": "main.main.func1",
    "line": 70
  },
  "time": "2025-07-26T12:12:47.063693+08:00",
  "trace_id": "trace-125ff68c8aa5fff9"
}
```

### Filebeat配置解析

1. **输入配置**
   ```yaml
   filebeat.inputs:
   - type: filestream
     id: go-logger-app
     paths:
       - /app_logs/app.log
   ```
   - `filestream`：使用文件流方式读取日志
   - `id`：为输入源指定唯一标识
   - `paths`：指定要监控的日志文件路径

2. **处理器配置**
   获取的日志格式为：
   ```json
   {
    "message":
    "{\"http_method\":\"GET\",\"http_path\":\"/hello\",\"level\":\"INFO\",\"msg\":\"Successfully processed /hello request.\",\"processing_time_ms\":78000000,\"request_id\":\"req-10362a7ab007475e\",\"response_payload\":\"Hello Gopher!\",\"service_name\":\"my-go-filebeat-app\",\"service_version\":\"1.0.0\",\"source\":{\"file\":\"/Users/litao/code/microsvc-practice/logger/main.go\",\"function\":\"main.main.func1\",\"line\":70},\"time\":\"2025-07-26T12:12:47.063693+08:00\",\"trace_id\":\"trace-125ff68c8aa5fff9\"}",
    "other": ""
   }

   ```
   通过配置从 `message` 中将 json string，解析到根级别字段。
   ```yaml
   processors:
     - decode_json_fields:
         fields: ["message"]
         target: ""
         overwrite_keys: true
         add_error_key: true
   ```
   - `decode_json_fields`：解析JSON格式的日志
   - `fields: ["message"]`：指定要解析的字段名
   - `target: ""`：将解析结果放到事件根级别，为空解析所有字段
   - `add_error_key: true`：解析失败时添加错误信息
   - `overwrite_keys: true`：如有同名字段，使用解析出的值覆盖

3. **时间戳处理**
   ```yaml
   - timestamp:
       field: time
       layouts:
         - '2006-01-02T15:04:05.999999999Z'
       on_failure:
       - append_to_array:
           field: error.message
           value: "Failed to parse application timestamp."
   ```
   - 使用应用日志中的`time`字段作为事件时间戳，替换 `@timestamp` 放到事件根级别
   - 指定时间格式，确保正确解析
   - 解析失败时添加错误信息而不中断处理

4. **字段清理**
   ```yaml
   - drop_fields:
       fields: [message]
       ignore_missing: true
   ```
   - 移除已解析的原始`message`字段，减少数据冗余
   当前日志如下:
   ```json
   {
     "http_method": "GET",
     "http_path": "/hello",
     "level": "INFO",
     "msg": "Successfully processed /hello request.",
     "processing_time_ms": 78000000,
     "request_id": "req-10362a7ab007475e",
     "response_payload": "Hello Gopher!",
     "service_name": "my-go-filebeat-app",
     "service_version": "1.0.0",
     "source": {
       "file": "/Users/litao/code/microsvc-practice/logger/main.go",
       "function": "main.main.func1",
       "line": 70
     },
     "@timestamp": "2025-07-26T12:12:47.063693+08:00",
     "trace_id": "trace-125ff68c8aa5fff9",
     "other": ""
   }
   ```

5. **输出配置**
   ```yaml
   output.elasticsearch:
     hosts: ["http://localhost:9428/insert/elasticsearch/"]
     parameters:
       _msg_field: "msg"
       _time_field: "@timestamp"
       _stream_fields: "service_name,level,http_method"
     allow_older_versions: true
   ```
  使用Elasticsearch兼容API将日志发送到VictoriaLogs
   - `_msg_field`: 指定用于存储日志消息的字段, 这个字段的内容会被分词存储用于搜索
   - `_time_field`：指定用于存储时间戳的字段
   - `_stream_fields`：指定了用于筛选的标签
   - `allow_older_versions`：兼容较旧版本的ES API

## 其他的同步方式、优劣对比

### 1. 直接写入日志存储

**方式**：应用直接将日志发送到存储系统（如Elasticsearch、VictoriaLogs）

**优点**：
- 减少中间环节，降低延迟
- 简化架构，减少维护组件

**缺点**：
- 增加应用代码复杂度
- 应用与日志系统耦合
- 日志发送失败可能影响应用性能
- 难以批量处理和缓冲

### 2. 日志聚合服务（如Fluentd、Logstash）

**方式**：应用写日志到文件，由Fluentd/Logstash收集并发送到存储系统

**优点**：
- 强大的数据转换和过滤能力
- 支持多种输入和输出插件
- 可进行复杂的数据处理和富化

**缺点**：
- 资源消耗较大
- 配置复杂
- 性能较Filebeat低

### 3. 系统日志（Syslog）

**方式**：使用系统日志协议传输日志

**优点**：
- 标准化协议，广泛支持
- 与系统日志集成
- 配置简单

**缺点**：
- 功能相对有限
- 结构化支持较弱
- 难以进行复杂处理

### 4. 容器日志收集（如Docker日志驱动）

**方式**：利用容器平台的日志机制收集日志

**优点**：
- 与容器平台无缝集成
- 不需要额外配置容器内部
- 统一管理所有容器日志

**缺点**：
- 依赖容器平台
- 自定义处理能力有限
- 可能增加存储压力

### 5. 本项目采用的Filebeat方案

**优点**：
- 轻量级，资源消耗低
- 可靠的传输机制，支持断点续传
- 简单的配置
- 良好的结构化日志支持
- 内置处理器满足基本需求

**缺点**：
- 处理能力不如Logstash强大
- 复杂转换需要额外配置
- 插件生态不如其他成熟
