package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLocalStorage_Save(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := t.TempDir()

	storage, err := NewLocalStorage(LocalStorageConfig{
		BasePath: tmpDir,
		Logger:   logger,
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Save file successfully", func(t *testing.T) {
		tenantID := "test-tenant"
		fileID := "abc123"
		content := []byte("test file content")
		reader := bytes.NewReader(content)

		metadata, err := storage.Save(ctx, tenantID, fileID, reader)
		require.NoError(t, err)
		assert.NotNil(t, metadata)
		assert.Equal(t, fileID, metadata.ID)
		assert.Equal(t, tenantID, metadata.TenantID)
		assert.Greater(t, metadata.FileSize, int64(0))
		assert.False(t, metadata.CreatedAt.IsZero())
	})

	t.Run("Save creates directory structure", func(t *testing.T) {
		tenantID := "tenant2"
		fileID := "def456"
		content := []byte("another file")
		reader := bytes.NewReader(content)

		_, err := storage.Save(ctx, tenantID, fileID, reader)
		require.NoError(t, err)

		// Verify directory was created
		expectedPath := filepath.Join(tmpDir, "tenants", tenantID, "files", fileID[:4], fileID)
		_, err = os.Stat(expectedPath)
		assert.NoError(t, err)
	})
}

func TestLocalStorage_Get(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := t.TempDir()

	storage, _ := NewLocalStorage(LocalStorageConfig{
		BasePath: tmpDir,
		Logger:   logger,
	})

	ctx := context.Background()

	t.Run("Get existing file", func(t *testing.T) {
		// First save a file
		tenantID := "test-tenant"
		fileID := "get-test-123"
		content := []byte("test content for get")
		_, err := storage.Save(ctx, tenantID, fileID, bytes.NewReader(content))
		require.NoError(t, err)

		// Then get it
		reader, metadata, err := storage.Get(ctx, tenantID, fileID)
		require.NoError(t, err)
		assert.NotNil(t, reader)
		assert.NotNil(t, metadata)
		assert.Equal(t, fileID, metadata.ID)

		// Read content
		var buf bytes.Buffer
		_, err = buf.ReadFrom(reader)
		require.NoError(t, err)
		assert.Equal(t, content, buf.Bytes())
	})

	t.Run("Get non-existent file", func(t *testing.T) {
		_, _, err := storage.Get(ctx, "non-existent", "file123")
		assert.Error(t, err)
		assert.Equal(t, ErrFileNotFound, err)
	})
}

func TestLocalStorage_Delete(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := t.TempDir()

	storage, _ := NewLocalStorage(LocalStorageConfig{
		BasePath: tmpDir,
		Logger:   logger,
	})

	ctx := context.Background()

	t.Run("Delete existing file", func(t *testing.T) {
		tenantID := "test-tenant"
		fileID := "delete-test-123"
		content := []byte("to be deleted")

		// Save first
		_, err := storage.Save(ctx, tenantID, fileID, bytes.NewReader(content))
		require.NoError(t, err)

		// Then delete
		err = storage.Delete(ctx, tenantID, fileID)
		assert.NoError(t, err)

		// Verify file is gone
		_, _, err = storage.Get(ctx, tenantID, fileID)
		assert.Equal(t, ErrFileNotFound, err)
	})

	t.Run("Delete non-existent file", func(t *testing.T) {
		err := storage.Delete(ctx, "non-existent", "file123")
		assert.Equal(t, ErrFileNotFound, err)
	})
}

func TestLocalStorage_Exists(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := t.TempDir()

	storage, _ := NewLocalStorage(LocalStorageConfig{
		BasePath: tmpDir,
		Logger:   logger,
	})

	ctx := context.Background()

	t.Run("Existing file", func(t *testing.T) {
		tenantID := "test-tenant"
		fileID := "exists-test-123"
		content := []byte("exists")

		_, err := storage.Save(ctx, tenantID, fileID, bytes.NewReader(content))
		require.NoError(t, err)

		exists, err := storage.Exists(ctx, tenantID, fileID)
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("Non-existent file", func(t *testing.T) {
		exists, err := storage.Exists(ctx, "non-existent", "file123")
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestLocalStorage_GetURL(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := t.TempDir()

	storage, _ := NewLocalStorage(LocalStorageConfig{
		BasePath: tmpDir,
		Logger:   logger,
	})

	ctx := context.Background()

	t.Run("Get URL for existing file", func(t *testing.T) {
		tenantID := "test-tenant"
		fileID := "url-test-123"
		content := []byte("file for url")

		_, err := storage.Save(ctx, tenantID, fileID, bytes.NewReader(content))
		require.NoError(t, err)

		url, err := storage.GetURL(ctx, tenantID, fileID, 0)
		assert.NoError(t, err)
		assert.NotEmpty(t, url)
		assert.Contains(t, url, tenantID)
		assert.Contains(t, url, fileID)
	})

	t.Run("Get URL for non-existent file", func(t *testing.T) {
		_, err := storage.GetURL(ctx, "non-existent", "file123", 0)
		assert.Error(t, err)
	})
}

func TestNewLocalStorage(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := t.TempDir()

	t.Run("Create with custom path", func(t *testing.T) {
		storage, err := NewLocalStorage(LocalStorageConfig{
			BasePath: tmpDir,
			Logger:   logger,
		})
		require.NoError(t, err)
		assert.NotNil(t, storage)
		assert.Equal(t, tmpDir, storage.basePath)
	})

	t.Run("Create with empty path uses default", func(t *testing.T) {
		storage, err := NewLocalStorage(LocalStorageConfig{
			Logger: logger,
		})
		require.NoError(t, err)
		assert.NotNil(t, storage)
		assert.NotEmpty(t, storage.basePath)
	})

	t.Run("Create directory on init", func(t *testing.T) {
		newDir := filepath.Join(tmpDir, "new-storage")
		storage, err := NewLocalStorage(LocalStorageConfig{
			BasePath: newDir,
			Logger:   logger,
		})
		require.NoError(t, err)
		assert.NotNil(t, storage)

		// Verify directory was created
		_, err = os.Stat(newDir)
		assert.NoError(t, err)
	})
}
