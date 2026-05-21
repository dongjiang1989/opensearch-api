package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/opensearch"
)

// ============================================================================
// E2E: Multi-tenant integrated workflow tests
// Each test covers the full request flow from handler → mock client,
// verifying both single-tenant (backward compat) and multi-tenant (new feature).
// ============================================================================

func setupSearchRouter(t *testing.T, mockClient *opensearch.MockClient, tenants []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger, _ := zap.NewDevelopment()
	handler := NewSearchHandler(mockClient, logger)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("tenant_ids", tenants)
		if len(tenants) > 0 {
			c.Set("tenant_id", tenants[0])
		}
		c.Next()
	})

	r.POST("/search", handler.Search)
	r.GET("/search", handler.SearchGET)
	r.POST("/aggregate", handler.Aggregate)
	r.GET("/count", handler.Count)
	r.POST("/knn", handler.KNNSearch)
	r.POST("/hybrid", handler.HybridSearch)

	return r
}

// seedTestData creates documents in the mock client across the given tenants
func seedTestData(mc *opensearch.MockClient) map[string]int {
	docs := map[string][]map[string]interface{}{
		"tenant-a": {
			{"id": "a1", "filename": "a1.pdf", "content": "report Q1 financial summary", "file_type": "pdf", "content_vector": []float32{0.1, 0.2, 0.3}},
			{"id": "a2", "filename": "a2.docx", "content": "report Q2 analysis", "file_type": "docx", "content_vector": []float32{0.2, 0.3, 0.4}},
			{"id": "a3", "filename": "a3.xlsx", "content": "spreadsheet data", "file_type": "xlsx", "content_vector": []float32{0.3, 0.4, 0.5}},
		},
		"tenant-b": {
			{"id": "b1", "filename": "b1.pdf", "content": "report Q3 financial summary", "file_type": "pdf", "content_vector": []float32{0.1, 0.2, 0.3}},
			{"id": "b2", "filename": "b2.pdf", "content": "report Q4 analysis", "file_type": "pdf", "content_vector": []float32{0.2, 0.3, 0.4}},
		},
		"tenant-c": {
			{"id": "c1", "filename": "c1.png", "content": "image data", "file_type": "png", "content_vector": []float32{0.4, 0.5, 0.6}},
		},
	}
	for tenantID, tenantDocs := range docs {
		for _, doc := range tenantDocs {
			_ = mc.IndexDocument(context.Background(), tenantID, doc["id"].(string), doc)
		}
	}
	return map[string]int{"tenant-a": 3, "tenant-b": 2, "tenant-c": 1}
}

