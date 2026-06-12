package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/obot-platform/obot/pkg/api"
	"github.com/obot-platform/obot/pkg/api/authn"
	"github.com/obot-platform/obot/pkg/proxy"
	"k8s.io/apiserver/pkg/authentication/authenticator"
)

func TestInvalidSessionClearsCookies(t *testing.T) {
	s := &Server{
		authenticator: authn.NewAuthenticator(invalidSessionAuthenticator{}),
	}
	handler := s.Wrap(func(api.Context) error {
		t.Fatal("handler should not run after invalid session")
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "http://obot.local/api/me", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status mismatch: got %d want %d", rr.Code, http.StatusFound)
	}
	if !hasClearingCookie(rr.Header(), proxy.ObotAccessTokenCookie) {
		t.Fatalf("response did not clear %s", proxy.ObotAccessTokenCookie)
	}
	if !hasClearingCookie(rr.Header(), proxy.ObotAccessTokenCookieZero) {
		t.Fatalf("response did not clear %s", proxy.ObotAccessTokenCookieZero)
	}
	if got := rr.Header().Get("Location"); got != req.URL.String() {
		t.Fatalf("redirect mismatch: got %q want %q", got, req.URL.String())
	}
}

type invalidSessionAuthenticator struct{}

func (invalidSessionAuthenticator) AuthenticateRequest(*http.Request) (*authenticator.Response, bool, error) {
	return nil, false, proxy.ErrInvalidSession
}

func hasClearingCookie(header http.Header, name string) bool {
	for _, cookie := range (&http.Response{Header: header}).Cookies() {
		if cookie.Name == name && cookie.MaxAge < 0 && cookie.Path == "/" {
			return true
		}
	}
	return false
}
