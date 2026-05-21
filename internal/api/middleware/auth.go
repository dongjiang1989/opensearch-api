package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/tenant"
)

var (
	ErrMissingToken       = errors.New("missing authorization token")
	ErrInvalidToken       = errors.New("invalid authorization token")
	ErrExpiredToken       = errors.New("token has expired")
	ErrMissingTenant      = errors.New("tenant ID not provided")
	ErrTenantMismatch     = errors.New("first tenant ID must match authenticated tenant")
)

// AuthMiddleware JWT 认证中间件
type AuthMiddleware struct {
	secret       []byte
	issuer       string
	logger       *zap.Logger
	skipPaths    map[string]bool
	tokenHeader  string
}

// AuthMiddlewareConfig 认证中间件配置
type AuthMiddlewareConfig struct {
	Secret      string
	Issuer      string
	Logger      *zap.Logger
	SkipPaths   []string // 不需要认证的路径
	TokenHeader string   // Token header 名称，默认 "Authorization"
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(cfg AuthMiddlewareConfig) *AuthMiddleware {
	skipPaths := make(map[string]bool)
	for _, path := range cfg.SkipPaths {
		skipPaths[path] = true
	}

	tokenHeader := cfg.TokenHeader
	if tokenHeader == "" {
		tokenHeader = "Authorization"
	}

	return &AuthMiddleware{
		secret:      []byte(cfg.Secret),
		issuer:      cfg.Issuer,
		logger:      cfg.Logger,
		skipPaths:   skipPaths,
		tokenHeader: tokenHeader,
	}
}

// Middleware Gin 中间件
func (m *AuthMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否需要跳过认证
		if m.skipPaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		// 从 Header 获取 Token
		tokenString := m.extractToken(c)
		if tokenString == "" {
			m.abortWithError(c, http.StatusUnauthorized, ErrMissingToken)
			return
		}

		// 验证 Token
		claims, err := m.validateToken(tokenString)
		if err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, jwt.ErrTokenExpired) {
				m.abortWithError(c, status, ErrExpiredToken)
			} else {
				m.abortWithError(c, status, ErrInvalidToken)
			}
			return
		}

		// 将 Claims 添加到上下文
		c.Set("claims", claims)
		c.Set("tenant_id", claims.TenantID)

		c.Next()
	}
}

// extractToken 从请求中提取 Token
func (m *AuthMiddleware) extractToken(c *gin.Context) string {
	authHeader := c.GetHeader(m.tokenHeader)
	if authHeader == "" {
		return ""
	}

	// 支持 "Bearer <token>" 格式
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
		return parts[1]
	}

	// 也支持直接传入 token
	return authHeader
}

