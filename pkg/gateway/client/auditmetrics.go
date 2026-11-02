package client

import (
	"log/slog"
	"time"

	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
)

// slowMCPAuditPersistThreshold is a third of the 30s persist deadline, so a slow
// batch is logged well before it times out and is put back into the buffer.
const slowMCPAuditPersistThreshold = 10 * time.Second

var (
	mcpAuditBufferedEntries = metrics.NewGauge(&metrics.GaugeOpts{
		Namespace: "obot",
		Subsystem: "mcp_audit_log",
		Name:      "buffered_entries",
		Help:      "Number of MCP audit log entries waiting in memory to be persisted.",
	})
	mcpAuditPersistInFlightEntries = metrics.NewGauge(&metrics.GaugeOpts{
		Namespace: "obot",
		Subsystem: "mcp_audit_log",
		Name:      "persist_in_flight_entries",
		Help:      "Number of MCP audit log entries in the batch currently being persisted.",
	})
	mcpAuditPersistFailures = metrics.NewCounter(&metrics.CounterOpts{
		Namespace: "obot",
		Subsystem: "mcp_audit_log",
		Name:      "persist_failures_total",
		Help:      "Number of failed MCP audit log persist attempts. Failed batches are put back into the buffer.",
	})
	mcpAuditPersistLastFailure = metrics.NewGauge(&metrics.GaugeOpts{
		Namespace: "obot",
		Subsystem: "mcp_audit_log",
		Name:      "persist_last_failure_timestamp_seconds",
		Help:      "Unix time of the last failed MCP audit log persist attempt, 0 if none.",
	})
	mcpAuditPersistLastSuccess = metrics.NewGauge(&metrics.GaugeOpts{
		Namespace: "obot",
		Subsystem: "mcp_audit_log",
		Name:      "persist_last_success_timestamp_seconds",
		Help:      "Unix time of the last successful MCP audit log persist attempt, 0 if none.",
	})
	mcpAuditPersistDuration = metrics.NewHistogramVec(&metrics.HistogramOpts{
		Namespace: "obot",
		Subsystem: "mcp_audit_log",
		Name:      "persist_duration_seconds",
		Help:      "Duration of MCP audit log persist attempts.",
		Buckets:   []float64{0.01, 0.05, 0.1, 0.5, 1, 2.5, 5, 10, 20, 30, 60},
	}, []string{"result"})
)

func init() {
	legacyregistry.MustRegister(
		mcpAuditBufferedEntries,
		mcpAuditPersistInFlightEntries,
		mcpAuditPersistFailures,
		mcpAuditPersistLastFailure,
		mcpAuditPersistLastSuccess,
		mcpAuditPersistDuration,
	)
}

func recordMCPAuditPersist(entries int, start time.Time, err error) {
	now := time.Now()
	duration := now.Sub(start)
	result := "success"
	if err != nil {
		result = "failure"
		mcpAuditPersistFailures.Inc()
		mcpAuditPersistLastFailure.Set(float64(now.Unix()))
	} else {
		mcpAuditPersistLastSuccess.Set(float64(now.Unix()))
	}
	mcpAuditPersistDuration.WithLabelValues(result).Observe(duration.Seconds())

	if duration >= slowMCPAuditPersistThreshold {
		slog.Warn("Slow MCP audit log persist", "entries", entries, "duration", duration, "result", result)
	}
}
