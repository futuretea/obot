package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/obot-platform/obot/pkg/proxy"
)

func TestLogoutAllSessionIDParser(t *testing.T) {
	sessionID := "0123456789abcdef"
	req := httptest.NewRequest(http.MethodPost, "/api/logout-all", nil)
	req.AddCookie(&http.Cookie{
		Name:  proxy.ObotAccessTokenCookie,
		Value: "djIuTURFeU16UTFOamM0T1dGaVkyUmxaZy5NREV5TXpRMU5qYzRPV0ZpWTJSbFpn|signature|csrf",
	})

	if got := getSessionID(req); got != sessionID {
		t.Fatalf("session ID mismatch: got %q want %q", got, sessionID)
	}
}

func TestLogoutAllSessionIDParserReturnsEmptyForMalformedCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/logout-all", nil)
	req.AddCookie(&http.Cookie{Name: proxy.ObotAccessTokenCookie, Value: "not-a-ticket"})

	if got := getSessionID(req); got != "" {
		t.Fatalf("expected empty session ID for malformed cookie, got %q", got)
	}
}
