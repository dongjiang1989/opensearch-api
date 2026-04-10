package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/opensearch"
)

func TestSearchHandler_New(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	handler := NewSearchHandler(mockClient, logger)

	assert.NotNil(t, handler)
	assert.Equal(t, logger, handler.logger)
}

func TestSearchHandler_Search_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	// Add some documents
	_ = mockClient.IndexDocument(context.Background(), "tenant_test-tenant_files", "doc1", map[string]interface{}{
		"filename": "test.pdf",
		"content":  "test content",
	})
	_ = mockClient.IndexDocument(context.Background(), "tenant_test-tenant_files", "doc2", map[string]interface{}{
		"filename": "doc.pdf",
		"content":  "more content",
	})

	handler := NewSearchHandler(mockClient, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.POST("/search", handler.Search)

	body := bytes.NewBufferString(`{"query": "test", "size": 10}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/search", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
	assert.Contains(t, w.Body.String(), `"hits"`)
}

func TestSearchHandler_Search_MissingTenant(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	handler := NewSearchHandler(mockClient, logger)

	router := gin.New()
	router.POST("/search", handler.Search)

	body := bytes.NewBufferString(`{"query": "test"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/search", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "tenant ID is required")
}

func TestSearchHandler_Search_InvalidJSON(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	handler := NewSearchHandler(mockClient, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.POST("/search", handler.Search)

	body := bytes.NewBufferString(`{invalid json}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/search", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearchHandler_SearchGET_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	// Add a document
	_ = mockClient.IndexDocument(context.Background(), "tenant_test-tenant_files", "doc1", map[string]interface{}{
		"filename":  "test.pdf",
		"content":   "test content",
		"file_type": "pdf",
	})

	handler := NewSearchHandler(mockClient, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.GET("/search", handler.SearchGET)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/search?q=test&file_type=pdf&size=20", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
}

func TestSearchHandler_SearchGET_MissingTenant(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	handler := NewSearchHandler(mockClient, logger)

	router := gin.New()
	router.GET("/search", handler.SearchGET)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/search?q=test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "tenant ID is required")
}

func TestSearchHandler_Aggregate_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	// Add documents with different file_type values
	_ = mockClient.IndexDocument(context.Background(), "tenant_test-tenant_files", "doc1", map[string]interface{}{
		"filename":  "test1.pdf",
		"file_type": "pdf",
	})
	_ = mockClient.IndexDocument(context.Background(), "tenant_test-tenant_files", "doc2", map[string]interface{}{
		"filename":  "test2.docx",
		"file_type": "docx",
	})

	handler := NewSearchHandler(mockClient, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.POST("/aggregate", handler.Aggregate)

	body := bytes.NewBufferString(`{"field": "file_type"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/aggregate", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
	assert.Contains(t, w.Body.String(), `"field":"file_type"`)
	assert.Contains(t, w.Body.String(), `"buckets"`)
}

func TestSearchHandler_Aggregate_MissingTenant(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	handler := NewSearchHandler(mockClient, logger)

	router := gin.New()
	router.POST("/aggregate", handler.Aggregate)

	body := bytes.NewBufferString(`{"field": "file_type"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/aggregate", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "tenant ID is required")
}

func TestSearchHandler_Aggregate_InvalidJSON(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	handler := NewSearchHandler(mockClient, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.POST("/aggregate", handler.Aggregate)

	body := bytes.NewBufferString(`{invalid}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/aggregate", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearchHandler_Count_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	// Add some documents - IndexDocument will use IndexName internally
	_ = mockClient.IndexDocument(context.Background(), "test-tenant", "doc1", map[string]interface{}{
		"filename": "test.pdf",
	})
	_ = mockClient.IndexDocument(context.Background(), "test-tenant", "doc2", map[string]interface{}{
		"filename": "test2.pdf",
	})

	handler := NewSearchHandler(mockClient, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.GET("/count", handler.Count)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/count", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
	assert.Contains(t, w.Body.String(), `"count":2`)
}

func TestSearchHandler_Count_MissingTenant(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	handler := NewSearchHandler(mockClient, logger)

	router := gin.New()
	router.GET("/count", handler.Count)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/count", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "tenant ID is required")
}

func TestSearchHandler_ListFiles_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	// Add a document - IndexDocument will use IndexName internally
	_ = mockClient.IndexDocument(context.Background(), "test-tenant", "file1", map[string]interface{}{
		"filename":     "test.pdf",
		"content_type": "application/pdf",
		"file_type":    "pdf",
		"file_size":    1024,
		"created_at":   "2024-01-01T00:00:00Z",
	})

	handler := NewSearchHandler(mockClient, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.GET("/files", handler.ListFiles)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/files", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
	assert.Contains(t, w.Body.String(), `"files"`)
	assert.Contains(t, w.Body.String(), `"test.pdf"`)
}

func TestSearchHandler_ListFiles_MissingTenant(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := opensearch.NewMockClient()

	handler := NewSearchHandler(mockClient, logger)

	router := gin.New()
	router.GET("/files", handler.ListFiles)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/files", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "tenant ID is required")
}
