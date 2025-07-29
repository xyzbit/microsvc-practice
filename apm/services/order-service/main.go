package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
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

// Order 订单结构体
type Order struct {
	ID       int     `json:"id"`
	UserID   int     `json:"user_id"`
	Product  string  `json:"product"`
	Amount   float64 `json:"amount"`
	Status   string  `json:"status"`
	CreateAt string  `json:"create_at"`
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

	orderOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "order_operations_total",
			Help: "Total number of order operations",
		},
		[]string{"operation", "status"},
	)

	// 模拟订单数据
	orders = []Order{
		{
			ID:       1,
			UserID:   1,
			Product:  "笔记本电脑",
			Amount:   5999.99,
			Status:   "completed",
			CreateAt: "2023-01-01T10:00:00Z",
		},
		{
			ID:       2,
			UserID:   2,
			Product:  "智能手机",
			Amount:   2999.99,
			Status:   "processing",
			CreateAt: "2023-01-02T11:00:00Z",
		},
	}
	nextOrderID = 3
)

func init() {
	// 注册 Prometheus 指标
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(orderOperations)
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
			semconv.ServiceName("order-service"),
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

func callUserService(ctx context.Context, userID int) (map[string]interface{}, error) {
	tracer := otel.Tracer("order-service")
	ctx, span := tracer.Start(ctx, "call-user-service")
	defer span.End()

	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "http://localhost:8081"
	}

	url := fmt.Sprintf("%s/users/%d", userServiceURL, userID)
	span.SetAttributes(
		attribute.String("http.url", url),
		attribute.Int("user.id", userID),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	// 注入追踪头
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	var user map[string]interface{}
	if err := json.Unmarshal(body, &user); err != nil {
		span.RecordError(err)
		return nil, err
	}

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
	return user, nil
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
			return fmt.Sprintf(`{"time":"%s","service":"order-service","method":"%s","uri":"%s","status":%d,"latency":"%s","user_agent":"%s","trace_id":"%s"}`+"\n",
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
	logFile, err := os.OpenFile("/app/logs/order-service.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("Failed to open log file: %v", err)
		return os.Stdout
	}
	return io.MultiWriter(os.Stdout, logFile)
}

func simulateWork(ctx context.Context, operation string) {
	tracer := otel.Tracer("order-service")
	_, span := tracer.Start(ctx, fmt.Sprintf("simulate-%s", operation))
	defer span.End()

	workTime := time.Duration(rand.Intn(300)+100) * time.Millisecond
	time.Sleep(workTime)

	span.SetAttributes(
		attribute.String("operation", operation),
		attribute.Int64("work_duration_ms", workTime.Milliseconds()),
	)
}

func getOrders(c *gin.Context) {
	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(attribute.String("operation", "get_orders"))

	simulateWork(c.Request.Context(), "database_query")

	orderOperations.WithLabelValues("get_orders", "success").Inc()
	span.SetAttributes(
		attribute.String("result", "success"),
		attribute.Int("orders.count", len(orders)),
	)

	c.JSON(200, gin.H{
		"orders": orders,
		"total":  len(orders),
	})
}

func createOrder(c *gin.Context) {
	var orderReq struct {
		UserID  int     `json:"user_id" binding:"required"`
		Product string  `json:"product" binding:"required"`
		Amount  float64 `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&orderReq); err != nil {
		orderOperations.WithLabelValues("create_order", "error").Inc()
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(
		attribute.String("operation", "create_order"),
		attribute.Int("user.id", orderReq.UserID),
		attribute.String("product", orderReq.Product),
		attribute.Float64("amount", orderReq.Amount),
	)

	// 验证用户是否存在
	user, err := callUserService(c.Request.Context(), orderReq.UserID)
	if err != nil {
		orderOperations.WithLabelValues("create_order", "error").Inc()
		span.RecordError(err)
		c.JSON(400, gin.H{"error": "用户验证失败"})
		return
	}

	// 模拟订单处理
	simulateWork(c.Request.Context(), "order_processing")

	newOrder := Order{
		ID:       nextOrderID,
		UserID:   orderReq.UserID,
		Product:  orderReq.Product,
		Amount:   orderReq.Amount,
		Status:   "processing",
		CreateAt: time.Now().Format(time.RFC3339),
	}

	orders = append(orders, newOrder)
	nextOrderID++

	orderOperations.WithLabelValues("create_order", "success").Inc()
	span.SetAttributes(
		attribute.String("result", "success"),
		attribute.Int("order.id", newOrder.ID),
	)

	c.JSON(201, gin.H{
		"order": newOrder,
		"user":  user,
	})
}

func getOrderByID(c *gin.Context) {
	orderIDStr := c.Param("id")
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		orderOperations.WithLabelValues("get_order", "error").Inc()
		c.JSON(400, gin.H{"error": "Invalid order ID"})
		return
	}

	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(
		attribute.Int("order.id", orderID),
		attribute.String("operation", "get_order"),
	)

	simulateWork(c.Request.Context(), "database_query")

	for _, order := range orders {
		if order.ID == orderID {
			orderOperations.WithLabelValues("get_order", "success").Inc()
			span.SetAttributes(attribute.String("result", "success"))
			c.JSON(200, order)
			return
		}
	}

	orderOperations.WithLabelValues("get_order", "not_found").Inc()
	span.SetAttributes(attribute.String("result", "not_found"))
	c.JSON(404, gin.H{"error": "Order not found"})
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
	r.Use(otelgin.Middleware("order-service")) // 必须在 loggingMiddleware 之前
	r.Use(prometheusMiddleware())
	r.Use(loggingMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy", "service": "order-service"})
	})

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/orders", getOrders)
	r.POST("/orders", createOrder)
	r.GET("/orders/:id", getOrderByID)

	log.Println("Order Service starting on port 8080...")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
