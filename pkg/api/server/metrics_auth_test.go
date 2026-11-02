package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/obot-platform/obot/pkg/api/authn"
	"github.com/obot-platform/obot/pkg/api/authz"
	"github.com/obot-platform/obot/pkg/api/server/audit"
	"github.com/obot-platform/obot/pkg/api/server/ratelimiter"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/component-base/metrics/legacyregistry"
)

func TestDebugMetricsAllowsAnonymousGETOnly(t *testing.T) {
	auditLogger, err := audit.New(t.Context(), audit.Options{AuditLogsMode: audit.ModeOff})
	if err != nil {
		t.Fatal(err)
	}
	rateLimiter, err := ratelimiter.New(ratelimiter.Options{UnauthenticatedRateLimit: 100, AuthenticatedRateLimit: 100})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(nil, nil, nil, "", authn.NewAuthenticator(authn.Anonymous{}),
		authz.NewAuthorizer(nil, nil, nil, false, nil, nil, nil, false),
		nil, auditLogger, rateLimiter, "http://obot.example.com", nil, false, nil)

	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	s.HTTPHandle("GET /debug/metrics", promhttp.HandlerFor(legacyregistry.DefaultGatherer, promhttp.HandlerOpts{
		ErrorHandling: promhttp.HTTPErrorOnError,
	}))
	s.HTTPHandle("GET /debug/pprof/", ok)
	s.HTTPHandle("GET /debug/triggers", ok)
	s.HTTPHandle("GET /api/mcp-audit-logs", ok)
	s.HTTPHandle("POST /debug/metrics", ok)

	do := func(method, path string) (int, string) {
		req := httptest.NewRequest(method, path, nil)
		req.RemoteAddr = "192.0.2.10:1234"
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		body, _ := io.ReadAll(rec.Result().Body)
		return rec.Code, string(body)
	}

	code, body := do(http.MethodGet, "/debug/metrics")
	if code != http.StatusOK {
		t.Fatalf("anonymous GET /debug/metrics = %d, want 200", code)
	}
	for _, name := range []string{
		"obot_mcp_audit_log_buffered_entries",
		"obot_mcp_audit_log_persist_in_flight_entries",
		"obot_mcp_audit_log_persist_failures_total",
		"obot_mcp_audit_log_persist_last_failure_timestamp_seconds",
		"obot_mcp_audit_log_persist_last_success_timestamp_seconds",
	} {
		if !strings.Contains(body, name) {
			t.Errorf("/debug/metrics output is missing %s", name)
		}
	}

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/debug/pprof/"},
		{http.MethodGet, "/debug/triggers"},
		{http.MethodGet, "/api/mcp-audit-logs"},
		{http.MethodPost, "/debug/metrics"},
	} {
		if code, _ := do(tc.method, tc.path); code != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s = %d, want 401", tc.method, tc.path, code)
		}
	}
}
