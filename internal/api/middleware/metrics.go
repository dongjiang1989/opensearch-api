package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsMiddlewareConfig metrics 中间件配置
type MetricsMiddlewareConfig struct {
	Registry *prometheus.Registry
}

// MetricsMetrics 包含所有用于追踪 HTTP 请求的 metrics
type MetricsMetrics struct {
	requestCount   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestSize    *prometheus.HistogramVec
	responseSize   *prometheus.HistogramVec
	inflightRequests prometheus.Gauge
}

var defaultMetrics *MetricsMetrics

func init() {
	defaultMetrics = &MetricsMetrics{
		requestCount: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "opensearch_api",
				Subsystem: "http",
				Name:      "requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		requestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "opensearch_api",
				Subsystem: "http",
				Name:      "request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		),
		requestSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "opensearch_api",
				Subsystem: "http",
				Name:      "request_size_bytes",
				Help:      "HTTP request size in bytes",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		responseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "opensearch_api",
				Subsystem: "http",
				Name:      "response_size_bytes",
				Help:      "HTTP response size in bytes",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		),
		inflightRequests: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "opensearch_api",
				Subsystem: "http",
				Name:      "inflight_requests",
				Help:      "Number of inflight HTTP requests",
			},
		),
	}
}

// MetricsMiddleware 返回一个用于收集 HTTP 请求 metrics 的中间件
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 增加 inflight 请求数
		defaultMetrics.inflightRequests.Inc()
		defer defaultMetrics.inflightRequests.Dec()

		// 记录请求体大小
		if c.Request.ContentLength > 0 {
			defaultMetrics.requestSize.WithLabelValues(c.Request.Method, c.FullPath()).
				Observe(float64(c.Request.ContentLength))
		}

		// 处理请求
		c.Next()

		// 计算延迟
		duration := time.Since(start).Seconds()

		// 记录 metrics
		status := c.Writer.Status()
		defaultMetrics.requestCount.WithLabelValues(c.Request.Method, c.FullPath(), statusString(status)).Inc()
		defaultMetrics.requestDuration.WithLabelValues(c.Request.Method, c.FullPath(), statusString(status)).Observe(duration)
		defaultMetrics.responseSize.WithLabelValues(c.Request.Method, c.FullPath(), statusString(status)).
			Observe(float64(c.Writer.Size()))
	}
}

func statusString(status int) string {
	return fmt.Sprintf("%d", status)
}
