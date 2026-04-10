package tenant

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockClient 简单的 OpenSearch 模拟客户端
type MockClient struct{}

func (m *MockClient) CreateIndexWithMapping(ctx context.Context, tenantID string) error {
	return nil
}

func (m *MockClient) DeleteIndex(ctx context.Context, tenantID string) error {
	return nil
}

func (m *MockClient) IndexName(tenantID string) string {
	return "tenant_" + tenantID + "_files"
}

func newTestService() (*Service, *MockClient) {
	logger, _ := zap.NewDevelopment()
	mockOS := &MockClient{}

	repo := NewInMemoryRepository()
	service := NewService(ServiceConfig{
		Repository: repo,
		OpenSearch: mockOS,
		Logger:     logger,
	})

	return service, mockOS
}

func TestService_Create(t *testing.T) {
	service, _ := newTestService()
	ctx := context.Background()

	tenant := &Tenant{
		ID:          "test-tenant",
		Name:        "Test Tenant",
		Description: "Test description",
	}

	// 创建租户
	err := service.Create(ctx, tenant)
	require.NoError(t, err)

	// 验证租户已创建
	result, err := service.Get(ctx, "test-tenant")
	require.NoError(t, err)
	assert.Equal(t, "test-tenant", result.ID)
	assert.Equal(t, "Test Tenant", result.Name)
	assert.Equal(t, TenantStatusActive, result.Status)
	assert.False(t, result.CreatedAt.IsZero())
}

func TestService_Create_Duplicate(t *testing.T) {
	service, _ := newTestService()
	ctx := context.Background()

	tenant := &Tenant{
		ID:   "test-tenant",
		Name: "Test Tenant",
	}

	// 第一次创建
	err := service.Create(ctx, tenant)
	require.NoError(t, err)

	// 重复创建
	err = service.Create(ctx, tenant)
	assert.Equal(t, ErrTenantAlreadyExists, err)
}

func TestService_Get_NotFound(t *testing.T) {
	service, _ := newTestService()
	ctx := context.Background()

	_, err := service.Get(ctx, "non-existent")
	assert.Equal(t, ErrTenantNotFound, err)
}

func TestService_Update(t *testing.T) {
	service, _ := newTestService()
	ctx := context.Background()

	// 创建租户
	tenant := &Tenant{
		ID:   "test-tenant",
		Name: "Original Name",
	}
	err := service.Create(ctx, tenant)
	require.NoError(t, err)

	// 更新租户
	tenant.Name = "Updated Name"
	tenant.Description = "New description"
	err = service.Update(ctx, tenant)
	require.NoError(t, err)

	// 验证更新
	result, err := service.Get(ctx, "test-tenant")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", result.Name)
	assert.Equal(t, "New description", result.Description)
	assert.False(t, result.UpdatedAt.IsZero())
}

func TestService_Delete(t *testing.T) {
	service, _ := newTestService()
	ctx := context.Background()

	// 创建租户
	tenant := &Tenant{
		ID:   "test-tenant",
		Name: "Test Tenant",
	}
	err := service.Create(ctx, tenant)
	require.NoError(t, err)

	// 软删除
	err = service.Delete(ctx, "test-tenant")
	require.NoError(t, err)

	// 验证删除后无法获取
	_, err = service.Get(ctx, "test-tenant")
	assert.Equal(t, ErrTenantNotFound, err)
}

func TestService_List(t *testing.T) {
	service, _ := newTestService()
	ctx := context.Background()

	// 创建多个租户
	for i := 1; i <= 5; i++ {
		tenant := &Tenant{
			ID:   func() string { return "tenant" }(),
			Name: "Tenant",
		}
		tenant.ID = "tenant-" + string(rune('0'+i))
		tenant.Name = "Tenant " + string(rune('0'+i))
		_ = service.Create(ctx, tenant)
	}

	// 列出租户
	tenants, total, err := service.List(ctx, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, tenants, 5)

	// 分页测试
	tenants, total, err = service.List(ctx, 2, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, tenants, 2)
}

func TestInMemoryRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()

	tenant := &Tenant{
		ID:        "test",
		Name:      "Test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create
	err := repo.Create(ctx, tenant)
	assert.NoError(t, err)

	// Get
	result, err := repo.Get(ctx, "test")
	assert.NoError(t, err)
	assert.Equal(t, "test", result.ID)

	// Get non-existent
	_, err = repo.Get(ctx, "non-existent")
	assert.Equal(t, ErrTenantNotFound, err)

	// Update
	tenant.Name = "Updated"
	err = repo.Update(ctx, tenant)
	assert.NoError(t, err)

	// List
	tenants, total, err := repo.List(ctx, 10, 0)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, tenants, 1)

	// Delete
	err = repo.Delete(ctx, "test")
	assert.NoError(t, err)

	_, err = repo.Get(ctx, "test")
	assert.Equal(t, ErrTenantNotFound, err)
}
