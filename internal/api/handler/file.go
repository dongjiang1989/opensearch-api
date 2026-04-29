package handler

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/api/middleware"
	"github.com/dongjiang1989/opensearch-api/internal/indexer"
	"github.com/dongjiang1989/opensearch-api/internal/storage"
)

// FileHandler 文件管理 Handler
type FileHandler struct {
	indexer *indexer.Indexer
	logger  *zap.Logger
}

// NewFileHandler 创建文件管理 Handler
func NewFileHandler(indexer *indexer.Indexer, logger *zap.Logger) *FileHandler {
	return &FileHandler{
		indexer: indexer,
		logger:  logger,
	}
}

// UploadFileRequest 文件上传请求
type UploadFileRequest struct {
	Description string   `form:"description"`
	Tags        []string `form:"tags"`
}

// FileUploadResponse 文件上传响应
type FileUploadResponse struct {
	Success     bool                   `json:"success"`
	FileID      string                 `json:"file_id"`
	Filename    string                 `json:"filename"`
	ContentType string                 `json:"content_type"`
	FileSize    int64                  `json:"file_size"`
	IndexedAt   time.Time              `json:"indexed_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// UploadFile 上传文件
// @Summary 上传文件
// @Description 上传文件并建立索引，支持 PDF、图片、文档等格式
// @Tags File
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "文件"
// @Param description formData string false "文件描述"
// @Param tags formData []string false "文件标签"
// @Success 200 {object} FileUploadResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Security X-Tenant-ID
// @Router /files [post]
func (h *FileHandler) UploadFile(c *gin.Context) {
	// 获取租户 ID
	tenantID, ok := middleware.GetTenantID(c)
	if !ok || tenantID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "tenant ID is required",
		})
		return
	}

	// 解析表单
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "file is required",
		})
		return
	}
	defer func() { _ = file.Close() }()

	// 读取额外字段
	description := c.PostForm("description")
	tags := c.PostFormArray("tags")

	h.logger.Debug("uploading file",
		zap.String("tenant_id", tenantID),
		zap.String("filename", header.Filename),
		zap.Int64("size", header.Size))

	// 使用索引器处理文件
	result, err := h.indexer.IndexFile(c.Request.Context(), tenantID, header.Filename, file)
	if err != nil {
		h.logger.Error("failed to index file",
			zap.String("tenant_id", tenantID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// 更新额外元数据（如果需要）
	if description != "" || len(tags) > 0 {
		doc, err := h.indexer.GetFileMetadata(c.Request.Context(), tenantID, result.FileID)
		if err == nil && doc != nil {
			if description != "" {
				doc["description"] = description
			}
			if len(tags) > 0 {
				doc["tags"] = tags
			}
			// TODO: 添加更新方法到 indexer
			_ = doc
		}
	}

	c.JSON(http.StatusOK, FileUploadResponse{
		Success:     true,
		FileID:      result.FileID,
		Filename:    result.Filename,
		ContentType: result.ContentType,
		FileSize:    result.FileSize,
		IndexedAt:   result.IndexedAt,
		Metadata:    result.Metadata,
	})
}

// GetFile 获取文件
// @Summary 下载文件
// @Description 根据文件 ID 下载文件
// @Tags File
// @Produce octet-stream
// @Param file_id path string true "文件 ID"
// @Success 200 {file} binary "文件内容"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Security X-Tenant-ID
// @Router /files/{file_id} [get]
func (h *FileHandler) GetFile(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok || tenantID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "tenant ID is required",
		})
		return
	}

	fileID := c.Param("file_id")

	// 获取文件内容
	reader, metadata, err := h.indexer.GetFile(c.Request.Context(), tenantID, fileID)
	if err != nil {
		if err == storage.ErrFileNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Success: false,
				Error:   "file not found",
			})
			return
		}

		h.logger.Error("failed to get file",
			zap.String("file_id", fileID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if metadata == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Success: false,
			Error:   "file not found",
		})
		return
	}

	// 返回文件内容
	c.Header("Content-Disposition", "attachment; filename=\""+metadata.Filename+"\"")
	c.Header("Content-Type", metadata.ContentType)
	c.Header("Content-Length", string(rune(metadata.FileSize)))
	if _, err := io.Copy(c.Writer, reader); err != nil {
		h.logger.Error("failed to write file response", zap.Error(err))
	}
}

// GetFileMetadata 获取文件元数据
// @Summary 获取文件元数据
// @Description 根据文件 ID 获取文件元数据
// @Tags File
// @Produce json
// @Param file_id path string true "文件 ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Security X-Tenant-ID
// @Router /files/{file_id}/metadata [get]
func (h *FileHandler) GetFileMetadata(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok || tenantID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "tenant ID is required",
		})
		return
	}

	fileID := c.Param("file_id")

	doc, err := h.indexer.GetFileMetadata(c.Request.Context(), tenantID, fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if doc == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Success: false,
			Error:   "file not found",
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Data:    doc,
	})
}

// DeleteFile 删除文件
// @Summary 删除文件
// @Description 根据文件 ID 删除文件
// @Tags File
// @Produce json
// @Param file_id path string true "文件 ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Security X-Tenant-ID
// @Router /files/{file_id} [delete]
func (h *FileHandler) DeleteFile(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok || tenantID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "tenant ID is required",
		})
		return
	}

	fileID := c.Param("file_id")

	if err := h.indexer.DeleteFile(c.Request.Context(), tenantID, fileID); err != nil {
		if err == storage.ErrFileNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Success: false,
				Error:   "file not found",
			})
			return
		}

		h.logger.Error("failed to delete file",
			zap.String("file_id", fileID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Message: "file deleted successfully",
	})
}

// ListFiles 列出文件
// @Summary 列出文件
// @Description 获取当前租户下的文件列表
// @Tags File
// @Produce json
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Security BearerAuth
// @Security X-Tenant-ID
// @Router /files [get]
func (h *FileHandler) ListFiles(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok || tenantID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "tenant ID is required",
		})
		return
	}

	// TODO: 实现文件列表查询
	// 目前返回空列表
	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Data:    []interface{}{},
	})
}

// FileMetadata 文件元数据结构
type FileMetadata struct {
	Filename    string      `json:"filename"`
	ContentType string      `json:"content_type"`
	FileSize    int64       `json:"file_size"`
	Description string      `json:"description,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	Metadata    interface{} `json:"metadata,omitempty"`
	CreatedAt   string      `json:"created_at"`
	UpdatedAt   string      `json:"updated_at"`
}
