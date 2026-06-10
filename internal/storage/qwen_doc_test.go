package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQwenDocExtractor_CanHandle(t *testing.T) {
	e := NewQwenDocExtractor(QwenDocExtractorConfig{
		APIURL: "http://localhost:9999/v1/chat/completions",
	})

	tests := []struct {
		contentType string
		expected    bool
	}{
		// PDF
		{"application/pdf", true},
		// 图片
		{"image/jpeg", true},
		{"image/png", true},
		{"image/gif", true},
		{"image/webp", true},
		{"image/tiff", true},
		{"image/bmp", true},
		// Office
		{"application/msword", true},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", true},
		{"application/vnd.ms-excel", true},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", true},
		{"application/vnd.ms-powerpoint", true},
		{"application/vnd.openxmlformats-officedocument.presentationml.presentation", true},
		{"application/rtf", true},
		{"text/rtf", true},
		// 文本
		{"text/plain", true},
		{"text/markdown", true},
		{"text/html", true},
		{"text/csv", true},
		{"application/json", true},
		// 电子书
		{"application/epub+zip", true},
		// 不支持
		{"video/mp4", false},
		{"audio/mpeg", false},
		{"application/octet-stream", false},
		{"image/svg+xml", false},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			assert.Equal(t, tt.expected, e.CanHandle(tt.contentType))
		})
	}
}

func TestQwenDocExtractor_Extract_TextContent(t *testing.T) {
	e := NewQwenDocExtractor(QwenDocExtractorConfig{
		APIURL: "http://localhost:9999/v1/chat/completions",
	})

	ctx := context.Background()

	// 纯文本：直接返回内容，不调用 API
	reader := strings.NewReader("Hello, this is a test document.")
	result, err := e.Extract(ctx, reader, "text/plain")
	assert.NoError(t, err)
	assert.Equal(t, "Hello, this is a test document.", result.Text)
	assert.Equal(t, "direct", result.Metadata["doc_parse_method"])
	assert.Equal(t, "qwen", result.Metadata["doc_parse_provider"])
}

func TestQwenDocExtractor_Extract_HTMLContent(t *testing.T) {
	e := NewQwenDocExtractor(QwenDocExtractorConfig{
		APIURL: "http://localhost:9999/v1/chat/completions",
	})

	ctx := context.Background()

	// HTML：提取纯文本
	html := `<html><body><h1>Title</h1><p>Hello world</p><script>alert(1)</script></body></html>`
	reader := strings.NewReader(html)
	result, err := e.Extract(ctx, reader, "text/html")
	assert.NoError(t, err)
	assert.NotEmpty(t, result.Text)
	assert.NotContains(t, result.Text, "alert")
	assert.Equal(t, "direct", result.Metadata["doc_parse_method"])
}

