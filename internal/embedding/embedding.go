package embedding

import (
	"context"
)

// EmbeddingModel 嵌入模型接口
type EmbeddingModel interface {
	// Generate 生成文本/内容的向量
	Generate(ctx context.Context, content string) ([]float32, error)
	// Dimensions 返回向量维度
	Dimensions() int
	// Name 返回模型名称
	Name() string
}

// EmbeddingConfig 嵌入服务配置
type EmbeddingConfig struct {
	Provider   string `mapstructure:"provider"`    // openai, local, clip
	Model      string `mapstructure:"model"`       // 模型名称
	APIKey     string `mapstructure:"api_key"`     // API 密钥
	APIURL     string `mapstructure:"api_url"`     // API 地址
	Dimensions int    `mapstructure:"dimensions"`  // 向量维度
	BatchSize  int    `mapstructure:"batch_size"`  // 批量处理大小
	Timeout    int    `mapstructure:"timeout"`     // 请求超时 (秒)
}

// ModelType 模型类型
type ModelType string

const (
	ModelTypeText      ModelType = "text"       // 文本嵌入
	ModelTypeImage     ModelType = "image"      // 图片嵌入
	ModelTypeMultimodal ModelType = "multimodal" // 多模态嵌入
)

// ModelInfo 模型信息
type ModelInfo struct {
	Name     string
	Type     ModelType
	Provider string
}
