package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LocalEmbedding 本地嵌入服务（兼容 ollama 等本地模型服务）
type LocalEmbedding struct {
	apiURL     string
	model      string
	dimensions int
	httpClient *http.Client
}

// LocalEmbeddingConfig 本地嵌入配置
type LocalEmbeddingConfig struct {
	APIURL     string        // 本地服务地址，如 http://localhost:11434
	Model      string        // 模型名称
	Dimensions int           // 向量维度
	Timeout    time.Duration // 请求超时
}

// localRequest 本地 API 请求（ollama 格式）
type localRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// localResponse 本地 API 响应（ollama 格式）
type localResponse struct {
	Model     string    `json:"model"`
	Embedding []float32 `json:"embedding"`
}

// NewLocalEmbedding 创建本地嵌入服务
func NewLocalEmbedding(cfg LocalEmbeddingConfig) *LocalEmbedding {
	if cfg.APIURL == "" {
		cfg.APIURL = "http://localhost:11434"
	}
	if cfg.Model == "" {
		cfg.Model = "all-minilm"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	return &LocalEmbedding{
		apiURL: cfg.APIURL,
		model:  cfg.Model,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Name 返回模型名称
func (e *LocalEmbedding) Name() string {
	return e.model
}

// Dimensions 返回向量维度
func (e *LocalEmbedding) Dimensions() int {
	return e.dimensions
}

// Generate 生成文本嵌入向量
func (e *LocalEmbedding) Generate(ctx context.Context, content string) ([]float32, error) {
	// ollama embeddings API
	apiURL := fmt.Sprintf("%s/api/embeddings", e.apiURL)

	reqBody := localRequest{
		Model:  e.model,
		Prompt: content,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp localResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	embedding := apiResp.Embedding
	if len(embedding) > 0 {
		e.dimensions = len(embedding)
	}

	return embedding, nil
}
