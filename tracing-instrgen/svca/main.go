package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/xyzbit/devops-demo/tracing-instrgen/proto"
)

const (
	serviceName = "svca"
	servicePort = ":8080"
	svcbAddress = "svcb:50051"
)

// UserHandler 用户处理器
type UserHandler struct {
	grpcClient pb.UserServiceClient
}

// UserResponse HTTP 响应结构
type UserResponse struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Status string `json:"status"`
}

// CreateUserRequest HTTP 创建用户请求
type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// 创建 gRPC 客户端连接
func createGRPCClient() pb.UserServiceClient {
	conn, err := grpc.Dial(svcbAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("连接 gRPC 服务失败: %v", err)
	}

	return pb.NewUserServiceClient(conn)
}

// GetUser HTTP 处理器 - 获取用户
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "缺少 user_id 参数", http.StatusBadRequest)
		return
	}

	// 调用 gRPC 服务
	grpcReq := &pb.GetUserRequest{UserId: userID}
	grpcResp, err := h.grpcClient.GetUser(context.Background(), grpcReq)
	if err != nil {
		log.Printf("调用 gRPC 服务失败: %v", err)
		http.Error(w, "内部服务错误", http.StatusInternalServerError)
		return
	}

	// 转换响应
	response := UserResponse{
		UserID: grpcResp.UserId,
		Name:   grpcResp.Name,
		Email:  grpcResp.Email,
		Status: grpcResp.Status,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	log.Printf("HTTP 获取用户成功: %s", userID)
}

// CreateUser HTTP 处理器 - 创建用户
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求格式错误", http.StatusBadRequest)
		return
	}

	// 调用 gRPC 服务
	grpcReq := &pb.CreateUserRequest{
		Name:  req.Name,
		Email: req.Email,
	}
	grpcResp, err := h.grpcClient.CreateUser(context.Background(), grpcReq)
	if err != nil {
		log.Printf("调用 gRPC 服务失败: %v", err)
		http.Error(w, "内部服务错误", http.StatusInternalServerError)
		return
	}

	// 转换响应
	response := UserResponse{
		UserID: grpcResp.UserId,
		Name:   grpcResp.Name,
		Email:  grpcResp.Email,
		Status: grpcResp.Status,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	log.Printf("HTTP 创建用户成功: %s (%s)", req.Name, req.Email)
}

// Health 健康检查处理器
func (h *UserHandler) Health(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "ok",
		"service":   serviceName,
		"timestamp": time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	// 创建 gRPC 客户端
	grpcClient := createGRPCClient()

	// 创建处理器
	handler := &UserHandler{
		grpcClient: grpcClient,
	}

	// 创建路由
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.GetUser(w, r)
		case http.MethodPost:
			handler.CreateUser(w, r)
		default:
			http.Error(w, "方法不支持", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/health", handler.Health)

	// 创建 HTTP 服务器
	server := &http.Server{
		Addr:    servicePort,
		Handler: mux,
	}

	log.Printf("svca HTTP 服务启动，监听端口 %s", servicePort)
	log.Printf("连接到 svcb gRPC 服务: %s", svcbAddress)

	// 优雅关闭
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		log.Println("收到关闭信号，正在关闭服务...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	// 启动服务
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("启动服务失败: %v", err)
	}
	log.Println("服务已关闭")
}
