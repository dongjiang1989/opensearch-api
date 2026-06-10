package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler provides Prometheus metrics endpoints.
// Uses the default registry which includes:
//   - HTTP request metrics (from MetricsMiddleware)
//   - External API call metrics (from metrics.Observe / InstrumentedTransport)
//   - Go runtime and process metrics (registered automatically by promauto)
type MetricsHandler struct{}

// NewMetricsHandler creates a new MetricsHandler
func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{}
}

// ServeHTTP implements http.Handler for Prometheus metrics
// @Summary Prometheus Metrics
// @Description 返回 Prometheus 格式的服务监控指标，包括 HTTP 请求指标和外部 API 调用指标
// @Tags Metrics
// @Produce text/plain
// @Success 200 {string} string "Prometheus metrics"
// @Router /metrics [get]
func (h *MetricsHandler) ServeHTTP(c *gin.Context) {
	promhttp.Handler().ServeHTTP(c.Writer, c.Request)
}
