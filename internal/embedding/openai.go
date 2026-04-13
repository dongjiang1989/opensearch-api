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

// OpenAIEmbedding OpenAI 嵌入服务
type OpenAIEmbedding struct {
	apiKey     string
	apiURL     string
	model      string
	dimensions int
	httpClient *http.Client
}

// OpenAIEmbeddingConfig OpenAI 嵌入配置
type OpenAIEmbeddingConfig struct {
	APIKey     string
	APIURL     string
	Model      string
	Dimensions int
	Timeout    time.Duration
}

// openAIRequest OpenAI API 请求
type openAIRequest struct {
	Input      string `json:"input"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions,omitempty"`
}

// openAIResponse OpenAI API 响应
type openAIResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// NewOpenAIEmbedding 创建 OpenAI 嵌入服务
func NewOpenAIEmbedding(cfg OpenAIEmbeddingConfig) *OpenAIEmbedding {
	if cfg.APIURL == "" {
		cfg.APIURL = "https://api.openai.com/v1/embeddings"
	}
	if cfg.Model == "" {
		cfg.Model = "text-embedding-3-small"
	}
	if cfg.Dimensions == 0 {
		// 默认维度，text-embedding-3-small 支持 512, 1536
		cfg.Dimensions = 1536
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &OpenAIEmbedding{
		apiKey: cfg.APIKey,
		apiURL: cfg.APIURL,
		model:  cfg.Model,
		dimensions: cfg.Dimensions,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Name 返回模型名称
func (e *OpenAIEmbedding) Name() string {
	return e.model
}

// Dimensions 返回向量维度
func (e *OpenAIEmbedding) Dimensions() int {
	return e.dimensions
}

// Generate 生成文本嵌入向量
func (e *OpenAIEmbedding) Generate(ctx context.Context, content string) ([]float32, error) {
	reqBody := openAIRequest{
		Input: content,
		Model: e.model,
	}

	// 如果模型支持指定维度，则添加维度参数
	if e.dimensions > 0 {
		reqBody.Dimensions = e.dimensions
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data in response")
	}

	embedding := apiResp.Data[0].Embedding
	if len(embedding) > 0 {
		e.dimensions = len(embedding)
	}

	return embedding, nil
}

// GenerateBatch 批量生成嵌入向量
func (e *OpenAIEmbedding) GenerateBatch(ctx context.Context, contents []string) ([][]float32, error) {
	// 对于批量请求，使用 inputs 数组
	body, err := json.Marshal(map[string]interface{}{
		"input":      contents,
		"model":      e.model,
		"dimensions": e.dimensions,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// 按 index 排序结果
	result := make([][]float32, len(contents))
	for _, item := range apiResp.Data {
		if item.Index < len(result) {
			result[item.Index] = item.Embedding
		}
	}

	return result, nil
}
