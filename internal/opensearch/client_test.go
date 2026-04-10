package opensearch

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/config"
)

// TestClient 测试用客户端
func newTestClient(handler http.HandlerFunc) (*Client, func()) {
	server := httptest.NewServer(handler)

	logger, _ := zap.NewDevelopment()

	// 使用测试服务器地址
	client, err := NewClient(&config.OpenSearchConfig{
		Host:        server.URL[7:], // 移除 "http://"
		Port:        80,
		Username:    "test",
		Password:    "test",
		Secure:      false,
		IndexPrefix: "tenant",
	}, logger)

	if err != nil {
		panic(err)
	}

	return client, server.Close
}

func TestIndexName(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &Client{
		config: &config.OpenSearchConfig{
			IndexPrefix: "tenant",
		},
		logger: logger,
	}

	tests := []struct {
		tenantID string
		expected string
	}{
		{"abc123", "tenant_abc123_files"},
		{"test-tenant", "tenant_test-tenant_files"},
		{"user_001", "tenant_user_001_files"},
	}

	for _, tt := range tests {
		t.Run(tt.tenantID, func(t *testing.T) {
			assert.Equal(t, tt.expected, client.IndexName(tt.tenantID))
		})
	}
}

func TestAliasName(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &Client{
		config: &config.OpenSearchConfig{
			IndexPrefix: "tenant",
		},
		logger: logger,
	}

	tests := []struct {
		tenantID string
		expected string
	}{
		{"abc123", "tenant_abc123"},
		{"test-tenant", "tenant_test-tenant"},
	}

	for _, tt := range tests {
		t.Run(tt.tenantID, func(t *testing.T) {
			assert.Equal(t, tt.expected, client.AliasName(tt.tenantID))
		})
	}
}

func TestFileMapping(t *testing.T) {
	mapping := FileMapping()

	assert.NotNil(t, mapping)
	assert.Contains(t, mapping, "properties")

	props, ok := mapping["properties"].(map[string]interface{})
	require.True(t, ok)

	// 验证关键字段存在
	expectedFields := []string{"filename", "content", "content_type", "file_type", "file_size", "metadata"}
	for _, field := range expectedFields {
		assert.Contains(t, props, field)
	}
}

func TestGetString(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		key      string
		expected string
	}{
		{"string value", map[string]interface{}{"key": "value"}, "key", "value"},
		{"non-string value", map[string]interface{}{"key": 123}, "key", ""},
		{"missing key", map[string]interface{}{}, "key", ""},
		{"nil value", nil, "key", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getString(tt.input, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSearchQuery_BuildQuery(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &Client{
		config: &config.OpenSearchConfig{},
		logger: logger,
	}

	tests := []struct {
		name      string
		query     *SearchQuery
		wantMatch string // 期望查询类型的关键字
	}{
		{
			name:      "empty query returns match_all",
			query:     &SearchQuery{},
			wantMatch: "match_all",
		},
		{
			name:      "text only query",
			query:     &SearchQuery{Query: "test"},
			wantMatch: "multi_match",
		},
		{
			name:      "filters only",
			query:     &SearchQuery{Filters: map[string]interface{}{"type": "pdf"}},
			wantMatch: "bool",
		},
		{
			name:      "query and filters",
			query:     &SearchQuery{Query: "test", Filters: map[string]interface{}{"type": "pdf"}},
			wantMatch: "bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.buildQuery(tt.query)
			resultStr, _ := result[tt.wantMatch]
			assert.NotNil(t, resultStr)
		})
	}
}

func TestParseTotal(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &Client{
		config: &config.OpenSearchConfig{},
		logger: logger,
	}

	tests := []struct {
		name     string
		input    map[string]interface{}
		expected int
	}{
		{"new format", map[string]interface{}{"total": map[string]interface{}{"value": float64(100)}}, 100},
		{"old format", map[string]interface{}{"total": float64(50)}, 50},
		{"missing", map[string]interface{}{}, 0},
		{"invalid type", map[string]interface{}{"total": "invalid"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.parseTotal(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
