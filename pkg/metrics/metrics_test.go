package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRegistry() (*prometheus.Registry, *Metrics) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	return reg, m
}

func gatherMetric(t *testing.T, reg *prometheus.Registry, name string) []*dto.MetricFamily {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	var out []*dto.MetricFamily
	for _, mf := range mfs {
		if mf.GetName() == name {
			out = append(out, mf)
		}
	}
	return out
}

func TestCounterIncrement(t *testing.T) {
	reg, m := newTestRegistry()

	m.OpsTotal.WithLabelValues("ADD", "success").Inc()
	m.OpsTotal.WithLabelValues("ADD", "error").Inc()
	m.OpsTotal.WithLabelValues("DEL", "success").Inc()

	mfs := gatherMetric(t, reg, "cni_operations_total")
	require.Len(t, mfs, 1)

	var total float64
	for _, metric := range mfs[0].GetMetric() {
		total += metric.GetCounter().GetValue()
	}
	assert.Equal(t, float64(3), total)
}

func TestHistogramObserve(t *testing.T) {
	reg, m := newTestRegistry()

	m.OpsDuration.WithLabelValues("ADD", "success").Observe(0.010)
	m.OpsDuration.WithLabelValues("ADD", "success").Observe(0.050)
	m.OpsDuration.WithLabelValues("DEL", "error").Observe(0.002)

	mfs := gatherMetric(t, reg, "cni_operation_duration_seconds")
	require.Len(t, mfs, 1)

	var totalCount uint64
	for _, metric := range mfs[0].GetMetric() {
		totalCount += metric.GetHistogram().GetSampleCount()
	}
	assert.Equal(t, uint64(3), totalCount)
}

func TestMetricNames(t *testing.T) {
	reg, m := newTestRegistry()
	// Seed at least one observation so Gather emits both families.
	m.OpsTotal.WithLabelValues("ADD", "success").Inc()
	m.OpsDuration.WithLabelValues("ADD", "success").Observe(0.001)

	mfs, err := reg.Gather()
	require.NoError(t, err)

	names := make([]string, 0, len(mfs))
	for _, mf := range mfs {
		names = append(names, mf.GetName())
	}
	joined := strings.Join(names, ",")
	assert.Contains(t, joined, "cni_operations_total")
	assert.Contains(t, joined, "cni_operation_duration_seconds")
}
