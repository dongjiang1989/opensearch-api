package storage

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
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
}

// ImageExtractorConfig 图片提取器配置
type ImageExtractorConfig struct {
	EnableOCR bool
}

// NewImageExtractor 创建图片提取器
func NewImageExtractor(cfg ImageExtractorConfig) *ImageExtractor {
	return &ImageExtractor{
		enableOCR: cfg.EnableOCR,
	}
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

// performOCR 执行 OCR（简化实现）
// 实际生产中可集成：
// 1. go-tesseract (需要安装 tesseract-ocr)
// 2. 调用云服务 API (AWS Rekognition, Google Vision, Azure Computer Vision)
func (e *ImageExtractor) performOCR(data []byte, contentType string) (string, error) {
	// 这里提供一个框架，实际实现需要添加 OCR 依赖
	// 示例：使用 go-tesseract
	/*
		 tess, err := gosseract.NewClient()
		if err != nil {
			return "", err
		}
		defer tess.Close()

		tess.SetImage(data)
		return tess.Text()
	*/

	// 当前返回空，表示 OCR 未实现
	return "", nil
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
