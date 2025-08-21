# 1. 构建阶段
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o multi-net-controller ./cmd/main.go

# 2. 运行阶段
FROM alpine
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

# [修改] 将工作目录设置为 /app
WORKDIR /app

# [修改] 创建 config 目录，用于挂载
RUN mkdir -p /app/config

# [修改] 将可执行文件拷贝到工作目录下
COPY --from=builder /app/multi-net-controller .

# [删除] 不再拷贝 config.yml，因为它将通过 volume 挂载

# [修改] 默认执行命令，程序会从相对路径寻找配置文件
ENTRYPOINT ["./multi-net-controller"]