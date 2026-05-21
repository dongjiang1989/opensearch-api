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

### 向量检索（自动 Embedding）

上传文件或传入文本，系统自动通过 Embedding 服务转换为向量后进行相似度检索：

```bash
# JSON 模式：传入文本 query
curl -X POST http://localhost:18080/api/v1/search/retrieve \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-1" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "machine learning algorithms",
    "k": 10,
    "field": "content_vector"
  }'

# Multipart 模式：上传文件
curl -X POST http://localhost:18080/api/v1/search/retrieve \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-1" \
  -F "file=@document.pdf" \
  -F "query=补充关键词" \
  -F "k=10"
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | string | JSON 模式必填 | 文本查询关键词 |
| `file` | file | Multipart 模式必填 | 上传文件，自动提取内容并转为向量 |
| `k` | int | 否 | 返回结果数量，默认 10，最大 100 |
| `field` | string | 否 | 向量字段名，默认 `content_vector` |
| `filters` | object | 否 | 过滤条件 |

> 注意：需要配置嵌入服务（`embedding.provider=openai/local/clip`），否则返回 503。

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

## 多租户搜索

支持通过 `X-Tenant-ID` 传入多个租户（逗号分隔），实现跨租户联合搜索。

```bash
# 跨租户全文搜索
curl -X POST http://localhost:18080/api/v1/search \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-a,tenant-b" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "合同",
    "size": 20
  }'

# 跨租户 KNN 向量搜索
curl -X POST http://localhost:18080/api/v1/search/knn \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-a,tenant-b" \
  -H "Content-Type: application/json" \
  -d '{
    "vector": [0.1, 0.2, 0.3, ...],
    "k": 10
  }'

# 跨租户混合搜索
curl -X POST http://localhost:18080/api/v1/search/hybrid \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-a,tenant-b" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "合同条款",
    "vector": [0.1, 0.2, 0.3, ...],
    "k": 10
  }'

# 跨租户聚合统计（含每租户细分）
curl -X POST http://localhost:18080/api/v1/search/aggregate \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-a,tenant-b" \
  -H "Content-Type: application/json" \
  -d '{"field": "file_type"}'

# 跨租户文件计数
curl -X GET http://localhost:18080/api/v1/search/count \
  -H "Authorization: Bearer <token>" \
  -H "X-Tenant-ID: tenant-a,tenant-b"
```

### 跨索引 Score 归一化（D11）

多租户搜索使用 OpenSearch `dfs_query_then_fetch` 搜索类型，在所有分片上收集词频信息后再评分，消除各租户独立索引带来的 IDF 偏差。搜索结果中每个 hit 携带 `index` 字段标识来源索引。

### 聚合租户细分（D12）

聚合查询返回两个维度的结果：
- `buckets`：合并后的总体聚合结果（向后兼容）
- `by_tenant`：每租户维度的细分数据（`tenantID -> field_value -> count`）

## 配置

### 配置文件

详见 `configs/config.yaml`，支持以下配置项：

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `server.port` | 服务端口 | 8080 |
| `server.host` | 监听地址 | 0.0.0.0 |
| `server.mode` | 运行模式 | release (debug/release/test) |
| `server.read_timeout` | 读超时 | 30s |
| `server.write_timeout` | 写超时 | 60s |
| `opensearch.host` | OpenSearch 主机 | localhost |
| `opensearch.port` | OpenSearch 端口 | 9200 |
| `opensearch.username` | OpenSearch 用户名 | admin |
| `opensearch.password` | OpenSearch 密码 | admin |
| `opensearch.secure` | 是否使用 HTTPS | true |
| `opensearch.index_prefix` | 租户索引前缀 | tenant |
| `storage.type` | 存储类型 | local (local/s3) |
| `storage.local_path` | 本地存储路径 | ./data/files |
| `storage.image_ocr` | 启用图片 OCR | false |
| `storage.image_ocr_lang` | OCR 语言 | eng |
| `jwt.secret` | JWT 密钥 | change-this-secret-key |
| `jwt.issuer` | JWT 签发者 | opensearch-file-api |
| `jwt.expire_time` | Token 过期时间 | 24h |
| `log.level` | 日志级别 | info |
| `log.format` | 日志格式 | json (json/console) |
| `embedding.provider` | 嵌入服务提供者 | openai (openai/local/clip/none) |
| `embedding.model` | 嵌入模型 | text-embedding-3-small |
| `embedding.dimensions` | 向量维度 | 1536 |
| `embedding.api_url` | 嵌入服务 API 地址 | - |
| `embedding.api_key` | 嵌入服务 API 密钥 | - |
| `embedding.timeout` | 请求超时（秒） | 30 |

### 启用向量检索

向量检索（`/search/retrieve`）需要配置嵌入服务。支持的 provider：

| Provider | 说明 | 适用场景 |
|----------|------|----------|
| `openai` | OpenAI 兼容 API | 使用 OpenAI 或兼容服务（如 Ollama v1 API） |
| `local` | 本地 Ollama 服务 | 使用本地 Ollama 部署模型 |
| `clip` | CLIP 多模态服务 | 图片+文本多模态嵌入 |
| `none` | 不启用（默认） | 仅使用全文搜索，无需向量服务 |

环境变量示例（使用 Ollama 本地部署）：

```bash
export OPENSEARCH_EMBEDDING_PROVIDER=openai
export OPENSEARCH_EMBEDDING_APIURL=http://localhost:11434/v1/embeddings
export OPENSEARCH_EMBEDDING_MODEL=nomic-embed-text
export OPENSEARCH_EMBEDDING_TIMEOUT=60
```

Docker Compose 示例：

```yaml
environment:
  - OPENSEARCH_EMBEDDING_PROVIDER=openai
  - OPENSEARCH_EMBEDDING_APIURL=http://ollama:11434/v1/embeddings
  - OPENSEARCH_EMBEDDING_MODEL=nomic-embed-text
  - OPENSEARCH_EMBEDDING_TIMEOUT=60
