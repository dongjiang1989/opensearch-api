package indexer

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/opensearch"
	"github.com/dongjiang1989/opensearch-api/internal/storage"
)

// MockStorage 模拟存储
type MockStorage struct {
	files map[string][]byte
}

func newMockStorage() *MockStorage {
	return &MockStorage{
		files: make(map[string][]byte),
	}
}

func (m *MockStorage) Save(ctx context.Context, tenantID, fileID string, reader io.Reader) (*storage.FileMetadata, error) {
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

func (m *MockStorage) Get(ctx context.Context, tenantID, fileID string) (io.ReadCloser, *storage.FileMetadata, error) {
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

func (m *MockStorage) Delete(ctx context.Context, tenantID, fileID string) error {
	key := tenantID + "/" + fileID
	if _, exists := m.files[key]; !exists {
		return storage.ErrFileNotFound
	}
	delete(m.files, key)
	return nil
}

func (m *MockStorage) Exists(ctx context.Context, tenantID, fileID string) (bool, error) {
	key := tenantID + "/" + fileID
	_, exists := m.files[key]
	return exists, nil
}

func (m *MockStorage) GetURL(ctx context.Context, tenantID, fileID string, expiry time.Duration) (string, error) {
	return "/files/" + tenantID + "/" + fileID, nil
}

// MockExtractor 模拟内容提取器
type MockExtractor struct {
	canHandle bool
	text      string
}

func (m *MockExtractor) CanHandle(contentType string) bool {
	return m.canHandle
}

func (m *MockExtractor) Extract(ctx context.Context, reader io.Reader, contentType string) (*storage.ExtractedContent, error) {
	return &storage.ExtractedContent{
		Text: m.text,
		Metadata: map[string]interface{}{
			"extracted": true,
		},
	}, nil
}

func TestNewIndexer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockOS := &opensearch.MockClient{}
	mockStore := newMockStorage()

	indexer := NewIndexer(IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	assert.NotNil(t, indexer)
	assert.Equal(t, mockOS, indexer.osClient)
	assert.Equal(t, mockStore, indexer.storage)
}

func TestIndexer_IndexFile(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockOS := opensearch.NewMockClient()
	mockStore := newMockStorage()
	mockExtractor := &MockExtractor{canHandle: true, text: "extracted content"}

	indexer := NewIndexer(IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Extractor:  mockExtractor,
		Logger:     logger,
	})

	ctx := context.Background()

	t.Run("Index text file successfully", func(t *testing.T) {
		tenantID := "test-tenant"
		filename := "test.txt"
		content := "This is test content"

		result, err := indexer.IndexFile(ctx, tenantID, filename, strings.NewReader(content))
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, tenantID, result.TenantID)
		assert.Equal(t, filename, result.Filename)
		assert.NotEmpty(t, result.FileID)
		assert.False(t, result.IndexedAt.IsZero())
	})

	t.Run("Index without extractor", func(t *testing.T) {
		indexerNoExtract := NewIndexer(IndexerConfig{
			OpenSearch: mockOS,
			Storage:    mockStore,
			Extractor:  nil,
			Logger:     logger,
		})

		tenantID := "test-tenant"
		filename := "test.bin"
		content := "binary content"

		result, err := indexerNoExtract.IndexFile(ctx, tenantID, filename, strings.NewReader(content))
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, tenantID, result.TenantID)
	})
}

func TestIndexer_GetFile(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockOS := opensearch.NewMockClient()
	mockStore := newMockStorage()

	indexer := NewIndexer(IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	ctx := context.Background()

	t.Run("Get existing file", func(t *testing.T) {
		// First save a file
		tenantID := "test-tenant"
		fileID := "get-test-123"
		content := "test content"
		_, _ = indexer.storage.Save(ctx, tenantID, fileID, strings.NewReader(content))

		// Then get it
		reader, metadata, err := indexer.GetFile(ctx, tenantID, fileID)
		require.NoError(t, err)
		assert.NotNil(t, reader)
		assert.NotNil(t, metadata)
		assert.Equal(t, fileID, metadata.ID)
	})

	t.Run("Get non-existent file", func(t *testing.T) {
		_, _, err := indexer.GetFile(ctx, "non-existent", "file123")
		assert.Error(t, err)
	})
}

func TestIndexer_DeleteFile(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockOS := opensearch.NewMockClient()
	mockStore := newMockStorage()

	indexer := NewIndexer(IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	ctx := context.Background()

	t.Run("Delete existing file", func(t *testing.T) {
		tenantID := "test-tenant"
		fileID := "delete-test-456"
		content := "to be deleted"

		// Save first
		_, _ = indexer.storage.Save(ctx, tenantID, fileID, strings.NewReader(content))

		// Delete from indexer
		err := indexer.DeleteFile(ctx, tenantID, fileID)
		assert.NoError(t, err)
	})

	t.Run("Delete non-existent file", func(t *testing.T) {
		err := indexer.DeleteFile(ctx, "non-existent", "file123")
		assert.Error(t, err)
	})
}

func TestIndexer_GetFileMetadata(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockOS := opensearch.NewMockClient()
	mockStore := newMockStorage()

	indexer := NewIndexer(IndexerConfig{
		OpenSearch: mockOS,
		Storage:    mockStore,
		Logger:     logger,
	})

	ctx := context.Background()

	t.Run("Get metadata for indexed file", func(t *testing.T) {
		tenantID := "test-tenant"
		fileID := "meta-test-789"
		content := "metadata test"

		// Index a file first
		_, _ = indexer.storage.Save(ctx, tenantID, fileID, strings.NewReader(content))

		// Simulate metadata in OpenSearch (mock)
		// In real scenario, this would be in OpenSearch
		doc := map[string]interface{}{
			"filename":     "test.txt",
			"content":      content,
			"content_type": "text/plain",
			"file_type":    "text",
			"file_size":    len(content),
		}
		_ = mockOS.IndexDocument(ctx, tenantID, fileID, doc)

		metadata, err := indexer.GetFileMetadata(ctx, tenantID, fileID)
		assert.NoError(t, err)
		assert.NotNil(t, metadata)
	})
}

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		filename string
		expected string
	}{
		{"PDF file", []byte("%PDF-1.4"), "document.pdf", "application/pdf"},
		{"PNG file", []byte("\x89PNG\r\n\x1a\n"), "image.png", "image/png"},
		{"JPEG file", []byte("\xFF\xD8\xFF\xE0"), "photo.jpg", "image/jpeg"},
		{"GIF file", []byte("GIF87a"), "anim.gif", "image/gif"},
		{"Text file", []byte("Hello World"), "readme.txt", "text/plain"},
		{"Unknown", []byte{0x00, 0x01, 0x02}, "unknown.bin", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectContentType(tt.data, tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBytesReader(t *testing.T) {
	content := []byte("test content")
	reader := newBytesReader(content)

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}