func TestQwenDocExtractor_Extract_NoAPIURL(t *testing.T) {
	e := NewQwenDocExtractor(QwenDocExtractorConfig{})

	ctx := context.Background()

	// 图片需要 API，无 URL 应返回 error
	reader := strings.NewReader("fake image data")
	_, err := e.Extract(ctx, reader, "image/png")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestQwenDocExtractor_Extract_ImageAPIError(t *testing.T) {
	e := NewQwenDocExtractor(QwenDocExtractorConfig{
		APIURL: "http://localhost:1/v1/chat/completions", // 不可达
	})

	ctx := context.Background()

	reader := strings.NewReader("fake image data")
	result, err := e.Extract(ctx, reader, "image/jpeg")
	assert.NoError(t, err) // Extract 捕获 API error，记录在 metadata
	assert.Equal(t, "", result.Text)
	assert.Contains(t, result.Metadata["doc_parse_error"], "qwen doc")
}

func TestQwenDocExtractor_DefaultModel(t *testing.T) {
	e := NewQwenDocExtractor(QwenDocExtractorConfig{
		APIURL: "http://localhost:9999",
	})
	assert.Equal(t, "qwen-long", e.docModel)
	assert.Equal(t, "qwen3-vl-plus", e.vlModel)

	e2 := NewQwenDocExtractor(QwenDocExtractorConfig{
		APIURL:  "http://localhost:9999",
		Model:   "custom-doc-model",
		VLModel: "custom-vl-model",
	})
	assert.Equal(t, "custom-doc-model", e2.docModel)
	assert.Equal(t, "custom-vl-model", e2.vlModel)
}

func TestQwenDocExtractor_VLFallback(t *testing.T) {
	// VL APIURL/APIKey 未配置时应复用文档模型的配置
	e := NewQwenDocExtractor(QwenDocExtractorConfig{
		APIURL: "http://localhost:9999/v1/chat",
		APIKey: "shared-key",
	})
	assert.Equal(t, "http://localhost:9999/v1/chat", e.vlAPIURL)
	assert.Equal(t, "shared-key", e.vlAPIKey)

	// 单独配置 VL 时应使用独立值
	e2 := NewQwenDocExtractor(QwenDocExtractorConfig{
		APIURL:   "http://doc:9999/v1/chat",
		APIKey:   "doc-key",
		VLAPIURL: "http://vl:8888/v1/chat",
		VLAPIKey: "vl-key",
	})
	assert.Equal(t, "http://vl:8888/v1/chat", e2.vlAPIURL)
	assert.Equal(t, "vl-key", e2.vlAPIKey)
}

func TestExtractTextFromHTML(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		contains string
		excludes string
	}{
		{
			name:     "simple text",
			html:     "<p>Hello World</p>",
			contains: "Hello",
		},
		{
			name:     "script excluded",
			html:     "<p>Text</p><script>var x=1;</script>",
			contains: "Text",
			excludes: "var",
		},
		{
			name:     "style excluded",
			html:     "<p>Content</p><style>.cls{color:red}</style>",
			contains: "Content",
			excludes: "color",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTextFromHTML(tt.html)
			if tt.contains != "" {
				assert.Contains(t, result, tt.contains)
			}
			if tt.excludes != "" {
				assert.NotContains(t, result, tt.excludes)
			}
		})
	}
}

func TestIsImageContent(t *testing.T) {
	assert.True(t, isImageContent("image/jpeg"))
	assert.True(t, isImageContent("image/png"))
	assert.True(t, isImageContent("image/tiff"))
	assert.False(t, isImageContent("application/pdf"))
	assert.False(t, isImageContent("text/plain"))
}

func TestIsDocumentContent(t *testing.T) {
	assert.True(t, isDocumentContent("application/pdf"))
	assert.True(t, isDocumentContent("application/msword"))
	assert.True(t, isDocumentContent("application/vnd.openxmlformats-officedocument.wordprocessingml.document"))
	assert.True(t, isDocumentContent("application/vnd.ms-excel"))
	assert.True(t, isDocumentContent("application/epub+zip"))
	assert.True(t, isDocumentContent("application/rtf"))
	assert.False(t, isDocumentContent("image/jpeg"))
	assert.False(t, isDocumentContent("text/plain"))
	assert.False(t, isDocumentContent("application/json"))
}

func TestIsTextContent(t *testing.T) {
	assert.True(t, isTextContent("text/plain"))
	assert.True(t, isTextContent("text/html"))
	assert.True(t, isTextContent("application/json"))
	assert.False(t, isTextContent("image/png"))
	assert.False(t, isTextContent("application/pdf"))
}

func TestBaseURLFrom(t *testing.T) {
	tests := []struct {
		name     string
		apiURL   string
		expected string
	}{
		{
			name:     "dashscope compatible mode",
			apiURL:   "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
			expected: "https://dashscope.aliyuncs.com",
		},
		{
			name:     "custom endpoint with /v1",
			apiURL:   "http://localhost:8080/v1/chat/completions",
			expected: "http://localhost:8080",
		},
		{
			name:     "bare host",
			apiURL:   "https://api.example.com",
			expected: "https://api.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, baseURLFrom(tt.apiURL))
		})
	}
}

