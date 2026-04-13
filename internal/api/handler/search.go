package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/api/middleware"
	"github.com/dongjiang1989/opensearch-api/internal/indexer"
	"github.com/dongjiang1989/opensearch-api/internal/opensearch"
)

// SearchHandler 搜索 Handler
type SearchHandler struct {
	osClient indexer.OpenSearchClient
	logger   *zap.Logger
}

// NewSearchHandler 创建搜索 Handler
func NewSearchHandler(osClient indexer.OpenSearchClient, logger *zap.Logger) *SearchHandler {
	return &SearchHandler{
		osClient: osClient,
		logger:   logger,
	}
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query     string                 `json:"query"`
	Filters   map[string]interface{} `json:"filters"`
	From      int                    `json:"from"`
	Size      int                    `json:"size"`
	Sort      []map[string]interface{} `json:"sort"`
	Highlight map[string]interface{} `json:"highlight"`
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Success bool         `json:"success"`
	Total   int          `json:"total"`
	Took    int          `json:"took_ms"`
	Hits    []SearchHit  `json:"hits"`
}

// SearchHit 搜索结果项
type SearchHit struct {
	ID        string                 `json:"id"`
	Score     float64                `json:"score"`
	Source    map[string]interface{} `json:"source"`
	Highlight map[string]interface{} `json:"highlight,omitempty"`
}

