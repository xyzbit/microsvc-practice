package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	// Prometheus 指标
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	serviceCalls = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "service_calls_total",
			Help: "Total number of service calls",
		},
		[]string{"service", "status"},
	)
)

func init() {
	// 注册 Prometheus 指标
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(serviceCalls)
}

func initTracer() (*sdktrace.TracerProvider, error) {
	endpoint := os.Getenv("JAEGER_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://tempo:4318"
	}

	// 创建 OTLP HTTP exporter
	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// 创建 resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName("api-gateway"),
			semconv.ServiceVersion("1.0.0"),
			attribute.String("environment", "development"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 创建 TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp, nil
}

func callService(ctx context.Context, serviceName, url string) (map[string]interface{}, error) {
	// 创建新的 span
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(ctx, fmt.Sprintf("call-%s", serviceName))
	defer span.End()

	// 添加 span 属性
	span.SetAttributes(
		attribute.String("service.name", serviceName),
		attribute.String("http.url", url),
		attribute.String("http.method", "GET"),
	)

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		span.RecordError(err)
		serviceCalls.WithLabelValues(serviceName, "error").Inc()
		return nil, err
	}

	// 注入追踪头
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		serviceCalls.WithLabelValues(serviceName, "error").Inc()
		return nil, err
	}
	defer resp.Body.Close()

	// 更新指标
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		serviceCalls.WithLabelValues(serviceName, "success").Inc()
	} else {
		serviceCalls.WithLabelValues(serviceName, "error").Inc()
	}

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		span.RecordError(err)
		return nil, err
	}

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
	return result, nil
}

func prometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 处理请求
		c.Next()

		// 记录指标
		duration := time.Since(start).Seconds()
		statusCode := fmt.Sprintf("%d", c.Writer.Status())

		httpRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), statusCode).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(duration)
	}
}

func loggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			// 获取 trace ID
			spanCtx := trace.SpanContextFromContext(param.Request.Context())
			traceID := spanCtx.TraceID().String()

			return fmt.Sprintf(`{"time":"%s","method":"%s","uri":"%s","status":%d,"latency":"%s","user_agent":"%s","trace_id":"%s"}`+"\n",
				param.TimeStamp.Format(time.RFC3339),
				param.Method,
				param.Path,
				param.StatusCode,
				param.Latency,
				param.Request.UserAgent(),
				traceID,
			)
		},
		Output: getLogWriter(),
	})
}

func getLogWriter() io.Writer {
	// 创建日志文件
	logFile, err := os.OpenFile("/app/logs/api-gateway.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("Failed to open log file: %v", err)
		return os.Stdout
	}

	// 同时输出到文件和控制台
	return io.MultiWriter(os.Stdout, logFile)
}

func main() {
	// 初始化追踪
	tp, err := initTracer()
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	// 创建 Gin 路由
	r := gin.New()

	// 添加中间件 - 顺序很重要！
	r.Use(gin.Recovery())
	r.Use(otelgin.Middleware("api-gateway")) // 必须在 loggingMiddleware 之前
	r.Use(prometheusMiddleware())
	r.Use(loggingMiddleware())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy", "service": "api-gateway"})
	})

	// Prometheus 指标端点
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API 路由
	api := r.Group("/api/v1")
	{
		// 用户相关接口
		api.GET("/users/:id", func(c *gin.Context) {
			userID := c.Param("id")
			userServiceURL := os.Getenv("USER_SERVICE_URL")
			if userServiceURL == "" {
				userServiceURL = "http://localhost:8081"
			}

			// 调用用户服务
			user, err := callService(c.Request.Context(), "user-service", fmt.Sprintf("%s/users/%s", userServiceURL, userID))
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}

			c.JSON(200, user)
		})

		// 订单相关接口
		api.POST("/orders", func(c *gin.Context) {
			var orderReq map[string]interface{}
			if err := c.ShouldBindJSON(&orderReq); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}

			orderServiceURL := os.Getenv("ORDER_SERVICE_URL")
			if orderServiceURL == "" {
				orderServiceURL = "http://localhost:8082"
			}

			// 创建订单
			order, err := callService(c.Request.Context(), "order-service", fmt.Sprintf("%s/orders", orderServiceURL))
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}

			// 发送通知
			notificationServiceURL := os.Getenv("NOTIFICATION_SERVICE_URL")
			if notificationServiceURL == "" {
				notificationServiceURL = "http://localhost:8083"
			}

			notification, err := callService(c.Request.Context(), "notification-service", fmt.Sprintf("%s/notify", notificationServiceURL))
			if err != nil {
				log.Printf("Failed to send notification: %v", err)
				// 不影响订单创建的结果
			}

			response := map[string]interface{}{
				"order":        order,
				"notification": notification,
			}

			c.JSON(200, response)
		})

		// 获取订单列表
		api.GET("/orders", func(c *gin.Context) {
			orderServiceURL := os.Getenv("ORDER_SERVICE_URL")
			if orderServiceURL == "" {
				orderServiceURL = "http://localhost:8082"
			}

			orders, err := callService(c.Request.Context(), "order-service", fmt.Sprintf("%s/orders", orderServiceURL))
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}

			c.JSON(200, orders)
		})
	}

	log.Println("API Gateway starting on port 8080...")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
