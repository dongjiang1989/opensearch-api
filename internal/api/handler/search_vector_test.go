package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/opensearch"
)

// mockOpenSearchClient 模拟 OpenSearch 客户端
type mockOpenSearchClient struct {
	knnSearchFunc    func(tenantID string, query *opensearch.KNNQuery) (*opensearch.SearchResult, error)
	hybridSearchFunc func(tenantID string, query *opensearch.HybridQuery) (*opensearch.SearchResult, error)
}

func (m *mockOpenSearchClient) IndexDocument(ctx context.Context, tenantID, docID string, doc map[string]interface{}) error {
	return nil
}

func (m *mockOpenSearchClient) GetDocument(ctx context.Context, tenantID, docID string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockOpenSearchClient) DeleteDocument(ctx context.Context, tenantID, docID string) error {
	return nil
}

func (m *mockOpenSearchClient) Search(ctx context.Context, tenantID string, query *opensearch.SearchQuery) (*opensearch.SearchResult, error) {
	return nil, nil
}

func (m *mockOpenSearchClient) KNNSearch(ctx context.Context, tenantID string, query *opensearch.KNNQuery) (*opensearch.SearchResult, error) {
	if m.knnSearchFunc != nil {
		return m.knnSearchFunc(tenantID, query)
	}
	return &opensearch.SearchResult{
		Total: 1,
		Hits: []opensearch.SearchHit{
			{
				ID:    "test-id",
				Score: 0.95,
				Source: map[string]interface{}{
					"filename": "test.pdf",
				},
			},
		},
		Took: 10,
	}, nil
}

func (m *mockOpenSearchClient) HybridSearch(ctx context.Context, tenantID string, query *opensearch.HybridQuery) (*opensearch.SearchResult, error) {
	if m.hybridSearchFunc != nil {
		return m.hybridSearchFunc(tenantID, query)
	}
	return &opensearch.SearchResult{
		Total: 1,
		Hits: []opensearch.SearchHit{
			{
				ID:    "test-id",
				Score: 0.95,
				Source: map[string]interface{}{
					"filename": "test.pdf",
				},
			},
		},
		Took: 10,
	}, nil
}

func (m *mockOpenSearchClient) IndexName(tenantID string) string {
	return "tenant_" + tenantID + "_files"
}

func (m *mockOpenSearchClient) Health(ctx context.Context) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockOpenSearchClient) Ping(ctx context.Context) error {
	return nil
}

func (m *mockOpenSearchClient) Count(ctx context.Context, tenantID string) (int64, error) {
	return 0, nil
}

func (m *mockOpenSearchClient) Aggregate(ctx context.Context, tenantID, fieldName string) (map[string]int64, error) {
	return nil, nil
}

func TestSearchHandler_KNNSearch_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &mockOpenSearchClient{}
	handler := NewSearchHandler(client, logger)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := map[string]interface{}{
		"vector": []float32{0.1, 0.2, 0.3},
		"field":  "content_vector",
		"k":      10,
	}
	bodyBytes, _ := json.Marshal(body)

	c.Request = &http.Request{
		Method: "POST",
		Header: http.Header{
			"Content-Type":  []string{"application/json"},
			"X-Tenant-ID":   []string{"tenant-1"},
			"Authorization": []string{"Bearer token"},
		},
		Body: io.NopCloser(bytes.NewReader(bodyBytes)),
	}
	// 手动设置租户 ID 到上下文（模拟中间件行为）
	c.Set("tenant_id", "tenant-1")

	handler.KNNSearch(c)

	if w.Code != http.StatusOK {
		t.Logf("Response body: %s", w.Body.String())
		t.Errorf("KNNSearch() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp KNNSearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !resp.Success {
		t.Error("Response should be successful")
	}
	if resp.Total != 1 {
		t.Errorf("Total = %v, want 1", resp.Total)
	}
	if len(resp.Hits) != 1 {
		t.Errorf("Hits length = %v, want 1", len(resp.Hits))
	}
}

