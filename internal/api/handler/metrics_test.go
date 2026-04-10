package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMetricsHandler_New(t *testing.T) {
	handler := NewMetricsHandler()
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.registry)
}

func TestMetricsHandler_ServeHTTP(t *testing.T) {
	handler := NewMetricsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.ServeHTTP(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "go_goroutines")
	assert.Contains(t, w.Body.String(), "process_")
}
