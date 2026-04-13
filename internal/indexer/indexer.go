package indexer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/embedding"
	"github.com/dongjiang1989/opensearch-api/internal/opensearch"
	"github.com/dongjiang1989/opensearch-api/internal/storage"
)

// OpenSearchClient defines the interface for OpenSearch operations
type OpenSearchClient interface {
	IndexDocument(ctx context.Context, tenantID, docID string, doc map[string]interface{}) error
	GetDocument(ctx context.Context, tenantID, docID string) (map[string]interface{}, error)
	DeleteDocument(ctx context.Context, tenantID, docID string) error
	Search(ctx context.Context, tenantID string, query *opensearch.SearchQuery) (*opensearch.SearchResult, error)
	KNNSearch(ctx context.Context, tenantID string, query *opensearch.KNNQuery) (*opensearch.SearchResult, error)
	HybridSearch(ctx context.Context, tenantID string, query *opensearch.HybridQuery) (*opensearch.SearchResult, error)
	IndexName(tenantID string) string
	Health(ctx context.Context) (map[string]interface{}, error)
	Ping(ctx context.Context) error
	Count(ctx context.Context, tenantID string) (int64, error)
	Aggregate(ctx context.Context, tenantID, fieldName string) (map[string]int64, error)
}

// Indexer 文件索引器
type Indexer struct {
	osClient    OpenSearchClient
	storage     storage.Storage
	extractor   storage.ContentExtractor
	embedder    embedding.EmbeddingModel
	clipModel   embedding.EmbeddingModel
	logger      *zap.Logger
}

// IndexerConfig 索引器配置
type IndexerConfig struct {
	OpenSearch OpenSearchClient
	Storage    storage.Storage
	Extractor  storage.ContentExtractor
	Embedder   embedding.EmbeddingModel      // 文本嵌入模型
	ClipModel  embedding.EmbeddingModel      // CLIP 多模态模型（可选）
	Logger     *zap.Logger
}

// NewIndexer 创建文件索引器
func NewIndexer(cfg IndexerConfig) *Indexer {
	return &Indexer{
		osClient:    cfg.OpenSearch,
		storage:     cfg.Storage,
		extractor:   cfg.Extractor,
		embedder:    cfg.Embedder,
		clipModel:   cfg.ClipModel,
		logger:      cfg.Logger,
	}
}

