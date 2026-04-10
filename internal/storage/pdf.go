package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

// PDFExtractor PDF 内容提取器
type PDFExtractor struct{}

// NewPDFExtractor 创建 PDF 提取器
func NewPDFExtractor() *PDFExtractor {
	return &PDFExtractor{}
}

// CanHandle 判断是否是 PDF 文件
func (e *PDFExtractor) CanHandle(contentType string) bool {
	return contentType == "application/pdf"
}

// Extract 提取 PDF 内容
// 注意：完整的 PDF 文本提取需要 pdfcpu 或类似库
// 这里提供基础实现，生产环境建议集成 pdfcpu
func (e *PDFExtractor) Extract(ctx context.Context, reader io.Reader, contentType string) (*ExtractedContent, error) {
	// 读取全部 PDF 数据
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read pdf data: %w", err)
	}

	// 检查是否是有效的 PDF 文件
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		return &ExtractedContent{
			Text: "",
			Metadata: map[string]interface{}{
				"error": "invalid PDF file",
				"pages": 0,
			},
		}, nil
	}

	// 简单提取 PDF 元数据
	metadata := map[string]interface{}{
		"format": "pdf",
		"size":   len(data),
		"pages":  0, // 需要 pdfcpu 来计算页数
	}

	// 尝试提取文本内容（简单实现）
	text := extractPDFTextSimple(data)

	return &ExtractedContent{
		Text:     text,
		Metadata: metadata,
	}, nil
}

// extractPDFTextSimple 简单提取 PDF 中的文本
// 注意：这是一个简化实现，仅用于演示
// 生产环境应该使用 pdfcpu 库进行完整的文本提取
func extractPDFTextSimple(data []byte) string {
	var text strings.Builder

	// 查找 PDF 流中的文本
	content := string(data)

	// 简单的文本提取（不完整，仅用于演示）
	// 实际生产环境请使用 pdfcpu.ExtractText

	// 查找 BT/ET 标记之间的内容（Begin Text/End Text）
	start := 0
	for {
		btIdx := strings.Index(content[start:], "BT")
		if btIdx == -1 {
			break
		}
		btIdx += start

		etIdx := strings.Index(content[btIdx:], "ET")
		if etIdx == -1 {
			break
		}
		etIdx += btIdx

		// 提取 BT 和 ET 之间的内容
		streamContent := content[btIdx:etIdx]

		// 提取 Tj 操作符中的文本
		for {
			tjIdx := strings.Index(streamContent, "Tj")
			if tjIdx == -1 {
				break
			}

			// 查找前面的括号
			parenStart := strings.LastIndex(streamContent[:tjIdx], "(")
			parenEnd := strings.Index(streamContent[tjIdx:], ")")

			if parenStart != -1 && parenEnd != -1 && parenStart < tjIdx {
				txt := streamContent[parenStart+1 : tjIdx]
				text.WriteString(txt)
				text.WriteString(" ")
			}

			streamContent = streamContent[tjIdx+2:]
		}

		start = etIdx + 2
	}

	return strings.TrimSpace(text.String())
}

// ExtractTextFromPDF 从 PDF 数据中提取文本
func ExtractTextFromPDF(data []byte) (string, error) {
	ctxReader := io.NopCloser(bytes.NewReader(data))
	defer func() { _ = ctxReader.Close() }()

	// 简单实现，生产环境应使用 pdfcpu
	return extractPDFTextSimple(data), nil
}

// GetPageCount 获取 PDF 页数
func GetPageCount(data []byte) (int, error) {
	// 简单计算 /Page 标记的数量
	count := bytes.Count(data, []byte("/Type/Page"))
	if count == 0 {
		count = bytes.Count(data, []byte("/Pages"))
	}
	if count == 0 {
		return 1, nil
	}
	return count, nil
}

// ValidatePDF 验证 PDF 文件是否有效
func ValidatePDF(data []byte) error {
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		return fmt.Errorf("not a valid PDF file")
	}
	return nil
}

// ValidatePDFReader 验证 PDF 读取器
func ValidatePDFReader(reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	return ValidatePDF(data)
}
