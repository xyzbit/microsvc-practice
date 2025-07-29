# 使用轻量级镜像作为运行环境
FROM alpine:latest

WORKDIR /

ADD bin/svcb /svcb

# 暴露端口
EXPOSE 8080

# 运行应用
CMD ["/svcb"] 