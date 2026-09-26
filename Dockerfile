# StrmSub：基于已刮削元数据的 STRM 字幕工具
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
# 非 root 运行（飞牛 PUID 惯例可按需覆盖）
RUN adduser -D -u 1026 strmsub
USER strmsub
WORKDIR /app
VOLUME ["/app/data"]
EXPOSE 8099
ENTRYPOINT ["strmsub"]
