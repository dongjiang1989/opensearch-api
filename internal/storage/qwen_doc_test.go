package storage

import (
	"context"
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
