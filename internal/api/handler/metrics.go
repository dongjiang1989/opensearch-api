package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// MetricsHandler provides Prometheus metrics endpoints
type MetricsHandler struct {
	registry *prometheus.Registry
}

// NewMetricsHandler creates a new MetricsHandler
func NewMetricsHandler() *MetricsHandler {
	registry := prometheus.NewRegistry()

	// Register default collectors (Go runtime and process metrics)
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return &MetricsHandler{
		registry: registry,
	}
}

// ServeHTTP implements http.Handler for Prometheus metrics
func (h *MetricsHandler) ServeHTTP(c *gin.Context) {
	handler := promhttp.HandlerFor(h.registry, promhttp.HandlerOpts{
		Registry: h.registry,
	})
	handler.ServeHTTP(c.Writer, c.Request)
}
