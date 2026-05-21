package opensearch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockClient_Search_MultiTenant_Aggregation(t *testing.T) {
	mc := NewMockClient()

	// Index documents in two different tenants
	_ = mc.IndexDocument(context.Background(), "tenant-a", "doc1", map[string]interface{}{
		"filename":  "a1.pdf",
		"content":   "hello world",
		"file_type": "pdf",
	})
	_ = mc.IndexDocument(context.Background(), "tenant-a", "doc2", map[string]interface{}{
		"filename":  "a2.docx",
		"content":   "hello go",
		"file_type": "docx",
	})
	_ = mc.IndexDocument(context.Background(), "tenant-b", "doc3", map[string]interface{}{
		"filename":  "b1.pdf",
		"content":   "hello rust",
		"file_type": "pdf",
	})

	t.Run("search across multiple tenants", func(t *testing.T) {
		result, err := mc.Search(context.Background(), []string{"tenant-a", "tenant-b"}, &SearchQuery{
			Query: "hello",
			Size:  10,
		})
		require.NoError(t, err)
		assert.Equal(t, 3, result.Total)
		assert.Len(t, result.Hits, 3)
	})

	t.Run("search single tenant", func(t *testing.T) {
		result, err := mc.Search(context.Background(), []string{"tenant-a"}, &SearchQuery{
			Query: "hello",
			Size:  10,
		})
		require.NoError(t, err)
		assert.Equal(t, 2, result.Total)
	})

	t.Run("aggregate with per-tenant breakdown", func(t *testing.T) {
		result, err := mc.Aggregate(context.Background(), []string{"tenant-a", "tenant-b"}, "file_type")
		require.NoError(t, err)
		require.NotNil(t, result)

		// Merged buckets should contain both tenants' data
		assert.Equal(t, int64(2), result.Buckets["pdf"])
		assert.Equal(t, int64(1), result.Buckets["docx"])

		// Per-tenant breakdown should exist
		require.Contains(t, result.ByTenant, "tenant-a")
		require.Contains(t, result.ByTenant, "tenant-b")

		// Tenant-a has 1 pdf and 1 docx
		assert.Equal(t, int64(1), result.ByTenant["tenant-a"]["pdf"])
		assert.Equal(t, int64(1), result.ByTenant["tenant-a"]["docx"])

		// Tenant-b has 1 pdf
		assert.Equal(t, int64(1), result.ByTenant["tenant-b"]["pdf"])
		assert.NotContains(t, result.ByTenant["tenant-b"], "docx")
	})
}

func TestMockClient_Aggregate_EmptyTenants(t *testing.T) {
	mc := NewMockClient()

	result, err := mc.Aggregate(context.Background(), []string{"nonexistent"}, "file_type")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Buckets)
	assert.Empty(t, result.ByTenant)
	assert.Equal(t, "file_type", result.Field)
}

func TestMockClient_Aggregate_PartialTenants(t *testing.T) {
	mc := NewMockClient()

	// Only tenant-a has documents
	_ = mc.IndexDocument(context.Background(), "tenant-a", "doc1", map[string]interface{}{
		"file_type": "pdf",
	})

	result, err := mc.Aggregate(context.Background(), []string{"tenant-a", "tenant-b"}, "file_type")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int64(1), result.Buckets["pdf"])
	require.Contains(t, result.ByTenant, "tenant-a")
	assert.NotContains(t, result.ByTenant, "tenant-b")
}

func TestMockClient_KNNSearch_MultiTenant(t *testing.T) {
	mc := NewMockClient()

	_ = mc.IndexDocument(context.Background(), "tenant-a", "doc1", map[string]interface{}{
		"filename":        "a1.pdf",
		"content_vector":  []float32{0.1, 0.2, 0.3},
	})
	_ = mc.IndexDocument(context.Background(), "tenant-b", "doc2", map[string]interface{}{
		"filename":        "b1.pdf",
		"content_vector":  []float32{0.1, 0.2, 0.3},
	})

	result, err := mc.KNNSearch(context.Background(), []string{"tenant-a", "tenant-b"}, &KNNQuery{
		Vector: []float32{0.1, 0.2, 0.3},
		K:      10,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)
	assert.Len(t, result.Hits, 2)
}

func TestMockClient_HybridSearch_MultiTenant(t *testing.T) {
	mc := NewMockClient()

	_ = mc.IndexDocument(context.Background(), "tenant-a", "doc1", map[string]interface{}{
		"filename":        "a1.pdf",
		"content":         "hello",
		"content_vector":  []float32{0.1, 0.2, 0.3},
	})
	_ = mc.IndexDocument(context.Background(), "tenant-b", "doc2", map[string]interface{}{
		"filename":        "b1.pdf",
		"content":         "world",
		"content_vector":  []float32{0.1, 0.2, 0.3},
	})

	result, err := mc.HybridSearch(context.Background(), []string{"tenant-a", "tenant-b"}, &HybridQuery{
		Query:       "hello",
		Vector:      []float32{0.1, 0.2, 0.3},
		VectorField: "content_vector",
		K:           10,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total) // only "hello" matches
}

func TestMockClient_Count_MultiTenant(t *testing.T) {
	mc := NewMockClient()

	_ = mc.IndexDocument(context.Background(), "tenant-a", "doc1", map[string]interface{}{"filename": "a1.pdf"})
	_ = mc.IndexDocument(context.Background(), "tenant-a", "doc2", map[string]interface{}{"filename": "a2.pdf"})
	_ = mc.IndexDocument(context.Background(), "tenant-b", "doc3", map[string]interface{}{"filename": "b1.pdf"})

	count, err := mc.Count(context.Background(), []string{"tenant-a", "tenant-b"})
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	countA, _ := mc.Count(context.Background(), []string{"tenant-a"})
	assert.Equal(t, int64(2), countA)
}

func TestMockClient_Search_Deduplication(t *testing.T) {
	mc := NewMockClient()

	_ = mc.IndexDocument(context.Background(), "tenant-a", "doc1", map[string]interface{}{
		"content": "hello",
	})

	// Duplicate tenant-a in the list
	result, err := mc.Search(context.Background(), []string{"tenant-a", "tenant-a"}, &SearchQuery{
		Query: "hello",
		Size:  10,
	})
	require.NoError(t, err)
	// Mock iterates over the tenantIDs list, so duplicates will appear
	// This tests that the mock doesn't deduplicate (the real client would have no issue
	// because indexNames deduplicates by mapping each tenant to the same index name)
	assert.Equal(t, 2, result.Total, "mock iterates all tenantIDs including duplicates")
}