```

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

# E2E 端到端测试（需要 Docker Compose 运行中）
bash scripts/test_e2e.sh

# Embedding E2E 测试（需要 mock embedding server，自动启动）
bash scripts/test_e2e_embedding.sh
```

E2E 测试覆盖：租户管理 → JWT 生成 → 文件上传 → 文件操作 → 文本搜索 → 租户隔离 → 向量搜索（KNN/混合/自动Embedding检索） → 单租户检索 → **跨租户联合搜索** → 聚合统计（含租户细分）→ 健康检查

Embedding E2E 测试覆盖：启动 mock embedding server → 重启 app（启用 embedding） → 上传文件（自动生成向量） → retrieve JSON 模式 → retrieve multipart 模式 → 文件+query 混合模式

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

## OpenSearch 索引映射

### 文件索引结构

每个租户的文件存储在独立的索引中，命名格式为 `tenant_{tenantID}_files`。

| 字段 | 类型 | 说明 |
|------|------|------|
| `filename` | text + keyword | 文件名，支持全文搜索和精确匹配 |
| `content` | text | 文件内容（从 PDF/文档中提取） |
| `content_type` | keyword | MIME 类型 |
| `file_type` | keyword | 文件类型（pdf, image, text 等） |
| `file_size` | long | 文件大小（字节） |
| `description` | text | 文件描述 |
| `tags` | keyword | 标签数组 |
| `metadata` | object | 元数据（width, height, duration, pages, author, created_at） |
| `storage_path` | keyword | 存储路径 |
| `tenant_id` | keyword | 租户 ID |
| `created_at` | date | 创建时间 |
| `updated_at` | date | 更新时间 |
| `content_vector` | knn_vector (1536维) | 文本嵌入向量，用于语义搜索 |
| `image_vector` | knn_vector (512维) | 图片嵌入向量（CLIP），用于图片相似度搜索 |

### 向量搜索

OpenSearch 使用 `knn_vector` 类型存储向量，底层采用 NMSLIB 引擎：

- **content_vector** (1536维): 适用于 OpenAI text-embedding-3-small 等模型
- **image_vector** (512维): 适用于 CLIP 多模态模型

> 注意：NMSLIB 引擎不支持 KNN 查询中的过滤器（filters），混合搜索已针对此限制进行优化。

## Swagger API 文档

项目使用 Swag 生成 OpenAPI 2.0 规范的文档：

```bash
# 生成 Swagger 文档
make swag

# 生成的文件位于 api/swagger.yaml
```

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
| `/api/v1/search` | GET/POST | 是 | 搜索文件（支持多租户，逗号分隔 `X-Tenant-ID`） |
| `/api/v1/search/aggregate` | POST | 是 | 聚合查询（返回 `buckets` + `by_tenant` 细分） |
| `/api/v1/search/count` | GET | 是 | 统计文件数量（支持多租户） |
| `/api/v1/search/knn` | POST | 是 | KNN 向量搜索（支持多租户） |
| `/api/v1/search/hybrid` | POST | 是 | 混合搜索（文本 + 向量，支持多租户） |
| `/api/v1/search/retrieve` | POST | 是 | 向量检索（自动 Embedding） |

## License

MIT License
