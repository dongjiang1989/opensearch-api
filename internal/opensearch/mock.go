package opensearch

import (
	"context"
	"sync"
)

// MockClient OpenSearch 模拟客户端
type MockClient struct {
	mu        sync.RWMutex
	documents map[string]map[string]interface{} // index -> docID -> doc
	indices   map[string]bool
}

// NewMockClient 创建模拟客户端
func NewMockClient() *MockClient {
	return &MockClient{
		documents: make(map[string]map[string]interface{}),
		indices:   make(map[string]bool),
	}
}

// CreateIndex 创建索引
func (m *MockClient) CreateIndex(ctx context.Context, tenantID string, mapping map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	indexName := m.IndexName(tenantID)
	m.indices[indexName] = true
	if m.documents[indexName] == nil {
		m.documents[indexName] = make(map[string]interface{})
	}
	return nil
}

// IndexExists 检查索引是否存在
func (m *MockClient) IndexExists(ctx context.Context, indexName string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.indices[indexName]
	return exists, nil
}

// DeleteIndex 删除索引
func (m *MockClient) DeleteIndex(ctx context.Context, tenantID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	indexName := m.IndexName(tenantID)
	delete(m.indices, indexName)
	delete(m.documents, indexName)
	return nil
}

// IndexDocument 索引文档
func (m *MockClient) IndexDocument(ctx context.Context, tenantID, docID string, doc map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	indexName := m.IndexName(tenantID)
	if m.documents[indexName] == nil {
		m.documents[indexName] = make(map[string]interface{})
	}
	m.documents[indexName][docID] = doc
	return nil
}

// GetDocument 获取文档
func (m *MockClient) GetDocument(ctx context.Context, tenantID, docID string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	indexName := m.IndexName(tenantID)
	docs, exists := m.documents[indexName]
	if !exists {
		return nil, nil
	}

	doc, exists := docs[docID]
	if !exists {
		return nil, nil
	}

	return doc.(map[string]interface{}), nil
}

// DeleteDocument 删除文档
func (m *MockClient) DeleteDocument(ctx context.Context, tenantID, docID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	indexName := m.IndexName(tenantID)
	if m.documents[indexName] != nil {
		delete(m.documents[indexName], docID)
	}
	return nil
}

// Search 搜索
func (m *MockClient) Search(ctx context.Context, tenantID string, query *SearchQuery) (*SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	indexName := m.IndexName(tenantID)
	docs, exists := m.documents[indexName]
	if !exists {
		return &SearchResult{
			Total: 0,
			Hits:  []SearchHit{},
			Took:  0,
		}, nil
	}

	var hits []SearchHit
	for docID, doc := range docs {
		// 简单过滤
		if query.Query != "" {
			found := false
			docMap, ok := doc.(map[string]interface{})
			if !ok {
				continue
			}
			for _, value := range docMap {
				if str, ok := value.(string); ok && contains(str, query.Query) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		hits = append(hits, SearchHit{
			ID:     docID,
			Score:  1.0,
			Source: doc.(map[string]interface{}),
		})
	}

	// 应用分页
	from := query.From
	size := query.Size
	if from >= len(hits) {
		return &SearchResult{
			Total: len(hits),
			Hits:  []SearchHit{},
			Took:  1,
		}, nil
	}

	end := from + size
	if end > len(hits) {
		end = len(hits)
	}

	return &SearchResult{
		Total: len(hits),
		Hits:  hits[from:end],
		Took:  1,
	}, nil
}

// Count 统计文档数量
func (m *MockClient) Count(ctx context.Context, tenantID string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	indexName := m.IndexName(tenantID)
	docs, exists := m.documents[indexName]
	if !exists {
		return 0, nil
	}

	return int64(len(docs)), nil
}

// Refresh 刷新索引
func (m *MockClient) Refresh(ctx context.Context, tenantID string) error {
	return nil
}

// Health 健康检查
func (m *MockClient) Health(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"status":          "green",
		"cluster_uuid":    "mock-cluster",
		"number_of_nodes": 1,
	}, nil
}

// Ping 检查连接
func (m *MockClient) Ping(ctx context.Context) error {
	return nil
}

// IndexName 生成索引名称
func (m *MockClient) IndexName(tenantID string) string {
	return "tenant_" + tenantID + "_files"
}

// AliasName 生成别名
func (m *MockClient) AliasName(tenantID string) string {
	return "tenant_" + tenantID
}

// CreateIndexWithMapping 创建带映射的索引
func (m *MockClient) CreateIndexWithMapping(ctx context.Context, tenantID string) error {
	return m.CreateIndex(ctx, tenantID, FileMapping())
}

// Aggregate 聚合查询
func (m *MockClient) Aggregate(ctx context.Context, tenantID, fieldName string) (map[string]int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	indexName := m.IndexName(tenantID)
	docs, exists := m.documents[indexName]
	if !exists {
		return map[string]int64{}, nil
	}

	buckets := make(map[string]int64)
	for _, doc := range docs {
		docMap := doc.(map[string]interface{})
		if value, ok := docMap[fieldName].(string); ok {
			buckets[value]++
		}
	}

	return buckets, nil
}

// BulkIndex 批量索引
func (m *MockClient) BulkIndex(ctx context.Context, tenantID string, docs []BulkDoc) error {
	for _, doc := range docs {
		if err := m.IndexDocument(ctx, tenantID, doc.ID, doc.Source); err != nil {
			return err
		}
	}
	return nil
}

// contains 辅助函数
func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
