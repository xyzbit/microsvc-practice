package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
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

// Notification 通知结构体
type Notification struct {
	ID      int    `json:"id"`
	Type    string `json:"type"`
	Message string `json:"message"`
	Status  string `json:"status"`
	SentAt  string `json:"sent_at"`
}

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

	notificationsSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notifications_sent_total",
			Help: "Total number of notifications sent",
		},
		[]string{"type", "status"},
	)

	nextNotificationID = 1
)

func init() {
	// 注册 Prometheus 指标
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(notificationsSent)
}

func initTracer() (*sdktrace.TracerProvider, error) {
	endpoint := os.Getenv("JAEGER_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://tempo:4318"
	}

	// 创建 OTLP HTTP exporter
	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithEndpoint(endpoint),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// 创建 resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName("notification-service"),
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

func prometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()
		statusCode := fmt.Sprintf("%d", c.Writer.Status())
		httpRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), statusCode).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(duration)
	}
}

func loggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			spanCtx := trace.SpanContextFromContext(param.Request.Context())
			traceID := spanCtx.TraceID().String()
			return fmt.Sprintf(`{"time":"%s","service":"notification-service","method":"%s","uri":"%s","status":%d,"latency":"%s","user_agent":"%s","trace_id":"%s"}`+"\n",
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
	logFile, err := os.OpenFile("/app/logs/notification-service.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("Failed to open log file: %v", err)
		return os.Stdout
	}
	return io.MultiWriter(os.Stdout, logFile)
}

func simulateNotificationSending(ctx context.Context, notificationType string) {
	tracer := otel.Tracer("notification-service")
	_, span := tracer.Start(ctx, fmt.Sprintf("send-%s-notification", notificationType))
	defer span.End()

	// 模拟发送时间（邮件比短信慢）
	var sendTime time.Duration
	switch notificationType {
	case "email":
		sendTime = time.Duration(rand.Intn(1000)+500) * time.Millisecond
	case "sms":
		sendTime = time.Duration(rand.Intn(300)+100) * time.Millisecond
	case "push":
		sendTime = time.Duration(rand.Intn(200)+50) * time.Millisecond
	default:
		sendTime = time.Duration(rand.Intn(500)+200) * time.Millisecond
	}

	time.Sleep(sendTime)

	span.SetAttributes(
		attribute.String("notification.type", notificationType),
		attribute.Int64("send_duration_ms", sendTime.Milliseconds()),
	)

	// 模拟偶发的发送失败
	if rand.Float32() < 0.05 { // 5% 失败率
		span.SetAttributes(attribute.String("result", "failed"))
		notificationsSent.WithLabelValues(notificationType, "failed").Inc()
		span.RecordError(fmt.Errorf("notification sending failed"))
	} else {
		span.SetAttributes(attribute.String("result", "success"))
		notificationsSent.WithLabelValues(notificationType, "success").Inc()
	}
}

func sendNotification(c *gin.Context) {
	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(attribute.String("operation", "send_notification"))

	// 随机选择通知类型
	notificationTypes := []string{"email", "sms", "push"}
	notificationType := notificationTypes[rand.Intn(len(notificationTypes))]

	// 模拟发送不同类型的通知
	simulateNotificationSending(c.Request.Context(), notificationType)

	notification := Notification{
		ID:      nextNotificationID,
		Type:    notificationType,
		Message: fmt.Sprintf("您的订单已创建成功，订单号：%d", rand.Intn(10000)+1000),
		Status:  "sent",
		SentAt:  time.Now().Format(time.RFC3339),
	}

	nextNotificationID++

	span.SetAttributes(
		attribute.String("notification.type", notificationType),
		attribute.Int("notification.id", notification.ID),
		attribute.String("result", "success"),
	)

	c.JSON(200, notification)
}

func sendEmail(c *gin.Context) {
	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(attribute.String("operation", "send_email"))

	simulateNotificationSending(c.Request.Context(), "email")

	notification := Notification{
		ID:      nextNotificationID,
		Type:    "email",
		Message: "邮件通知已发送",
		Status:  "sent",
		SentAt:  time.Now().Format(time.RFC3339),
	}

	nextNotificationID++

	span.SetAttributes(
		attribute.Int("notification.id", notification.ID),
		attribute.String("result", "success"),
	)

	c.JSON(200, notification)
}

func sendSMS(c *gin.Context) {
	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(attribute.String("operation", "send_sms"))

	simulateNotificationSending(c.Request.Context(), "sms")

	notification := Notification{
		ID:      nextNotificationID,
		Type:    "sms",
		Message: "短信通知已发送",
		Status:  "sent",
		SentAt:  time.Now().Format(time.RFC3339),
	}

	nextNotificationID++

	span.SetAttributes(
		attribute.Int("notification.id", notification.ID),
		attribute.String("result", "success"),
	)

	c.JSON(200, notification)
}

func main() {
	tp, err := initTracer()
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	r := gin.New()
	// 添加中间件 - 顺序很重要！
	r.Use(gin.Recovery())
	r.Use(otelgin.Middleware("notification-service")) // 必须在 loggingMiddleware 之前
	r.Use(prometheusMiddleware())
	r.Use(loggingMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy", "service": "notification-service"})
	})

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/notify", sendNotification)
	r.POST("/email", sendEmail)
	r.POST("/sms", sendSMS)

	log.Println("Notification Service starting on port 8080...")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
