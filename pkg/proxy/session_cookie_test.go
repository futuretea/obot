package proxy

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestTicketSessionID(t *testing.T) {
	sessionID := "0123456789abcdef"
	cookieValue := ticketCookieValue(t, sessionID)

	got, err := TicketSessionID(cookieValue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != sessionID {
		t.Fatalf("session ID mismatch: got %q want %q", got, sessionID)
	}
}

func TestTicketSessionIDFromOAuth2ProxyV2Ticket(t *testing.T) {
	sessionID := "obot_access_token-0123456789abcdef0123456789abcdef"
	cookieValue := v2TicketCookieValue(t, sessionID)

	got, err := TicketSessionID(cookieValue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != sessionID {
		t.Fatalf("session ID mismatch: got %q want %q", got, sessionID)
	}
}

func TestTicketSessionIDRejectsMalformedCookie(t *testing.T) {
	_, err := TicketSessionID("not-a-ticket")
	if err == nil {
		t.Fatal("expected malformed ticket error")
	}
}

func TestTicketSessionIDRejectsShortPrefix(t *testing.T) {
	_, err := TicketSessionID(ticketCookieValue(t, "short"))
	if err == nil {
		t.Fatal("expected short session ID error")
	}
}

func TestTicketSessionIDRejectsMalformedInnerSessionID(t *testing.T) {
	firstPart := base64.StdEncoding.EncodeToString([]byte(strings.Join([]string{
		"ticket",
		"%%%%%%%%%%%%",
		"nonce",
	}, ".")))

	_, err := TicketSessionID(strings.Join([]string{firstPart, "signature", "csrf"}, "|"))
	if err == nil {
		t.Fatal("expected malformed inner session ID error")
	}
}

func ticketCookieValue(t *testing.T, sessionID string) string {
	t.Helper()

	encodedSessionID := base64.StdEncoding.EncodeToString([]byte(sessionID))
	firstPart := base64.StdEncoding.EncodeToString([]byte(strings.Join([]string{
		"ticket",
		encodedSessionID,
		"nonce",
	}, ".")))

	return strings.Join([]string{firstPart, "signature", "csrf"}, "|")
}

func v2TicketCookieValue(t *testing.T, sessionID string) string {
	t.Helper()

	ticketPayload := strings.Join([]string{
		"v2",
		base64.RawURLEncoding.EncodeToString([]byte(sessionID)),
		base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef")),
	}, ".")
	firstPart := base64.StdEncoding.EncodeToString([]byte(ticketPayload))

	return strings.Join([]string{firstPart, "signature", "csrf"}, "|")
}
