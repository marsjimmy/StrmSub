# StrmSub v2：文件名正则识别 + 6 字幕源聚合的 STRM 字幕工具
# 注意：发布镜像用 crane + 预编译二进制组装（见发布记录），此 Dockerfile 仅供本地构建参考。
FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /strmsub ./cmd/strmsub

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /strmsub /usr/local/bin/strmsub
# 默认 root 运行，保证能读写媒体目录和数据目录（家用 NAS 场景）
WORKDIR /app
VOLUME ["/app/data"]
EXPOSE 8099
ENV TZ=Asia/Shanghai
ENTRYPOINT ["strmsub"]
