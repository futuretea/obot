package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/apiserver/pkg/authentication/user"
)

func TestDebugMetricsIsTheOnlyAnonymousDebugRoute(t *testing.T) {
	authorizer := NewAuthorizer(nil, nil, nil, false, nil, nil, nil, false)
	anonymous := &user.DefaultInfo{Name: "anonymous", UID: "anonymous", Groups: []string{UnauthenticatedGroup}}

	if !authorizer.Authorize(httptest.NewRequest(http.MethodGet, "/debug/metrics", nil), anonymous) {
		t.Fatal("anonymous GET /debug/metrics denied, want allowed")
	}

	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/debug/metrics"},
		{http.MethodGet, "/debug/metrics/extra"},
		{http.MethodGet, "/debug/pprof/"},
		{http.MethodGet, "/debug/pprof/heap"},
		{http.MethodGet, "/debug/triggers"},
	} {
		if authorizer.Authorize(httptest.NewRequest(tc.method, tc.path, nil), anonymous) {
			t.Errorf("anonymous %s %s allowed, want denied", tc.method, tc.path)
		}
	}
}
