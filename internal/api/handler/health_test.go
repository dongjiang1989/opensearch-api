package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/opensearch"
)

func TestHealthHandler_New(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	handler := NewHealthHandler(mockClient, logger)

	assert.NotNil(t, handler)
	assert.Equal(t, logger, handler.logger)
}

func TestHealthHandler_Check_Healthy(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	handler := NewHealthHandler(mockClient, logger)

	router := gin.New()
	router.GET("/health", handler.Check)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
	assert.Contains(t, w.Body.String(), `"status":"healthy"`)
	assert.Contains(t, w.Body.String(), `"opensearch"`)
}

func TestHealthHandler_Check_Degraded(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := &failingHealthClient{
		MockClient: opensearch.NewMockClient(),
	}

	handler := NewHealthHandler(mockClient, logger)

	router := gin.New()
	router.GET("/health", handler.Check)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"degraded"`)
	assert.Contains(t, w.Body.String(), `"success":false`)
}

func TestHealthHandler_Ping_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	handler := NewHealthHandler(mockClient, logger)

	router := gin.New()
	router.GET("/ping", handler.Ping)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
	assert.Contains(t, w.Body.String(), `"message":"pong"`)
}

func TestHealthHandler_Ping_Failure(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := &failingHealthClient{
		MockClient: opensearch.NewMockClient(),
	}

	handler := NewHealthHandler(mockClient, logger)

	router := gin.New()
	router.GET("/ping", handler.Ping)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), `"success":false`)
}

func TestParsePagination(t *testing.T) {
	t.Run("Default values", func(t *testing.T) {
		router := gin.New()
		router.GET("/test", func(c *gin.Context) {
			page, size := ParsePagination(c)
			assert.Equal(t, 1, page)
			assert.Equal(t, 20, size)
			c.JSON(http.StatusOK, gin.H{"done": true})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Custom values", func(t *testing.T) {
		router := gin.New()
		router.GET("/test", func(c *gin.Context) {
			page, size := ParsePagination(c)
			assert.Equal(t, 5, page)
			assert.Equal(t, 50, size)
			c.JSON(http.StatusOK, gin.H{"done": true})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test?page=5&size=50", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Invalid values", func(t *testing.T) {
		router := gin.New()
		router.GET("/test", func(c *gin.Context) {
			page, size := ParsePagination(c)
			assert.Equal(t, 1, page)
			assert.Equal(t, 20, size)
			c.JSON(http.StatusOK, gin.H{"done": true})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test?page=invalid&size=invalid", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Size limit", func(t *testing.T) {
		router := gin.New()
		router.GET("/test", func(c *gin.Context) {
			page, size := ParsePagination(c)
			assert.Equal(t, 1, page)
			assert.Equal(t, 100, size)
			c.JSON(http.StatusOK, gin.H{"done": true})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test?page=1&size=200", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandleError(t *testing.T) {
	t.Run("Nil error", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)

		HandleError(ctx, nil, "default message")
		// When error is nil, HandleError returns without writing response
		// The recorder body should be empty
		assert.Empty(t, w.Body.String())
	})

	t.Run("With error", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)

		HandleError(ctx, assert.AnError, "default message")
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), assert.AnError.Error())
	})
}

// failingHealthClient wraps MockClient to simulate health check failures
type failingHealthClient struct {
	*opensearch.MockClient
}

func (f *failingHealthClient) Health(ctx context.Context) (map[string]interface{}, error) {
	return nil, assert.AnError
}

func (f *failingHealthClient) Ping(ctx context.Context) error {
	return assert.AnError
}
