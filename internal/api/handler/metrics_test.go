package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dongjiang1989/opensearch-api/internal/metrics"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMetricsHandler_New(t *testing.T) {
	handler := NewMetricsHandler()
	assert.NotNil(t, handler)
}

func TestMetricsHandler_ServeHTTP(t *testing.T) {
	// Trigger external metrics so they appear in the default registry
	metrics.Observe("test_target", "test_op", 100*time.Millisecond, nil)

	handler := NewMetricsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.ServeHTTP(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "go_goroutines")
	assert.Contains(t, w.Body.String(), "process_")
	// External API metrics
	assert.Contains(t, w.Body.String(), "opensearch_api_external_call_duration_seconds")
	assert.Contains(t, w.Body.String(), "opensearch_api_external_calls_total")
}
