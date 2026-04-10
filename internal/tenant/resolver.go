package tenant

import (
	"context"
	"errors"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// Claims JWT Claims
type Claims struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// ResolverConfig 解析器配置
type ResolverConfig struct {
	// Header 名称
	TenantHeader string
	// JWT Secret 用于验证
	JWTSecret []byte
	// 是否启用 JWT 验证
	EnableJWT bool
	// Logger
	Logger *zap.Logger
}

// Resolver 租户解析器
type Resolver struct {
	config ResolverConfig
}

// NewResolver 创建租户解析器
func NewResolver(config ResolverConfig) *Resolver {
	if config.TenantHeader == "" {
		config.TenantHeader = "X-Tenant-ID"
	}

	return &Resolver{
		config: config,
	}
}

// ResolveFromHeader 从 HTTP 头解析租户 ID
func (r *Resolver) ResolveFromHeader(headerValue string) (string, error) {
	if headerValue == "" {
		return "", errors.New("tenant header is empty")
	}

	return headerValue, nil
}

// ResolveFromToken 从 JWT Token 解析租户信息
func (r *Resolver) ResolveFromToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return r.config.JWTSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token claims")
}

// TenantFromContext 从上下文获取租户 ID
func TenantFromContext(ctx context.Context) (string, bool) {
	tenantID, ok := ctx.Value(tenantContextKey).(string)
	return tenantID, ok
}

// ClaimsFromContext 从上下文获取 JWT Claims
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*Claims)
	return claims, ok
}

// context keys
type contextKey int

const (
	tenantContextKey contextKey = iota
	claimsContextKey
)

// WithTenant 将租户 ID 添加到上下文
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantContextKey, tenantID)
}

// WithClaims 将 Claims 添加到上下文
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey, claims)
}

// InMemoryRepository 内存中的租户仓库（用于测试和简单场景）
type InMemoryRepository struct {
	mu      sync.RWMutex
	tenants map[string]*Tenant
}

// NewInMemoryRepository 创建内存仓库
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		tenants: make(map[string]*Tenant),
	}
}

func (r *InMemoryRepository) Get(ctx context.Context, id string) (*Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tenant, exists := r.tenants[id]
	if !exists {
		return nil, ErrTenantNotFound
	}

	return tenant, nil
}

func (r *InMemoryRepository) Create(ctx context.Context, tenant *Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tenants[tenant.ID]; exists {
		return ErrTenantAlreadyExists
	}

	r.tenants[tenant.ID] = tenant
	return nil
}

func (r *InMemoryRepository) Update(ctx context.Context, tenant *Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tenants[tenant.ID]; !exists {
		return ErrTenantNotFound
	}

	r.tenants[tenant.ID] = tenant
	return nil
}

func (r *InMemoryRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tenants[id]; !exists {
		return ErrTenantNotFound
	}

	delete(r.tenants, id)
	return nil
}

func (r *InMemoryRepository) List(ctx context.Context, limit, offset int) ([]*Tenant, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Tenant
	total := int64(len(r.tenants))

	i := 0
	for _, tenant := range r.tenants {
		if i >= offset && len(result) < limit {
			result = append(result, tenant)
		}
		i++
	}

	return result, total, nil
}
