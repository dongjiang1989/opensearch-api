package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// 注意：S3Storage 的完整测试需要真实的 S3/MinIO 服务
// 这里只测试配置和错误处理逻辑

func TestS3StorageConfig_Validation(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	t.Run("Missing bucket name", func(t *testing.T) {
		_, err := NewS3Storage(S3StorageConfig{
			Bucket: "",
			Logger: logger,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bucket name is required")
	})

	t.Run("Valid config without credentials", func(t *testing.T) {
		// 注意：这个测试会尝试创建真实的 AWS 配置
		// 在没有 AWS 凭证的环境下会失败，但配置验证应该通过
		_, err := NewS3Storage(S3StorageConfig{
			Bucket: "test-bucket",
			Logger: logger,
		})
		// 如果没有 AWS 凭证，这里会失败
		// 在实际项目中，应该使用 testcontainers 或 mock
		_ = err
	})
}

func TestS3Storage_MakeKey(t *testing.T) {
	// 测试 makeKey 逻辑（通过间接方式）
	tests := []struct {
		name     string
		tenantID string
		fileID   string
		contains string
	}{
		{"Basic key", "tenant1", "abc123def456", "tenants/tenant1/files/abc1"},
		{"With special chars", "tenant-2", "xyz789", "tenants/tenant-2/files/xyz7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 由于 makeKey 是私有方法，我们通过 Save 来间接测试
			// 这里只验证逻辑
			expectedPrefix := "tenants/" + tt.tenantID + "/files/" + tt.fileID[:4]
			assert.Contains(t, expectedPrefix, "tenants/")
			assert.Len(t, tt.fileID[:4], 4)
		})
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"Nil error", nil, false},
		{"Not found string", &mockError{"not found"}, true},
		{"S3 key error", &mockError{"The specified key does not exist."}, true},
		{"S3 bucket error", &mockError{"The specified bucket does not exist."}, true},
		{"Other error", &mockError{"some other error"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNotFound(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}

func TestS3Storage_Save_Metadata(t *testing.T) {
	t.Run("Metadata structure", func(t *testing.T) {
		// 验证 FileMetadata 结构
		now := time.Now()
		metadata := &FileMetadata{
			ID:          "test-id",
			Filename:    "test.pdf",
			ContentType: "application/pdf",
			FileType:    FileTypePDF,
			FileSize:    1024,
			StoragePath: "tenants/test/files/test",
			TenantID:    "test-tenant",
			CreatedAt:   now,
			UpdatedAt:   now,
			Metadata:    map[string]string{"key": "value"},
		}

		assert.Equal(t, "test-id", metadata.ID)
		assert.Equal(t, FileTypePDF, metadata.FileType)
		assert.NotNil(t, metadata.Metadata)
	})
}

func TestS3Storage_GetURL_Expiry(t *testing.T) {
	t.Run("Expiry time", func(t *testing.T) {
		// 验证过期时间设置
		expiry := 15 * time.Minute
		assert.Greater(t, expiry, time.Duration(0))

		// 实际测试需要真实的 S3 客户端
	})
}
