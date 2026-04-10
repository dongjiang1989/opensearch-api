package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/tenant"
)

func TestAuthMiddleware_New(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	middleware := NewAuthMiddleware(AuthMiddlewareConfig{
		Secret:      "test-secret",
		Issuer:      "test-issuer",
		Logger:      logger,
		SkipPaths:   []string{"/health", "/ping"},
		TokenHeader: "Authorization",
	})

	assert.NotNil(t, middleware)
	assert.Equal(t, "test-issuer", middleware.issuer)
}

func TestAuthMiddleware_SkipPaths(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	middleware := NewAuthMiddleware(AuthMiddlewareConfig{
		Secret:      "test-secret",
		Issuer:      "test-issuer",
		Logger:      logger,
		SkipPaths:   []string{"/health", "/ping"},
		TokenHeader: "Authorization",
	})

	router := gin.New()
	router.Use(middleware.Middleware())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Test skip path
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	secret := "test-secret"

	middleware := NewAuthMiddleware(AuthMiddlewareConfig{
		Secret:      secret,
		Issuer:      "test-issuer",
		Logger:      logger,
		SkipPaths:   []string{},
		TokenHeader: "Authorization",
	})

	// Generate valid token
	claims := &tenant.Claims{
		TenantID: "test-tenant",
		UserID:   "user123",
		Role:     "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			Issuer:    "test-issuer",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	middleware := NewAuthMiddleware(AuthMiddlewareConfig{
		Secret:      "test-secret",
		Issuer:      "test-issuer",
		Logger:      logger,
		SkipPaths:   []string{},
		TokenHeader: "Authorization",
	})

	router := gin.New()
	router.Use(middleware.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	middleware := NewAuthMiddleware(AuthMiddlewareConfig{
		Secret:      "test-secret",
		Issuer:      "test-issuer",
		Logger:      logger,
		SkipPaths:   []string{},
		TokenHeader: "Authorization",
	})

	router := gin.New()
	router.Use(middleware.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	secret := "test-secret"

	middleware := NewAuthMiddleware(AuthMiddlewareConfig{
		Secret:      secret,
		Issuer:      "test-issuer",
		Logger:      logger,
		SkipPaths:   []string{},
		TokenHeader: "Authorization",
	})

	// Generate expired token
	claims := &tenant.Claims{
		TenantID: "test-tenant",
		UserID:   "user123",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			Issuer:    "test-issuer",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	router := gin.New()
	router.Use(middleware.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestTenantMiddleware_New(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	middleware := NewTenantMiddleware(TenantMiddlewareConfig{
		HeaderName:    "X-Tenant-ID",
		RequireTenant: true,
		Logger:        logger,
	})

	assert.NotNil(t, middleware)
	assert.Equal(t, "X-Tenant-ID", middleware.headerName)
	assert.True(t, middleware.requireTenant)
}

func TestTenantMiddleware_FromHeader(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	middleware := NewTenantMiddleware(TenantMiddlewareConfig{
		HeaderName:    "X-Tenant-ID",
		RequireTenant: false, // 设为 false 以便测试 header 解析
		Logger:        logger,
	})

	router := gin.New()
	router.Use(middleware.Middleware())
	router.GET("/test", func(c *gin.Context) {
		tenantID, _ := GetTenantID(c)
		c.JSON(http.StatusOK, gin.H{"tenant_id": tenantID})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Tenant-ID", "test-tenant-123")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test-tenant-123")
}

func TestTenantMiddleware_Required(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	middleware := NewTenantMiddleware(TenantMiddlewareConfig{
		HeaderName:    "X-Tenant-ID",
		RequireTenant: true,
		Logger:        logger,
	})

	router := gin.New()
	router.Use(middleware.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "tenant ID not provided")
}

func TestGenerateToken(t *testing.T) {
	secret := "test-secret"
	issuer := "test-issuer"
	tenantID := "test-tenant"
	userID := "user123"
	role := "admin"
	expiry := time.Hour

	token, err := GenerateToken(secret, issuer, tenantID, userID, role, expiry)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestGenerateToken_Validation(t *testing.T) {
	secret := "test-secret"
	issuer := "test-issuer"

	tokenString, err := GenerateToken(secret, issuer, "tenant1", "user1", "admin", time.Hour)
	require.NoError(t, err)

	// Parse and validate
	token, err := jwt.ParseWithClaims(tokenString, &tenant.Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	require.NoError(t, err)
	assert.True(t, token.Valid)

	claims, ok := token.Claims.(*tenant.Claims)
	require.True(t, ok)
	assert.Equal(t, "tenant1", claims.TenantID)
	assert.Equal(t, "user1", claims.UserID)
	assert.Equal(t, "admin", claims.Role)
	assert.Equal(t, issuer, claims.Issuer)
}

func TestGetTenantID(t *testing.T) {
	router := gin.New()

	t.Run("From context", func(t *testing.T) {
		router.GET("/test", func(c *gin.Context) {
			c.Set("tenant_id", "test-tenant")
			tenantID, exists := GetTenantID(c)
			assert.True(t, exists)
			assert.Equal(t, "test-tenant", tenantID)
			c.JSON(http.StatusOK, gin.H{"done": true})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Not set", func(t *testing.T) {
		router.GET("/test2", func(c *gin.Context) {
			tenantID, exists := GetTenantID(c)
			assert.False(t, exists)
			assert.Equal(t, "", tenantID)
			c.JSON(http.StatusOK, gin.H{"done": true})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test2", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestCORSMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(CORSMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	router.ServeHTTP(w, req)

	// Check CORS headers
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
}

func TestLoggingMiddleware(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	router := gin.New()
	router.Use(LoggingMiddleware(logger))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
