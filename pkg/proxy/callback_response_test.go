package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCallbackResponseCapture(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/callback", nil)
	resp := captureCallbackResponse(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: ObotAccessTokenCookie, Value: "new-ticket", Path: "/"})
		http.Redirect(w, req, "/", http.StatusFound)
	}), req)

	if resp.StatusCode() != http.StatusFound {
		t.Fatalf("status mismatch: got %d want %d", resp.StatusCode(), http.StatusFound)
	}
	if got := resp.Header().Get("Location"); got != "/" {
		t.Fatalf("location mismatch: got %q want %q", got, "/")
	}
	if !hasSetCookie(resp.Header(), ObotAccessTokenCookie, "new-ticket") {
		t.Fatalf("captured response missing %s Set-Cookie", ObotAccessTokenCookie)
	}

	rr := httptest.NewRecorder()
	resp.WriteTo(rr)
	if rr.Code != http.StatusFound {
		t.Fatalf("written status mismatch: got %d want %d", rr.Code, http.StatusFound)
	}
	if !hasSetCookie(rr.Header(), ObotAccessTokenCookie, "new-ticket") {
		t.Fatalf("written response missing %s Set-Cookie", ObotAccessTokenCookie)
	}
}

func TestSingleSessionCallbackFailureStripsCookie(t *testing.T) {
	sessionID := "0123456789abcdef"
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/callback":
			http.SetCookie(w, &http.Cookie{Name: ObotAccessTokenCookie, Value: ticketCookieValue(t, sessionID), Path: "/"})
			http.Redirect(w, r, "/", http.StatusFound)
		case "/obot-get-state":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"user":"u1","email":"u1@example.com"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	providerURL, err := url.Parse(provider.URL)
	if err != nil {
		t.Fatalf("parse provider URL: %v", err)
	}
	p := &Proxy{
		proxy:     httputil.NewSingleHostReverseProxy(providerURL),
		url:       provider.URL,
		name:      "google-auth-provider",
		namespace: "default",
	}
	client := &failingSingleSessionClient{}
	enforcer := newSingleSessionEnforcer(client, nil)

	req := httptest.NewRequest(http.MethodGet, "/oauth2/callback", nil)
	rr := httptest.NewRecorder()
	p.serveHTTPWithSingleSession(rr, req, enforcer)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status mismatch: got %d want %d", rr.Code, http.StatusServiceUnavailable)
	}
	if hasSetCookie(rr.Header(), ObotAccessTokenCookie, ticketCookieValue(t, sessionID)) {
		t.Fatalf("failure response leaked new %s cookie", ObotAccessTokenCookie)
	}
	if !hasClearingCookie(rr.Header(), ObotAccessTokenCookie) {
		t.Fatalf("failure response did not clear %s", ObotAccessTokenCookie)
	}
	if !hasClearingCookie(rr.Header(), ObotAccessTokenCookieZero) {
		t.Fatalf("failure response did not clear %s", ObotAccessTokenCookieZero)
	}
	if client.deletedSessionID != sessionID {
		t.Fatalf("best-effort delete session mismatch: got %q want %q", client.deletedSessionID, sessionID)
	}
}

