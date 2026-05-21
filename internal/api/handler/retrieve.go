package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/api/middleware"
	"github.com/dongjiang1989/opensearch-api/internal/embedding"
	"github.com/dongjiang1989/opensearch-api/internal/indexer"
	"github.com/dongjiang1989/opensearch-api/internal/opensearch"
	"github.com/dongjiang1989/opensearch-api/internal/storage"
)

// RetrieveHandler 向量检索 Handler
type RetrieveHandler struct {
	osClient  indexer.OpenSearchClient
	extractor storage.ContentExtractor
	embedder  embedding.EmbeddingModel
	logger    *zap.Logger
}

// NewRetrieveHandler 创建向量检索 Handler
func NewRetrieveHandler(osClient indexer.OpenSearchClient, extractor storage.ContentExtractor, embedder embedding.EmbeddingModel, logger *zap.Logger) *RetrieveHandler {
	return &RetrieveHandler{
		osClient:  osClient,
		extractor: extractor,
		embedder:  embedder,
		logger:    logger,
	}
}

// RetrieveRequest JSON 请求体
type RetrieveRequest struct {
	Query   string                 `json:"query" binding:"required"`
	K       int                    `json:"k"`
	Field   string                 `json:"field"`
	Filters map[string]interface{} `json:"filters,omitempty"`
}

// RetrieveResponse 检索响应
type RetrieveResponse struct {
	Success bool        `json:"success"`
	Total   int         `json:"total"`
	Took    int         `json:"took_ms"`
	Hits    []VectorHit `json:"hits"`
}

// Retrieve 向量混合检索接口
// @Summary 向量混合检索
// @Description 接收文本 query 或文件，通过 Embedding 服务转换为向量，再从 OpenSearch 检索相关文档
// @Tags Retrieve
// @Accept json
// @Accept multipart/form-data
// @Produce json
// @Param request body RetrieveRequest true "检索请求"
// @Success 200 {object} RetrieveResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Security X-Tenant-ID
// @Router /search/retrieve [post]
func (h *RetrieveHandler) Retrieve(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok || tenantID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "tenant ID is required",
		})
		return
	}

	if h.embedder == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Success: false,
			Error:   "embedding service is not configured",
		})
		return
	}

	contentType := c.ContentType()

	var text string
	var err error

	switch {
	case strings.HasPrefix(contentType, "multipart/form-data"):
		text, err = h.extractMultipartText(c)
	case strings.HasPrefix(contentType, "application/json"):
		text, err = h.extractJSONText(c)
	default:
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "Content-Type must be application/json or multipart/form-data",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if text == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "query text is required",
		})
		return
	}

	// 生成向量
	vector, err := h.embedder.Generate(c.Request.Context(), text)
	if err != nil {
		h.logger.Error("failed to generate embedding",
			zap.String("tenant_id", tenantID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to generate embedding: %v", err),
		})
		return
	}

	// 执行 KNN 检索
	knnQuery := &opensearch.KNNQuery{
		Vector:  vector,
		Field:   "content_vector",
		K:       10,
		Filters: nil,
	}

	result, err := h.osClient.KNNSearch(c.Request.Context(), tenantID, knnQuery)
	if err != nil {
		h.logger.Error("knn retrieve failed",
			zap.String("tenant_id", tenantID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	hits := make([]VectorHit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		hits = append(hits, VectorHit{
			ID:     hit.ID,
			Score:  hit.Score,
			Source: hit.Source,
		})
	}

	c.JSON(http.StatusOK, RetrieveResponse{
		Success: true,
		Total:   result.Total,
		Took:    result.Took,
		Hits:    hits,
	})
}

// extractJSONText 从 JSON body 提取查询文本
func (h *RetrieveHandler) extractJSONText(c *gin.Context) (string, error) {
	var req RetrieveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return "", fmt.Errorf("invalid request body: %v", err)
	}
	return req.Query, nil
}

// extractMultipartText 从 multipart form 提取文本内容
func (h *RetrieveHandler) extractMultipartText(c *gin.Context) (string, error) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		return "", fmt.Errorf("file is required")
	}
	defer func() { _ = file.Close() }()

	query := c.PostForm("query")

	// 提取文件内容
	var fileText string
	if h.extractor != nil && h.extractor.CanHandle(header.Header.Get("Content-Type")) {
		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = detectContentTypeFromFilename(header.Filename)
		}
		extracted, err := h.extractor.Extract(c.Request.Context(), file, contentType)
		if err != nil {
			h.logger.Warn("failed to extract file content",
				zap.String("filename", header.Filename),
				zap.Error(err))
		} else {
			fileText = extracted.Text
		}
	}

	// 无法提取文件内容时，用文件名作为查询
	if fileText == "" {
		fileText = header.Filename
	}

	// 拼接额外查询词
	if query != "" {
		fileText = fileText + "\n" + query
	}

	return fileText, nil
}

// detectContentTypeFromFilename 根据文件名检测内容类型
func detectContentTypeFromFilename(filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(lower, ".txt"), strings.HasSuffix(lower, ".md"):
		return "text/plain"
	case strings.HasSuffix(lower, ".html"), strings.HasSuffix(lower, ".htm"):
		return "text/html"
	case strings.HasSuffix(lower, ".json"):
		return "application/json"
	case strings.HasSuffix(lower, ".csv"):
		return "text/csv"
	default:
		return "application/octet-stream"
	}
}
