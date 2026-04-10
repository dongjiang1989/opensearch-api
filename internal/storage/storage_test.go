package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFileType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    FileType
	}{
		{"PDF", "application/pdf", FileTypePDF},
		{"JPEG", "image/jpeg", FileTypeImage},
		{"PNG", "image/png", FileTypeImage},
		{"GIF", "image/gif", FileTypeImage},
		{"WebP", "image/webp", FileTypeImage},
		{"Word doc", "application/msword", FileTypeDocument},
		{"Word docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", FileTypeDocument},
		{"Excel xls", "application/vnd.ms-excel", FileTypeDocument},
		{"Excel xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", FileTypeDocument},
		{"PowerPoint ppt", "application/vnd.ms-powerpoint", FileTypeDocument},
		{"PowerPoint pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation", FileTypeDocument},
		{"MP4", "video/mp4", FileTypeVideo},
		{"AVI", "video/x-msvideo", FileTypeVideo},
		{"MP3", "audio/mpeg", FileTypeAudio},
		{"WAV", "audio/wav", FileTypeAudio},
		{"Plain text", "text/plain", FileTypeText},
		{"Markdown", "text/markdown", FileTypeText},
		{"HTML", "text/html", FileTypeText},
		{"JSON", "application/json", FileTypeText},
		{"Unknown", "application/unknown", FileTypeOther},
		{"Empty", "", FileTypeOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFileType(tt.contentType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{"PDF file", "document.pdf", ".pdf"},
		{"Word file", "report.docx", ".docx"},
		{"No extension", "readme", ""},
		{"Multiple dots", "file.name.tar.gz", ".gz"},
		{"Hidden file", ".gitignore", ".gitignore"}, // filepath.Ext returns full name for hidden files
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFileExtension(tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantLen  int // expected max length
	}{
		{"Normal file", "document.pdf", 12},
		{"Path traversal", "../../../etc/passwd", 8}, // passwd
		{"Very long name", string(make([]byte, 300)) + ".txt", 204}, // 200 + .txt
		{"Empty", "", 1}, // filepath.Base of empty string returns "."
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeFilename(tt.filename)
			assert.LessOrEqual(t, len(result), tt.wantLen)
			assert.NotContains(t, result, "/")
			assert.NotContains(t, result, "..")
		})
	}
}

func TestGenerateStoragePath(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		fileID   string
		filename string
		contains string
	}{
		{"Basic path", "tenant1", "abc123", "doc.pdf", "tenants/tenant1/files/abc1/abc123.pdf"},
		{"With special chars", "tenant-2", "xyz789", "report.docx", "tenants/tenant-2/files/xyz7/xyz789.docx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateStoragePath(tt.tenantID, tt.fileID, tt.filename)
			assert.Contains(t, result, tt.contains)
			assert.Contains(t, result, tt.fileID[:4])
		})
	}
}