func TestSearchHandler_KNNSearch_MissingTenantID(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &mockOpenSearchClient{}
	handler := NewSearchHandler(client, logger)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := map[string]interface{}{
		"vector": []float32{0.1, 0.2, 0.3},
		"k":      10,
	}
	bodyBytes, _ := json.Marshal(body)

	c.Request = &http.Request{
		Method: "POST",
		Header: http.Header{
			"Content-Type":  []string{"application/json"},
			"Authorization": []string{"Bearer token"},
		},
		Body: io.NopCloser(bytes.NewReader(bodyBytes)),
	}

	handler.KNNSearch(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("KNNSearch() status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestSearchHandler_KNNSearch_InvalidBody(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &mockOpenSearchClient{}
	handler := NewSearchHandler(client, logger)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = &http.Request{
		Method: "POST",
		Header: http.Header{
			"Content-Type":  []string{"application/json"},
			"X-Tenant-ID":   []string{"tenant-1"},
			"Authorization": []string{"Bearer token"},
		},
		Body: io.NopCloser(bytes.NewReader([]byte("invalid json"))),
	}

	handler.KNNSearch(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("KNNSearch() status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestSearchHandler_KNNSearch_DefaultValues(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	var capturedQuery *opensearch.KNNQuery
	client := &mockOpenSearchClient{
		knnSearchFunc: func(tenantID string, query *opensearch.KNNQuery) (*opensearch.SearchResult, error) {
			capturedQuery = query
			return &opensearch.SearchResult{
				Total: 0,
				Hits:  []opensearch.SearchHit{},
				Took:  10,
			}, nil
		},
	}
	handler := NewSearchHandler(client, logger)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := map[string]interface{}{
		"vector": []float32{0.1, 0.2, 0.3},
	}
	bodyBytes, _ := json.Marshal(body)

	c.Request = &http.Request{
		Method: "POST",
		Header: http.Header{
			"Content-Type":  []string{"application/json"},
			"X-Tenant-ID":   []string{"tenant-1"},
			"Authorization": []string{"Bearer token"},
		},
		Body: io.NopCloser(bytes.NewReader(bodyBytes)),
	}
	// 手动设置租户 ID 到上下文（模拟中间件行为）
	c.Set("tenant_id", "tenant-1")

	handler.KNNSearch(c)

	if capturedQuery == nil {
		t.Fatal("Query should not be nil")
	}
	if capturedQuery.K != 10 {
		t.Errorf("Default K = %v, want 10", capturedQuery.K)
	}
	if capturedQuery.Field != "content_vector" {
		t.Errorf("Default Field = %v, want content_vector", capturedQuery.Field)
	}
}

func TestSearchHandler_HybridSearch_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &mockOpenSearchClient{}
	handler := NewSearchHandler(client, logger)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := map[string]interface{}{
		"query":        "test document",
		"vector":       []float32{0.1, 0.2, 0.3},
		"vector_field": "content_vector",
		"k":            10,
	}
	bodyBytes, _ := json.Marshal(body)

	c.Request = &http.Request{
		Method: "POST",
		Header: http.Header{
			"Content-Type":  []string{"application/json"},
			"X-Tenant-ID":   []string{"tenant-1"},
			"Authorization": []string{"Bearer token"},
		},
		Body: io.NopCloser(bytes.NewReader(bodyBytes)),
	}
	// 手动设置租户 ID 到上下文（模拟中间件行为）
	c.Set("tenant_id", "tenant-1")

	handler.HybridSearch(c)

	if w.Code != http.StatusOK {
		t.Logf("Response body: %s", w.Body.String())
		t.Errorf("HybridSearch() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp KNNSearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !resp.Success {
		t.Error("Response should be successful")
	}
}

func TestSearchHandler_HybridSearch_MissingQuery(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &mockOpenSearchClient{}
	handler := NewSearchHandler(client, logger)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := map[string]interface{}{
		"vector": []float32{0.1, 0.2, 0.3},
		"k":      10,
	}
	bodyBytes, _ := json.Marshal(body)

	c.Request = &http.Request{
		Method: "POST",
		Header: http.Header{
			"Content-Type":  []string{"application/json"},
			"X-Tenant-ID":   []string{"tenant-1"},
			"Authorization": []string{"Bearer token"},
		},
		Body: io.NopCloser(bytes.NewReader(bodyBytes)),
	}

	handler.HybridSearch(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("HybridSearch() status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestSearchHandler_HybridSearch_DefaultValues(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	var capturedQuery *opensearch.HybridQuery
	client := &mockOpenSearchClient{
		hybridSearchFunc: func(tenantID string, query *opensearch.HybridQuery) (*opensearch.SearchResult, error) {
			capturedQuery = query
			return &opensearch.SearchResult{
				Total: 0,
				Hits:  []opensearch.SearchHit{},
				Took:  10,
			}, nil
		},
	}
	handler := NewSearchHandler(client, logger)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := map[string]interface{}{
		"query":  "test",
		"vector": []float32{0.1, 0.2, 0.3},
	}
	bodyBytes, _ := json.Marshal(body)

	c.Request = &http.Request{
		Method: "POST",
		Header: http.Header{
			"Content-Type":  []string{"application/json"},
			"X-Tenant-ID":   []string{"tenant-1"},
			"Authorization": []string{"Bearer token"},
		},
		Body: io.NopCloser(bytes.NewReader(bodyBytes)),
	}
	// 手动设置租户 ID 到上下文（模拟中间件行为）
	c.Set("tenant_id", "tenant-1")

	handler.HybridSearch(c)

	if capturedQuery == nil {
		t.Fatal("Query should not be nil")
	}
	if capturedQuery.K != 10 {
		t.Errorf("Default K = %v, want 10", capturedQuery.K)
	}
	if capturedQuery.VectorField != "content_vector" {
		t.Errorf("Default VectorField = %v, want content_vector", capturedQuery.VectorField)
	}
}

func TestVectorHit_Struct(t *testing.T) {
	hit := VectorHit{
		ID:    "test-id",
		Score: 0.95,
		Source: map[string]interface{}{
			"filename": "test.pdf",
		},
	}

	if hit.ID != "test-id" {
		t.Errorf("ID = %v, want test-id", hit.ID)
	}
	if hit.Score != 0.95 {
		t.Errorf("Score = %v, want 0.95", hit.Score)
	}
	if hit.Source["filename"] != "test.pdf" {
		t.Error("Source should contain filename")
	}
}
