# OpenSearch File API

多租户文件索引服务，基于 OpenSearch 实现图片、PDF、文档等文件的索引、存储和搜索功能。

## 功能特性

- **多租户支持**: 基于 JWT 认证和租户隔离的索引管理
- **文件索引**: 支持 PDF、图片、Office 文档、文本文件等格式
- **全文搜索**: 基于 OpenSearch 的高效全文检索
- **灵活存储**: 支持本地存储和 S3 兼容存储（MinIO、AWS S3）
- **容器化部署**: 提供 Docker、Docker Compose 和 Helm Chart
- **监控指标**: Prometheus 格式的丰富监控指标

## 快速开始

### 使用 Docker Compose

```bash
# 启动服务
cd deployments/docker
docker-compose up -d

# 查看日志
docker-compose logs -f opensearch-file-api

# 停止服务
docker-compose down
```

### 本地开发

```bash
# 安装依赖
go mod download

# 运行服务
make run

# 或构建后运行
make build
./bin/opensearch-file-api
```

## API 文档

### 认证

获取 JWT Token:

```bash
curl -X POST http://localhost:8080/api/v1/token \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "tenant-1",
    "user_id": "user-1",
    "role": "admin"
  }'
```

### 文件上传

```bash
curl -X POST http://localhost:8080/api/v1/files \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-1" \
  -F "file=@document.pdf" \
  -F "description=示例文档" \
  -F "tags[]=重要" \
  -F "tags[]=合同"
```

### 文件搜索

```bash
# GET 方式搜索
curl -X GET "http://localhost:8080/api/v1/search?q=合同&file_type=pdf" \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-1"

# POST 方式高级搜索
curl -X POST http://localhost:8080/api/v1/search \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-1" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "合同",
    "filters": {
      "file_type": "pdf"
    },
    "size": 20,
    "from": 0
  }'
```

### 文件列表

```bash
curl -X GET "http://localhost:8080/api/v1/files?page=1&size=20" \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-1"
```

### 删除文件

```bash
curl -X DELETE http://localhost:8080/api/v1/files/<file_id> \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-1"
```

### 健康检查

```bash
# 健康检查（检查 OpenSearch 连接）
curl http://localhost:8080/health

# Ping 检查（轻量级）
curl http://localhost:8080/ping
```

### 监控指标

```bash
# 获取 Prometheus 格式指标
curl http://localhost:8080/metrics
```

可用的指标包括：
- `opensearch_api_http_requests_total` - HTTP 请求总数
- `opensearch_api_http_request_duration_seconds` - 请求延迟（秒）
- `opensearch_api_http_request_size_bytes` - 请求体大小（字节）
- `opensearch_api_http_response_size_bytes` - 响应体大小（字节）
- `opensearch_api_http_inflight_requests` - 正在处理的请求数
- `go_*` - Go 运行时指标（goroutines、GC 等）
- `process_*` - 进程指标（CPU、内存等）

### 租户管理

```bash
# 创建租户
curl -X POST http://localhost:8080/api/v1/admin/tenants \
  -H "Content-Type: application/json" \
  -d '{
    "id": "tenant-1",
    "name": "测试租户",
    "description": "用于测试的租户"
  }'

# 获取租户信息
curl -X GET http://localhost:8080/api/v1/admin/tenants/tenant-1

# 列出租户
curl -X GET http://localhost:8080/api/v1/admin/tenants
```

## 配置

### 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `OPENSEARCH_SERVER_PORT` | 服务端口 | 8080 |
| `OPENSEARCH_OPENSEARCH_HOST` | OpenSearch 主机 | localhost |
| `OPENSEARCH_OPENSEARCH_PORT` | OpenSearch 端口 | 9200 |
| `OPENSEARCH_OPENSEARCH_USERNAME` | OpenSearch 用户名 | admin |
| `OPENSEARCH_OPENSEARCH_PASSWORD` | OpenSearch 密码 | admin |
| `OPENSEARCH_OPENSEARCH_SECURE` | 是否使用 HTTPS | false |
| `OPENSEARCH_STORAGE_TYPE` | 存储类型 (local/s3) | local |
| `OPENSEARCH_STORAGE_LOCAL_PATH` | 本地存储路径 | ./data/files |
| `OPENSEARCH_JWT_SECRET` | JWT 密钥 | 需修改 |
| `OPENSEARCH_LOG_LEVEL` | 日志级别 | info |
| `OPENSEARCH_LOG_FORMAT` | 日志格式 (json/console) | json |
| `OPENSEARCH_METRICS_PORT` | Metrics 端口 | 与 Server Port 相同 |

### 配置文件

详见 `configs/config.yaml`

## Kubernetes 部署

```bash
# 安装 Helm Chart
helm install opensearch-file-api ./deployments/helm/opensearch-file-api \
  --values values.yaml

# 自定义配置
helm install opensearch-file-api ./deployments/helm/opensearch-file-api \
  --set config.opensearch.host=opensearch.example.com \
  --set config.storage.type=s3 \
  --set config.storage.s3Bucket=my-bucket
```

## 开发

### 运行测试

```bash
# 单元测试
make test

# 集成测试（需要 Docker）
make test-integration

# 生成覆盖率报告
make test-coverage
```

### 代码质量

```bash
# 运行 linter
make lint

# 自动修复
make lint-fix
```

### 构建 Docker 镜像

```bash
make docker-build
```

## 支持的文件格式

| 类型 | 格式 | 内容提取 |
|------|------|----------|
| PDF | .pdf | 文本内容、元数据（作者、标题、页数） |
| 图片 | .jpg, .png, .gif, .webp, .svg | 元数据（尺寸、格式）、可选 OCR |
| 文本 | .txt, .md, .json, .csv | 纯文本 |
| HTML | .html, .htm | 提取纯文本 |
| Office | .doc, .docx, .xls, .xlsx, .ppt, .pptx | 基础支持 |
| RTF | .rtf | 文本内容 |

## 架构

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Client    │────▶│  File API    │────▶│ OpenSearch  │
│             │     │  (Gin + Go)  │     │   Cluster   │
└─────────────┘     └──────┬───────┘     └─────────────┘
                           │
                    ┌──────▼───────┐
                    │   Storage    │
                    │  (Local/S3)  │
                    └──────────────┘

┌─────────────┐
│  Prometheus │◀─── GET /metrics
│  / Grafana  │     (监控指标)
└─────────────┘
```

## API 接口概览

| 接口 | 方法 | 认证 | 说明 |
|------|------|------|------|
| `/health` | GET | 否 | 健康检查（检查 OpenSearch 连接） |
| `/ping` | GET | 否 | 轻量级 Ping 检查 |
| `/metrics` | GET | 否 | Prometheus 监控指标 |
| `/api/v1/token` | POST | 否 | 生成 JWT Token（测试用） |
| `/api/v1/admin/tenants` | POST/GET | 是 | 创建/列出租户 |
| `/api/v1/admin/tenants/:id` | GET/PUT/DELETE | 是 | 获取/更新/删除租户 |
| `/api/v1/files` | POST/GET | 是 | 上传文件/列出文件 |
| `/api/v1/files/:id` | GET/DELETE | 是 | 下载文件/删除文件 |
| `/api/v1/files/:id/metadata` | GET | 是 | 获取文件元数据 |
| `/api/v1/search` | GET/POST | 是 | 搜索文件 |
| `/api/v1/search/aggregate` | POST | 是 | 聚合查询 |
| `/api/v1/search/count` | GET | 是 | 统计文件数量 |

## License

MIT License
