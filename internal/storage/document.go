package storage

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"text/scanner"
)

// TextExtractor 文本文件提取器
type TextExtractor struct {
	maxSize int // 最大读取大小
}

// TextExtractorConfig 文本提取器配置
type TextExtractorConfig struct {
	MaxSize int // 最大读取字节数，默认 10MB
}

// NewTextExtractor 创建文本提取器
func NewTextExtractor(cfg TextExtractorConfig) *TextExtractor {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 10 * 1024 * 1024 // 10MB
	}

	return &TextExtractor{
		maxSize: cfg.MaxSize,
	}
}

// CanHandle 判断是否是文本文件
func (e *TextExtractor) CanHandle(contentType string) bool {
	switch contentType {
	case "text/plain", "text/markdown", "text/html", "application/json", "text/csv":
		return true
	}

	// 也处理一些常见的文本文件扩展名
	if strings.HasSuffix(contentType, ".txt") ||
		strings.HasSuffix(contentType, ".md") ||
		strings.HasSuffix(contentType, ".json") ||
		strings.HasSuffix(contentType, ".csv") {
		return true
	}

	return false
}

// Extract 提取文本内容
func (e *TextExtractor) Extract(ctx context.Context, reader io.Reader, contentType string) (*ExtractedContent, error) {
	// 限制读取大小
	limitedReader := io.LimitReader(reader, int64(e.maxSize))

	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read text data: %w", err)
	}

	text := string(data)

	// 根据内容类型处理
	switch contentType {
	case "text/html":
		text = extractTextFromHTML(text)
	case "application/json":
		// JSON 保持原样，或者可以选择只提取值
		text = formatJSON(text)
	case "text/csv":
		// CSV 保持原样
		text = normalizeCSV(text)
	}

	metadata := map[string]interface{}{
		"size":         len(data),
		"lines":        countLines(text),
		"content_type": contentType,
	}

	return &ExtractedContent{
		Text:     text,
		Metadata: metadata,
	}, nil
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

		// 检测标签开始/结束
		if literal == "<" {
			inTag = true
			continue
		}
		if literal == ">" {
			inTag = false
			continue
		}

		if inTag {
			// 检测 script/style 标签
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

		// 跳过 script/style 内容
		if inScript || inStyle {
			continue
		}

		// 添加文本内容
		if token == scanner.Ident || token == scanner.String {
			text.WriteString(literal)
			text.WriteString(" ")
		}
	}

	return strings.TrimSpace(text.String())
}

// formatJSON 格式化 JSON
func formatJSON(json string) string {
	// 简单的格式化，生产环境可以使用 json.MarshalIndent
	return strings.TrimSpace(json)
}

// normalizeCSV 规范化 CSV
func normalizeCSV(csv string) string {
	return strings.TrimSpace(csv)
}

// countLines 计算行数
func countLines(text string) int {
	reader := strings.NewReader(text)
	scanner := bufio.NewScanner(reader)

	count := 0
	for scanner.Scan() {
		count++
	}

	return count
}

// DocumentExtractor Office 文档提取器
type DocumentExtractor struct{}

// NewDocumentExtractor 创建文档提取器
func NewDocumentExtractor() *DocumentExtractor {
	return &DocumentExtractor{}
}

// CanHandle 判断是否是 Office 文档
func (e *DocumentExtractor) CanHandle(contentType string) bool {
	switch contentType {
	case "application/msword", // .doc
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document", // .docx
		"application/vnd.ms-excel", // .xls
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",         // .xlsx
		"application/vnd.ms-powerpoint",                                             // .ppt
		"application/vnd.openxmlformats-officedocument.presentationml.presentation", // .pptx
		"application/rtf", // .rtf
		"text/rtf":
		return true
	}
	return false
}

// Extract 提取 Office 文档内容
func (e *DocumentExtractor) Extract(ctx context.Context, reader io.Reader, contentType string) (*ExtractedContent, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read document data: %w", err)
	}

	// 根据类型提取内容
	switch contentType {
	case "application/rtf", "text/rtf":
		return e.extractRTF(data)
	default:
		// 对于二进制 Office 文档，返回占位符
		// 实际生产环境应使用 go-ooxml 或调用 LibreOffice
		return &ExtractedContent{
			Text: fmt.Sprintf("[Binary document: %s - content extraction requires additional dependencies]", contentType),
			Metadata: map[string]interface{}{
				"content_type": contentType,
				"size":         len(data),
				"note":         "Full text extraction requires go-ooxml or LibreOffice integration",
			},
		}, nil
	}
}

// extractRTF 提取 RTF 内容
func (e *DocumentExtractor) extractRTF(data []byte) (*ExtractedContent, error) {
	// 简化实现：去除 RTF 控制字符
	// 生产环境应使用 rtf-go 等库
	text := string(data)

	// 简单去除 RTF 控制代码
	var result strings.Builder
	escape := false
	group := 0

	for i := 0; i < len(text); i++ {
		c := text[i]

		if c == '\\' {
			escape = true
			continue
		}

		if c == '{' {
			group++
			continue
		}

		if c == '}' {
			group--
			continue
		}

		if escape {
			// 跳过控制代码
			escape = false
			continue
		}

		if group <= 0 {
			result.WriteByte(c)
		}
	}

	return &ExtractedContent{
		Text: result.String(),
		Metadata: map[string]interface{}{
			"format": "rtf",
			"size":   len(data),
		},
	}, nil
}

// CompositeExtractor 组合提取器
type CompositeExtractor struct {
	extractors []ContentExtractor
}

// NewCompositeExtractor 创建组合提取器
func NewCompositeExtractor(extractors ...ContentExtractor) *CompositeExtractor {
	return &CompositeExtractor{
		extractors: extractors,
	}
}

// CanHandle 判断是否有提取器能处理该类型
func (e *CompositeExtractor) CanHandle(contentType string) bool {
	for _, ext := range e.extractors {
		if ext.CanHandle(contentType) {
			return true
		}
	}
	return false
}

// Extract 使用合适的提取器提取内容
func (e *CompositeExtractor) Extract(ctx context.Context, reader io.Reader, contentType string) (*ExtractedContent, error) {
	for _, ext := range e.extractors {
		if ext.CanHandle(contentType) {
			return ext.Extract(ctx, reader, contentType)
		}
	}

	// 默认：尝试作为文本读取
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("no suitable extractor for content type: %s", contentType)
	}

	return &ExtractedContent{
		Text: string(data),
		Metadata: map[string]interface{}{
			"content_type": contentType,
			"note":         "Read as raw bytes, no specific extractor available",
		},
	}, nil
}
