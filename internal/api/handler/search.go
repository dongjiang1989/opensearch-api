package handler

import (
	"net/http"

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
	Index     string                 `json:"index,omitempty"` // 来源索引名称（跨租户搜索时用于标识来源）
}

// Search 搜索接口
// @Summary 搜索文件（POST 高级搜索）
// @Description 使用 JSON 请求体进行全文搜索、过滤和排序。支持多租户联合搜索，X-Tenant-ID 可传入逗号分隔的多个租户 ID。使用 dfs_query_then_fetch 实现跨索引 Score 归一化。
// @Tags Search
// @Accept json
// @Produce json
// @Param request body SearchRequest true "搜索请求"
// @Success 200 {object} SearchResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Security X-Tenant-ID
// @Router /search [post]
func (h *SearchHandler) Search(c *gin.Context) {
	tenantIDs, ok := middleware.GetTenantIDs(c)
	if !ok || len(tenantIDs) == 0 {
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
		zap.Strings("tenant_ids", tenantIDs),
		zap.String("query", req.Query))

	result, err := h.osClient.Search(c.Request.Context(), tenantIDs, query)
	if err != nil {
		h.logger.Error("search failed",
			zap.Strings("tenant_ids", tenantIDs),
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
			Index:     hit.Index,
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
// @Summary 搜索文件（GET）
// @Description 使用 URL 查询参数进行搜索。支持多租户联合搜索，X-Tenant-ID 可传入逗号分隔的多个租户 ID。
// @Tags Search
// @Produce json
// @Param q query string false "搜索关键词"
// @Param file_type query string false "文件类型"
// @Param content_type query string false "MIME 类型"
// @Param from query int false "起始位置" default(0)
// @Param size query int false "返回数量" default(10)
// @Success 200 {object} SearchResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Security X-Tenant-ID
// @Router /search [get]
func (h *SearchHandler) SearchGET(c *gin.Context) {
	tenantIDs, ok := middleware.GetTenantIDs(c)
	if !ok || len(tenantIDs) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "tenant ID is required",
		})
		return
	}

	queryStr := c.Query("q")
	fileType := c.Query("file_type")
	contentType := c.Query("content_type")

	searchQuery := &opensearch.SearchQuery{
		Query: queryStr,
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
		zap.Strings("tenant_ids", tenantIDs),
		zap.String("query", queryStr))

	result, err := h.osClient.Search(c.Request.Context(), tenantIDs, searchQuery)
	if err != nil {
		h.logger.Error("search failed",
			zap.Strings("tenant_ids", tenantIDs),
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
			Index:  hit.Index,
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
	Success  bool                            `json:"success"`
	Field    string                          `json:"field"`
	Buckets  map[string]int64                `json:"buckets"`               // 合并后的聚合结果（向后兼容）
	ByTenant map[string]map[string]int64     `json:"by_tenant,omitempty"` // 每租户维度的聚合结果（tenantID -> field_value -> count）
}

// Aggregate 聚合接口
// @Summary 聚合查询
// @Description 按指定字段聚合统计。单租户时返回合并结果，多租户时额外返回每租户维度的细分数据（by_tenant）。
// @Tags Search
// @Accept json
// @Produce json
// @Param request body AggregateRequest true "聚合请求"
// @Success 200 {object} AggregateResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Security X-Tenant-ID
// @Router /search/aggregate [post]
func (h *SearchHandler) Aggregate(c *gin.Context) {
	tenantIDs, ok := middleware.GetTenantIDs(c)
	if !ok || len(tenantIDs) == 0 {
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

	result, err := h.osClient.Aggregate(c.Request.Context(), tenantIDs, req.Field)
	if err != nil {
		h.logger.Error("aggregation failed",
			zap.Strings("tenant_ids", tenantIDs),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, AggregateResponse{
		Success:  true,
		Field:    result.Field,
		Buckets:  result.Buckets,
		ByTenant: result.ByTenant,
	})
}

// CountResponse 计数响应
type CountResponse struct {
	Success bool  `json:"success"`
	Count   int64 `json:"count"`
}

// Count 统计文件数量
// @Summary 统计文件数量
// @Description 统计当前租户下的文件总数。支持多租户联合计数。
// @Tags Search
// @Produce json
// @Success 200 {object} CountResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Security X-Tenant-ID
// @Router /search/count [get]
func (h *SearchHandler) Count(c *gin.Context) {
	tenantIDs, ok := middleware.GetTenantIDs(c)
	if !ok || len(tenantIDs) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "tenant ID is required",
		})
		return
	}

	count, err := h.osClient.Count(c.Request.Context(), tenantIDs)
	if err != nil {
		h.logger.Error("count failed",
			zap.Strings("tenant_ids", tenantIDs),
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
	Index  string                 `json:"index,omitempty"` // 来源索引名称（跨租户搜索时用于标识来源）
}

// KNNSearch KNN 向量搜索接口
// @Summary KNN 向量搜索
// @Description 使用向量进行 K 近邻搜索，支持 content_vector (1536维) 和 image_vector (512维)。支持多租户联合搜索。
// @Tags Vector Search
// @Accept json
// @Produce json
// @Param request body KNNSearchRequest true "KNN 搜索请求"
// @Success 200 {object} KNNSearchResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Security X-Tenant-ID
// @Router /search/knn [post]
func (h *SearchHandler) KNNSearch(c *gin.Context) {
	tenantIDs, ok := middleware.GetTenantIDs(c)
	if !ok || len(tenantIDs) == 0 {
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

	result, err := h.osClient.KNNSearch(c.Request.Context(), tenantIDs, query)
	if err != nil {
		h.logger.Error("knn search failed",
			zap.Strings("tenant_ids", tenantIDs),
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
			Index:  hit.Index,
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
// @Summary 混合搜索（文本 + 向量）
// @Description 结合文本关键词和向量进行混合搜索，同时匹配文本相关性和向量相似性。支持多租户联合搜索。
// @Tags Vector Search
// @Accept json
// @Produce json
// @Param request body HybridSearchRequest true "混合搜索请求"
// @Success 200 {object} KNNSearchResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Security X-Tenant-ID
// @Router /search/hybrid [post]
func (h *SearchHandler) HybridSearch(c *gin.Context) {
	tenantIDs, ok := middleware.GetTenantIDs(c)
	if !ok || len(tenantIDs) == 0 {
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

	result, err := h.osClient.HybridSearch(c.Request.Context(), tenantIDs, query)
	if err != nil {
		h.logger.Error("hybrid search failed",
			zap.Strings("tenant_ids", tenantIDs),
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
			Index:  hit.Index,
		})
	}

	c.JSON(http.StatusOK, KNNSearchResponse{
		Success: true,
		Total:   result.Total,
		Took:    result.Took,
		Hits:    hits,
	})
}
