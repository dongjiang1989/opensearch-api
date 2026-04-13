package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// CLIPEmbedding CLIP 多模态嵌入服务（支持图片和文本）
type CLIPEmbedding struct {
	apiURL     string
	dimensions int
	httpClient *http.Client
}

// CLIPEmbeddingConfig CLIP 嵌入配置
type CLIPEmbeddingConfig struct {
	APIURL     string        // CLIP 服务地址，如 http://localhost:8000
	Dimensions int           // 向量维度，CLIP 通常为 512 或 768
	Timeout    time.Duration // 请求超时
}

// clipTextRequest 文本嵌入请求
type clipTextRequest struct {
	Text string `json:"text"`
}

// clipImageRequest 图片嵌入请求
type clipImageRequest struct {
	ImageURL string `json:"image_url,omitempty"`
}

// clipResponse CLIP API 响应
type clipResponse struct {
	Embedding []float32 `json:"embedding"`
	Model     string    `json:"model"`
}

// NewCLIPEmbedding 创建 CLIP 嵌入服务
func NewCLIPEmbedding(cfg CLIPEmbeddingConfig) *CLIPEmbedding {
	if cfg.APIURL == "" {
		cfg.APIURL = "http://localhost:8000"
	}
	if cfg.Dimensions == 0 {
		cfg.Dimensions = 512 // CLIP ViT-B/32 的默认维度
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	return &CLIPEmbedding{
		apiURL: cfg.APIURL,
		dimensions: cfg.Dimensions,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Name 返回模型名称
func (e *CLIPEmbedding) Name() string {
	return "clip"
}

// Dimensions 返回向量维度
func (e *CLIPEmbedding) Dimensions() int {
	return e.dimensions
}

// Generate 生成文本嵌入向量（文本到 CLIP 空间）
func (e *CLIPEmbedding) Generate(ctx context.Context, content string) ([]float32, error) {
	apiURL := fmt.Sprintf("%s/embed/text", e.apiURL)

	reqBody := clipTextRequest{
		Text: content,
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

	var apiResp clipResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	embedding := apiResp.Embedding
	if len(embedding) > 0 {
		e.dimensions = len(embedding)
	}

	return embedding, nil
}

// GenerateImage 生成图片嵌入向量
func (e *CLIPEmbedding) GenerateImage(ctx context.Context, imageData []byte, contentType string) ([]float32, error) {
	apiURL := fmt.Sprintf("%s/embed/image", e.apiURL)

	// 使用 multipart/form-data 上传图片
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("image", "image.jpg")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(imageData); err != nil {
		return nil, fmt.Errorf("failed to write image data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp clipResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return apiResp.Embedding, nil
}

// GenerateImageFromURL 从 URL 生成图片嵌入向量
func (e *CLIPEmbedding) GenerateImageFromURL(ctx context.Context, imageURL string) ([]float32, error) {
	apiURL := fmt.Sprintf("%s/embed/image", e.apiURL)

	reqBody := clipImageRequest{
		ImageURL: imageURL,
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

	var apiResp clipResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return apiResp.Embedding, nil
}
