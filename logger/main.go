package main

import (
	"context" // 引入context，虽然本例中未深度使用OTel的context，但为最佳实践预留
	"fmt"
	"io"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
	// "go.opentelemetry.io/otel/trace" // 假设如果用了OTel，会这样获取traceID
)

const (
	serviceName    = "my-go-filebeat-app"
	serviceVersion = "1.0.0"
)

// 模拟从context获取TraceID (在实际OTel集成中，这会由OTel库提供)
func getMockTraceID(ctx context.Context) string {
	// In a real app with OTel:
	// span := trace.SpanFromContext(ctx)
	// if span.SpanContext().HasTraceID() {
	//  return span.SpanContext().TraceID().String()
	// }
	// For demo, generate a random-like one
	return fmt.Sprintf("trace-%x", rand.Int63n(time.Now().UnixNano()))
}

func main() {
	InitLogger()
	slog.Info("Application starting...")

	// --- HTTP服务器逻辑 ---
	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		// 为每个请求创建上下文相关的logger
		// 在实际应用中，trace_id和request_id会由中间件或上游服务注入到context或请求头
		ctx := r.Context()             // 假设context中已包含追踪信息
		traceID := getMockTraceID(ctx) // 模拟获取TraceID
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("req-%x", rand.Int63n(time.Now().UnixNano()))
		}

		reqLogger := slog.Default().With( // 使用默认logger并添加属性
			slog.String("trace_id", traceID),
			slog.String("request_id", requestID),
			slog.String("http_method", r.Method),
			slog.String("http_path", r.URL.Path),
		)

		reqLogger.Info("Received request for /hello.")

		// 模拟业务处理
		processingTime := time.Duration(rand.Intn(100)+20) * time.Millisecond
		time.Sleep(processingTime)

		if rand.Intn(10) < 2 { // 20%概率出错
			reqLogger.Error("Failed to process /hello request.",
				slog.String("error", "simulated internal error processing hello request"),
				slog.Duration("processing_time_ms", processingTime),
			)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		reqLogger.Info("Successfully processed /hello request.",
			slog.Duration("processing_time_ms", processingTime),
			slog.String("response_payload", "Hello Gopher!"),
		)
		fmt.Fprintln(w, "Hello Gopher!")
	})

	port := "8088" // Go应用监听的端口
	slog.Info("HTTP server listening on port.", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		slog.Error("Failed to start HTTP server.", slog.Any("error", err))
		os.Exit(1)
	}
}

func InitLogger() {

	// 创建日志目录
	os.MkdirAll("/app_logs", 0755)

	// 创建日志文件
	logFile, err := os.OpenFile("/app_logs/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}

	// 同时输出到stdout和文件
	multiWriter := io.MultiWriter(os.Stdout, logFile)

	// --- 初始化slog Logger (输出JSON到stdout) ---
	logLevel := new(slog.LevelVar) // Default to Info
	logLevel.Set(slog.LevelInfo)   // 可以从配置读取和设置
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel.Set(slog.LevelDebug)
	}

	jsonHandler := slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
		AddSource: true, // 添加源码位置
		Level:     logLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey { // 标准化时间格式
				a.Value = slog.StringValue(a.Value.Time().Format(time.RFC3339Nano))
			}
			if a.Key == slog.LevelKey { // 将级别转为大写字符串
				level := a.Value.Any().(slog.Level)
				a.Value = slog.StringValue(strings.ToUpper(level.String()))
			}
			return a
		},
	})

	// 创建基础Logger，并添加全局属性
	baseLogger := slog.New(jsonHandler).With(
		slog.String("service_name", serviceName),
		slog.String("service_version", serviceVersion),
	)
	slog.SetDefault(baseLogger) // 设置为全局默认logger，方便各处使用
}
