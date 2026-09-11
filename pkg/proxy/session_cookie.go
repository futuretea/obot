package proxy

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var (
	ErrMalformedTicketCookie = errors.New("malformed oauth2-proxy ticket cookie")
	ErrShortSessionID        = errors.New("oauth2-proxy ticket session ID is too short")
)

func TicketSessionID(cookieValue string) (string, error) {
	parts := strings.Split(cookieValue, "|")
	if len(parts) != 3 {
		return "", ErrMalformedTicketCookie
	}

	firstPart, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("%w: decode ticket payload: %v", ErrMalformedTicketCookie, err)
	}

	payloadParts := strings.Split(string(firstPart), ".")
	sessionID, err := ticketSessionIDFromPayload(payloadParts)
	if err != nil {
		return "", err
	}
	if len(sessionID) < 10 {
		return "", ErrShortSessionID
	}

	return sessionID, nil
}

func ticketSessionIDFromPayload(parts []string) (string, error) {
	switch {
	case len(parts) == 2:
		return parts[0], nil
	case len(parts) == 3 && parts[0] == "v2":
		decodedID, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return "", fmt.Errorf("%w: decode session ID: %v", ErrMalformedTicketCookie, err)
		}
		return string(decodedID), nil
	case len(parts) == 3:
		decodedID, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return "", fmt.Errorf("%w: decode session ID: %v", ErrMalformedTicketCookie, err)
		}
		return string(decodedID), nil
	default:
		return "", ErrMalformedTicketCookie
	}
}

func TicketSessionIDFromRequest(req *http.Request) (string, error) {
	cookie, err := req.Cookie(ObotAccessTokenCookie)
	if err != nil {
		return "", err
	}
	return TicketSessionID(cookie.Value)
}

func ClearObotAccessTokenCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   ObotAccessTokenCookie,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:   ObotAccessTokenCookieZero,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}
