package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/opensearch"
	"github.com/dongjiang1989/opensearch-api/internal/storage"
)

// MockEmbedder 模拟嵌入服务
type MockEmbedder struct {
	vector    []float32
	dimensions int
	name      string
	generateErr error
}

func (m *MockEmbedder) Generate(ctx context.Context, content string) ([]float32, error) {
	if m.generateErr != nil {
		return nil, m.generateErr
	}
	return m.vector, nil
}

func (m *MockEmbedder) Dimensions() int {
	return m.dimensions
}

func (m *MockEmbedder) Name() string {
	return m.name
}

// MockExtractorForRetrieve 模拟内容提取器
type MockExtractorForRetrieve struct {
	text      string
	canHandle bool
}

func (m *MockExtractorForRetrieve) CanHandle(contentType string) bool {
	return m.canHandle
}

func (m *MockExtractorForRetrieve) Extract(ctx context.Context, reader io.Reader, contentType string) (*storage.ExtractedContent, error) {
	return &storage.ExtractedContent{
		Text:     m.text,
		Metadata: map[string]interface{}{"extracted": true},
	}, nil
}

func TestRetrieveHandler_New(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockOS := opensearch.NewMockClient()
	mockEmbedder := &MockEmbedder{vector: []float32{0.1, 0.2}, dimensions: 2, name: "test"}

	handler := NewRetrieveHandler(mockOS, nil, mockEmbedder, logger)
	assert.NotNil(t, handler)
	assert.Equal(t, logger, handler.logger)
}

func TestRetrieveHandler_Retrieve_JSON_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockOS := opensearch.NewMockClient()
	mockEmbedder := &MockEmbedder{
		vector:     []float32{0.1, 0.2, 0.3},
		dimensions: 3,
		name:       "test-model",
	}

	// Index a test document
	_ = mockOS.IndexDocument(context.Background(), "test-tenant", "doc-1", map[string]interface{}{
		"filename":     "test.txt",
		"content":      "machine learning test",
		"content_type": "text/plain",
		"file_type":    "text",
	})

	handler := NewRetrieveHandler(mockOS, nil, mockEmbedder, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.POST("/retrieve", handler.Retrieve)

	body, _ := json.Marshal(RetrieveRequest{
		Query: "machine learning",
		K:     10,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/retrieve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp RetrieveResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

func TestRetrieveHandler_Retrieve_JSON_MissingQuery(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockOS := opensearch.NewMockClient()
	mockEmbedder := &MockEmbedder{vector: []float32{0.1}}

	handler := NewRetrieveHandler(mockOS, nil, mockEmbedder, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.POST("/retrieve", handler.Retrieve)

	body, _ := json.Marshal(map[string]interface{}{"k": 10})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/retrieve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRetrieveHandler_Retrieve_JSON_MissingTenant(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockOS := opensearch.NewMockClient()
	mockEmbedder := &MockEmbedder{vector: []float32{0.1}}

	handler := NewRetrieveHandler(mockOS, nil, mockEmbedder, logger)

	router := gin.New()
	router.POST("/retrieve", handler.Retrieve)

	body, _ := json.Marshal(RetrieveRequest{Query: "test"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/retrieve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "tenant ID is required")
}

func TestRetrieveHandler_Retrieve_EmbedderNotConfigured(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockOS := opensearch.NewMockClient()

	handler := NewRetrieveHandler(mockOS, nil, nil, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.POST("/retrieve", handler.Retrieve)

	body, _ := json.Marshal(RetrieveRequest{Query: "test"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/retrieve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "embedding service is not configured")
}

func TestRetrieveHandler_Retrieve_Multipart_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockOS := opensearch.NewMockClient()
	mockEmbedder := &MockEmbedder{
		vector:     []float32{0.1, 0.2, 0.3},
		dimensions: 3,
		name:       "test-model",
	}
	mockExtract := &MockExtractorForRetrieve{
		text:      "extracted file content about machine learning",
		canHandle: true,
	}

	// Index a test document
	_ = mockOS.IndexDocument(context.Background(), "test-tenant", "doc-1", map[string]interface{}{
		"filename":     "test.txt",
		"content":      "machine learning test",
		"content_type": "text/plain",
	})

	handler := NewRetrieveHandler(mockOS, mockExtract, mockEmbedder, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.POST("/retrieve", handler.Retrieve)

	// Create multipart form
	bodyBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuf)
	part, err := writer.CreateFormFile("file", "test.txt")
	require.NoError(t, err)
	_, err = part.Write([]byte("test content"))
	require.NoError(t, err)
	_ = writer.WriteField("query", "additional search term")
	_ = writer.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/retrieve", bodyBuf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp RetrieveResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

func TestRetrieveHandler_Retrieve_Multipart_MissingFile(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockOS := opensearch.NewMockClient()
	mockEmbedder := &MockEmbedder{vector: []float32{0.1}}
	mockExtract := &MockExtractorForRetrieve{canHandle: true}

	handler := NewRetrieveHandler(mockOS, mockExtract, mockEmbedder, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.POST("/retrieve", handler.Retrieve)

	bodyBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuf)
	_ = writer.WriteField("query", "test")
	_ = writer.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/retrieve", bodyBuf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "file is required")
}

func TestRetrieveHandler_Retrieve_InvalidContentType(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockOS := opensearch.NewMockClient()
	mockEmbedder := &MockEmbedder{vector: []float32{0.1}}

	handler := NewRetrieveHandler(mockOS, nil, mockEmbedder, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.POST("/retrieve", handler.Retrieve)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/retrieve", strings.NewReader("plain text"))
	req.Header.Set("Content-Type", "text/plain")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Content-Type must be")
}

func TestRetrieveHandler_Retrieve_EmbedderError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockOS := opensearch.NewMockClient()
	mockEmbedder := &MockEmbedder{
		generateErr: assert.AnError,
	}

	handler := NewRetrieveHandler(mockOS, nil, mockEmbedder, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.POST("/retrieve", handler.Retrieve)

	body, _ := json.Marshal(RetrieveRequest{Query: "test"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/retrieve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to generate embedding")
}

func TestDetectContentTypeFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"doc.pdf", "application/pdf"},
		{"readme.txt", "text/plain"},
		{"notes.md", "text/plain"},
		{"page.html", "text/html"},
		{"data.json", "application/json"},
		{"report.csv", "text/csv"},
		{"unknown.bin", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := detectContentTypeFromFilename(tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}
