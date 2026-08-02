// Package metrics provides Prometheus metrics for workloadguard.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics.
type Metrics struct {
	// System metrics
	LoadAverage1  prometheus.Gauge
	LoadAverage5  prometheus.Gauge
	LoadAverage15 prometheus.Gauge
	CPUCount      prometheus.Gauge
	MemoryTotal   prometheus.Gauge
	MemoryFree    prometheus.Gauge

	// Process metrics
	ProcessCount       *prometheus.GaugeVec
	ProcessCountByName *prometheus.GaugeVec

	// Policy metrics
	PolicyEvaluations *prometheus.CounterVec
	PolicyTriggers    *prometheus.CounterVec
	PolicyActions     *prometheus.CounterVec

	// Check metrics
	CheckDuration prometheus.Histogram
	ChecksTotal   *prometheus.CounterVec

	// Termination metrics
	ProcessesTerminated *prometheus.CounterVec
	ProcessesKilled     *prometheus.CounterVec

	registry *prometheus.Registry
}

// New creates a new Metrics instance with all metrics registered.
func New() *Metrics {
	registry := prometheus.NewRegistry()

	m := &Metrics{
		registry: registry,

		LoadAverage1: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "workloadguard",
			Subsystem: "system",
			Name:      "load_average_1m",
			Help:      "1-minute load average",
		}),
		LoadAverage5: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "workloadguard",
			Subsystem: "system",
			Name:      "load_average_5m",
			Help:      "5-minute load average",
		}),
		LoadAverage15: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "workloadguard",
			Subsystem: "system",
			Name:      "load_average_15m",
			Help:      "15-minute load average",
		}),
		CPUCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "workloadguard",
			Subsystem: "system",
			Name:      "cpu_count",
			Help:      "Number of logical CPUs",
		}),
		MemoryTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "workloadguard",
			Subsystem: "system",
			Name:      "memory_total_bytes",
			Help:      "Total physical memory in bytes",
		}),
		MemoryFree: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "workloadguard",
			Subsystem: "system",
			Name:      "memory_free_bytes",
			Help:      "Free memory in bytes",
		}),

		ProcessCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "workloadguard",
				Subsystem: "process",
				Name:      "count_total",
				Help:      "Total number of processes",
			},
			[]string{},
		),
		ProcessCountByName: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "workloadguard",
				Subsystem: "process",
				Name:      "count_by_name",
				Help:      "Number of processes by name",
			},
			[]string{"name"},
		),

		PolicyEvaluations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "workloadguard",
				Subsystem: "policy",
				Name:      "evaluations_total",
				Help:      "Total policy evaluations",
			},
			[]string{"policy"},
		),
		PolicyTriggers: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "workloadguard",
				Subsystem: "policy",
				Name:      "triggers_total",
				Help:      "Total policy triggers",
			},
			[]string{"policy"},
		),
		PolicyActions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "workloadguard",
				Subsystem: "policy",
				Name:      "actions_total",
				Help:      "Total actions executed",
			},
			[]string{"policy", "action"},
		),

		CheckDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "workloadguard",
			Subsystem: "check",
			Name:      "duration_seconds",
			Help:      "Time taken for policy checks",
			Buckets:   prometheus.ExponentialBuckets(0.01, 2, 10),
		}),
		ChecksTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "workloadguard",
				Subsystem: "check",
				Name:      "total",
				Help:      "Total checks performed",
			},
			[]string{"trigger"},
		),

		ProcessesTerminated: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "workloadguard",
				Subsystem: "termination",
				Name:      "sigterm_total",
				Help:      "Processes sent SIGTERM",
			},
			[]string{"policy", "process"},
		),
		ProcessesKilled: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "workloadguard",
				Subsystem: "termination",
				Name:      "sigkill_total",
				Help:      "Processes sent SIGKILL",
			},
			[]string{"policy", "process"},
		),
	}

	// Register all metrics.
	registry.MustRegister(
		m.LoadAverage1,
		m.LoadAverage5,
		m.LoadAverage15,
		m.CPUCount,
		m.MemoryTotal,
		m.MemoryFree,
		m.ProcessCount,
		m.ProcessCountByName,
		m.PolicyEvaluations,
		m.PolicyTriggers,
		m.PolicyActions,
		m.CheckDuration,
		m.ChecksTotal,
		m.ProcessesTerminated,
		m.ProcessesKilled,
	)

	return m
}

// Handler returns an HTTP handler for the metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// UpdateSystemMetrics updates system-level metrics.
func (m *Metrics) UpdateSystemMetrics(load1, load5, load15 float64, cpuCount int, memTotal, memFree uint64) {
	m.LoadAverage1.Set(load1)
	m.LoadAverage5.Set(load5)
	m.LoadAverage15.Set(load15)
	m.CPUCount.Set(float64(cpuCount))
	m.MemoryTotal.Set(float64(memTotal))
	m.MemoryFree.Set(float64(memFree))
}

// UpdateProcessCount updates total process count.
func (m *Metrics) UpdateProcessCount(count int) {
	m.ProcessCount.WithLabelValues().Set(float64(count))
}

// UpdateProcessCountByName updates process count for a specific process name.
func (m *Metrics) UpdateProcessCountByName(name string, count int) {
	m.ProcessCountByName.WithLabelValues(name).Set(float64(count))
}

// RecordPolicyEvaluation records a policy evaluation.
func (m *Metrics) RecordPolicyEvaluation(policy string) {
	m.PolicyEvaluations.WithLabelValues(policy).Inc()
}

// RecordPolicyTrigger records a policy trigger.
func (m *Metrics) RecordPolicyTrigger(policy string) {
	m.PolicyTriggers.WithLabelValues(policy).Inc()
}

// RecordPolicyAction records a policy action execution.
func (m *Metrics) RecordPolicyAction(policy, action string) {
	m.PolicyActions.WithLabelValues(policy, action).Inc()
}

// RecordCheckDuration records the duration of a check.
func (m *Metrics) RecordCheckDuration(duration time.Duration) {
	m.CheckDuration.Observe(duration.Seconds())
}

// RecordCheck records a check with its trigger type.
func (m *Metrics) RecordCheck(trigger string) {
	m.ChecksTotal.WithLabelValues(trigger).Inc()
}

// RecordTermination records process termination.
func (m *Metrics) RecordTermination(policy, process string, sigkill bool) {
	if sigkill {
		m.ProcessesKilled.WithLabelValues(policy, process).Inc()
	} else {
		m.ProcessesTerminated.WithLabelValues(policy, process).Inc()
	}
}
