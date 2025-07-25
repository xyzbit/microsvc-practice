FROM alpine:latest

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates tzdata curl

# 设置工作目录
WORKDIR /

# 复制预编译的二进制文件
ADD ./server /

# 暴露端口
EXPOSE 8088

# 运行命令
ENTRYPOINT ["/server"] 