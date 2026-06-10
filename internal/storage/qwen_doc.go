package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"text/scanner"
	"time"

	"github.com/dongjiang1989/opensearch-api/internal/metrics"
)

// QwenDocExtractor Qwen 双模型文档解析提取器
// PDF/Office 文档 → docModel (qwen-long，支持文件上传 file_id)
// Image/Text       → vlModel (qwen3-vl-plus，支持 image_url/text)
// 通过 DashScope OpenAI 兼容 API 调用
type QwenDocExtractor struct {
	// 文档解析模型（PDF/Office）
	docAPIURL string // DashScope chat completions 地址
	docAPIKey string // DashScope API 密钥
	docModel  string // 模型名称，默认 qwen-long

	// 视觉解析模型（Image）
	vlAPIURL string // 视觉模型 API 地址（可选，默认同 docAPIURL）
	vlAPIKey string // 视觉模型 API Key（可选，默认同 docAPIKey）
	vlModel  string // 视觉模型名称，默认 qwen3-vl-plus

	httpClient *http.Client
}

// QwenDocExtractorConfig QwenDocExtractor 配置
type QwenDocExtractorConfig struct {
	// 文档解析模型（PDF/Office）
	APIURL string // DashScope chat completions 地址
	APIKey string // DashScope API 密钥
	Model  string // 文档模型名称，默认 qwen-long

	// 视觉解析模型（Image）
	VLAPIURL string // 视觉模型 API 地址（可选，默认同 APIURL）
	VLAPIKey string // 视觉模型 API Key（可选，默认同 APIKey）
	VLModel  string // 视觉模型名称，默认 qwen3-vl-plus
}