// IndexResult 索引结果
type IndexResult struct {
	FileID      string                 `json:"file_id"`
	TenantID    string                 `json:"tenant_id"`
	Filename    string                 `json:"filename"`
	ContentType string                 `json:"content_type"`
	FileSize    int64                  `json:"file_size"`
	IndexedAt   time.Time              `json:"indexed_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// IndexFile 索引文件
func (i *Indexer) IndexFile(ctx context.Context, tenantID, filename string, reader io.Reader) (*IndexResult, error) {
	// 生成文件 ID
	fileID := uuid.New().String()

	// 读取文件内容到内存（用于提取内容）
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// 检测内容类型（简化实现，可根据文件头检测）
	contentType := detectContentType(data, filename)
	fileType := storage.GetFileType(contentType)

	i.logger.Debug("indexing file",
		zap.String("tenant_id", tenantID),
		zap.String("file_id", fileID),
		zap.String("filename", filename),
		zap.String("content_type", contentType),
		zap.String("file_type", string(fileType)))

	// 保存文件
	metadata, err := i.storage.Save(ctx, tenantID, fileID, newBytesReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// 提取内容
	var extracted *storage.ExtractedContent
	if i.extractor != nil && i.extractor.CanHandle(contentType) {
		extracted, err = i.extractor.Extract(ctx, newBytesReader(data), contentType)
		if err != nil {
			i.logger.Warn("failed to extract content",
				zap.String("file_id", fileID),
				zap.Error(err))
			// 提取失败不影响索引，使用空内容
			extracted = &storage.ExtractedContent{
				Text:     "",
				Metadata: map[string]interface{}{"extraction_error": err.Error()},
			}
		}
	} else {
		extracted = &storage.ExtractedContent{
			Text: "",
			Metadata: map[string]interface{}{
				"note": "No extractor available for this content type",
			},
		}
	}

	// 生成文本嵌入向量
	if i.embedder != nil && extracted.Text != "" {
		embedding, err := i.embedder.Generate(ctx, extracted.Text)
		if err != nil {
			i.logger.Warn("failed to generate text embedding",
				zap.String("file_id", fileID),
				zap.Error(err))
		} else {
			extracted.Embedding = embedding
			i.logger.Debug("text embedding generated",
				zap.String("file_id", fileID),
				zap.Int("dimensions", len(embedding)))
		}
	}

	// 生成图片嵌入向量（使用 CLIP）
	if i.clipModel != nil && fileType == storage.FileTypeImage {
		clipEmbedding, err := i.clipModel.(*embedding.CLIPEmbedding).GenerateImage(ctx, data, contentType)
		if err != nil {
			i.logger.Warn("failed to generate image embedding",
				zap.String("file_id", fileID),
				zap.Error(err))
		} else {
			extracted.ImageEmbedding = clipEmbedding
			i.logger.Debug("image embedding generated",
				zap.String("file_id", fileID),
				zap.Int("dimensions", len(clipEmbedding)))
		}
	}

	// 构建索引文档
	doc := i.buildIndexDocument(tenantID, fileID, filename, contentType, fileType, metadata, extracted)

	// 索引到 OpenSearch
	if err := i.osClient.IndexDocument(ctx, tenantID, fileID, doc); err != nil {
		// 索引失败，尝试回滚存储
		if delErr := i.storage.Delete(ctx, tenantID, fileID); delErr != nil {
			i.logger.Warn("failed to rollback storage", zap.Error(delErr))
		}
		return nil, fmt.Errorf("failed to index document: %w", err)
	}

	result := &IndexResult{
		FileID:      fileID,
		TenantID:    tenantID,
		Filename:    filename,
		ContentType: contentType,
		FileSize:    metadata.FileSize,
		IndexedAt:   time.Now(),
		Metadata:    extracted.Metadata,
	}

	i.logger.Info("file indexed successfully",
		zap.String("tenant_id", tenantID),
		zap.String("file_id", fileID),
		zap.Int64("file_size", metadata.FileSize))

	return result, nil
}

// buildIndexDocument 构建索引文档
func (i *Indexer) buildIndexDocument(
	tenantID, fileID, filename, contentType string,
	fileType storage.FileType,
	fileMetadata *storage.FileMetadata,
	extracted *storage.ExtractedContent,
) map[string]interface{} {
	now := time.Now()

	doc := map[string]interface{}{
		"filename":     filename,
		"content":      extracted.Text,
		"content_type": contentType,
		"file_type":    string(fileType),
		"file_size":    fileMetadata.FileSize,
		"storage_path": fileMetadata.StoragePath,
		"tenant_id":    tenantID,
		"description":  fileMetadata.Description,
		"tags":         fileMetadata.Tags,
		"created_at":   now.Format(time.RFC3339),
		"updated_at":   now.Format(time.RFC3339),
	}

	// 添加文本嵌入向量
	if extracted.Embedding != nil && len(extracted.Embedding) > 0 {
		doc["content_vector"] = extracted.Embedding
	}

	// 添加图片嵌入向量
	if extracted.ImageEmbedding != nil && len(extracted.ImageEmbedding) > 0 {
		doc["image_vector"] = extracted.ImageEmbedding
	}

	// 添加提取的元数据
	if extracted.Metadata != nil {
		doc["metadata"] = extracted.Metadata
	}

	return doc
}

// GetFile 获取文件
func (i *Indexer) GetFile(ctx context.Context, tenantID, fileID string) (io.ReadCloser, *storage.FileMetadata, error) {
	return i.storage.Get(ctx, tenantID, fileID)
}

// DeleteFile 删除文件
func (i *Indexer) DeleteFile(ctx context.Context, tenantID, fileID string) error {
	// 从 OpenSearch 删除
	if err := i.osClient.DeleteDocument(ctx, tenantID, fileID); err != nil {
		i.logger.Warn("failed to delete from opensearch",
			zap.String("file_id", fileID),
			zap.Error(err))
	}

	// 从存储删除
	return i.storage.Delete(ctx, tenantID, fileID)
}

// SearchFiles 搜索文件
func (i *Indexer) SearchFiles(ctx context.Context, tenantID string, query *opensearch.SearchQuery) (*opensearch.SearchResult, error) {
	return i.osClient.Search(ctx, tenantID, query)
}

// GetFileMetadata 获取文件元数据
func (i *Indexer) GetFileMetadata(ctx context.Context, tenantID, fileID string) (map[string]interface{}, error) {
	return i.osClient.GetDocument(ctx, tenantID, fileID)
}

// detectContentType 检测内容类型
func detectContentType(data []byte, filename string) string {
	// 简化实现，生产环境应使用 http.DetectContentType 或更精确的检测
	if len(data) == 0 {
		return "application/octet-stream"
	}

	// 根据文件扩展名
	switch {
	case hasExtension(filename, ".pdf"):
		return "application/pdf"
	case hasExtension(filename, ".jpg", ".jpeg"):
		return "image/jpeg"
	case hasExtension(filename, ".png"):
		return "image/png"
	case hasExtension(filename, ".gif"):
		return "image/gif"
	case hasExtension(filename, ".svg"):
		return "image/svg+xml"
	case hasExtension(filename, ".txt"):
		return "text/plain"
	case hasExtension(filename, ".md"):
		return "text/markdown"
	case hasExtension(filename, ".json"):
		return "application/json"
	case hasExtension(filename, ".html", ".htm"):
		return "text/html"
	case hasExtension(filename, ".doc"):
		return "application/msword"
	case hasExtension(filename, ".docx"):
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case hasExtension(filename, ".xls"):
		return "application/vnd.ms-excel"
	case hasExtension(filename, ".xlsx"):
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case hasExtension(filename, ".ppt"):
		return "application/vnd.ms-powerpoint"
	case hasExtension(filename, ".pptx"):
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case hasExtension(filename, ".rtf"):
		return "application/rtf"
	case hasExtension(filename, ".csv"):
		return "text/csv"
	}

	// 根据文件头检测
	if isPDF(data) {
		return "application/pdf"
	}
	if isJPEG(data) {
		return "image/jpeg"
	}
	if isPNG(data) {
		return "image/png"
	}
	if isGIF(data) {
		return "image/gif"
	}

	return "application/octet-stream"
}

// hasExtension 检查文件是否有指定扩展名
func hasExtension(filename string, exts ...string) bool {
	for _, ext := range exts {
		if len(filename) >= len(ext) && filename[len(filename)-len(ext):] == ext {
			return true
		}
	}
	return false
}

// isPDF 检查是否是 PDF 文件
func isPDF(data []byte) bool {
	return len(data) >= 4 && string(data[:4]) == "%PDF"
}

// isJPEG 检查是否是 JPEG 文件
func isJPEG(data []byte) bool {
	return len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8
}

// isPNG 检查是否是 PNG 文件
func isPNG(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	return string(data[:8]) == "\x89PNG\r\n\x1a\n"
}

// isGIF 检查是否是 GIF 文件
func isGIF(data []byte) bool {
	if len(data) < 6 {
		return false
	}
	s := string(data[:6])
	return s == "GIF87a" || s == "GIF89a"
}

// bytesReader 包装字节读取器
type bytesReader struct {
	*bytes.Reader
}

func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{
		Reader: bytes.NewReader(data),
	}
}
