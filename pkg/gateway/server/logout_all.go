package server

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/obot-platform/obot/pkg/api"
	"github.com/obot-platform/obot/pkg/proxy"
)

func (s *Server) logoutAll(apiContext api.Context) error {
	sessionID := getSessionID(apiContext.Request)

	// For local-auth sessions, the cookie format is "id:secret".
	// getSessionID only handles the OAuth proxy ticket format, so we must
	// extract the token ID separately to preserve the current session.
	localSessionID := getLocalAuthSessionID(apiContext.Request)

	identities, err := apiContext.GatewayClient.FindIdentitiesForUser(apiContext.Context(), apiContext.UserID())
	if err != nil {
		return err
	}

	// For non-local-auth identities, use the OAuth proxy session ID.
	// For local-auth identities, use the local token ID.
	return apiContext.GatewayClient.DeleteSessionsForUser(
		apiContext.Context(), apiContext.Storage, identities, sessionID, localSessionID,
	)
}

// getLocalAuthSessionID extracts the token ID portion from a local-auth cookie.
// Local-auth cookies have the format "id:secret"; the id is returned.
// Returns "" if the cookie is absent or not in local-auth format.
func getLocalAuthSessionID(req *http.Request) string {
	cookie, err := req.Cookie(proxy.ObotAccessTokenCookie)
	if err != nil || cookie.Value == "" {
		return ""
	}
	id, _, ok := strings.Cut(cookie.Value, ":")
	if !ok {
		return ""
	}
	// OAuth proxy tickets also contain ":" characters after base64 decoding,
	// but they always start with a base64 segment followed by "|".
	// A local-auth id is a short hex string with no "|", so guard against false positives.
	if strings.Contains(id, "|") {
		return ""
	}
	return id
}

func getSessionID(req *http.Request) string {
	cookie, err := req.Cookie(proxy.ObotAccessTokenCookie)
	if err != nil {
		return ""
	}

	// If the cookie is an oauth2-proxy ticket cookie, it should be three segments separated by pipes.
	// The first one contains the session ID.
	parts := strings.Split(cookie.Value, "|")
	if len(parts) != 3 {
		return ""
	}

	firstPart, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return ""
	}

	// This first part, after decoding, is three parts, separated by dots.
	// The middle one is the session ID encoded in base64.
	parts = strings.Split(string(firstPart), ".")
	if len(parts) != 3 {
		return ""
	}

	// Strangely, the session ID is usually not quite complete.
	// I think it gets truncated at some point. So we have to ignore errors when decoding.
	// We will still get part of the decoded session ID, and it's a long enough prefix to work.
	decodedID, _ := base64.StdEncoding.DecodeString(parts[1])
	// If it's not at least 10 characters, we can't really use it.
	// I've never seen this happen in testing, but it's best to be safe.
	if len(decodedID) < 10 {
		return ""
	}

	return string(decodedID)
}