// NewQwenDocExtractor 创建 Qwen 双模型文档解析提取器
func NewQwenDocExtractor(cfg QwenDocExtractorConfig) *QwenDocExtractor {
	docModel := cfg.Model
	if docModel == "" {
		docModel = "qwen-long"
	}
	vlModel := cfg.VLModel
	if vlModel == "" {
		vlModel = "qwen3-vl-plus"
	}

	// VL API 地址/Key 未配置时复用文档模型的配置
	vlAPIURL := cfg.VLAPIURL
	if vlAPIURL == "" {
		vlAPIURL = cfg.APIURL
	}
	vlAPIKey := cfg.VLAPIKey
	if vlAPIKey == "" {
		vlAPIKey = cfg.APIKey
	}

	return &QwenDocExtractor{
		docAPIURL: cfg.APIURL,
		docAPIKey: cfg.APIKey,
		docModel:  docModel,
		vlAPIURL:  vlAPIURL,
		vlAPIKey:  vlAPIKey,
		vlModel:   vlModel,
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
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}

	// 根据文件类型选择模型
	var modelUsed string
	if isDocumentContent(contentType) {
		modelUsed = e.docModel
	} else {
		modelUsed = e.vlModel
	}

	metadata := map[string]interface{}{
		"doc_parse_provider": "qwen",
		"doc_parse_model":    modelUsed,
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
		// 图片类型：使用 VL 模型 (qwen3-vl-plus)
		if e.vlAPIURL == "" {
			return nil, fmt.Errorf("qwen VL API URL is not configured")
		}
		text, err = e.parseImage(ctx, data, contentType)
	} else {
		// PDF / Office / EPUB：使用文档模型 (qwen-long)
		if e.docAPIURL == "" {
			return nil, fmt.Errorf("qwen doc parse API URL is not configured")
		}
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

// parseImage 使用 VL 模型 (qwen3-vl-plus) + base64 编码解析图片
func (e *QwenDocExtractor) parseImage(ctx context.Context, data []byte, contentType string) (string, error) {
	b64 := base64.StdEncoding.EncodeToString(data)
	dataURL := fmt.Sprintf("data:%s;base64,%s", contentType, b64)

	return e.performChat(ctx, e.vlAPIURL, e.vlAPIKey, e.vlModel, []interface{}{
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

// parseDocument 使用文档模型 (qwen-long) 上传文件到 DashScope 后通过 fileid:// 引用解析 PDF/Office 文档
func (e *QwenDocExtractor) parseDocument(ctx context.Context, data []byte, contentType string) (string, error) {
	// Step 1: 上传文件到 DashScope (OpenAI compatible endpoint) 获取 file_id
	filename := "document" + extensionFromContentType(contentType)
	fileID, err := e.uploadFile(ctx, data, filename)
	if err != nil {
		return "", fmt.Errorf("failed to upload document to DashScope: %w", err)
	}
	defer e.deleteFile(ctx, fileID)

	// Step 2: 通过 fileid:// 在 system message 中引用已上传文件
	prompt := "请提取这个文档中的所有文字内容，包括表格、公式等结构化内容。请保持原文的段落和层次结构。"

	messages := []map[string]interface{}{
		{
			"role":    "system",
			"content": fmt.Sprintf("fileid://%s", fileID),
		},
		{
			"role":    "user",
			"content": prompt,
		},
	}

	return e.performChatMessages(ctx, e.docAPIURL, e.docAPIKey, e.docModel, messages)
}

// baseURLFrom 从 apiURL 中提取 DashScope 基础 URL（用于文件上传/删除）
func baseURLFrom(apiURL string) string {
	u, err := url.Parse(apiURL)
	if err != nil {
		// fallback: 截取到 /compatible-mode 或 /v1
		if idx := strings.Index(apiURL, "/compatible-mode"); idx > 0 {
			return apiURL[:idx]
		}
		if idx := strings.Index(apiURL, "/v1"); idx > 0 {
			return apiURL[:idx]
		}
		return apiURL
	}
	return u.Scheme + "://" + u.Host
}

// uploadFile 上传文件到 DashScope compatible-mode /v1/files，返回 file_id
// 必须使用 OpenAI compatible 端点，返回的 file_id（如 file-fe-xxx）才能通过 fileid:// 协议在 chat 中引用
func (e *QwenDocExtractor) uploadFile(ctx context.Context, data []byte, filename string) (string, error) {
	uploadURL := baseURLFrom(e.docAPIURL) + "/compatible-mode/v1/files"

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := fw.Write(data); err != nil {
		return "", fmt.Errorf("write file data: %w", err)
	}

	if err := w.WriteField("purpose", "file-extract"); err != nil {
		return "", fmt.Errorf("write purpose field: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &buf)
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	if e.docAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.docAPIKey)
	}

	uploadStart := time.Now()
	resp, err := e.httpClient.Do(req)
	metrics.Observe("dashscope", "upload_file", time.Since(uploadStart), err)
	if err != nil {
		return "", fmt.Errorf("upload request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DashScope file upload returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		// OpenAI-compatible format (compatible-mode/v1/files): {"id": "file-fe-xxx", ...}
		ID     string `json:"id"`
		Object string `json:"object"`
		Status string `json:"status"`
		// DashScope native format fallback: {"data": {"uploaded_files": [{"file_id": "..."}]}}
		Data struct {
			UploadedFiles []struct {
				Name   string `json:"name"`
				FileID string `json:"file_id"`
			} `json:"uploaded_files"`
			FailedUploads []interface{} `json:"failed_uploads"`
		} `json:"data"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse upload response: %w", err)
	}

	// OpenAI-compatible format (primary): {"id": "file-fe-xxx"}
	var fileID string
	if result.ID != "" {
		fileID = result.ID
	}
	// DashScope native format fallback: data.uploaded_files[0].file_id
	if fileID == "" && len(result.Data.UploadedFiles) > 0 {
		fileID = result.Data.UploadedFiles[0].FileID
	}
	if fileID == "" {
		fileID = result.ID
	}

	if fileID == "" {
		return "", fmt.Errorf("DashScope file upload returned empty file id, response: %s", string(respBody))
	}

	return fileID, nil
}

// deleteFile 删除 DashScope 上已上传的文件（best-effort cleanup）
func (e *QwenDocExtractor) deleteFile(ctx context.Context, fileID string) {
	deleteURL := baseURLFrom(e.docAPIURL) + "/compatible-mode/v1/files/" + fileID

	req, err := http.NewRequestWithContext(ctx, "DELETE", deleteURL, nil)
	if err != nil {
		return
	}
	if e.docAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.docAPIKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

// performChat 调用 OpenAI 兼容 chat completions API（支持指定模型/地址/密钥）
func (e *QwenDocExtractor) performChat(ctx context.Context, apiURL, apiKey, model string, content []interface{}) (string, error) {
	reqBody := map[string]interface{}{
		"model": model,
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

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create qwen doc request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	chatStart := time.Now()
	resp, err := e.httpClient.Do(req)
	metrics.Observe("dashscope", "chat", time.Since(chatStart), err)
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

// performChatMessages 调用 OpenAI 兼容 chat completions API（接受完整 messages 数组）
// 用于文档解析场景，需要在 system message 中通过 fileid:// 引用已上传文件
func (e *QwenDocExtractor) performChatMessages(ctx context.Context, apiURL, apiKey, model string, messages []map[string]interface{}) (string, error) {
	reqBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal qwen doc request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create qwen doc request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	chatStart := time.Now()
	resp, err := e.httpClient.Do(req)
	metrics.Observe("dashscope", "chat", time.Since(chatStart), err)
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

// isDocumentContent 判断是否是文档类型（PDF/Office/EPUB），需要走 qwen-long
func isDocumentContent(contentType string) bool {
	switch contentType {
	case "application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/rtf", "text/rtf",
		"application/epub+zip":
		return true
	}
	return false
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

// extensionFromContentType 根据 MIME 类型返回文件扩展名
func extensionFromContentType(contentType string) string {
	switch contentType {
	case "application/pdf":
		return ".pdf"
	case "application/msword":
		return ".doc"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.ms-excel":
		return ".xls"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "application/vnd.ms-powerpoint":
		return ".ppt"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return ".pptx"
	case "application/rtf", "text/rtf":
		return ".rtf"
	case "application/epub+zip":
		return ".epub"
	default:
		return ""
	}
}
