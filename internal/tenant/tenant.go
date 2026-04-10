package tenant

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/opensearch"
)

var (
	ErrTenantNotFound      = errors.New("tenant not found")
	ErrTenantAlreadyExists = errors.New("tenant already exists")
)

// Tenant 租户模型
type Tenant struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Status      TenantStatus      `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// TenantStatus 租户状态
type TenantStatus string

const (
	TenantStatusActive   TenantStatus = "active"
	TenantStatusInactive TenantStatus = "inactive"
	TenantStatusDeleted  TenantStatus = "deleted"
)

// Repository 租户仓库接口
type Repository interface {
	Get(ctx context.Context, id string) (*Tenant, error)
	Create(ctx context.Context, tenant *Tenant) error
	Update(ctx context.Context, tenant *Tenant) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, limit, offset int) ([]*Tenant, int64, error)
}

// OpenSearchClient OpenSearch 客户端接口
type OpenSearchClient interface {
	CreateIndexWithMapping(ctx context.Context, tenantID string) error
	DeleteIndex(ctx context.Context, tenantID string) error
	IndexName(tenantID string) string
}

// Service 租户服务
type Service struct {
	repo          Repository
	osClient      OpenSearchClient
	logger        *zap.Logger
	indexMappings map[string]interface{}
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	Repository    Repository
	OpenSearch    OpenSearchClient
	Logger        *zap.Logger
	IndexMappings map[string]interface{}
}

// NewService 创建租户服务
func NewService(cfg ServiceConfig) *Service {
	if cfg.IndexMappings == nil {
		cfg.IndexMappings = opensearch.FileMapping()
	}

	return &Service{
		repo:          cfg.Repository,
		osClient:      cfg.OpenSearch,
		logger:        cfg.Logger,
		indexMappings: cfg.IndexMappings,
	}
}

// Get 获取租户
func (s *Service) Get(ctx context.Context, id string) (*Tenant, error) {
	tenant, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if tenant.Status == TenantStatusDeleted {
		return nil, ErrTenantNotFound
	}

	return tenant, nil
}

// Create 创建租户
func (s *Service) Create(ctx context.Context, tenant *Tenant) error {
	// 检查是否已存在
	existing, err := s.repo.Get(ctx, tenant.ID)
	if err == nil && existing != nil && existing.Status != TenantStatusDeleted {
		return ErrTenantAlreadyExists
	}

	// 设置默认值
	now := time.Now()
	tenant.CreatedAt = now
	tenant.UpdatedAt = now
	tenant.Status = TenantStatusActive

	// 保存到仓库
	if err := s.repo.Create(ctx, tenant); err != nil {
		return err
	}

	// 创建 OpenSearch 索引
	if err := s.osClient.CreateIndexWithMapping(ctx, tenant.ID); err != nil {
		s.logger.Error("failed to create opensearch index for tenant",
			zap.String("tenant_id", tenant.ID),
			zap.Error(err))
		// 索引创建失败不影响租户创建，但需要记录日志
	}

	s.logger.Info("tenant created",
		zap.String("tenant_id", tenant.ID),
		zap.String("name", tenant.Name))

	return nil
}

// Update 更新租户
func (s *Service) Update(ctx context.Context, tenant *Tenant) error {
	existing, err := s.repo.Get(ctx, tenant.ID)
	if err != nil {
		return err
	}

	if existing.Status == TenantStatusDeleted {
		return ErrTenantNotFound
	}

	tenant.UpdatedAt = time.Now()
	// 保持原有状态（除非明确更新）
	if tenant.Status == "" {
		tenant.Status = existing.Status
	}

	return s.repo.Update(ctx, tenant)
}

// Delete 删除租户（软删除）
func (s *Service) Delete(ctx context.Context, id string) error {
	tenant, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}

	if tenant.Status == TenantStatusDeleted {
		return ErrTenantNotFound
	}

	// 软删除
	tenant.Status = TenantStatusDeleted
	tenant.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, tenant); err != nil {
		return err
	}

	s.logger.Info("tenant deleted", zap.String("tenant_id", id))
	return nil
}

// HardDelete 彻底删除租户（包括索引）
func (s *Service) HardDelete(ctx context.Context, id string) error {
	// 删除 OpenSearch 索引
	if err := s.osClient.DeleteIndex(ctx, id); err != nil {
		s.logger.Error("failed to delete opensearch index",
			zap.String("tenant_id", id),
			zap.Error(err))
	}

	// 从仓库删除
	return s.repo.Delete(ctx, id)
}

// List 列出租户
func (s *Service) List(ctx context.Context, limit, offset int) ([]*Tenant, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	return s.repo.List(ctx, limit, offset)
}

// EnsureIndex 确保租户索引存在
func (s *Service) EnsureIndex(ctx context.Context, tenantID string) error {
	return s.osClient.CreateIndexWithMapping(ctx, tenantID)
}

// GetIndexName 获取租户索引名称
func (s *Service) GetIndexName(tenantID string) string {
	return s.osClient.IndexName(tenantID)
}
