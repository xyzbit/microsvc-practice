package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
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

// User 用户结构体
type User struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Age      int    `json:"age"`
	Status   string `json:"status"`
	CreateAt string `json:"create_at"`
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

	userOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "user_operations_total",
			Help: "Total number of user operations",
		},
		[]string{"operation", "status"},
	)

	// 模拟用户数据
	users = map[int]User{
		1: {
			ID:       1,
			Name:     "张三",
			Email:    "zhangsan@example.com",
			Age:      25,
			Status:   "active",
			CreateAt: "2023-01-01T00:00:00Z",
		},
		2: {
			ID:       2,
			Name:     "李四",
			Email:    "lisi@example.com",
			Age:      30,
			Status:   "active",
			CreateAt: "2023-01-02T00:00:00Z",
		},
		3: {
			ID:       3,
			Name:     "王五",
			Email:    "wangwu@example.com",
			Age:      28,
			Status:   "inactive",
			CreateAt: "2023-01-03T00:00:00Z",
		},
	}
)

func init() {
	// 注册 Prometheus 指标
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(userOperations)
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
			semconv.ServiceName("user-service"),
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

			return fmt.Sprintf(`{"time":"%s","service":"user-service","method":"%s","uri":"%s","status":%d,"latency":"%s","user_agent":"%s","trace_id":"%s"}`+"\n",
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
	logFile, err := os.OpenFile("/app/logs/user-service.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("Failed to open log file: %v", err)
		return os.Stdout
	}

	// 同时输出到文件和控制台
	return io.MultiWriter(os.Stdout, logFile)
}

func simulateWork(ctx context.Context, operation string) {
	// 创建子 span
	tracer := otel.Tracer("user-service")
	_, span := tracer.Start(ctx, fmt.Sprintf("simulate-%s", operation))
	defer span.End()

	// 模拟一些工作时间
	workTime := time.Duration(rand.Intn(500)+50) * time.Millisecond
	time.Sleep(workTime)

	span.SetAttributes(
		attribute.String("operation", operation),
		attribute.Int64("work_duration_ms", workTime.Milliseconds()),
	)
}

func getUserByID(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		userOperations.WithLabelValues("get_user", "error").Inc()
		c.JSON(400, gin.H{"error": "Invalid user ID"})
		return
	}

	// 添加追踪信息
	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(
		attribute.Int("user.id", userID),
		attribute.String("operation", "get_user"),
	)

	// 模拟数据库查询
	simulateWork(c.Request.Context(), "database_query")

	user, exists := users[userID]
	if !exists {
		userOperations.WithLabelValues("get_user", "not_found").Inc()
		span.SetAttributes(attribute.String("result", "not_found"))
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}

	// 模拟一些额外的处理
	simulateWork(c.Request.Context(), "data_processing")

	userOperations.WithLabelValues("get_user", "success").Inc()
	span.SetAttributes(
		attribute.String("result", "success"),
		attribute.String("user.name", user.Name),
		attribute.String("user.email", user.Email),
	)

	c.JSON(200, user)
}

func getAllUsers(c *gin.Context) {
	// 添加追踪信息
	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(attribute.String("operation", "get_all_users"))

	// 模拟数据库查询
	simulateWork(c.Request.Context(), "database_query_all")

	var userList []User
	for _, user := range users {
		userList = append(userList, user)
	}

	// 模拟数据处理
	simulateWork(c.Request.Context(), "data_processing")

	userOperations.WithLabelValues("get_all_users", "success").Inc()
	span.SetAttributes(
		attribute.String("result", "success"),
		attribute.Int("users.count", len(userList)),
	)

	c.JSON(200, gin.H{
		"users": userList,
		"total": len(userList),
	})
}

func createUser(c *gin.Context) {
	var newUser User
	if err := c.ShouldBindJSON(&newUser); err != nil {
		userOperations.WithLabelValues("create_user", "error").Inc()
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// 添加追踪信息
	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(
		attribute.String("operation", "create_user"),
		attribute.String("user.name", newUser.Name),
		attribute.String("user.email", newUser.Email),
	)

	// 生成新的用户 ID
	newUser.ID = len(users) + 1
	newUser.CreateAt = time.Now().Format(time.RFC3339)
	newUser.Status = "active"

	// 模拟数据库写入
	simulateWork(c.Request.Context(), "database_insert")

	users[newUser.ID] = newUser

	userOperations.WithLabelValues("create_user", "success").Inc()
	span.SetAttributes(
		attribute.String("result", "success"),
		attribute.Int("user.id", newUser.ID),
	)

	c.JSON(201, newUser)
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
	r.Use(otelgin.Middleware("user-service")) // 必须在 loggingMiddleware 之前
	r.Use(prometheusMiddleware())
	r.Use(loggingMiddleware())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy", "service": "user-service"})
	})

	// Prometheus 指标端点
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// 用户 API 路由
	r.GET("/users/:id", getUserByID)
	r.GET("/users", getAllUsers)
	r.POST("/users", createUser)

	log.Println("User Service starting on port 8080...")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