func TestSingleSessionCallbackSuccessWritesOriginalResponse(t *testing.T) {
	sessionID := "fedcba9876543210"
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/callback":
			http.SetCookie(w, &http.Cookie{Name: ObotAccessTokenCookie, Value: ticketCookieValue(t, sessionID), Path: "/"})
			http.Redirect(w, r, "/after-login", http.StatusFound)
		case "/obot-get-state":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"user":"u1","email":"u1@example.com"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	providerURL, err := url.Parse(provider.URL)
	if err != nil {
		t.Fatalf("parse provider URL: %v", err)
	}
	p := &Proxy{
		proxy:     httputil.NewSingleHostReverseProxy(providerURL),
		url:       provider.URL,
		name:      "google-auth-provider",
		namespace: "default",
	}
	client := &recordingSingleSessionClient{}
	enforcer := newSingleSessionEnforcer(client, nil)

	req := httptest.NewRequest(http.MethodGet, "/oauth2/callback", nil)
	rr := httptest.NewRecorder()
	p.serveHTTPWithSingleSession(rr, req, enforcer)

	if rr.Code != http.StatusFound {
		t.Fatalf("status mismatch: got %d want %d", rr.Code, http.StatusFound)
	}
	if got := rr.Header().Get("Location"); got != "/after-login" {
		t.Fatalf("location mismatch: got %q want %q", got, "/after-login")
	}
	if !hasSetCookie(rr.Header(), ObotAccessTokenCookie, ticketCookieValue(t, sessionID)) {
		t.Fatalf("success response missing original %s cookie", ObotAccessTokenCookie)
	}
	if !client.locked {
		t.Fatal("expected provider lock to be acquired")
	}
	if client.providerNamespace != "default" || client.providerName != "google-auth-provider" {
		t.Fatalf("provider mismatch: got %s/%s", client.providerNamespace, client.providerName)
	}
	if client.currentSessionID != sessionID {
		t.Fatalf("session mismatch: got %q want %q", client.currentSessionID, sessionID)
	}
	if client.user != "u1" || client.email != "u1@example.com" {
		t.Fatalf("state mismatch: got user=%q email=%q", client.user, client.email)
	}
}

func TestSingleSessionCallbackGitHubUsesPreferredUsername(t *testing.T) {
	sessionID := "fedcba9876543210"
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/callback":
			http.SetCookie(w, &http.Cookie{Name: ObotAccessTokenCookie, Value: ticketCookieValue(t, sessionID), Path: "/"})
			http.Redirect(w, r, "/after-login", http.StatusFound)
		case "/obot-get-state":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"user":"github-user-id","preferredUsername":"octocat","email":"octocat@example.com"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	providerURL, err := url.Parse(provider.URL)
	if err != nil {
		t.Fatalf("parse provider URL: %v", err)
	}
	p := &Proxy{
		proxy:     httputil.NewSingleHostReverseProxy(providerURL),
		url:       provider.URL,
		name:      "github-auth-provider",
		namespace: "default",
	}
	client := &recordingSingleSessionClient{}
	enforcer := newSingleSessionEnforcer(client, nil)

	req := httptest.NewRequest(http.MethodGet, "/oauth2/callback", nil)
	rr := httptest.NewRecorder()
	p.serveHTTPWithSingleSession(rr, req, enforcer)

	if rr.Code != http.StatusFound {
		t.Fatalf("status mismatch: got %d want %d", rr.Code, http.StatusFound)
	}
	if client.user != "octocat" {
		t.Fatalf("github session user mismatch: got %q want %q", client.user, "octocat")
	}
	if client.email != "octocat@example.com" {
		t.Fatalf("github session email mismatch: got %q want %q", client.email, "octocat@example.com")
	}
}

