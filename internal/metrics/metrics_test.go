package metrics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestObserve_Success(t *testing.T) {
	before := testutil.ToFloat64(ExternalTotal.WithLabelValues("test_observe", "op_ok", "success"))
	Observe("test_observe", "op_ok", 50*time.Millisecond, nil)
	after := testutil.ToFloat64(ExternalTotal.WithLabelValues("test_observe", "op_ok", "success"))
	assert.Equal(t, before+1, after)
}

func TestObserve_Error(t *testing.T) {
	before := testutil.ToFloat64(ExternalTotal.WithLabelValues("test_observe", "op_err", "error"))
	Observe("test_observe", "op_err", 50*time.Millisecond, errors.New("boom"))
	after := testutil.ToFloat64(ExternalTotal.WithLabelValues("test_observe", "op_err", "error"))
	assert.Equal(t, before+1, after)
}

func TestInstrumentedTransport_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	transport := &InstrumentedTransport{
		Target:    "test_transport",
		Operation: "GET",
		Inner:     http.DefaultTransport,
	}
	client := &http.Client{Transport: transport}

	before := testutil.ToFloat64(ExternalTotal.WithLabelValues("test_transport", "GET", "success"))
	resp, err := client.Get(srv.URL)
	assert.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	after := testutil.ToFloat64(ExternalTotal.WithLabelValues("test_transport", "GET", "success"))
	assert.Equal(t, before+1, after)
}

func TestInstrumentedTransport_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	transport := &InstrumentedTransport{
		Target:    "test_transport_err",
		Operation: "POST",
		Inner:     http.DefaultTransport,
	}
	client := &http.Client{Transport: transport}

	before := testutil.ToFloat64(ExternalTotal.WithLabelValues("test_transport_err", "POST", "error"))
	resp, err := client.Post(srv.URL, "application/json", nil)
	assert.NoError(t, err) // HTTP error status is not a Go error
	defer func() { _ = resp.Body.Close() }()

	after := testutil.ToFloat64(ExternalTotal.WithLabelValues("test_transport_err", "POST", "error"))
	assert.Equal(t, before+1, after)
}

func TestInstrumentedTransport_DefaultOperation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	transport := &InstrumentedTransport{
		Target: "test_transport_default",
		// Operation is empty — should fall back to HTTP method
		Inner: http.DefaultTransport,
	}
	client := &http.Client{Transport: transport}

	before := testutil.ToFloat64(ExternalTotal.WithLabelValues("test_transport_default", "GET", "success"))
	resp, err := client.Get(srv.URL)
	assert.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	after := testutil.ToFloat64(ExternalTotal.WithLabelValues("test_transport_default", "GET", "success"))
	assert.Equal(t, before+1, after)
}
