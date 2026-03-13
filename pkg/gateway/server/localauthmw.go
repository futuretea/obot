package server

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"k8s.io/apiserver/pkg/authentication/authenticator"

	"github.com/obot-platform/obot/pkg/auth"
)

// CookieAuthenticator wraps an authenticator.Request and promotes the
// obot_access_token cookie to an Authorization header before delegating.
// This lets local-auth sessions work through the standard token-review chain
// without modifying tokenreview.go.
//
// Only local-auth tokens are promoted. OAuth proxy tickets (which contain "|")
// are left untouched so that the ProxyManager authenticator can handle them.
type CookieAuthenticator struct {
	next authenticator.Request
}

// NewCookieAuthenticator converts the obot_access_token cookie into an
// Authorization Bearer header before delegating to the wrapped authenticator.
func NewCookieAuthenticator(next authenticator.Request) authenticator.Request {
	return &CookieAuthenticator{next: next}
}

func (c *CookieAuthenticator) AuthenticateRequest(req *http.Request) (*authenticator.Response, bool, error) {
	if req.Header.Get("Authorization") == "" && req.Header.Get("X-API-Key") == "" {
		if cookie, err := req.Cookie(auth.ObotAccessTokenCookie); err == nil && isLocalAuthToken(cookie.Value) {
			req.Header.Set("Authorization", "Bearer "+cookie.Value)
		}
	}
	return c.next.AuthenticateRequest(req)
}

// isLocalAuthToken reports whether value looks like a local-auth session token.
// Local-auth tokens are formatted as "<hex_id>:<hex_secret>" with no "|" characters.
// OAuth proxy ticket cookies contain "|" as a field separator and must NOT be promoted.
func isLocalAuthToken(value string) bool {
	// Empty value or OAuth proxy tickets (containing "|") are not local-auth tokens.
	// Local-auth tokens must contain ":" (id:secret separator).
	return value != "" && !strings.Contains(value, "|") && strings.Contains(value, ":")
}

// ---- IP-based rate limiter for unauthenticated login endpoint ----

type loginAttempt struct {
	count       int
	windowStart time.Time
}

type loginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt

	window   time.Duration // sliding window size
	maxBurst int           // max attempts per window
}

func newLoginRateLimiter(ctx context.Context, window time.Duration, maxBurst int) *loginRateLimiter {
	rl := &loginRateLimiter{
		attempts: make(map[string]*loginAttempt),
		window:   window,
		maxBurst: maxBurst,
	}
	// Periodic cleanup of stale entries; stops when ctx is cancelled.
	go func() {
		ticker := time.NewTicker(rl.window)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				rl.cleanup()
			}
		}
	}()
	return rl
}

func (rl *loginRateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	a, ok := rl.attempts[key]
	if !ok || now.Sub(a.windowStart) > rl.window {
		rl.attempts[key] = &loginAttempt{count: 1, windowStart: now}
		return true
	}
	a.count++
	return a.count <= rl.maxBurst
}

func (rl *loginRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-rl.window)
	for k, a := range rl.attempts {
		if a.windowStart.Before(cutoff) {
			delete(rl.attempts, k)
		}
	}
}

// clientIP extracts the client IP from the request, preferring X-Forwarded-For.
// XFF is a comma-separated list of IPs; we take the leftmost (original client).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip, _, _ := strings.Cut(xff, ",")
		return strings.TrimSpace(ip)
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}
