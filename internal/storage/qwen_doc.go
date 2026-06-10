package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/scanner"
	"time"
)

// QwenDocExtractor Qwen3.7-Plus 统一文档解析提取器
// 支持 PDF（含扫描件）、图片、Office、文本等全格式
// 通过 DashScope OpenAI 兼容 API 调用
type QwenDocExtractor struct {
	apiURL     string // DashScope chat completions 地址
	apiKey     string // DashScope API 密钥
	model      string // 模型名称，默认 qwen3.7-plus
	httpClient *http.Client
}

// QwenDocExtractorConfig QwenDocExtractor 配置
type QwenDocExtractorConfig struct {
	APIURL string // DashScope chat completions 地址
	APIKey string // DashScope API 密钥
	Model  string // 模型名称，默认 qwen3.7-plus
}

// NewQwenDocExtractor 创建 Qwen3.7-Plus 文档解析提取器
func NewQwenDocExtractor(cfg QwenDocExtractorConfig) *QwenDocExtractor {
	model := cfg.Model
	if model == "" {
		model = "qwen3.7-plus"
	}
	return &QwenDocExtractor{
		apiURL: cfg.APIURL,
		apiKey: cfg.APIKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // 文档解析可能较慢
		},
	}
}

// CanHandle 判断是否是支持的文件类型
func (e *QwenDocExtractor) CanHandle(contentType string) bool {
	switch contentType {
	// PDF
	case "application/pdf":
		return true
	// 图片
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/tiff", "image/bmp":
		return true
	// Office
	case "application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/rtf", "text/rtf":
		return true
	// 文本
	case "text/plain", "text/markdown", "text/html", "text/csv", "application/json":
		return true
	// 电子书
	case "application/epub+zip":
		return true
	}
	return false
}

// Extract 提取文档内容
func (e *QwenDocExtractor) Extract(ctx context.Context, reader io.Reader, contentType string) (*ExtractedContent, error) {
	if e.apiURL == "" {
		return nil, fmt.Errorf("qwen doc parse API URL is not configured")
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}

	metadata := map[string]interface{}{
		"doc_parse_provider": "qwen",
		"doc_parse_model":    e.model,
		"content_type":       contentType,
		"size":               len(data),
	}

	var text string

	if isTextContent(contentType) {
		// 纯文本类型：直接返回内容，不调用 API
		text = string(data)
		if contentType == "text/html" {
			text = extractTextFromHTML(text)
		}
		metadata["doc_parse_method"] = "direct"
	} else if isImageContent(contentType) {
		// 图片类型：base64 data URL → image_url
		text, err = e.parseImage(ctx, data, contentType)
	} else {
		// PDF / Office / EPUB：base64 data URL → file
		text, err = e.parseDocument(ctx, data, contentType)
	}

	if err != nil {
		metadata["doc_parse_error"] = err.Error()
		return &ExtractedContent{
			Text:     "",
			Metadata: metadata,
		}, nil
	}

	return &ExtractedContent{
		Text:     text,
		Metadata: metadata,
	}, nil
}

// parseImage 使用 base64 编码解析图片
func (e *QwenDocExtractor) parseImage(ctx context.Context, data []byte, contentType string) (string, error) {
	b64 := base64.StdEncoding.EncodeToString(data)
	dataURL := fmt.Sprintf("data:%s;base64,%s", contentType, b64)

	return e.performChat(ctx, []interface{}{
		map[string]interface{}{
			"type":      "image_url",
			"image_url": map[string]interface{}{"url": dataURL},
		},
		map[string]interface{}{
			"type": "text",
			"text": "请提取这张图片中的所有文字内容。如果没有文字，请描述图片的主要内容。",
		},
	})
}

// parseDocument 使用 base64 编码解析 PDF/Office 文档
func (e *QwenDocExtractor) parseDocument(ctx context.Context, data []byte, contentType string) (string, error) {
	b64 := base64.StdEncoding.EncodeToString(data)
	dataURL := fmt.Sprintf("data:%s;base64,%s", contentType, b64)

	prompt := "请提取这个文档中的所有文字内容，包括表格、公式等结构化内容。请保持原文的段落和层次结构。"

	return e.performChat(ctx, []interface{}{
		map[string]interface{}{
			"type": "file",
			"file": map[string]interface{}{"url": dataURL},
		},
		map[string]interface{}{
			"type": "text",
			"text": prompt,
		},
	})
}

// performChat 调用 OpenAI 兼容 chat completions API
func (e *QwenDocExtractor) performChat(ctx context.Context, content []interface{}) (string, error) {
	reqBody := map[string]interface{}{
		"model": e.model,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": content,
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal qwen doc request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create qwen doc request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("qwen doc request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("qwen doc API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析 OpenAI 兼容格式的响应
	type chatResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode qwen doc response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("qwen doc API returned no choices")
	}

	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}

// isImageContent 判断是否是图片类型
func isImageContent(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/tiff", "image/bmp":
		return true
	}
	return false
}

// isTextContent 判断是否是纯文本类型
func isTextContent(contentType string) bool {
	switch contentType {
	case "text/plain", "text/markdown", "text/html", "text/csv", "application/json":
		return true
	}
	return false
}

// extractTextFromHTML 从 HTML 中提取纯文本
func extractTextFromHTML(html string) string {
	var text strings.Builder
	s := &scanner.Scanner{}
	s.Init(strings.NewReader(html))
	s.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanRawStrings | scanner.ScanComments

	inScript := false
	inStyle := false
	inTag := false

	for {
		token := s.Scan()
		if token == scanner.EOF {
			break
		}

		literal := s.TokenText()

		if literal == "<" {
			inTag = true
			continue
		}
		if literal == ">" {
			inTag = false
			continue
		}

		if inTag {
			if strings.HasPrefix(literal, "script") {
				inScript = true
			} else if strings.HasPrefix(literal, "/script") {
				inScript = false
			} else if strings.HasPrefix(literal, "style") {
				inStyle = true
			} else if strings.HasPrefix(literal, "/style") {
				inStyle = false
			}
			continue
		}

		if inScript || inStyle {
			continue
		}

		if token == scanner.Ident || token == scanner.String {
			text.WriteString(literal)
			text.WriteString(" ")
		}
	}

	return strings.TrimSpace(text.String())
}