// validateToken 验证 Token
func (m *AuthMiddleware) validateToken(tokenString string) (*tenant.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &tenant.Claims{}, func(token *jwt.Token) (interface{}, error) {
		return m.secret, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*tenant.Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	// 验证 Issuer
	if m.issuer != "" && claims.Issuer != m.issuer {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}

func (m *AuthMiddleware) abortWithError(c *gin.Context, status int, err error) {
	m.logger.Debug("authentication failed",
		zap.String("path", c.Request.URL.Path),
		zap.String("error", err.Error()))

	c.JSON(status, gin.H{
		"error":   err.Error(),
		"code":    status,
		"success": false,
	})
	c.Abort()
}

// GenerateToken 生成 JWT Token
func GenerateToken(secret, issuer, tenantID, role string, expireTime time.Duration) (string, error) {
	now := time.Now()

	claims := &tenant.Claims{
		TenantID: tenantID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expireTime)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    issuer,
			ID:        generateTokenID(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// generateTokenID 生成 Token ID
func generateTokenID() string {
	// 简化实现，生产环境应使用 uuid
	return time.Now().Format("20060102150405")
}

// TenantMiddleware 租户中间件
type TenantMiddleware struct {
	logger        *zap.Logger
	headerName    string
	requireTenant bool
	maxTenants    int
}

// TenantMiddlewareConfig 租户中间件配置
type TenantMiddlewareConfig struct {
	Logger        *zap.Logger
	HeaderName    string // 租户 Header 名称，默认 "X-Tenant-ID"
	RequireTenant bool   // 是否要求必须提供租户 ID
	MaxTenants    int    // 最多允许的租户数量，0 表示不限制（建议设为 10）
}

// NewTenantMiddleware 创建租户中间件
func NewTenantMiddleware(cfg TenantMiddlewareConfig) *TenantMiddleware {
	headerName := cfg.HeaderName
	if headerName == "" {
		headerName = "X-Tenant-ID"
	}

	maxTenants := cfg.MaxTenants
	if maxTenants <= 0 {
		maxTenants = 10
	}

	return &TenantMiddleware{
		logger:        cfg.Logger,
		headerName:    headerName,
		requireTenant: cfg.RequireTenant,
		maxTenants:    maxTenants,
	}
}

// Middleware Gin 中间件
func (m *TenantMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 从 JWT Claims 获取本租户 ID
		jwtTenantID := ""
		if claimsVal, exists := c.Get("claims"); exists {
			if claims, ok := claimsVal.(*tenant.Claims); ok && claims.TenantID != "" {
				jwtTenantID = claims.TenantID
			}
		}

		// 2. 从 Header 获取租户 ID（支持逗号分隔）
		rawValue := c.GetHeader(m.headerName)
		headerIDs := parseTenantIDs(rawValue)

		// 3. 合并租户 ID 列表
		var tenantIDs []string
		source := "header"

		if jwtTenantID != "" && len(headerIDs) > 0 {
			// JWT 和 Header 同时存在：Header 必须以 JWT 租户开头
			if headerIDs[0] != jwtTenantID {
				m.logger.Debug("first tenant ID in header must match JWT tenant ID",
					zap.String("jwt_tenant", jwtTenantID),
					zap.String("header_first", headerIDs[0]))

				c.JSON(http.StatusForbidden, gin.H{
					"error":   "first tenant ID in X-Tenant-ID must match your authenticated tenant",
					"code":    http.StatusForbidden,
					"success": false,
				})
				c.Abort()
				return
			}
			tenantIDs = headerIDs
			source = "jwt+header"
		} else if jwtTenantID != "" {
			// 仅有 JWT
			tenantIDs = []string{jwtTenantID}
			source = "jwt"
		} else {
			// 仅有 Header
			tenantIDs = headerIDs
			source = "header"
		}

		// 去重（保持顺序）
		tenantIDs = deduplicateIDs(tenantIDs)

		// 4. 校验租户 ID 是否存在
		if len(tenantIDs) == 0 {
			if m.requireTenant {
				m.logger.Debug("tenant ID required but not provided",
					zap.String("path", c.Request.URL.Path))

				c.JSON(http.StatusBadRequest, gin.H{
					"error":   ErrMissingTenant.Error(),
					"code":    http.StatusBadRequest,
					"success": false,
				})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		// 5. 检查租户数量上限
		if m.maxTenants > 0 && len(tenantIDs) > m.maxTenants {
			m.logger.Debug("too many tenant IDs",
				zap.Int("count", len(tenantIDs)),
				zap.Int("max", m.maxTenants))

			c.JSON(http.StatusBadRequest, gin.H{
				"error":   fmt.Sprintf("too many tenant IDs: %d (max %d)", len(tenantIDs), m.maxTenants),
				"code":    http.StatusBadRequest,
				"success": false,
			})
			c.Abort()
			return
		}

		c.Set("tenant_ids", tenantIDs)
		c.Set("tenant_id", tenantIDs[0])
		c.Set("tenant_source", source)

		m.logger.Debug("tenant resolved",
			zap.Strings("tenant_ids", tenantIDs),
			zap.String("source", source))

		c.Next()
	}
}

// deduplicateIDs 去重租户 ID 列表（保持顺序）
func deduplicateIDs(ids []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

// GetTenantID 从上下文获取租户 ID（返回第一个，保持向后兼容）
func GetTenantID(c *gin.Context) (string, bool) {
	tenantID, exists := c.Get("tenant_id")
	if !exists {
		return "", false
	}
	return tenantID.(string), true
}

// GetTenantIDs 从上下文获取租户 ID 列表
func GetTenantIDs(c *gin.Context) ([]string, bool) {
	val, exists := c.Get("tenant_ids")
	if !exists {
		return nil, false
	}
	return val.([]string), true
}

// parseTenantIDs 解析逗号分隔的租户 ID 字符串
func parseTenantIDs(rawValue string) []string {
	parts := strings.Split(rawValue, ",")
	seen := make(map[string]bool)
	var ids []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			ids = append(ids, p)
		}
	}
	return ids
}

// GetClaims 从上下文获取 Claims
func GetClaims(c *gin.Context) (*tenant.Claims, bool) {
	claims, exists := c.Get("claims")
	if !exists {
		return nil, false
	}
	return claims.(*tenant.Claims), true
}

// LoggingMiddleware 日志中间件
func LoggingMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		logger.Info("request completed",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()))
	}
}

// CORSMiddleware CORS 中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Tenant-ID")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
