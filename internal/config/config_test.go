package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	// 清空环境变量
	os.Clearenv()

	cfg, err := Load("")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// 验证默认值
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, "release", cfg.Server.Mode)

	assert.Equal(t, "localhost", cfg.OpenSearch.Host)
	assert.Equal(t, 9200, cfg.OpenSearch.Port)
	assert.Equal(t, "admin", cfg.OpenSearch.Username)
	assert.True(t, cfg.OpenSearch.Secure)

	assert.Equal(t, "local", cfg.Storage.Type)
	assert.Equal(t, "./data/files", cfg.Storage.LocalPath)

	assert.Equal(t, "json", cfg.Log.Format)
}

func TestLoad_EnvironmentVariables(t *testing.T) {
	// 清空环境变量
	os.Clearenv()

	// 设置环境变量
	_ = os.Setenv("OPENSEARCH_SERVER_PORT", "9090")
	_ = os.Setenv("OPENSEARCH_OPENSEARCH_HOST", "opensearch.example.com")
	_ = os.Setenv("OPENSEARCH_JWT_SECRET", "test-secret")

	cfg, err := Load("")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "opensearch.example.com", cfg.OpenSearch.Host)
	assert.Equal(t, "test-secret", cfg.JWT.Secret)
}

func TestConfigHelpers(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		OpenSearch: OpenSearchConfig{
			Host:   "opensearch",
			Port:   9200,
			Secure: false,
		},
	}

	// 测试 Server.Address()
	assert.Equal(t, "localhost:8080", cfg.Server.Address())

	// 测试 OpenSearch.Address()
	assert.Equal(t, "opensearch:9200", cfg.OpenSearch.Address())

	// 测试 OpenSearch.URL() - HTTP
	assert.Equal(t, "http://opensearch:9200", cfg.OpenSearch.URL())

	// 测试 OpenSearch.URL() - HTTPS
	cfg.OpenSearch.Secure = true
	assert.Equal(t, "https://opensearch:9200", cfg.OpenSearch.URL())
}

func TestStorageConfig_IsS3(t *testing.T) {
	tests := []struct {
		name     string
		cfg      StorageConfig
		expected bool
	}{
		{"local storage", StorageConfig{Type: "local"}, false},
		{"s3 storage", StorageConfig{Type: "s3"}, true},
		{"empty defaults to local", StorageConfig{Type: ""}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cfg.IsS3())
		})
	}
}
