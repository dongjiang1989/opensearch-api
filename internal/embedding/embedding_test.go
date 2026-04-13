package embedding

import (
	"context"
	"testing"
	"time"
)

func TestOpenAIEmbedding_Name(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{"default model", "", "text-embedding-3-small"},
		{"custom model", "text-embedding-3-large", "text-embedding-3-large"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewOpenAIEmbedding(OpenAIEmbeddingConfig{
				Model: tt.model,
			})
			if e.Name() != tt.expected {
				t.Errorf("Name() = %v, want %v", e.Name(), tt.expected)
			}
		})
	}
}

func TestOpenAIEmbedding_Dimensions(t *testing.T) {
	tests := []struct {
		name       string
		dimensions int
		expected   int
	}{
		{"default dimensions", 0, 1536},
		{"custom dimensions", 512, 512},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewOpenAIEmbedding(OpenAIEmbeddingConfig{
				Dimensions: tt.dimensions,
			})
			// Dimensions() 返回内部存储的维度值
			if e.Dimensions() != tt.expected {
				t.Errorf("Dimensions() = %v, want %v", e.Dimensions(), tt.expected)
			}
		})
	}
}

func TestOpenAIEmbedding_Timeout(t *testing.T) {
	e := NewOpenAIEmbedding(OpenAIEmbeddingConfig{
		Timeout: 60 * time.Second,
	})

	if e.httpClient.Timeout != 60*time.Second {
		t.Errorf("httpClient.Timeout = %v, want %v", e.httpClient.Timeout, 60*time.Second)
	}
}

func TestOpenAIEmbedding_APIURL(t *testing.T) {
	tests := []struct {
		name     string
		apiURL   string
		expected string
	}{
		{"default URL", "", "https://api.openai.com/v1/embeddings"},
		{"custom URL", "http://localhost:11434/api/embeddings", "http://localhost:11434/api/embeddings"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewOpenAIEmbedding(OpenAIEmbeddingConfig{
				APIURL: tt.apiURL,
			})
			if e.apiURL != tt.expected {
				t.Errorf("apiURL = %v, want %v", e.apiURL, tt.expected)
			}
		})
	}
}

func TestOpenAIEmbedding_Generate_ContextCancellation(t *testing.T) {
	e := NewOpenAIEmbedding(OpenAIEmbeddingConfig{
		APIKey: "test-key",
	})

	// 测试上下文取消
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.Generate(ctx, "test content")
	if err == nil {
		t.Error("Generate() expected error for cancelled context, got nil")
	}
}

func TestLocalEmbedding_Name(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{"default model", "", "all-minilm"},
		{"custom model", "bge-m3", "bge-m3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewLocalEmbedding(LocalEmbeddingConfig{
				Model: tt.model,
			})
			if e.Name() != tt.expected {
				t.Errorf("Name() = %v, want %v", e.Name(), tt.expected)
			}
		})
	}
}

func TestLocalEmbedding_APIURL(t *testing.T) {
	tests := []struct {
		name     string
		apiURL   string
		expected string
	}{
		{"default URL", "", "http://localhost:11434"},
		{"custom URL", "http://localhost:8080", "http://localhost:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewLocalEmbedding(LocalEmbeddingConfig{
				APIURL: tt.apiURL,
			})
			if e.apiURL != tt.expected {
				t.Errorf("apiURL = %v, want %v", e.apiURL, tt.expected)
			}
		})
	}
}

func TestLocalEmbedding_Generate_ContextCancellation(t *testing.T) {
	e := NewLocalEmbedding(LocalEmbeddingConfig{})

	// 测试上下文取消
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.Generate(ctx, "test content")
	if err == nil {
		t.Error("Generate() expected error for cancelled context, got nil")
	}
}

func TestCLIPEmbedding_Name(t *testing.T) {
	e := NewCLIPEmbedding(CLIPEmbeddingConfig{})

	if e.Name() != "clip" {
		t.Errorf("Name() = %v, want clip", e.Name())
	}
}

func TestCLIPEmbedding_Dimensions(t *testing.T) {
	tests := []struct {
		name       string
		dimensions int
		expected   int
	}{
		{"default dimensions", 0, 512},
		{"custom dimensions", 768, 768},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewCLIPEmbedding(CLIPEmbeddingConfig{
				Dimensions: tt.dimensions,
			})
			if e.Dimensions() != tt.expected {
				t.Errorf("Dimensions() = %v, want %v", e.Dimensions(), tt.expected)
			}
		})
	}
}

func TestCLIPEmbedding_APIURL(t *testing.T) {
	tests := []struct {
		name     string
		apiURL   string
		expected string
	}{
		{"default URL", "", "http://localhost:8000"},
		{"custom URL", "http://clip-server:8000", "http://clip-server:8000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewCLIPEmbedding(CLIPEmbeddingConfig{
				APIURL: tt.apiURL,
			})
			if e.apiURL != tt.expected {
				t.Errorf("apiURL = %v, want %v", e.apiURL, tt.expected)
			}
		})
	}
}

func TestCLIPEmbedding_Generate_ContextCancellation(t *testing.T) {
	e := NewCLIPEmbedding(CLIPEmbeddingConfig{})

	// 测试上下文取消
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.Generate(ctx, "test content")
	if err == nil {
		t.Error("Generate() expected error for cancelled context, got nil")
	}
}

func TestCLIPEmbedding_GenerateImage_ContextCancellation(t *testing.T) {
	e := NewCLIPEmbedding(CLIPEmbeddingConfig{})

	// 测试上下文取消
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.GenerateImage(ctx, []byte("fake image data"), "image/jpeg")
	if err == nil {
		t.Error("GenerateImage() expected error for cancelled context, got nil")
	}
}

func TestCLIPEmbedding_GenerateImageFromURL_ContextCancellation(t *testing.T) {
	e := NewCLIPEmbedding(CLIPEmbeddingConfig{})

	// 测试上下文取消
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.GenerateImageFromURL(ctx, "http://example.com/image.jpg")
	if err == nil {
		t.Error("GenerateImageFromURL() expected error for cancelled context, got nil")
	}
}
