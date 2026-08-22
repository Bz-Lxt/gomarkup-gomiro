# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=0 \
    GOOS=linux
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk add --no-cache git ca-certificates tzdata
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.21
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata \
    && mkdir -p /data/uploads
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /out/server /app/server
COPY migrations /app/migrations
RUN chmod 0777 /data/uploads
EXPOSE 8080
ENTRYPOINT ["/app/server"]