// doSearch sends a POST /search request and parses the response
func doSearch(r *gin.Engine, query string, size int) (*SearchResponse, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	body := bytes.NewBufferString(fmt.Sprintf(`{"query":"%s","size":%d}`, query, size))
	req, _ := http.NewRequest("POST", "/search", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var resp SearchResponse
	if w.Code == http.StatusOK {
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
	}
	return &resp, w
}

// doSearchRaw sends a POST /search request and returns raw JSON map
func doSearchRaw(r *gin.Engine, query string, from, size int) (map[string]interface{}, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	body := bytes.NewBufferString(fmt.Sprintf(`{"query":"%s","from":%d,"size":%d}`, query, from, size))
	req, _ := http.NewRequest("POST", "/search", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var resp map[string]interface{}
	if w.Code == http.StatusOK {
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
	}
	return resp, w
}

// doAggregate sends a POST /aggregate request and parses the response
func doAggregate(r *gin.Engine, field string) (*AggregateResponse, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"field":"` + field + `"}`)
	req, _ := http.NewRequest("POST", "/aggregate", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var resp AggregateResponse
	if w.Code == http.StatusOK {
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
	}
	return &resp, w
}

// doCount sends a GET /count request and parses the response
func doCount(r *gin.Engine) (*CountResponse, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/count", nil)
	r.ServeHTTP(w, req)
	var resp CountResponse
	if w.Code == http.StatusOK {
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
	}
	return &resp, w
}

// doKNN sends a POST /knn request
func doKNN(r *gin.Engine) (*KNNSearchResponse, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"vector":[0.1,0.2,0.3],"k":10}`)
	req, _ := http.NewRequest("POST", "/knn", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var resp KNNSearchResponse
	if w.Code == http.StatusOK {
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
	}
	return &resp, w
}

// doHybrid sends a POST /hybrid request
func doHybrid(r *gin.Engine) (*KNNSearchResponse, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"query":"report","vector":[0.1,0.2,0.3],"k":10}`)
	req, _ := http.NewRequest("POST", "/hybrid", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var resp KNNSearchResponse
	if w.Code == http.StatusOK {
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
	}
	return &resp, w
}

// ============================================================================
// E2E Search Flow
// ============================================================================

func TestE2E_SearchFlow(t *testing.T) {
	mc := opensearch.NewMockClient()
	_ = seedTestData(mc)

	t.Run("single-tenant", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a"})

		t.Run("POST search returns correct hits", func(t *testing.T) {
			resp, w := doSearch(r, "report", 20)
			require.Equal(t, http.StatusOK, w.Code, "status: %s", w.Body.String())
			assert.True(t, resp.Success)
			assert.Equal(t, 2, resp.Total) // a1 and a2 contain "report"
			assert.Len(t, resp.Hits, 2)

			// Verify backward compatibility: all original fields present
			for _, hit := range resp.Hits {
				assert.NotEmpty(t, hit.ID)
				assert.NotZero(t, hit.Score)
				assert.Contains(t, hit.Source, "filename")
				assert.Contains(t, hit.Source, "content")
				assert.Contains(t, hit.Source, "file_type")
			}
		})

		t.Run("GET search works with query params", func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/search?q=report&size=20", nil)
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)

			var resp SearchResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.True(t, resp.Success)
			assert.Equal(t, 2, resp.Total)
		})

		t.Run("pagination works", func(t *testing.T) {
			w := httptest.NewRecorder()
			body := bytes.NewBufferString(`{"query":"report","from":0,"size":1}`)
			req, _ := http.NewRequest("POST", "/search", body)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			var resp SearchResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, 2, resp.Total) // total still shows 2
			assert.Len(t, resp.Hits, 1)    // but only 1 returned
		})

		t.Run("count matches", func(t *testing.T) {
			resp, w := doCount(r)
			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, int64(3), resp.Count) // 3 docs in tenant-a
		})
	})

	t.Run("multi-tenant", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a", "tenant-b"})

		t.Run("POST search aggregates across both tenants", func(t *testing.T) {
			resp, w := doSearch(r, "report", 20)
			require.Equal(t, http.StatusOK, w.Code, "status: %s", w.Body.String())
			assert.True(t, resp.Success)
			assert.Equal(t, 4, resp.Total) // a1,a2 from tenant-a + b1,b2 from tenant-b
			assert.Len(t, resp.Hits, 4)
		})

		t.Run("hits contain source data from all tenants", func(t *testing.T) {
			resp, w := doSearch(r, "report", 20)
			require.Equal(t, http.StatusOK, w.Code)

			// Verify all hits have intact source fields
			for _, hit := range resp.Hits {
				assert.NotEmpty(t, hit.ID)
				assert.NotZero(t, hit.Score)
				assert.Contains(t, hit.Source, "filename")
				assert.Contains(t, hit.Source, "content")
			}
		})

		t.Run("count sums across both tenants", func(t *testing.T) {
			resp, w := doCount(r)
			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, int64(5), resp.Count) // 3 + 2
		})
	})

	t.Run("three-tenant", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a", "tenant-b", "tenant-c"})

		t.Run("search includes all 3 tenants", func(t *testing.T) {
			resp, w := doSearch(r, "report", 20)
			require.Equal(t, http.StatusOK, w.Code)
			assert.True(t, resp.Success)
			assert.Equal(t, 4, resp.Total) // only 4 docs contain "report", tenant-c has none
		})

		t.Run("count includes all 3 tenants", func(t *testing.T) {
			resp, w := doCount(r)
			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, int64(6), resp.Count) // 3 + 2 + 1
		})
	})
}

// ============================================================================
// E2E Aggregate Flow
// ============================================================================

func TestE2E_AggregateFlow(t *testing.T) {
	mc := opensearch.NewMockClient()
	_ = seedTestData(mc)

	t.Run("single-tenant", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a"})

		t.Run("aggregate returns buckets and by_tenant", func(t *testing.T) {
			resp, w := doAggregate(r, "file_type")
			require.Equal(t, http.StatusOK, w.Code, "status: %s", w.Body.String())

			assert.True(t, resp.Success)
			assert.Equal(t, "file_type", resp.Field)

			// Buckets: tenant-a has pdf=1, docx=1, xlsx=1
			assert.Equal(t, int64(1), resp.Buckets["pdf"])
			assert.Equal(t, int64(1), resp.Buckets["docx"])
			assert.Equal(t, int64(1), resp.Buckets["xlsx"])

			// ByTenant must exist even for single tenant
			require.NotNil(t, resp.ByTenant)
			require.Contains(t, resp.ByTenant, "tenant-a")
			assert.Equal(t, int64(1), resp.ByTenant["tenant-a"]["pdf"])
			assert.Equal(t, int64(1), resp.ByTenant["tenant-a"]["docx"])
			assert.Equal(t, int64(1), resp.ByTenant["tenant-a"]["xlsx"])
		})

		t.Run("backward compat: raw JSON has buckets field", func(t *testing.T) {
			raw, w := doSearchRaw(r, "report", 0, 10)
			require.Equal(t, http.StatusOK, w.Code)
			assert.NotNil(t, raw["hits"])
			assert.True(t, raw["success"].(bool))
		})
	})

	t.Run("multi-tenant", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a", "tenant-b"})

		t.Run("aggregate merges buckets and breaks down by tenant", func(t *testing.T) {
			resp, w := doAggregate(r, "file_type")
			require.Equal(t, http.StatusOK, w.Code)

			// Merged: pdf=3 (a1 + b1 + b2), docx=1 (a2), xlsx=1 (a3)
			assert.Equal(t, int64(3), resp.Buckets["pdf"])
			assert.Equal(t, int64(1), resp.Buckets["docx"])
			assert.Equal(t, int64(1), resp.Buckets["xlsx"])

			// ByTenant breakdown
			require.NotNil(t, resp.ByTenant)
			require.Contains(t, resp.ByTenant, "tenant-a")
			require.Contains(t, resp.ByTenant, "tenant-b")

			assert.Equal(t, int64(1), resp.ByTenant["tenant-a"]["pdf"])
			assert.Equal(t, int64(1), resp.ByTenant["tenant-a"]["docx"])
			assert.Equal(t, int64(1), resp.ByTenant["tenant-a"]["xlsx"])

			assert.Equal(t, int64(2), resp.ByTenant["tenant-b"]["pdf"])
			assert.NotContains(t, resp.ByTenant["tenant-b"], "docx")
			assert.NotContains(t, resp.ByTenant["tenant-b"], "xlsx")
		})

		t.Run("aggregate on a different field", func(t *testing.T) {
			resp, w := doAggregate(r, "filename")
			require.Equal(t, http.StatusOK, w.Code)

			// Merged: each filename is unique
			assert.Len(t, resp.Buckets, 5)
			assert.Len(t, resp.ByTenant, 2)
			assert.Len(t, resp.ByTenant["tenant-a"], 3)
			assert.Len(t, resp.ByTenant["tenant-b"], 2)
		})
	})

	t.Run("three-tenant", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a", "tenant-b", "tenant-c"})

		t.Run("aggregate across all 3 tenants", func(t *testing.T) {
			resp, w := doAggregate(r, "file_type")
			require.Equal(t, http.StatusOK, w.Code)

			assert.Equal(t, int64(3), resp.Buckets["pdf"])
			assert.Equal(t, int64(1), resp.Buckets["docx"])
			assert.Equal(t, int64(1), resp.Buckets["xlsx"])
			assert.Equal(t, int64(1), resp.Buckets["png"])

			assert.Len(t, resp.ByTenant, 3)
			assert.Contains(t, resp.ByTenant, "tenant-c")
			assert.Equal(t, int64(1), resp.ByTenant["tenant-c"]["png"])
		})
	})
}

// ============================================================================
// E2E Vector Search Flow
// ============================================================================

func TestE2E_VectorSearchFlow(t *testing.T) {
	mc := opensearch.NewMockClient()
	_ = seedTestData(mc)

	t.Run("single-tenant KNN", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a"})

		t.Run("KNN search returns hits", func(t *testing.T) {
			resp, w := doKNN(r)
			require.Equal(t, http.StatusOK, w.Code, "status: %s", w.Body.String())
			assert.True(t, resp.Success)
			assert.Equal(t, 3, resp.Total)
			assert.Len(t, resp.Hits, 3)

			for _, hit := range resp.Hits {
				assert.NotEmpty(t, hit.ID)
				assert.NotZero(t, hit.Score)
				assert.Contains(t, hit.Source, "filename")
				assert.Contains(t, hit.Source, "content_vector")
			}
		})
	})

	t.Run("multi-tenant KNN", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a", "tenant-b"})

		t.Run("KNN search aggregates across tenants", func(t *testing.T) {
			resp, w := doKNN(r)
			require.Equal(t, http.StatusOK, w.Code)
			assert.True(t, resp.Success)
			assert.Equal(t, 5, resp.Total) // 3 + 2
			assert.Len(t, resp.Hits, 5)
		})
	})

	t.Run("single-tenant Hybrid", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a"})

		t.Run("Hybrid search returns matching hits", func(t *testing.T) {
			resp, w := doHybrid(r)
			require.Equal(t, http.StatusOK, w.Code)
			assert.True(t, resp.Success)
			assert.Equal(t, 2, resp.Total) // a1 and a2 contain "report"
		})
	})

	t.Run("multi-tenant Hybrid", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a", "tenant-b"})

		t.Run("Hybrid search aggregates across tenants", func(t *testing.T) {
			resp, w := doHybrid(r)
			require.Equal(t, http.StatusOK, w.Code)
			assert.True(t, resp.Success)
			assert.Equal(t, 4, resp.Total) // 2 from each tenant contain "report"
		})
	})
}

// ============================================================================
// E2E Backward Compatibility: Response Contract
// ============================================================================

func TestE2E_BackwardCompatibility(t *testing.T) {
	mc := opensearch.NewMockClient()
	_ = seedTestData(mc)

	t.Run("search response contract", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a"})

		t.Run("POST search response fields", func(t *testing.T) {
			resp, w := doSearch(r, "report", 20)
			require.Equal(t, http.StatusOK, w.Code)

			// All mandatory original fields
			assert.True(t, resp.Success, "success must be true")
			assert.NotZero(t, resp.Total, "total must be non-zero")
			assert.NotZero(t, resp.Took, "took_ms must be present")
			assert.NotNil(t, resp.Hits, "hits must be non-nil")

			// Each hit has original fields
			for _, hit := range resp.Hits {
				assert.NotEmpty(t, hit.ID)
				assert.NotZero(t, hit.Score)
				assert.NotNil(t, hit.Source)
				// Index is omitempty, so it's optional (D11 new field)
			}
		})

		t.Run("GET search response fields", func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/search?q=report&size=20", nil)
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)

			var resp SearchResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.True(t, resp.Success)
			assert.NotZero(t, resp.Total)
			assert.NotNil(t, resp.Hits)
		})
	})

	t.Run("aggregate response contract", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a"})
		resp, w := doAggregate(r, "file_type")
		require.Equal(t, http.StatusOK, w.Code)

		// Original fields still present
		assert.True(t, resp.Success)
		assert.Equal(t, "file_type", resp.Field)
		assert.NotNil(t, resp.Buckets, "buckets must exist (backward compat)")

		// New field added
		assert.NotNil(t, resp.ByTenant, "by_tenant should be present (new feature)")
	})

	t.Run("count response contract", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a"})
		resp, w := doCount(r)
		require.Equal(t, http.StatusOK, w.Code)
		assert.True(t, resp.Success)
		assert.NotZero(t, resp.Count)
	})

	t.Run("KNN response contract", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a"})
		resp, w := doKNN(r)
		require.Equal(t, http.StatusOK, w.Code)
		assert.True(t, resp.Success)
		assert.NotZero(t, resp.Total)
		assert.NotNil(t, resp.Hits)
	})

	t.Run("hybrid response contract", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a"})
		resp, w := doHybrid(r)
		require.Equal(t, http.StatusOK, w.Code)
		assert.True(t, resp.Success)
		assert.NotNil(t, resp.Hits)
	})

	t.Run("raw JSON: no breaking changes", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a"})
		raw, w := doSearchRaw(r, "report", 0, 10)
		require.Equal(t, http.StatusOK, w.Code)

		// Ensure no top-level fields were removed or renamed
		assert.NotNil(t, raw["success"], "success must exist")
		assert.NotNil(t, raw["total"], "total must exist")
		assert.NotNil(t, raw["took_ms"], "took_ms must exist")
		assert.NotNil(t, raw["hits"], "hits must exist")
	})
}

// ============================================================================
// E2E Error Paths
// ============================================================================

func TestE2E_ErrorPaths(t *testing.T) {
	mc := opensearch.NewMockClient()

	t.Run("single-tenant error handling", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"t1"})

		t.Run("search with invalid JSON → 400", func(t *testing.T) {
			w := httptest.NewRecorder()
			body := bytes.NewBufferString(`{invalid}`)
			req, _ := http.NewRequest("POST", "/search", body)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("aggregate with missing field → 400", func(t *testing.T) {
			w := httptest.NewRecorder()
			body := bytes.NewBufferString(`{}`)
			req, _ := http.NewRequest("POST", "/aggregate", body)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("KNN search with missing vector → 400", func(t *testing.T) {
			w := httptest.NewRecorder()
			body := bytes.NewBufferString(`{"k":5}`)
			req, _ := http.NewRequest("POST", "/knn", body)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("hybrid search with missing query → 400", func(t *testing.T) {
			w := httptest.NewRecorder()
			body := bytes.NewBufferString(`{"vector":[0.1,0.2]}`)
			req, _ := http.NewRequest("POST", "/hybrid", body)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	})

	t.Run("multi-tenant error handling", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"t1", "t2"})

		t.Run("search with invalid JSON → 400 (multi-tenant)", func(t *testing.T) {
			w := httptest.NewRecorder()
			body := bytes.NewBufferString(`{invalid}`)
			req, _ := http.NewRequest("POST", "/search", body)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("KNN search with missing vector → 400 (multi-tenant)", func(t *testing.T) {
			w := httptest.NewRecorder()
			body := bytes.NewBufferString(`{"k":5}`)
			req, _ := http.NewRequest("POST", "/knn", body)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("aggregate with missing field → 400 (multi-tenant)", func(t *testing.T) {
			w := httptest.NewRecorder()
			body := bytes.NewBufferString(`{}`)
			req, _ := http.NewRequest("POST", "/aggregate", body)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	})
}

// ============================================================================
// E2E Edge Cases
// ============================================================================

func TestE2E_EdgeCases(t *testing.T) {
	mc := opensearch.NewMockClient()
	_ = seedTestData(mc)

	t.Run("search with no matching results", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a"})
		resp, w := doSearch(r, "nonexistent_xyz", 10)
		require.Equal(t, http.StatusOK, w.Code)
		assert.True(t, resp.Success)
		assert.Equal(t, 0, resp.Total)
		assert.Empty(t, resp.Hits)
	})

	t.Run("aggregate on non-existent field", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a", "tenant-b"})
		resp, w := doAggregate(r, "nonexistent_field")
		require.Equal(t, http.StatusOK, w.Code)
		assert.True(t, resp.Success)
		assert.Empty(t, resp.Buckets)
	})

	t.Run("count with only non-existent tenant", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-does-not-exist"})
		resp, w := doCount(r)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, int64(0), resp.Count)
	})

	t.Run("KNN with no matching vector field", func(t *testing.T) {
		// tenant-c has content_vector but no docs match the vector search in mock
		r := setupSearchRouter(t, mc, []string{"tenant-c"})
		resp, w := doKNN(r)
		require.Equal(t, http.StatusOK, w.Code)
		assert.True(t, resp.Success)
		// Mock returns all docs for any vector, so 1 doc
		assert.Equal(t, 1, resp.Total)
	})

	t.Run("search with empty query matches all", func(t *testing.T) {
		r := setupSearchRouter(t, mc, []string{"tenant-a"})
		resp, w := doSearch(r, "", 20)
		require.Equal(t, http.StatusOK, w.Code)
		assert.True(t, resp.Success)
		assert.Equal(t, 3, resp.Total) // all 3 docs
	})
}
