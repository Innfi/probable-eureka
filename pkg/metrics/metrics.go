package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the registered Prometheus instruments for CNI operations.
type Metrics struct {
	OpsTotal    *prometheus.CounterVec
	OpsDuration *prometheus.HistogramVec
}

// NewMetrics creates and registers instruments with reg.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		OpsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "cni_operations_total",
			Help: "Total number of CNI operations by command and status.",
		}, []string{"command", "status"}),
		OpsDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "cni_operation_duration_seconds",
			Help:    "Duration of CNI operations in seconds by command and status.",
			Buckets: prometheus.DefBuckets,
		}, []string{"command", "status"}),
	}
	reg.MustRegister(m.OpsTotal, m.OpsDuration)
	return m
}

// StartMetricsServer starts an HTTP server exposing /metrics on addr in a background goroutine.
func StartMetricsServer(addr string, gatherer prometheus.Gatherer) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	go func() {
		_ = http.ListenAndServe(addr, mux)
	}()
}
