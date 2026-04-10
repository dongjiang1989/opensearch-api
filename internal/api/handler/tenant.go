package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	tenantpkg "github.com/dongjiang1989/opensearch-api/internal/tenant"
)

// TenantService defines the interface for tenant operations
type TenantService interface {
	Create(ctx context.Context, tenant *tenantpkg.Tenant) error
	Get(ctx context.Context, id string) (*tenantpkg.Tenant, error)
	List(ctx context.Context, limit, offset int) ([]*tenantpkg.Tenant, int64, error)
	Update(ctx context.Context, tenant *tenantpkg.Tenant) error
	Delete(ctx context.Context, id string) error
	HardDelete(ctx context.Context, id string) error
}

// TenantHandler 租户管理 Handler
type TenantHandler struct {
	service TenantService
	logger  *zap.Logger
}

// NewTenantHandler 创建租户管理 Handler
func NewTenantHandler(service TenantService, logger *zap.Logger) *TenantHandler {
	return &TenantHandler{
		service: service,
		logger:  logger,
	}
}

// CreateTenantRequest 创建租户请求
type CreateTenantRequest struct {
	ID          string            `json:"id" binding:"required"`
	Name        string            `json:"name" binding:"required"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
}

// TenantResponse 租户响应
type TenantResponse struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Status      string            `json:"status"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// CreateTenant 创建租户
func (h *TenantHandler) CreateTenant(c *gin.Context) {
	var req CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	tenant := &tenantpkg.Tenant{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		Metadata:    req.Metadata,
	}

	if err := h.service.Create(c.Request.Context(), tenant); err != nil {
		if errors.Is(err, tenantpkg.ErrTenantAlreadyExists) {
			c.JSON(http.StatusConflict, ErrorResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		h.logger.Error("failed to create tenant",
			zap.String("tenant_id", req.ID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	h.logger.Info("tenant created", zap.String("tenant_id", req.ID))

	c.JSON(http.StatusCreated, SuccessResponse{
		Success: true,
		Message: "tenant created successfully",
		Data:    h.toResponse(tenant),
	})
}

// GetTenant 获取租户信息
func (h *TenantHandler) GetTenant(c *gin.Context) {
	tenantID := c.Param("id")

	tenant, err := h.service.Get(c.Request.Context(), tenantID)
	if err != nil {
		if errors.Is(err, tenantpkg.ErrTenantNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Success: false,
				Error:   "tenant not found",
			})
			return
		}

		h.logger.Error("failed to get tenant",
			zap.String("tenant_id", tenantID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Data:    h.toResponse(tenant),
	})
}

// ListTenants 列出租户
func (h *TenantHandler) ListTenants(c *gin.Context) {
	page, size := ParsePagination(c)

	tenants, total, err := h.service.List(c.Request.Context(), size, (page-1)*size)
	if err != nil {
		h.logger.Error("failed to list tenants", zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	response := make([]*TenantResponse, 0, len(tenants))
	for _, t := range tenants {
		response = append(response, h.toResponse(t))
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		Success: true,
		Data:    response,
		Total:   total,
		Page:    page,
		Size:    size,
	})
}

// UpdateTenant 更新租户
func (h *TenantHandler) UpdateTenant(c *gin.Context) {
	tenantID := c.Param("id")

	var req CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	t, err := h.service.Get(c.Request.Context(), tenantID)
	if err != nil {
		if errors.Is(err, tenantpkg.ErrTenantNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Success: false,
				Error:   "tenant not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// 更新字段
	if req.Name != "" {
		t.Name = req.Name
	}
	t.Description = req.Description
	if req.Metadata != nil {
		t.Metadata = req.Metadata
	}

	if err := h.service.Update(c.Request.Context(), t); err != nil {
		h.logger.Error("failed to update tenant",
			zap.String("tenant_id", tenantID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Message: "tenant updated successfully",
		Data:    h.toResponse(t),
	})
}

// DeleteTenant 删除租户（软删除）
func (h *TenantHandler) DeleteTenant(c *gin.Context) {
	tenantID := c.Param("id")

	if err := h.service.Delete(c.Request.Context(), tenantID); err != nil {
		if errors.Is(err, tenantpkg.ErrTenantNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Success: false,
				Error:   "tenant not found",
			})
			return
		}

		h.logger.Error("failed to delete tenant",
			zap.String("tenant_id", tenantID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Message: "tenant deleted successfully",
	})
}

// HardDeleteTenant 彻底删除租户
func (h *TenantHandler) HardDeleteTenant(c *gin.Context) {
	tenantID := c.Param("id")

	if err := h.service.HardDelete(c.Request.Context(), tenantID); err != nil {
		if errors.Is(err, tenantpkg.ErrTenantNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Success: false,
				Error:   "tenant not found",
			})
			return
		}

		h.logger.Error("failed to hard delete tenant",
			zap.String("tenant_id", tenantID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Message: "tenant permanently deleted",
	})
}

// toResponse 转换为响应格式
func (h *TenantHandler) toResponse(t *tenantpkg.Tenant) *TenantResponse {
	return &TenantResponse{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Metadata:    t.Metadata,
		Status:      string(t.Status),
		CreatedAt:   t.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   t.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
