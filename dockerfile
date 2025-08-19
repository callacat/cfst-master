# 1. 构建阶段
FROM golang:1.25-alpine AS builder

WORKDIR /app

# 下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 拷贝源代码并编译
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o multi-net-controller ./cmd/main.go

# 2. 运行阶段
FROM alpine

# 安装根证书
RUN apk add --no-cache ca-certificates

WORKDIR /root/

# 拷贝可执行文件
COPY --from=builder /app/multi-net-controller .

# 拷贝默认配置，可在运行时通过 -v 覆盖
COPY config.yml .

# 默认执行命令，使用 --config 指定配置路径
ENTRYPOINT ["./multi-net-controller", "--config", "config.yml"]