func TestQwenDocExtractor_Extract_DocumentWithMockServer(t *testing.T) {
	// Start a mock HTTP server that handles file upload and chat completions
	mux := http.NewServeMux()

	// File upload endpoint (OpenAI compatible-mode)
	mux.HandleFunc("/compatible-mode/v1/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			// OpenAI-compatible response format
			_, _ = fmt.Fprintf(w, `{"id":"file-fe-test-123","object":"file","bytes":1024,"filename":"doc.pdf","purpose":"file-extract","status":"processed","created_at":1781084839}`)
			return
		}
		w.WriteHeader(405)
	})

	// File delete endpoint
	mux.HandleFunc("/compatible-mode/v1/files/file-fe-test-123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = fmt.Fprintf(w, `{"id":"file-fe-test-123","deleted":true}`)
			return
		}
		w.WriteHeader(405)
	})

	// Chat completions endpoint
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Verify the message contains fileid:// reference in system message
		messages := req["messages"].([]interface{})

		// First message should be system with fileid://
		sysMsg := messages[0].(map[string]interface{})
		assert.Equal(t, "system", sysMsg["role"])
		assert.Equal(t, "fileid://file-fe-test-123", sysMsg["content"])

		// Second message should be user
		userMsg := messages[1].(map[string]interface{})
		assert.Equal(t, "user", userMsg["role"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"content":"Extracted PDF content from mock"}}]}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	e := NewQwenDocExtractor(QwenDocExtractorConfig{
		APIURL: server.URL + "/v1/chat/completions",
		APIKey: "test-key",
	})

	ctx := context.Background()
	reader := strings.NewReader("fake pdf data")
	result, err := e.Extract(ctx, reader, "application/pdf")

	assert.NoError(t, err)
	assert.Equal(t, "Extracted PDF content from mock", result.Text)
	assert.Equal(t, "qwen", result.Metadata["doc_parse_provider"])
}

func TestQwenDocExtractor_Extract_DocumentUploadFailure(t *testing.T) {
	// Mock server that rejects file uploads
	mux := http.NewServeMux()
	mux.HandleFunc("/compatible-mode/v1/files", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = fmt.Fprintf(w, `{"error":"upload failed"}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	e := NewQwenDocExtractor(QwenDocExtractorConfig{
		APIURL: server.URL + "/v1/chat/completions",
		APIKey: "test-key",
	})

	ctx := context.Background()
	reader := strings.NewReader("fake pdf data")
	result, err := e.Extract(ctx, reader, "application/pdf")

	// Upload failure is captured in metadata, not returned as error
	assert.NoError(t, err)
	assert.Equal(t, "", result.Text)
	assert.Contains(t, result.Metadata["doc_parse_error"], "upload")
}

func TestQwenDocExtractor_Extract_ModelRouting(t *testing.T) {
	// 验证图片走 VL 模型，文档走 doc 模型
	var receivedModel string

	mux := http.NewServeMux()

	// 文件上传（文档模型需要，使用 compatible-mode 端点）
	mux.HandleFunc("/compatible-mode/v1/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = fmt.Fprintf(w, `{"id":"file-fe-abc","object":"file","bytes":1024,"filename":"doc.pdf","purpose":"file-extract","status":"processed"}`)
			return
		}
		w.WriteHeader(405)
	})
	mux.HandleFunc("/compatible-mode/v1/files/file-fe-abc", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = fmt.Fprintf(w, `{"id":"file-fe-abc","deleted":true}`)
	})

	// Chat completions：记录请求中的 model 字段
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		receivedModel = req["model"].(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"content":"extracted"}}]}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	e := NewQwenDocExtractor(QwenDocExtractorConfig{
		APIURL:  server.URL + "/v1/chat/completions",
		APIKey:  "doc-key",
		Model:   "qwen-long",
		VLModel: "qwen3-vl-plus",
	})

	ctx := context.Background()

	// 测试图片 → VL 模型
	receivedModel = ""
	reader := strings.NewReader("fake image")
	result, err := e.Extract(ctx, reader, "image/png")
	assert.NoError(t, err)
	assert.Equal(t, "qwen3-vl-plus", receivedModel)
	assert.Equal(t, "qwen3-vl-plus", result.Metadata["doc_parse_model"])

	// 测试 PDF → 文档模型
	receivedModel = ""
	reader = strings.NewReader("fake pdf")
	result, err = e.Extract(ctx, reader, "application/pdf")
	assert.NoError(t, err)
	assert.Equal(t, "qwen-long", receivedModel)
	assert.Equal(t, "qwen-long", result.Metadata["doc_parse_model"])
}
