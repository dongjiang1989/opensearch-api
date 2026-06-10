package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// External API metrics — duration and total count for all outbound calls.
//
// Labels:
//   - target:    backend name (opensearch, dashscope, embedding_openai, embedding_local, embedding_clip, s3)
//   - operation: specific operation (search, index_document, upload_file, chat, generate, etc.)
//   - status:    "success" or "error"

var (
	ExternalDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "opensearch_api",
			Subsystem: "external",
			Name:      "call_duration_seconds",
			Help:      "Duration of external API calls in seconds",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
		},
		[]string{"target", "operation", "status"},
	)

	ExternalTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "opensearch_api",
			Subsystem: "external",
			Name:      "calls_total",
			Help:      "Total number of external API calls",
		},
		[]string{"target", "operation", "status"},
	)
)

// Observe records an external API call metric.
//
// Usage:
//
//	start := time.Now()
//	err := doSomething()
//	metrics.Observe("dashscope", "chat", time.Since(start), err)
func Observe(target, operation string, elapsed time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	seconds := elapsed.Seconds()
	ExternalDuration.WithLabelValues(target, operation, status).Observe(seconds)
	ExternalTotal.WithLabelValues(target, operation, status).Inc()
}

// InstrumentedTransport wraps an http.RoundTripper and records metrics for
// every HTTP request. This is useful for instrumenting third-party SDK clients
// (e.g. opensearch-go) that manage their own HTTP calls internally.
type InstrumentedTransport struct {
	Target    string
	Operation string // if empty, derived from HTTP method + path
	Inner     http.RoundTripper
}

// RoundTrip implements http.RoundTripper
func (t *InstrumentedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	op := t.Operation
	if op == "" {
		op = req.Method
	}

	start := time.Now()
	resp, err := t.Inner.RoundTrip(req)
	elapsed := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
	} else if resp != nil && resp.StatusCode >= 400 {
		status = "error"
	}

	ExternalDuration.WithLabelValues(t.Target, op, status).Observe(elapsed.Seconds())
	ExternalTotal.WithLabelValues(t.Target, op, status).Inc()

	return resp, err
}
