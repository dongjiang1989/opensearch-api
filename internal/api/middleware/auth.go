package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/tenant"
)

var (
	ErrMissingToken  = errors.New("missing authorization token")
	ErrInvalidToken  = errors.New("invalid authorization token")
	ErrExpiredToken  = errors.New("token has expired")
	ErrMissingTenant = errors.New("tenant ID not provided")
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
		c.Set("user_id", claims.UserID)

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
func GenerateToken(secret, issuer, tenantID, userID, role string, expireTime time.Duration) (string, error) {
	now := time.Now()

	claims := &tenant.Claims{
		TenantID: tenantID,
		UserID:   userID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expireTime)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    issuer,
			Subject:   userID,
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
	logger       *zap.Logger
	headerName   string
	requireTenant bool
}

// TenantMiddlewareConfig 租户中间件配置
type TenantMiddlewareConfig struct {
	Logger        *zap.Logger
	HeaderName    string // 租户 Header 名称，默认 "X-Tenant-ID"
	RequireTenant bool   // 是否要求必须提供租户 ID
}

// NewTenantMiddleware 创建租户中间件
func NewTenantMiddleware(cfg TenantMiddlewareConfig) *TenantMiddleware {
	headerName := cfg.HeaderName
	if headerName == "" {
		headerName = "X-Tenant-ID"
	}

	return &TenantMiddleware{
		logger:        cfg.Logger,
		headerName:    headerName,
		requireTenant: cfg.RequireTenant,
	}
}

// Middleware Gin 中间件
func (m *TenantMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 优先从 JWT Claims 获取租户 ID
		claimsVal, exists := c.Get("claims")
		if exists {
			if claims, ok := claimsVal.(*tenant.Claims); ok && claims.TenantID != "" {
				c.Set("tenant_id", claims.TenantID)
				c.Set("tenant_source", "jwt")
				c.Next()
				return
			}
		}

		// 从 Header 获取租户 ID
		tenantID := c.GetHeader(m.headerName)
		if tenantID == "" {
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
			// 不要求租户 ID 时继续
			c.Next()
			return
		}

		c.Set("tenant_id", tenantID)
		c.Set("tenant_source", "header")

		m.logger.Debug("tenant resolved from header",
			zap.String("tenant_id", tenantID))

		c.Next()
	}
}

// GetTenantID 从上下文获取租户 ID
func GetTenantID(c *gin.Context) (string, bool) {
	tenantID, exists := c.Get("tenant_id")
	if !exists {
		return "", false
	}
	return tenantID.(string), true
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
