package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	tenantpkg "github.com/dongjiang1989/opensearch-api/internal/tenant"
)

// MockTenantService 模拟租户服务
type MockTenantService struct {
	CreateFunc   func(ctx context.Context, tenant *tenantpkg.Tenant) error
	GetFunc      func(ctx context.Context, id string) (*tenantpkg.Tenant, error)
	ListFunc     func(ctx context.Context, limit, offset int) ([]*tenantpkg.Tenant, int64, error)
	UpdateFunc   func(ctx context.Context, tenant *tenantpkg.Tenant) error
	DeleteFunc   func(ctx context.Context, id string) error
	HardDeleteFunc func(ctx context.Context, id string) error
}

func (m *MockTenantService) Create(ctx context.Context, tenant *tenantpkg.Tenant) error {
	return m.CreateFunc(ctx, tenant)
}

func (m *MockTenantService) Get(ctx context.Context, id string) (*tenantpkg.Tenant, error) {
	return m.GetFunc(ctx, id)
}

func (m *MockTenantService) List(ctx context.Context, limit, offset int) ([]*tenantpkg.Tenant, int64, error) {
	return m.ListFunc(ctx, limit, offset)
}

func (m *MockTenantService) Update(ctx context.Context, tenant *tenantpkg.Tenant) error {
	return m.UpdateFunc(ctx, tenant)
}

func (m *MockTenantService) Delete(ctx context.Context, id string) error {
	return m.DeleteFunc(ctx, id)
}

func (m *MockTenantService) HardDelete(ctx context.Context, id string) error {
	return m.HardDeleteFunc(ctx, id)
}

func TestTenantHandler_New(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{}

	handler := NewTenantHandler(mockService, logger)

	assert.NotNil(t, handler)
	assert.Equal(t, mockService, handler.service)
	assert.Equal(t, logger, handler.logger)
}

