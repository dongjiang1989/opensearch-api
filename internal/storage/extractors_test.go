package storage

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPDFExtractor_CanHandle(t *testing.T) {
	extractor := NewPDFExtractor()

	assert.True(t, extractor.CanHandle("application/pdf"))
	assert.False(t, extractor.CanHandle("text/plain"))
	assert.False(t, extractor.CanHandle("image/jpeg"))
}

func TestPDFExtractor_Extract(t *testing.T) {
	extractor := NewPDFExtractor()
	ctx := context.Background()

	t.Run("Invalid PDF", func(t *testing.T) {
		data := []byte("not a pdf file")
		reader := bytes.NewReader(data)

		result, err := extractor.Extract(ctx, reader, "application/pdf")
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "", result.Text)
		assert.Contains(t, result.Metadata, "error")
	})

	t.Run("Valid PDF header", func(t *testing.T) {
		// 简单的 PDF 文件头
		data := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF")
		reader := bytes.NewReader(data)

		result, err := extractor.Extract(ctx, reader, "application/pdf")
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "pdf", result.Metadata["format"])
	})

	t.Run("PDF with BT/ET text", func(t *testing.T) {
		// 包含 BT/ET 文本块的简化 PDF
		data := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Page /Contents 2 0 R >>\nendobj\n2 0 obj\n<< /Length 44 >>\nstream\nBT\n(Hello World) Tj\nET\nendstream\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF")
		reader := bytes.NewReader(data)

		result, err := extractor.Extract(ctx, reader, "application/pdf")
		require.NoError(t, err)
		assert.NotNil(t, result)
		// 简化实现可能提取到部分文本
		assert.NotNil(t, result.Metadata)
	})
}

func TestExtractTextFromPDF(t *testing.T) {
	t.Run("Empty data", func(t *testing.T) {
		data := []byte{}
		text, err := ExtractTextFromPDF(data)
		assert.NoError(t, err)
		assert.Equal(t, "", text)
	})

	t.Run("Invalid PDF", func(t *testing.T) {
		data := []byte("not a pdf")
		text, err := ExtractTextFromPDF(data)
		assert.NoError(t, err)
		assert.Equal(t, "", text)
	})
}

func TestValidatePDF(t *testing.T) {
	t.Run("Valid PDF header", func(t *testing.T) {
		data := []byte("%PDF-1.4")
		err := ValidatePDF(data)
		assert.NoError(t, err)
	})

	t.Run("Invalid header", func(t *testing.T) {
		data := []byte("not a pdf")
		err := ValidatePDF(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a valid PDF")
	})

	t.Run("Empty data", func(t *testing.T) {
		data := []byte{}
		err := ValidatePDF(data)
		assert.Error(t, err)
	})
}

func TestTextExtractor_CanHandle(t *testing.T) {
	extractor := NewTextExtractor(TextExtractorConfig{})

	tests := []struct {
		contentType string
		expected    bool
	}{
		{"text/plain", true},
		{"text/markdown", true},
		{"text/html", true},
		{"application/json", true},
		{"text/csv", true},
		{"file.txt", true},
		{"file.md", true},
		{"file.json", true},
		{"image/jpeg", false},
		{"application/pdf", false},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractor.CanHandle(tt.contentType))
		})
	}
}

func TestTextExtractor_Extract(t *testing.T) {
	extractor := NewTextExtractor(TextExtractorConfig{MaxSize: 1024})
	ctx := context.Background()

	t.Run("Plain text", func(t *testing.T) {
		data := "Hello, World!"
		reader := strings.NewReader(data)

		result, err := extractor.Extract(ctx, reader, "text/plain")
		require.NoError(t, err)
		assert.Equal(t, data, result.Text)
		assert.Equal(t, "text/plain", result.Metadata["content_type"])
	})

	t.Run("HTML text", func(t *testing.T) {
		html := "<html><body><p>Hello World</p></body></html>"
		reader := strings.NewReader(html)

		result, err := extractor.Extract(ctx, reader, "text/html")
		require.NoError(t, err)
		// HTML 提取应该去除标签
		assert.NotEqual(t, html, result.Text)
	})

	t.Run("JSON text", func(t *testing.T) {
		json := `{"key": "value"}`
		reader := strings.NewReader(json)

		result, err := extractor.Extract(ctx, reader, "application/json")
		require.NoError(t, err)
		assert.Equal(t, json, result.Text)
	})

	t.Run("CSV text", func(t *testing.T) {
		csv := "name,age\nJohn,30\nJane,25"
		reader := strings.NewReader(csv)

		result, err := extractor.Extract(ctx, reader, "text/csv")
		require.NoError(t, err)
		assert.Equal(t, csv, result.Text)
	})
}

func TestDocumentExtractor_CanHandle(t *testing.T) {
	extractor := NewDocumentExtractor()

	tests := []struct {
		contentType string
		expected    bool
	}{
		{"application/msword", true},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", true},
		{"application/vnd.ms-excel", true},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", true},
		{"application/vnd.ms-powerpoint", true},
		{"application/rtf", true},
		{"text/rtf", true},
		{"text/plain", false},
		{"application/pdf", false},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractor.CanHandle(tt.contentType))
		})
	}
}

func TestDocumentExtractor_Extract_RTF(t *testing.T) {
	extractor := NewDocumentExtractor()
	ctx := context.Background()

	t.Run("Simple RTF", func(t *testing.T) {
		rtf := `{\rtf1 Hello World}`
		reader := strings.NewReader(rtf)

		result, err := extractor.Extract(ctx, reader, "application/rtf")
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "rtf", result.Metadata["format"])
	})

	t.Run("Binary document", func(t *testing.T) {
		data := []byte{0x00, 0x01, 0x02, 0x03}
		reader := bytes.NewReader(data)

		result, err := extractor.Extract(ctx, reader, "application/msword")
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Contains(t, result.Text, "Binary document")
	})
}

func TestCompositeExtractor(t *testing.T) {
	ctx := context.Background()
	textExtractor := NewTextExtractor(TextExtractorConfig{})
	pdfExtractor := NewPDFExtractor()

	t.Run("Uses correct extractor", func(t *testing.T) {
		composite := NewCompositeExtractor(textExtractor, pdfExtractor)

		assert.True(t, composite.CanHandle("text/plain"))
		assert.True(t, composite.CanHandle("application/pdf"))
		assert.False(t, composite.CanHandle("image/jpeg"))
	})

	t.Run("Text extraction", func(t *testing.T) {
		composite := NewCompositeExtractor(textExtractor)
		reader := strings.NewReader("Hello World")

		result, err := composite.Extract(ctx, reader, "text/plain")
		require.NoError(t, err)
		assert.Equal(t, "Hello World", result.Text)
	})

	t.Run("No suitable extractor", func(t *testing.T) {
		composite := NewCompositeExtractor()
		data := []byte("some binary data")
		reader := bytes.NewReader(data)

		result, err := composite.Extract(ctx, reader, "application/octet-stream")
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.Text)
	})
}