// Search 搜索接口
func (h *SearchHandler) Search(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok || tenantID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "tenant ID is required",
		})
		return
	}

	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// 默认值
	if req.Size <= 0 {
		req.Size = 10
	}
	if req.Size > 100 {
		req.Size = 100
	}

	query := &opensearch.SearchQuery{
		Query:     req.Query,
		Filters:   req.Filters,
		From:      req.From,
		Size:      req.Size,
		Sort:      req.Sort,
		Highlight: req.Highlight,
	}

	h.logger.Debug("searching files",
		zap.String("tenant_id", tenantID),
		zap.String("query", req.Query))

	result, err := h.osClient.Search(c.Request.Context(), tenantID, query)
	if err != nil {
		h.logger.Error("search failed",
			zap.String("tenant_id", tenantID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	hits := make([]SearchHit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		hits = append(hits, SearchHit{
			ID:        hit.ID,
			Score:     hit.Score,
			Source:    hit.Source,
			Highlight: hit.Highlight,
		})
	}

	c.JSON(http.StatusOK, SearchResponse{
		Success: true,
		Total:   result.Total,
		Took:    result.Took,
		Hits:    hits,
	})
}

// SearchGET GET 搜索接口（使用查询参数）
func (h *SearchHandler) SearchGET(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok || tenantID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "tenant ID is required",
		})
		return
	}

	query := c.Query("q")
	fileType := c.Query("file_type")
	contentType := c.Query("content_type")

	searchQuery := &opensearch.SearchQuery{
		Query: query,
		From:  0,
		Size:  10,
	}

	// 解析分页
	if f := c.Query("from"); f != "" {
		if _, err := parseParam(f, &searchQuery.From); err != nil {
			searchQuery.From = 0
		}
	}
	if s := c.Query("size"); s != "" {
		if _, err := parseParam(s, &searchQuery.Size); err != nil {
			searchQuery.Size = 10
		}
		if searchQuery.Size > 100 {
			searchQuery.Size = 100
		}
	}

	// 构建过滤器
	filters := make(map[string]interface{})
	if fileType != "" {
		filters["file_type"] = fileType
	}
	if contentType != "" {
		filters["content_type"] = contentType
	}
	if len(filters) > 0 {
		searchQuery.Filters = filters
	}

	h.logger.Debug("searching files (GET)",
		zap.String("tenant_id", tenantID),
		zap.String("query", query))

	result, err := h.osClient.Search(c.Request.Context(), tenantID, searchQuery)
	if err != nil {
		h.logger.Error("search failed",
			zap.String("tenant_id", tenantID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	hits := make([]SearchHit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		hits = append(hits, SearchHit{
			ID:     hit.ID,
			Score:  hit.Score,
			Source: hit.Source,
		})
	}

	c.JSON(http.StatusOK, SearchResponse{
		Success: true,
		Total:   result.Total,
		Took:    result.Took,
		Hits:    hits,
	})
}

// AggregateRequest 聚合请求
type AggregateRequest struct {
	Field string `json:"field" binding:"required"`
}

// AggregateResponse 聚合响应
type AggregateResponse struct {
	Success bool              `json:"success"`
	Field   string            `json:"field"`
	Buckets map[string]int64  `json:"buckets"`
}

// Aggregate 聚合接口
func (h *SearchHandler) Aggregate(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok || tenantID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "tenant ID is required",
		})
		return
	}

	var req AggregateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	buckets, err := h.osClient.Aggregate(c.Request.Context(), tenantID, req.Field)
	if err != nil {
		h.logger.Error("aggregation failed",
			zap.String("tenant_id", tenantID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, AggregateResponse{
		Success: true,
		Field:   req.Field,
		Buckets: buckets,
	})
}

// CountResponse 计数响应
type CountResponse struct {
	Success bool  `json:"success"`
	Count   int64 `json:"count"`
}

// Count 统计文件数量
func (h *SearchHandler) Count(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok || tenantID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "tenant ID is required",
		})
		return
	}

	count, err := h.osClient.Count(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("count failed",
			zap.String("tenant_id", tenantID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, CountResponse{
		Success: true,
		Count:   count,
	})
}

// ListFilesResponse 文件列表响应
type ListFilesResponse struct {
	Success bool     `json:"success"`
	Total   int      `json:"total"`
	Files   []FileInfo `json:"files"`
}

// FileInfo 文件信息
type FileInfo struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	FileType    string    `json:"file_type"`
	FileSize    int64     `json:"file_size"`
	Description string    `json:"description,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListFiles 列出文件
func (h *SearchHandler) ListFiles(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok || tenantID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "tenant ID is required",
		})
		return
	}

	page, size := ParsePagination(c)

	query := &opensearch.SearchQuery{
		From: (page - 1) * size,
		Size: size,
		Sort: []map[string]interface{}{
			{"created_at": map[string]string{"order": "desc"}},
		},
	}

	result, err := h.osClient.Search(c.Request.Context(), tenantID, query)
	if err != nil {
		h.logger.Error("list files failed",
			zap.String("tenant_id", tenantID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	files := make([]FileInfo, 0, len(result.Hits))
	for _, hit := range result.Hits {
		source := hit.Source
		file := FileInfo{
			ID: hit.ID,
		}

		if v, ok := source["filename"].(string); ok {
			file.Filename = v
		}
		if v, ok := source["content_type"].(string); ok {
			file.ContentType = v
		}
		if v, ok := source["file_type"].(string); ok {
			file.FileType = v
		}
		if v, ok := source["file_size"].(float64); ok {
			file.FileSize = int64(v)
		}
		if v, ok := source["description"].(string); ok {
			file.Description = v
		}
		if v, ok := source["tags"].([]interface{}); ok {
			file.Tags = make([]string, len(v))
			for i, t := range v {
				if s, ok := t.(string); ok {
					file.Tags[i] = s
				}
			}
		}
		if v, ok := source["created_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				file.CreatedAt = t
			}
		}

		files = append(files, file)
	}

	c.JSON(http.StatusOK, ListFilesResponse{
		Success: true,
		Total:   result.Total,
		Files:   files,
	})
}

// KNNSearchRequest KNN 向量搜索请求
type KNNSearchRequest struct {
	Vector     []float32              `json:"vector" binding:"required"`
	Field      string                 `json:"field"`             // 向量字段名：content_vector, image_vector
	K          int                    `json:"k"`                 // 返回结果数量
	Filters    map[string]interface{} `json:"filters,omitempty"` // 过滤条件
}

// KNNSearchResponse KNN 向量搜索响应
type KNNSearchResponse struct {
	Success bool        `json:"success"`
	Total   int         `json:"total"`
	Took    int         `json:"took_ms"`
	Hits    []VectorHit `json:"hits"`
}

// VectorHit 向量搜索结果项
type VectorHit struct {
	ID     string                 `json:"id"`
	Score  float64                `json:"score"`
	Source map[string]interface{} `json:"source"`
}

// KNNSearch KNN 向量搜索接口
func (h *SearchHandler) KNNSearch(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok || tenantID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "tenant ID is required",
		})
		return
	}

	var req KNNSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// 默认值
	if req.K <= 0 {
		req.K = 10
	}
	if req.K > 100 {
		req.K = 100
	}
	if req.Field == "" {
		req.Field = "content_vector" // 默认使用文本向量
	}

	query := &opensearch.KNNQuery{
		Vector:  req.Vector,
		Field:   req.Field,
		K:       req.K,
		Filters: req.Filters,
	}

	result, err := h.osClient.KNNSearch(c.Request.Context(), tenantID, query)
	if err != nil {
		h.logger.Error("knn search failed",
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

	c.JSON(http.StatusOK, KNNSearchResponse{
		Success: true,
		Total:   result.Total,
		Took:    result.Took,
		Hits:    hits,
	})
}

// HybridSearchRequest 混合搜索请求
type HybridSearchRequest struct {
	Query       string                 `json:"query" binding:"required"`
	Vector      []float32              `json:"vector"`
	VectorField string                 `json:"vector_field"`
	K           int                    `json:"k"`
	Filters     map[string]interface{} `json:"filters,omitempty"`
}

// HybridSearch 混合搜索接口（文本 + 向量）
func (h *SearchHandler) HybridSearch(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok || tenantID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "tenant ID is required",
		})
		return
	}

	var req HybridSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// 默认值
	if req.K <= 0 {
		req.K = 10
	}
	if req.K > 100 {
		req.K = 100
	}
	if req.VectorField == "" {
		req.VectorField = "content_vector"
	}

	query := &opensearch.HybridQuery{
		Query:       req.Query,
		Vector:      req.Vector,
		VectorField: req.VectorField,
		K:           req.K,
		Filters:     req.Filters,
	}

	result, err := h.osClient.HybridSearch(c.Request.Context(), tenantID, query)
	if err != nil {
		h.logger.Error("hybrid search failed",
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

	c.JSON(http.StatusOK, KNNSearchResponse{
		Success: true,
		Total:   result.Total,
		Took:    result.Took,
		Hits:    hits,
	})
}
