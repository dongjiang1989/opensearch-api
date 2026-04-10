package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/indexer"
)

// HealthHandler 健康检查 Handler
type HealthHandler struct {
	osClient indexer.OpenSearchClient
	logger   *zap.Logger
}

// NewHealthHandler 创建健康检查 Handler
func NewHealthHandler(osClient indexer.OpenSearchClient, logger *zap.Logger) *HealthHandler {
	return &HealthHandler{
		osClient: osClient,
		logger:   logger,
	}
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Success    bool                   `json:"success"`
	Status     string                 `json:"status"`
	Version    map[string]interface{} `json:"version,omitempty"`
	OpenSearch *OpenSearchHealth      `json:"opensearch,omitempty"`
}

// OpenSearchHealth OpenSearch 健康状态
type OpenSearchHealth struct {
	Status    string `json:"status"`
	ClusterID string `json:"cluster_id"`
	Nodes     int    `json:"nodes"`
}

// Check 健康检查接口
func (h *HealthHandler) Check(c *gin.Context) {
	response := HealthResponse{
		Success: true,
		Status:  "healthy",
	}

	// 检查 OpenSearch 连接
	health, err := h.osClient.Health(c.Request.Context())
	if err != nil {
		h.logger.Warn("opensearch health check failed", zap.Error(err))
		response.Status = "degraded"
		response.Success = false
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}

	// 提取 OpenSearch 健康信息
	osHealth := &OpenSearchHealth{}
	if status, ok := health["status"].(string); ok {
		osHealth.Status = status
	}
	if clusterID, ok := health["cluster_uuid"].(string); ok {
		osHealth.ClusterID = clusterID
	}
	if nodes, ok := health["number_of_nodes"].(float64); ok {
		osHealth.Nodes = int(nodes)
	}

	response.OpenSearch = osHealth

	// 根据 OpenSearch 状态判断整体状态
	if osHealth.Status == "red" {
		response.Status = "unhealthy"
		response.Success = false
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}

	h.logger.Debug("health check passed")
	c.JSON(http.StatusOK, response)
}

// Ping Ping 接口
func (h *HealthHandler) Ping(c *gin.Context) {
	err := h.osClient.Ping(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "pong",
	})
}

// ErrorResponse API 错误响应
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Code    int    `json:"code,omitempty"`
}

// SuccessResponse API 成功响应
type SuccessResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// PaginatedResponse 分页响应
type PaginatedResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Total   int64       `json:"total"`
	Page    int         `json:"page"`
	Size    int         `json:"size"`
}

// HandleError 统一错误处理
func HandleError(c *gin.Context, err error, defaultMessage string) {
	if err == nil {
		return
	}

	c.JSON(http.StatusInternalServerError, ErrorResponse{
		Success: true,
		Error:   err.Error(),
	})
}

// ParsePagination 解析分页参数
func ParsePagination(c *gin.Context) (page, size int) {
	page = 1
	size = 20

	if p := c.Query("page"); p != "" {
		if _, err := parseParam(p, &page); err != nil {
			page = 1
		}
	}

	if s := c.Query("size"); s != "" {
		if _, err := parseParam(s, &size); err != nil {
			size = 20
		}
		if size > 100 {
			size = 100
		}
	}

	return
}

func parseParam(s string, dst interface{}) (bool, error) {
	switch d := dst.(type) {
	case *int:
		var v int
		_, err := fmt.Sscanf(s, "%d", &v)
		if err != nil {
			return false, err
		}
		*d = v
	case *int64:
		var v int64
		_, err := fmt.Sscanf(s, "%d", &v)
		if err != nil {
			return false, err
		}
		*d = v
	}
	return true, nil
}
