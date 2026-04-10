package storage

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"os/exec"
	"strings"

	// 图片格式支持
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	_ "golang.org/x/image/webp"
)

// ImageExtractor 图片内容提取器
type ImageExtractor struct {
	enableOCR bool
	ocrLang   string
}

// ImageExtractorConfig 图片提取器配置
type ImageExtractorConfig struct {
	EnableOCR bool
	OCRLang   string // OCR 语言，默认 "eng"
}

// NewImageExtractor 创建图片提取器
func NewImageExtractor(cfg ImageExtractorConfig) *ImageExtractor {
	lang := cfg.OCRLang
	if lang == "" {
		lang = "eng"
	}
	return &ImageExtractor{
		enableOCR: cfg.EnableOCR,
		ocrLang:   lang,
	}
}

// getOCRLang 获取 OCR 语言
func (e *ImageExtractor) getOCRLang() string {
	if e.ocrLang == "" {
		return "eng"
	}
	return e.ocrLang
}

// CanHandle 判断是否是图片文件
func (e *ImageExtractor) CanHandle(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/svg+xml":
		return true
	}
	return false
}

// Extract 提取图片内容
func (e *ImageExtractor) Extract(ctx context.Context, reader io.Reader, contentType string) (*ExtractedContent, error) {
	// 读取图片数据
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	// 解码图片获取元数据
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return &ExtractedContent{
			Text: "",
			Metadata: map[string]interface{}{
				"error": err.Error(),
			},
		}, nil
	}

	bounds := img.Bounds()
	metadata := map[string]interface{}{
		"format": format,
		"width":  bounds.Dx(),
		"height": bounds.Dy(),
	}

	var text string

	// OCR 功能（如果需要且支持）
	if e.enableOCR && contentType != "image/svg+xml" {
		// OCR 需要外部依赖，这里提供一个框架
		// 实际使用中可集成 go-tesseract 或调用外部 OCR 服务
		ocrText, err := e.performOCR(data, contentType)
		if err == nil && ocrText != "" {
			text = ocrText
			metadata["ocr_enabled"] = true
		}
	}

	return &ExtractedContent{
		Text:     text,
		Metadata: metadata,
	}, nil
}

// performOCR 执行 OCR
// 使用 tesseract 命令行工具 (需要先安装 tesseract-ocr)
// macOS: brew install tesseract
// Linux: apt-get install tesseract-ocr 或 yum install tesseract
// Docker: 已在 Dockerfile 中安装
func (e *ImageExtractor) performOCR(data []byte, contentType string) (string, error) {
	// 使用 tesseract 命令行工具
	// tesseract stdin stdout -l <lang>
	cmd := exec.Command("tesseract", "stdin", "stdout", "-l", e.getOCRLang())
	cmd.Stdin = bytes.NewReader(data)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("tesseract failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("tesseract failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetImageMetadata 获取图片元数据（便捷函数）
func GetImageMetadata(data []byte) (map[string]interface{}, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	return map[string]interface{}{
		"format": format,
		"width":  bounds.Dx(),
		"height": bounds.Dy(),
	}, nil
}

// ValidateImage 验证图片文件是否有效
func ValidateImage(data []byte) error {
	_, _, err := image.Decode(bytes.NewReader(data))
	return err
}

// SVGExtractor SVG 文件提取器
type SVGExtractor struct{}

// NewSVGExtractor 创建 SVG 提取器
func NewSVGExtractor() *SVGExtractor {
	return &SVGExtractor{}
}

// CanHandle 判断是否是 SVG 文件
func (e *SVGExtractor) CanHandle(contentType string) bool {
	return contentType == "image/svg+xml"
}

// Extract 提取 SVG 内容
func (e *SVGExtractor) Extract(ctx context.Context, reader io.Reader, contentType string) (*ExtractedContent, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read svg data: %w", err)
	}

	// SVG 是 XML 格式，可以提取文本内容
	text := extractTextFromSVG(string(data))

	return &ExtractedContent{
		Text: text,
		Metadata: map[string]interface{}{
			"format": "svg",
			"size":   len(data),
		},
	}, nil
}

// extractTextFromSVG 从 SVG 中提取文本
func extractTextFromSVG(svg string) string {
	var text strings.Builder

	// 简单提取 <text> 和 <tspan> 标签中的内容
	// 生产环境建议使用 XML 解析器
	inTag := false
	tagName := ""
	for i := 0; i < len(svg); i++ {
		if svg[i] == '<' {
			inTag = true
			// 检查标签类型
			if i+6 < len(svg) && strings.ToLower(svg[i+1:i+6]) == "text" {
				tagName = "text"
			} else if i+6 < len(svg) && strings.ToLower(svg[i+1:i+6]) == "tspa" {
				tagName = "tspan"
			} else {
				tagName = ""
			}
		} else if svg[i] == '>' {
			inTag = false
		} else if !inTag && tagName != "" {
			text.WriteByte(svg[i])
		}
	}

	return text.String()
}
