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
# 启动服务（OpenSearch + MinIO + API）
docker compose -f deployments/docker/docker-compose.yml up -d

# 查看日志
docker compose -f deployments/docker/docker-compose.yml logs -f opensearch-file-api

# 停止服务
docker compose -f deployments/docker/docker-compose.yml down
```

> 注意：服务端口映射已调整为 `18080:8080`，API 地址为 `http://localhost:18080`

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
curl -X POST http://localhost:18080/api/v1/token \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "tenant-1",
    "role": "admin"
  }'
```

### 文件上传

```bash
curl -X POST http://localhost:18080/api/v1/files \
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
curl -X GET "http://localhost:18080/api/v1/search?q=合同&file_type=pdf" \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-1"

# POST 方式高级搜索
curl -X POST http://localhost:18080/api/v1/search \
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

### KNN 向量搜索

通过向量进行相似度搜索，适用于语义搜索、图片相似度检索等场景：

```bash
curl -X POST http://localhost:18080/api/v1/search/knn \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-1" \
  -H "Content-Type: application/json" \
  -d '{
    "vector": [0.1, 0.2, 0.3, ...],
    "field": "content_vector",
    "k": 10,
    "filters": {
      "file_type": "pdf"
    }
  }'
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `vector` | array | 是 | 查询向量 |
| `field` | string | 否 | 向量字段名，默认 `content_vector` |
| `k` | int | 否 | 返回结果数量，默认 10，最大 100 |
| `filters` | object | 否 | 过滤条件 |

### 混合搜索（文本 + 向量）

结合全文搜索和向量搜索，获得更精准的搜索结果：

```bash
curl -X POST http://localhost:18080/api/v1/search/hybrid \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-1" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "合同条款",
    "vector": [0.1, 0.2, 0.3, ...],
    "vector_field": "content_vector",
    "k": 10,
    "filters": {}
  }'
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | string | 是 | 文本查询关键词 |
| `vector` | array | 否 | 查询向量 |
| `vector_field` | string | 否 | 向量字段名，默认 `content_vector` |
| `k` | int | 否 | 返回结果数量，默认 10，最大 100 |
| `filters` | object | 否 | 过滤条件 |

### 文件列表

```bash
curl -X GET "http://localhost:18080/api/v1/files?page=1&size=20" \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-1"
```

### 删除文件

```bash
curl -X DELETE http://localhost:18080/api/v1/files/<file_id> \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-1"
```

### 健康检查

```bash
# 健康检查（检查 OpenSearch 连接）
curl http://localhost:18080/health

# Ping 检查（轻量级）
curl http://localhost:18080/ping
```

### 监控指标

```bash
# 获取 Prometheus 格式指标
curl http://localhost:18080/metrics
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
curl -X POST http://localhost:18080/api/v1/admin/tenants \
  -H "Content-Type: application/json" \
  -d '{
    "id": "tenant-1",
    "name": "测试租户",
    "description": "用于测试的租户"
  }'

# 获取租户信息
curl -X GET http://localhost:18080/api/v1/admin/tenants/tenant-1

# 列出租户
curl -X GET http://localhost:18080/api/v1/admin/tenants

# 更新租户
curl -X PUT http://localhost:18080/api/v1/admin/tenants/tenant-1 \
  -H "Content-Type: application/json" \
  -d '{
    "name": "新租户名称",
    "description": "更新后的描述"
  }'

# 删除租户（软删除，标记为已删除但保留数据）
curl -X DELETE http://localhost:18080/api/v1/admin/tenants/tenant-1

# 彻底删除租户（不可恢复）
curl -X DELETE http://localhost:18080/api/v1/admin/tenants/tenant-1/hard
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

## 图片 OCR 识别

服务支持对上传的图片进行 OCR 识别，提取图片中的文字内容。

### Docker 中使用 OCR

Docker 镜像已预装 Tesseract OCR 及多语言包，无需额外配置即可使用。

```bash
# 构建 Docker 镜像（包含 OCR 支持）
make docker-build

# 或使用 docker-compose
cd deployments/docker
docker-compose up -d
```

### 本地开发启用 OCR

1. **安装 Tesseract OCR**

   ```bash
   # macOS
   brew install tesseract

   # Ubuntu/Debian
   apt-get install tesseract-ocr

   # 安装语言包（可选）
   brew install tesseract-lang  # macOS
   apt-get install tesseract-ocr-eng tesseract-ocr-chi-sim  # Linux
   ```

2. **配置文件** (`configs/config.yaml`)

   ```yaml
   storage:
     image_ocr: true        # 启用 OCR
     image_ocr_lang: "eng"  # 语言：eng(英文), chi_sim(简体中文), jpn(日文)
   ```

### 支持的语言

- `eng` - 英语（默认）
- `chi_sim` - 简体中文
- `chi_tra` - 繁体中文
- `jpn` - 日语
- `kor` - 韩语

更多语言请参考 [Tesseract 文档](https://tesseract-ocr.github.io/tessdoc/Data-Files-in-different-versions.html)

### 环境变量

```bash
# 启用 OCR
export OPENSEARCH_STORAGE_IMAGE_OCR=true

# 设置 OCR 语言
export OPENSEARCH_STORAGE_IMAGE_OCR_LANG=chi_sim
```

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
| `/api/v1/admin/tenants/:id` | GET | 是 | 获取租户 |
| `/api/v1/admin/tenants/:id` | PUT | 是 | 更新租户 |
| `/api/v1/admin/tenants/:id` | DELETE | 是 | 删除租户（软删除） |
| `/api/v1/admin/tenants/:id/hard` | DELETE | 是 | 彻底删除租户 |
| `/api/v1/files` | POST/GET | 是 | 上传文件/列出文件 |
| `/api/v1/files/:id` | GET/DELETE | 是 | 下载文件/删除文件 |
| `/api/v1/files/:id/metadata` | GET | 是 | 获取文件元数据 |
| `/api/v1/search` | GET/POST | 是 | 搜索文件 |
| `/api/v1/search/aggregate` | POST | 是 | 聚合查询 |
| `/api/v1/search/count` | GET | 是 | 统计文件数量 |
| `/api/v1/search/knn` | POST | 是 | KNN 向量搜索 |
| `/api/v1/search/hybrid` | POST | 是 | 混合搜索（文本 + 向量） |

## License

MIT License
