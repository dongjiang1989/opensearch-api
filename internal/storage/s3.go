package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
)

// S3Storage S3 兼容存储
type S3Storage struct {
	client *s3.Client
	bucket string
	logger *zap.Logger
}

// S3StorageConfig S3 存储配置
type S3StorageConfig struct {
	Bucket   string
	Region   string
	Endpoint string // MinIO 等兼容服务地址
	KeyID    string
	Secret   string
	Logger   *zap.Logger
}

// NewS3Storage 创建 S3 存储
func NewS3Storage(cfg S3StorageConfig) (*S3Storage, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	var opts []func(*config.LoadOptions) error

	// 配置凭证
	if cfg.KeyID != "" && cfg.Secret != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.KeyID,
			cfg.Secret,
			"",
		)))
	}

	// 配置区域
	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}

	// 加载配置
	awsConfig, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	// 创建 S3 客户端
	client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // MinIO 需要路径风格
		}
	})

	return &S3Storage{
		client: client,
		bucket: cfg.Bucket,
		logger: cfg.Logger,
	}, nil
}

// Save 保存文件到 S3
func (s *S3Storage) Save(ctx context.Context, tenantID, fileID string, reader io.Reader) (*FileMetadata, error) {
	key := s.makeKey(tenantID, fileID)

	// 上传到 S3
	result, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   reader,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload to s3: %w", err)
	}

	now := time.Now()
	metadata := &FileMetadata{
		ID:          fileID,
		Filename:    fileID,
		StoragePath: key,
		TenantID:    tenantID,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    make(map[string]string),
	}

	if result.ETag != nil {
		metadata.Metadata["etag"] = *result.ETag
	}

	s.logger.Debug("file saved to s3",
		zap.String("tenant_id", tenantID),
		zap.String("file_id", fileID),
		zap.String("key", key))

	return metadata, nil
}

// Get 从 S3 获取文件
func (s *S3Storage) Get(ctx context.Context, tenantID, fileID string) (io.ReadCloser, *FileMetadata, error) {
	key := s.makeKey(tenantID, fileID)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, nil, ErrFileNotFound
		}
		return nil, nil, fmt.Errorf("failed to get object from s3: %w", err)
	}

	metadata := &FileMetadata{
		ID:          fileID,
		TenantID:    tenantID,
		StoragePath: key,
	}

	if result.ContentLength != nil {
		metadata.FileSize = *result.ContentLength
	}
	if result.ContentType != nil {
		metadata.ContentType = *result.ContentType
	}

	return result.Body, metadata, nil
}

// Delete 从 S3 删除文件
func (s *S3Storage) Delete(ctx context.Context, tenantID, fileID string) error {
	key := s.makeKey(tenantID, fileID)

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object from s3: %w", err)
	}

	s.logger.Debug("file deleted from s3",
		zap.String("tenant_id", tenantID),
		zap.String("file_id", fileID),
		zap.String("key", key))

	return nil
}

// Exists 检查 S3 文件是否存在
func (s *S3Storage) Exists(ctx context.Context, tenantID, fileID string) (bool, error) {
	key := s.makeKey(tenantID, fileID)

	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// GetURL 获取 S3 预签名 URL
func (s *S3Storage) GetURL(ctx context.Context, tenantID, fileID string, expiry time.Duration) (string, error) {
	key := s.makeKey(tenantID, fileID)

	// 创建预签名客户端
	presignClient := s3.NewPresignClient(s.client)

	result, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(o *s3.PresignOptions) {
		o.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("failed to presign url: %w", err)
	}

	return result.URL, nil
}

func (s *S3Storage) makeKey(tenantID, fileID string) string {
	return fmt.Sprintf("tenants/%s/files/%s/%s", tenantID, fileID[:4], fileID)
}

// isNotFound 判断是否是 404 错误
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return errStr == "not found" || errStr == "The specified key does not exist." || errStr == "The specified bucket does not exist."
}
