package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"
)

// FileType 文件类型枚举
type FileType string

const (
	FileTypePDF      FileType = "pdf"
	FileTypeImage    FileType = "image"
	FileTypeDocument FileType = "document"
	FileTypeText     FileType = "text"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeOther    FileType = "other"
)

// FileMetadata 文件元数据
type FileMetadata struct {
	ID          string            `json:"id"`
	Filename    string            `json:"filename"`
	ContentType string            `json:"content_type"`
	FileType    FileType          `json:"file_type"`
	FileSize    int64             `json:"file_size"`
	StoragePath string            `json:"storage_path"`
	TenantID    string            `json:"tenant_id"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ExtractedContent 提取的文件内容
type ExtractedContent struct {
	Text     string                 `json:"text"`
	Metadata map[string]interface{} `json:"metadata"`
}

// Storage 存储接口
type Storage interface {
	// Save 保存文件
	Save(ctx context.Context, tenantID, fileID string, reader io.Reader) (*FileMetadata, error)
	// Get 获取文件
	Get(ctx context.Context, tenantID, fileID string) (io.ReadCloser, *FileMetadata, error)
	// Delete 删除文件
	Delete(ctx context.Context, tenantID, fileID string) error
	// Exists 检查文件是否存在
	Exists(ctx context.Context, tenantID, fileID string) (bool, error)
	// GetURL 获取文件访问 URL（对于 S3 返回预签名 URL）
	GetURL(ctx context.Context, tenantID, fileID string, expiry time.Duration) (string, error)
}

// ContentExtractor 内容提取器接口
type ContentExtractor interface {
	// Extract 提取文件内容
	Extract(ctx context.Context, reader io.Reader, contentType string) (*ExtractedContent, error)
	// CanHandle 是否能处理该文件类型
	CanHandle(contentType string) bool
}

// GetFileType 根据内容类型获取文件类型
func GetFileType(contentType string) FileType {
	switch {
	case contentType == "application/pdf":
		return FileTypePDF
	case contentType == "application/msword",
		contentType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		contentType == "application/vnd.ms-excel",
		contentType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		contentType == "application/vnd.ms-powerpoint",
		contentType == "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return FileTypeDocument
	case contentType == "image/jpeg",
		contentType == "image/png",
		contentType == "image/gif",
		contentType == "image/webp",
		contentType == "image/svg+xml":
		return FileTypeImage
	case contentType == "video/mp4",
		contentType == "video/x-msvideo",
		contentType == "video/quicktime":
		return FileTypeVideo
	case contentType == "audio/mpeg",
		contentType == "audio/wav",
		contentType == "audio/ogg":
		return FileTypeAudio
	case contentType == "text/plain",
		contentType == "text/markdown",
		contentType == "text/html",
		contentType == "application/json":
		return FileTypeText
	default:
		return FileTypeOther
	}
}

// GetFileExtension 获取文件扩展名
func GetFileExtension(filename string) string {
	return filepath.Ext(filename)
}

// SanitizeFilename 清理文件名
func SanitizeFilename(filename string) string {
	// 简单的实现，可以扩展为更安全的方式
	clean := filepath.Base(filename)
	if len(clean) > 255 {
		ext := filepath.Ext(clean)
		name := clean[:len(clean)-len(ext)]
		if len(name) > 200 {
			name = name[:200]
		}
		clean = name + ext
	}
	return clean
}

// GenerateStoragePath 生成存储路径
func GenerateStoragePath(tenantID, fileID, filename string) string {
	ext := filepath.Ext(filename)
	return fmt.Sprintf("tenants/%s/files/%s/%s%s", tenantID, fileID[:4], fileID, ext)
}
