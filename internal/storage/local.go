package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// LocalStorage 本地文件存储
type LocalStorage struct {
	basePath string
	mu       sync.RWMutex
	files    map[string]*fileInfo
	logger   *zap.Logger
}

type fileInfo struct {
	metadata *FileMetadata
	path     string
}

// LocalStorageConfig 本地存储配置
type LocalStorageConfig struct {
	BasePath string
	Logger   *zap.Logger
}

// NewLocalStorage 创建本地存储
func NewLocalStorage(cfg LocalStorageConfig) (*LocalStorage, error) {
	if cfg.BasePath == "" {
		cfg.BasePath = "./data/files"
	}

	// 创建基础目录
	if err := os.MkdirAll(cfg.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &LocalStorage{
		basePath: cfg.BasePath,
		files:    make(map[string]*fileInfo),
		logger:   cfg.Logger,
	}, nil
}

// Save 保存文件
func (s *LocalStorage) Save(ctx context.Context, tenantID, fileID string, reader io.Reader) (*FileMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.makeKey(tenantID, fileID)

	// 创建存储路径
	storagePath := filepath.Join(s.basePath, "tenants", tenantID, "files", fileID[:4], fileID)
	dir := filepath.Dir(storagePath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// 创建临时文件
	tmpPath := storagePath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	// 复制内容
	written, err := io.Copy(file, reader)
	if err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to write file: %w", err)
	}
	_ = file.Close()

	// 重命名临时文件
	if err := os.Rename(tmpPath, storagePath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to finalize file: %w", err)
	}

	// 创建元数据
	now := time.Now()
	metadata := &FileMetadata{
		ID:          fileID,
		Filename:    fileID,
		FileSize:    written,
		StoragePath: storagePath,
		TenantID:    tenantID,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    make(map[string]string),
	}

	s.files[key] = &fileInfo{
		metadata: metadata,
		path:     storagePath,
	}

	s.logger.Debug("file saved locally",
		zap.String("tenant_id", tenantID),
		zap.String("file_id", fileID),
		zap.Int64("size", written))

	return metadata, nil
}

// Get 获取文件
func (s *LocalStorage) Get(ctx context.Context, tenantID, fileID string) (io.ReadCloser, *FileMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.makeKey(tenantID, fileID)

	fInfo, exists := s.files[key]
	if !exists {
		// 尝试从文件系统加载
		storagePath := filepath.Join(s.basePath, "tenants", tenantID, "files", fileID[:4], fileID)
		if _, err := os.Stat(storagePath); os.IsNotExist(err) {
			return nil, nil, ErrFileNotFound
		}

		// 懒加载
		metadata := &FileMetadata{
			ID:          fileID,
			TenantID:    tenantID,
			StoragePath: storagePath,
		}
		s.files[key] = &fileInfo{
			metadata: metadata,
			path:     storagePath,
		}
		fInfo = s.files[key]
	}

	file, err := os.Open(fInfo.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrFileNotFound
		}
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, fInfo.metadata, nil
}

// Delete 删除文件
func (s *LocalStorage) Delete(ctx context.Context, tenantID, fileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.makeKey(tenantID, fileID)

	fileInfo, exists := s.files[key]
	if !exists {
		return ErrFileNotFound
	}

	// 删除文件
	if err := os.Remove(fileInfo.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	delete(s.files, key)

	s.logger.Debug("file deleted locally",
		zap.String("tenant_id", tenantID),
		zap.String("file_id", fileID))

	return nil
}

// Exists 检查文件是否存在
func (s *LocalStorage) Exists(ctx context.Context, tenantID, fileID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.makeKey(tenantID, fileID)

	if _, exists := s.files[key]; exists {
		return true, nil
	}

	// 检查文件系统
	storagePath := filepath.Join(s.basePath, "tenants", tenantID, "files", fileID[:4], fileID)
	_, err := os.Stat(storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// GetURL 获取文件访问路径（本地存储返回本地路径）
func (s *LocalStorage) GetURL(ctx context.Context, tenantID, fileID string, expiry time.Duration) (string, error) {
	_, metadata, err := s.Get(ctx, tenantID, fileID)
	if err != nil {
		return "", err
	}

	return metadata.StoragePath, nil
}

func (s *LocalStorage) makeKey(tenantID, fileID string) string {
	return tenantID + "/" + fileID
}

// ErrFileNotFound 文件未找到错误
var ErrFileNotFound = fmt.Errorf("file not found")
