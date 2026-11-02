package client

import (
	"errors"
	"testing"
	"time"

	"k8s.io/component-base/metrics/legacyregistry"
)

// gatheredValue returns the counter, gauge, or histogram sample count of the first
// series of the named metric whose labels include wantLabels, read through the
// registry that /debug/metrics serves.
func gatheredValue(t *testing.T, name string, wantLabels map[string]string) float64 {
	t.Helper()
	families, err := legacyregistry.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, m := range family.GetMetric() {
			matched := 0
			for _, label := range m.GetLabel() {
				if want, ok := wantLabels[label.GetName()]; ok && want == label.GetValue() {
					matched++
				}
			}
			if matched != len(wantLabels) {
				continue
			}
			switch {
			case m.GetCounter() != nil:
				return m.GetCounter().GetValue()
			case m.GetGauge() != nil:
				return m.GetGauge().GetValue()
			case m.GetHistogram() != nil:
				return float64(m.GetHistogram().GetSampleCount())
			}
		}
	}
	t.Fatalf("metric %s %v not found in legacyregistry", name, wantLabels)
	return 0
}

func TestRecordMCPAuditPersistExposesFailureState(t *testing.T) {
	before := gatheredValue(t, "obot_mcp_audit_log_persist_failures_total", nil)

	recordMCPAuditPersist(3, time.Now().Add(-time.Second), errors.New("context deadline exceeded"))

	if failures := gatheredValue(t, "obot_mcp_audit_log_persist_failures_total", nil); failures != before+1 {
		t.Fatalf("persist failures = %v, want %v", failures, before+1)
	}
	if lastFailure := gatheredValue(t, "obot_mcp_audit_log_persist_last_failure_timestamp_seconds", nil); lastFailure <= 0 {
		t.Fatalf("last failure timestamp = %v, want > 0", lastFailure)
	}
	if samples := gatheredValue(t, "obot_mcp_audit_log_persist_duration_seconds", map[string]string{"result": "failure"}); samples < 1 {
		t.Fatalf("persist duration failure samples = %v, want >= 1", samples)
	}
}
