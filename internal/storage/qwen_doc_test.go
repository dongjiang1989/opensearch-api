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
	assert.Equal(t, "qwen3.7-plus", e.model)

	e2 := NewQwenDocExtractor(QwenDocExtractorConfig{
		APIURL: "http://localhost:9999",
		Model:  "custom-model",
	})
	assert.Equal(t, "custom-model", e2.model)
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

func TestIsTextContent(t *testing.T) {
	assert.True(t, isTextContent("text/plain"))
	assert.True(t, isTextContent("text/html"))
	assert.True(t, isTextContent("application/json"))
	assert.False(t, isTextContent("image/png"))
	assert.False(t, isTextContent("application/pdf"))
}

func TestQwenDocExtractor_baseURL(t *testing.T) {
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
			e := NewQwenDocExtractor(QwenDocExtractorConfig{APIURL: tt.apiURL})
			assert.Equal(t, tt.expected, e.baseURL())
		})
	}
}

func TestQwenDocExtractor_Extract_DocumentWithMockServer(t *testing.T) {
	// Start a mock HTTP server that handles file upload and chat completions
	mux := http.NewServeMux()

	// File upload endpoint
	mux.HandleFunc("/api/v1/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = fmt.Fprintf(w, `{"data":{"uploaded_files":[{"name":"doc.pdf","file_id":"file-test-123"}],"failed_uploads":[]},"request_id":"mock-req-001"}`)
			return
		}
		w.WriteHeader(405)
	})

	// File delete endpoint
	mux.HandleFunc("/api/v1/files/file-test-123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = fmt.Fprintf(w, `{"id":"file-test-123","deleted":true}`)
			return
		}
		w.WriteHeader(405)
	})

	// Chat completions endpoint
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Verify the message contains file_id reference
		messages := req["messages"].([]interface{})
		msg := messages[0].(map[string]interface{})
		content := msg["content"].([]interface{})

		hasFile := false
		for _, item := range content {
			m := item.(map[string]interface{})
			if m["type"] == "file" {
				hasFile = true
				assert.Equal(t, "file-test-123", m["file"])
			}
		}
		assert.True(t, hasFile, "chat request should contain file reference")

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
	mux.HandleFunc("/api/v1/files", func(w http.ResponseWriter, r *http.Request) {
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
