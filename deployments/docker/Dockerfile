# 多阶段构建 Dockerfile
FROM golang:1.26-alpine AS builder

# 安装必要的工具
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
ARG VERSION=dev
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -w -s" \
    -o /build/opensearch-file-api ./cmd/server

# 运行阶段
FROM alpine:3.19

# 安装运行时依赖，包括 Tesseract OCR 及其语言包
RUN apk add --no-cache ca-certificates tzdata \
    tesseract-ocr \
    tesseract-ocr-data-eng \
    tesseract-ocr-data-chi_sim \
    tesseract-ocr-data-chi_tra \
    tesseract-ocr-data-jpn \
    tesseract-ocr-data-kor

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/opensearch-file-api /app/opensearch-file-api

# 创建非 root 用户
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/app/opensearch-file-api", "-healthcheck"] || exit 1

# 启动应用
ENTRYPOINT ["/app/opensearch-file-api"]
