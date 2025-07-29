#!/bin/bash

echo "=== 生成 proto 文件和初始化依赖 ==="

# 检查是否安装了 protoc
if ! command -v protoc &> /dev/null; then
    echo "错误: protoc 未安装，请先安装 Protocol Buffers compiler"
    echo "macOS: brew install protobuf"
    echo "Ubuntu: sudo apt-get install protobuf-compiler"
    exit 1
fi

# 检查是否安装了 protoc-gen-go
if ! command -v protoc-gen-go &> /dev/null; then
    echo "安装 protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

# 检查是否安装了 protoc-gen-go-grpc
if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "安装 protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# 创建 proto 目录下的 go.mod
echo "初始化 proto 模块..."
cd proto
if [ ! -f "go.mod" ]; then
    go mod init proto
fi

# 生成 Go 代码
echo "生成 proto Go 代码..."
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       service.proto

cd ..

# 初始化 svcb 依赖
echo "初始化 svcb 依赖..."
cd svcb
go mod tidy

cd ..

# 初始化 svca 依赖
echo "初始化 svca 依赖..."
cd svca
go mod tidy

cd ..

echo "=== 生成完成 ==="
echo "现在可以运行以下命令启动服务："
echo "1. 启动追踪基础设施: docker-compose up -d"
echo "2. 启动 svcb: cd svcb && go run main.go"
echo "3. 启动 svca: cd svca && go run main.go" 