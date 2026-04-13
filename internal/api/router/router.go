package router

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/api/handler"
	"github.com/dongjiang1989/opensearch-api/internal/api/middleware"
	"github.com/dongjiang1989/opensearch-api/internal/indexer"
	"github.com/dongjiang1989/opensearch-api/internal/opensearch"
	"github.com/dongjiang1989/opensearch-api/internal/tenant"
)

// Config 路由配置
type Config struct {
	OpenSearch    *opensearch.Client
	TenantService *tenant.Service
	Indexer       *indexer.Indexer
	Logger        *zap.Logger
	Mode          string // debug, release, test
}

// Setup 设置路由
func Setup(cfg Config) *gin.Engine {
	// 设置 Gin 模式
	if cfg.Mode != "" {
		gin.SetMode(cfg.Mode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// 全局中间件
	r.Use(gin.Recovery())
	r.Use(middleware.LoggingMiddleware(cfg.Logger))
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.MetricsMiddleware())

	// Health checks (不需要认证)
	healthHandler := handler.NewHealthHandler(cfg.OpenSearch, cfg.Logger)
	r.GET("/health", healthHandler.Check)
	r.GET("/ping", healthHandler.Ping)

	// Metrics (不需要认证)
	metricsHandler := handler.NewMetricsHandler()
	r.GET("/metrics", metricsHandler.ServeHTTP)

	// API v1 组
	v1 := r.Group("/api/v1")
	{
		// 认证中间件
		authMiddleware := middleware.NewAuthMiddleware(middleware.AuthMiddlewareConfig{
			Secret:    "change-this-secret-key", // 应该从配置读取
			Issuer:    "opensearch-file-api",
			Logger:    cfg.Logger,
			SkipPaths: []string{}, // 健康检查已经在上面注册了
		})

		// 租户中间件
		tenantMiddleware := middleware.NewTenantMiddleware(middleware.TenantMiddlewareConfig{
			Logger:        cfg.Logger,
			HeaderName:    "X-Tenant-ID",
			RequireTenant: true,
		})

		// 租户管理 API（可选：这些可能需要管理员权限）
		tenantHandler := handler.NewTenantHandler(cfg.TenantService, cfg.Logger)
		admin := v1.Group("/admin")
		{
			admin.POST("/tenants", tenantHandler.CreateTenant)
			admin.GET("/tenants", tenantHandler.ListTenants)
			admin.GET("/tenants/:id", tenantHandler.GetTenant)
			admin.PUT("/tenants/:id", tenantHandler.UpdateTenant)
			admin.DELETE("/tenants/:id", tenantHandler.DeleteTenant)
			admin.DELETE("/tenants/:id/hard", tenantHandler.HardDeleteTenant)
		}

		// Token 生成 API（用于测试，生产环境应该移除或由单独的服务提供）
		v1.POST("/token", GenerateTokenHandler)

		// 需要租户认证的文件 API
		files := v1.Group("/files")
		files.Use(authMiddleware.Middleware())
		files.Use(tenantMiddleware.Middleware())
		{
			fileHandler := handler.NewFileHandler(cfg.Indexer, cfg.Logger)

			files.POST("", fileHandler.UploadFile)
			files.GET("", fileHandler.ListFiles)
			files.GET("/:file_id", fileHandler.GetFile)
			files.GET("/:file_id/metadata", fileHandler.GetFileMetadata)
			files.DELETE("/:file_id", fileHandler.DeleteFile)
		}

		// 搜索 API
		search := v1.Group("/search")
		search.Use(authMiddleware.Middleware())
		search.Use(tenantMiddleware.Middleware())
		{
			searchHandler := handler.NewSearchHandler(cfg.OpenSearch, cfg.Logger)

			search.GET("", searchHandler.SearchGET)
			search.POST("", searchHandler.Search)
			search.POST("/aggregate", searchHandler.Aggregate)
			search.GET("/count", searchHandler.Count)

			// 向量搜索 API
			search.POST("/knn", searchHandler.KNNSearch)
			search.POST("/hybrid", searchHandler.HybridSearch)
		}
	}

	return r
}

// TokenRequest Token 生成请求
type TokenRequest struct {
	TenantID string `json:"tenant_id" binding:"required"`
	UserID   string `json:"user_id" binding:"required"`
	Role     string `json:"role"`
}

// TokenResponse Token 生成响应
type TokenResponse struct {
	Success   bool   `json:"success"`
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expires_in"`
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

// GenerateTokenHandler 生成 Token（用于测试）
func GenerateTokenHandler(c *gin.Context) {
	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// 生成 Token（使用默认密钥，生产环境应该从配置读取）
	token, err := middleware.GenerateToken(
		"change-this-secret-key",
		"opensearch-file-api",
		req.TenantID,
		req.UserID,
		req.Role,
		24*time.Hour,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		Success:   true,
		Token:     token,
		ExpiresIn: int64(24 * time.Hour / time.Second),
	})
}