func TestTenantHandler_CreateTenant_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		CreateFunc: func(ctx context.Context, tenant *tenantpkg.Tenant) error {
			tenant.CreatedAt = time.Now()
			tenant.UpdatedAt = time.Now()
			tenant.Status = tenantpkg.TenantStatusActive
			return nil
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.POST("/tenants", handler.CreateTenant)

	body := bytes.NewBufferString(`{"id": "tenant-123", "name": "Test Tenant"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/tenants", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
	assert.Contains(t, w.Body.String(), `"message":"tenant created successfully"`)
	assert.Contains(t, w.Body.String(), `"id":"tenant-123"`)
}

func TestTenantHandler_CreateTenant_InvalidJSON(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.POST("/tenants", handler.CreateTenant)

	body := bytes.NewBufferString(`{invalid json}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/tenants", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTenantHandler_CreateTenant_MissingID(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.POST("/tenants", handler.CreateTenant)

	body := bytes.NewBufferString(`{"name": "Test Tenant"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/tenants", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTenantHandler_CreateTenant_MissingName(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.POST("/tenants", handler.CreateTenant)

	body := bytes.NewBufferString(`{"id": "tenant-123"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/tenants", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTenantHandler_CreateTenant_AlreadyExists(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		CreateFunc: func(ctx context.Context, tenant *tenantpkg.Tenant) error {
			return tenantpkg.ErrTenantAlreadyExists
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.POST("/tenants", handler.CreateTenant)

	body := bytes.NewBufferString(`{"id": "tenant-123", "name": "Test Tenant"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/tenants", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "already exists")
}

func TestTenantHandler_CreateTenant_InternalError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		CreateFunc: func(ctx context.Context, tenant *tenantpkg.Tenant) error {
			return assert.AnError
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.POST("/tenants", handler.CreateTenant)

	body := bytes.NewBufferString(`{"id": "tenant-123", "name": "Test Tenant"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/tenants", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestTenantHandler_GetTenant_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		GetFunc: func(ctx context.Context, id string) (*tenantpkg.Tenant, error) {
			return &tenantpkg.Tenant{
				ID:          id,
				Name:        "Test Tenant",
				Description: "A test tenant",
				Status:      tenantpkg.TenantStatusActive,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}, nil
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.GET("/tenants/:id", handler.GetTenant)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tenants/tenant-123", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
	assert.Contains(t, w.Body.String(), `"id":"tenant-123"`)
	assert.Contains(t, w.Body.String(), `"name":"Test Tenant"`)
}

func TestTenantHandler_GetTenant_NotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		GetFunc: func(ctx context.Context, id string) (*tenantpkg.Tenant, error) {
			return nil, tenantpkg.ErrTenantNotFound
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.GET("/tenants/:id", handler.GetTenant)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tenants/non-existent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "tenant not found")
}

func TestTenantHandler_GetTenant_InternalError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		GetFunc: func(ctx context.Context, id string) (*tenantpkg.Tenant, error) {
			return nil, assert.AnError
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.GET("/tenants/:id", handler.GetTenant)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tenants/tenant-123", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestTenantHandler_ListTenants_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		ListFunc: func(ctx context.Context, limit, offset int) ([]*tenantpkg.Tenant, int64, error) {
			return []*tenantpkg.Tenant{
				{
					ID:        "tenant-1",
					Name:      "Tenant 1",
					Status:    tenantpkg.TenantStatusActive,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				{
					ID:        "tenant-2",
					Name:      "Tenant 2",
					Status:    tenantpkg.TenantStatusActive,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
			}, 2, nil
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.GET("/tenants", handler.ListTenants)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tenants", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
	assert.Contains(t, w.Body.String(), `"total":2`)
	assert.Contains(t, w.Body.String(), `"tenant-1"`)
	assert.Contains(t, w.Body.String(), `"tenant-2"`)
}

func TestTenantHandler_ListTenants_WithPagination(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		ListFunc: func(ctx context.Context, limit, offset int) ([]*tenantpkg.Tenant, int64, error) {
			assert.Equal(t, 10, limit)
			assert.Equal(t, 20, offset)
			return []*tenantpkg.Tenant{}, 0, nil
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.GET("/tenants", handler.ListTenants)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tenants?page=3&size=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTenantHandler_ListTenants_InternalError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		ListFunc: func(ctx context.Context, limit, offset int) ([]*tenantpkg.Tenant, int64, error) {
			return nil, 0, assert.AnError
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.GET("/tenants", handler.ListTenants)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tenants", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestTenantHandler_UpdateTenant_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		GetFunc: func(ctx context.Context, id string) (*tenantpkg.Tenant, error) {
			return &tenantpkg.Tenant{
				ID:          id,
				Name:        "Old Name",
				Description: "Old description",
				Status:      tenantpkg.TenantStatusActive,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}, nil
		},
		UpdateFunc: func(ctx context.Context, tenant *tenantpkg.Tenant) error {
			tenant.UpdatedAt = time.Now()
			return nil
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.PUT("/tenants/:id", handler.UpdateTenant)

	body := bytes.NewBufferString(`{"id": "tenant-123", "name": "New Name", "description": "New description"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/tenants/tenant-123", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
	assert.Contains(t, w.Body.String(), `"message":"tenant updated successfully"`)
}

func TestTenantHandler_UpdateTenant_NotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		GetFunc: func(ctx context.Context, id string) (*tenantpkg.Tenant, error) {
			return nil, tenantpkg.ErrTenantNotFound
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.PUT("/tenants/:id", handler.UpdateTenant)

	body := bytes.NewBufferString(`{"id": "tenant-123", "name": "New Name"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/tenants/non-existent", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "tenant not found")
}

func TestTenantHandler_UpdateTenant_InvalidJSON(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.PUT("/tenants/:id", handler.UpdateTenant)

	body := bytes.NewBufferString(`{invalid}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/tenants/tenant-123", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTenantHandler_UpdateTenant_InternalError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		GetFunc: func(ctx context.Context, id string) (*tenantpkg.Tenant, error) {
			return &tenantpkg.Tenant{ID: id, Name: "Test", Status: tenantpkg.TenantStatusActive}, nil
		},
		UpdateFunc: func(ctx context.Context, tenant *tenantpkg.Tenant) error {
			return assert.AnError
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.PUT("/tenants/:id", handler.UpdateTenant)

	body := bytes.NewBufferString(`{"id": "tenant-123", "name": "New Name"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/tenants/tenant-123", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestTenantHandler_DeleteTenant_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		DeleteFunc: func(ctx context.Context, id string) error {
			return nil
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.DELETE("/tenants/:id", handler.DeleteTenant)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/tenants/tenant-123", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
	assert.Contains(t, w.Body.String(), `"message":"tenant deleted successfully"`)
}

func TestTenantHandler_DeleteTenant_NotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		DeleteFunc: func(ctx context.Context, id string) error {
			return tenantpkg.ErrTenantNotFound
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.DELETE("/tenants/:id", handler.DeleteTenant)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/tenants/non-existent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "tenant not found")
}

func TestTenantHandler_DeleteTenant_InternalError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		DeleteFunc: func(ctx context.Context, id string) error {
			return assert.AnError
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.DELETE("/tenants/:id", handler.DeleteTenant)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/tenants/tenant-123", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestTenantHandler_HardDeleteTenant_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		HardDeleteFunc: func(ctx context.Context, id string) error {
			return nil
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.DELETE("/tenants/:id/hard", handler.HardDeleteTenant)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/tenants/tenant-123/hard", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
	assert.Contains(t, w.Body.String(), `"message":"tenant permanently deleted"`)
}

func TestTenantHandler_HardDeleteTenant_NotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		HardDeleteFunc: func(ctx context.Context, id string) error {
			return tenantpkg.ErrTenantNotFound
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.DELETE("/tenants/:id/hard", handler.HardDeleteTenant)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/tenants/non-existent/hard", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "tenant not found")
}

func TestTenantHandler_HardDeleteTenant_InternalError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{
		HardDeleteFunc: func(ctx context.Context, id string) error {
			return assert.AnError
		},
	}

	handler := NewTenantHandler(mockService, logger)

	router := gin.New()
	router.DELETE("/tenants/:id/hard", handler.HardDeleteTenant)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/tenants/tenant-123/hard", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestTenantHandler_toResponse(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockService := &MockTenantService{}

	handler := NewTenantHandler(mockService, logger)

	tenant := &tenantpkg.Tenant{
		ID:          "test-id",
		Name:        "Test Name",
		Description: "Test Description",
		Metadata:    map[string]string{"key": "value"},
		Status:      tenantpkg.TenantStatusActive,
		CreatedAt:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2024, 1, 16, 14, 45, 0, 0, time.UTC),
	}

	response := handler.toResponse(tenant)

	assert.Equal(t, "test-id", response.ID)
	assert.Equal(t, "Test Name", response.Name)
	assert.Equal(t, "Test Description", response.Description)
	assert.Equal(t, "active", response.Status)
	assert.Equal(t, map[string]string{"key": "value"}, response.Metadata)
	assert.Contains(t, response.CreatedAt, "2024-01-15")
	assert.Contains(t, response.UpdatedAt, "2024-01-16")
}
