package server

import (
	"net/http"

	"github.com/obot-platform/obot/pkg/api"
	"github.com/obot-platform/obot/pkg/hash"
	"github.com/obot-platform/obot/pkg/proxy"
)

func (s *Server) logoutAll(apiContext api.Context) error {
	sessionID := getSessionID(apiContext.Request)
	localSessionID := getLocalSessionID(apiContext.Request)

	identities, err := apiContext.GatewayClient.FindIdentitiesForUser(apiContext.Context(), apiContext.UserID())
	if err != nil {
		return err
	}

	return apiContext.GatewayClient.DeleteSessionsForUser(apiContext.Context(), apiContext.Storage, identities, sessionID, localSessionID)
}

// getLocalSessionID returns the local auth provider's session ID for the current request, which is
// the hash of the access-token cookie (matching how the local provider stores sessions). It is
// empty when there is no cookie. For a non-local cookie the value simply won't match any local
// session, so passing it is harmless.
func getLocalSessionID(req *http.Request) string {
	cookie, err := req.Cookie(proxy.ObotAccessTokenCookie)
	if err != nil {
		return ""
	}
	return hash.String(cookie.Value)
}

func getSessionID(req *http.Request) string {
	sessionID, err := proxy.TicketSessionIDFromRequest(req)
	if err != nil {
		return ""
	}
	return sessionID
}
