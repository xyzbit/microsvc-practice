package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/xyzbit/devops-demo/tracing-instrgen/proto"
)

const (
	serviceName = "svcb"
	servicePort = ":50051"
)

// UserServer 实现 gRPC 用户服务
type UserServer struct {
	pb.UnimplementedUserServiceServer
}

// GetUser 获取用户信息
func (s *UserServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	// 模拟数据库查询延迟
	time.Sleep(50 * time.Millisecond)

	// 模拟用户数据
	response := &pb.GetUserResponse{
		UserId: req.UserId,
		Name:   fmt.Sprintf("用户_%s", req.UserId),
		Email:  fmt.Sprintf("user%s@example.com", req.UserId),
		Status: "active",
	}

	log.Printf("获取用户: %s", req.UserId)
	return response, nil
}

// CreateUser 创建用户
func (s *UserServer) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	// 模拟数据库写入延迟
	time.Sleep(100 * time.Millisecond)

	// 生成用户ID
	userID := fmt.Sprintf("user_%d", time.Now().Unix())

	response := &pb.CreateUserResponse{
		UserId: userID,
		Name:   req.Name,
		Email:  req.Email,
		Status: "active",
	}

	log.Printf("创建用户: %s (%s)", req.Name, req.Email)
	return response, nil
}

func main() {
	// 创建 gRPC 服务器
	s := grpc.NewServer()

	// 注册用户服务
	userServer := &UserServer{}
	pb.RegisterUserServiceServer(s, userServer)

	// 启用反射 (用于调试)
	reflection.Register(s)

	// 创建监听器
	lis, err := net.Listen("tcp", servicePort)
	if err != nil {
		log.Fatalf("监听端口失败: %v", err)
	}

	log.Printf("svcb gRPC 服务启动，监听端口 %s", servicePort)

	// 优雅关闭
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		log.Println("收到关闭信号，正在关闭服务...")
		s.GracefulStop()
	}()

	// 启动服务
	if err := s.Serve(lis); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
}
