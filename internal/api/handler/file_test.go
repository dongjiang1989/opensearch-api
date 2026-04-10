package handler

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/indexer"
	"github.com/dongjiang1989/opensearch-api/internal/opensearch"
	"github.com/dongjiang1989/opensearch-api/internal/storage"
)

// MockStorageForHandler 模拟存储
type MockStorageForHandler struct {
	files map[string][]byte
}

func newMockStorageForHandler() *MockStorageForHandler {
	return &MockStorageForHandler{
		files: make(map[string][]byte),
	}
}

func (m *MockStorageForHandler) Save(ctx context.Context, tenantID, fileID string, reader io.Reader) (*storage.FileMetadata, error) {
	data, _ := io.ReadAll(reader)
	m.files[tenantID+"/"+fileID] = data

	return &storage.FileMetadata{
		ID:          fileID,
		TenantID:    tenantID,
		ContentType: "text/plain",
		FileSize:    int64(len(data)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func (m *MockStorageForHandler) Get(ctx context.Context, tenantID, fileID string) (io.ReadCloser, *storage.FileMetadata, error) {
	key := tenantID + "/" + fileID
	data, exists := m.files[key]
	if !exists {
		return nil, nil, storage.ErrFileNotFound
	}

	return io.NopCloser(bytes.NewReader(data)), &storage.FileMetadata{
		ID:        fileID,
		TenantID:  tenantID,
		FileSize:  int64(len(data)),
		CreatedAt: time.Now(),
	}, nil
}

func (m *MockStorageForHandler) Delete(ctx context.Context, tenantID, fileID string) error {
	key := tenantID + "/" + fileID
	if _, exists := m.files[key]; !exists {
		return storage.ErrFileNotFound
	}
	delete(m.files, key)
	return nil
}

func (m *MockStorageForHandler) Exists(ctx context.Context, tenantID, fileID string) (bool, error) {
	key := tenantID + "/" + fileID
	_, exists := m.files[key]
	return exists, nil
}

func (m *MockStorageForHandler) GetURL(ctx context.Context, tenantID, fileID string, expiry time.Duration) (string, error) {
	return "/files/" + tenantID + "/" + fileID, nil
}

// MockExtractorForHandler 模拟内容提取器
type MockExtractorForHandler struct {
	canHandle bool
	text      string
}

func (m *MockExtractorForHandler) CanHandle(contentType string) bool {
	return m.canHandle
}

func (m *MockExtractorForHandler) Extract(ctx context.Context, reader io.Reader, contentType string) (*storage.ExtractedContent, error) {
	return &storage.ExtractedContent{
		Text: m.text,
		Metadata: map[string]interface{}{
			"extracted": true,
		},
	}, nil
}

func TestFileHandler_New(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockStore := newMockStorageForHandler()
	mockOS := opensearch.NewMockClient()
	mockExtractor := &MockExtractorForHandler{canHandle: true, text: "test"}

	idx := indexer.NewIndexer(indexer.IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Extractor:  mockExtractor,
		Logger:     logger,
	})

	handler := NewFileHandler(idx, logger)

	assert.NotNil(t, handler)
	assert.Equal(t, logger, handler.logger)
}

func TestFileHandler_UploadFile_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockStore := newMockStorageForHandler()
	mockOS := opensearch.NewMockClient()
	mockExtractor := &MockExtractorForHandler{canHandle: true, text: "extracted content"}

	idx := indexer.NewIndexer(indexer.IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Extractor:  mockExtractor,
		Logger:     logger,
	})

	handler := NewFileHandler(idx, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.POST("/upload", handler.UploadFile)

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.pdf")
	require.NoError(t, err)
	_, err = part.Write([]byte("test content"))
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
	assert.Contains(t, w.Body.String(), `"file_id"`)
}

func TestFileHandler_UploadFile_MissingTenant(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockStore := newMockStorageForHandler()
	mockOS := opensearch.NewMockClient()

	idx := indexer.NewIndexer(indexer.IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	handler := NewFileHandler(idx, logger)

	router := gin.New()
	router.POST("/upload", handler.UploadFile)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.pdf")
	if err != nil {
		t.Fatal(err)
	}
	_, err = part.Write([]byte("test"))
	if err != nil {
		t.Fatal(err)
	}
	writer.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "tenant ID is required")
}

func TestFileHandler_UploadFile_MissingFile(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockStore := newMockStorageForHandler()
	mockOS := opensearch.NewMockClient()

	idx := indexer.NewIndexer(indexer.IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	handler := NewFileHandler(idx, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.POST("/upload", handler.UploadFile)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/upload", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "file is required")
}

func TestFileHandler_GetFile_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockStore := newMockStorageForHandler()
	mockOS := opensearch.NewMockClient()

	idx := indexer.NewIndexer(indexer.IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	// First save a file
	ctx := context.Background()
	_, _ = mockStore.Save(ctx, "test-tenant", "file-123", strings.NewReader("file content"))

	handler := NewFileHandler(idx, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.GET("/files/:file_id", handler.GetFile)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/files/file-123", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "file content")
}

func TestFileHandler_GetFile_MissingTenant(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockStore := newMockStorageForHandler()
	mockOS := opensearch.NewMockClient()

	idx := indexer.NewIndexer(indexer.IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	handler := NewFileHandler(idx, logger)

	router := gin.New()
	router.GET("/files/:file_id", handler.GetFile)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/files/file-123", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "tenant ID is required")
}

func TestFileHandler_GetFile_NotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockStore := newMockStorageForHandler()
	mockOS := opensearch.NewMockClient()

	idx := indexer.NewIndexer(indexer.IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	handler := NewFileHandler(idx, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.GET("/files/:file_id", handler.GetFile)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/files/non-existent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "file not found")
}

func TestFileHandler_GetFileMetadata_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockStore := newMockStorageForHandler()
	mockOS := opensearch.NewMockClient()

	// Add a document to the mock
	_ = mockOS.IndexDocument(context.Background(), "test-tenant", "file-123", map[string]interface{}{
		"filename":     "test.pdf",
		"content_type": "application/pdf",
		"file_size":    1024,
	})

	idx := indexer.NewIndexer(indexer.IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	handler := NewFileHandler(idx, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.GET("/files/:file_id/metadata", handler.GetFileMetadata)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/files/file-123/metadata", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
}

func TestFileHandler_GetFileMetadata_MissingTenant(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockStore := newMockStorageForHandler()
	mockOS := opensearch.NewMockClient()

	idx := indexer.NewIndexer(indexer.IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	handler := NewFileHandler(idx, logger)

	router := gin.New()
	router.GET("/files/:file_id/metadata", handler.GetFileMetadata)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/files/file-123/metadata", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "tenant ID is required")
}

func TestFileHandler_DeleteFile_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockStore := newMockStorageForHandler()
	mockOS := opensearch.NewMockClient()

	idx := indexer.NewIndexer(indexer.IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	// First save a file
	ctx := context.Background()
	_, _ = mockStore.Save(ctx, "test-tenant", "file-123", strings.NewReader("to be deleted"))

	handler := NewFileHandler(idx, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.DELETE("/files/:file_id", handler.DeleteFile)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/files/file-123", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
}

func TestFileHandler_DeleteFile_MissingTenant(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockStore := newMockStorageForHandler()
	mockOS := opensearch.NewMockClient()

	idx := indexer.NewIndexer(indexer.IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	handler := NewFileHandler(idx, logger)

	router := gin.New()
	router.DELETE("/files/:file_id", handler.DeleteFile)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/files/file-123", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "tenant ID is required")
}

func TestFileHandler_DeleteFile_NotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockStore := newMockStorageForHandler()
	mockOS := opensearch.NewMockClient()

	idx := indexer.NewIndexer(indexer.IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	handler := NewFileHandler(idx, logger)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "test-tenant")
		c.Next()
	})
	router.DELETE("/files/:file_id", handler.DeleteFile)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/files/non-existent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "file not found")
}

func TestFileHandler_ListFiles_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockStore := newMockStorageForHandler()
	mockOS := opensearch.NewMockClient()

	idx := indexer.NewIndexer(indexer.IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	handler := NewFileHandler(idx, logger)

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
}

func TestFileHandler_ListFiles_MissingTenant(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockStore := newMockStorageForHandler()
	mockOS := opensearch.NewMockClient()

	idx := indexer.NewIndexer(indexer.IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	handler := NewFileHandler(idx, logger)

	router := gin.New()
	router.GET("/files", handler.ListFiles)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/files", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "tenant ID is required")
}