func TestSingleSessionCallbackEmptyStateFailsClosed(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
		state        string
	}{
		{
			name:         "empty user",
			providerName: "google-auth-provider",
			state:        `{"email":"u1@example.com"}`,
		},
		{
			name:         "empty email",
			providerName: "google-auth-provider",
			state:        `{"user":"u1"}`,
		},
		{
			name:         "empty github preferred username",
			providerName: "github-auth-provider",
			state:        `{"user":"github-id","email":"u1@example.com"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionID := "0123456789abcdef"
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/oauth2/callback":
					http.SetCookie(w, &http.Cookie{Name: ObotAccessTokenCookie, Value: ticketCookieValue(t, sessionID), Path: "/"})
					http.Redirect(w, r, "/", http.StatusFound)
				case "/obot-get-state":
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(tt.state))
				default:
					http.NotFound(w, r)
				}
			}))
			defer provider.Close()

			providerURL, err := url.Parse(provider.URL)
			if err != nil {
				t.Fatalf("parse provider URL: %v", err)
			}
			p := &Proxy{
				proxy:     httputil.NewSingleHostReverseProxy(providerURL),
				url:       provider.URL,
				name:      tt.providerName,
				namespace: "default",
			}
			client := &recordingSingleSessionClient{}
			enforcer := newSingleSessionEnforcer(client, nil)

			req := httptest.NewRequest(http.MethodGet, "/oauth2/callback", nil)
			rr := httptest.NewRecorder()
			p.serveHTTPWithSingleSession(rr, req, enforcer)

			if rr.Code != http.StatusServiceUnavailable {
				t.Fatalf("status mismatch: got %d want %d", rr.Code, http.StatusServiceUnavailable)
			}
			if hasSetCookie(rr.Header(), ObotAccessTokenCookie, ticketCookieValue(t, sessionID)) {
				t.Fatalf("failure response leaked new %s cookie", ObotAccessTokenCookie)
			}
			if !hasClearingCookie(rr.Header(), ObotAccessTokenCookie) {
				t.Fatalf("failure response did not clear %s", ObotAccessTokenCookie)
			}
			if !hasClearingCookie(rr.Header(), ObotAccessTokenCookieZero) {
				t.Fatalf("failure response did not clear %s", ObotAccessTokenCookieZero)
			}
			if client.enforceCalled {
				t.Fatal("single-session cleanup should not run with incomplete provider state")
			}
			if client.deletedSessionID != sessionID {
				t.Fatalf("best-effort delete session mismatch: got %q want %q", client.deletedSessionID, sessionID)
			}
		})
	}
}

type failingSingleSessionClient struct {
	deletedSessionID string
}

func (f *failingSingleSessionClient) WithSingleSessionProviderLock(ctx context.Context, namespace, name string, fn func(context.Context) error) error {
	return fn(ctx)
}

func (f *failingSingleSessionClient) EnforceSingleSession(ctx context.Context, storage client.Client, providerNamespace, providerName, currentSessionID, user, email string) error {
	return errors.New("enforcement failed")
}

func (f *failingSingleSessionClient) DeleteCurrentSession(ctx context.Context, storage client.Client, providerNamespace, providerName, currentSessionID string) error {
	f.deletedSessionID = currentSessionID
	return nil
}

type recordingSingleSessionClient struct {
	locked            bool
	enforceCalled     bool
	deletedSessionID  string
	providerNamespace string
	providerName      string
	currentSessionID  string
	user              string
	email             string
}

func (r *recordingSingleSessionClient) WithSingleSessionProviderLock(ctx context.Context, namespace, name string, fn func(context.Context) error) error {
	r.locked = true
	return fn(ctx)
}

func (r *recordingSingleSessionClient) EnforceSingleSession(ctx context.Context, storage client.Client, providerNamespace, providerName, currentSessionID, user, email string) error {
	r.enforceCalled = true
	r.providerNamespace = providerNamespace
	r.providerName = providerName
	r.currentSessionID = currentSessionID
	r.user = user
	r.email = email
	return nil
}

func (r *recordingSingleSessionClient) DeleteCurrentSession(ctx context.Context, storage client.Client, providerNamespace, providerName, currentSessionID string) error {
	r.deletedSessionID = currentSessionID
	return nil
}

func hasSetCookie(header http.Header, name, value string) bool {
	for _, cookie := range (&http.Response{Header: header}).Cookies() {
		if cookie.Name == name && cookie.Value == value {
			return true
		}
	}
	return false
}

func hasClearingCookie(header http.Header, name string) bool {
	for _, cookie := range (&http.Response{Header: header}).Cookies() {
		if cookie.Name == name && cookie.MaxAge < 0 && cookie.Path == "/" {
			return true
		}
	}
	return false
}
