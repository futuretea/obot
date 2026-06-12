package server

import (
	"net/http"

	"github.com/obot-platform/obot/pkg/api"
	"github.com/obot-platform/obot/pkg/proxy"
)

func (s *Server) logoutAll(apiContext api.Context) error {
	sessionID := getSessionID(apiContext.Request)

	identities, err := apiContext.GatewayClient.FindIdentitiesForUser(apiContext.Context(), apiContext.UserID())
	if err != nil {
		return err
	}

	return apiContext.GatewayClient.DeleteSessionsForUser(apiContext.Context(), apiContext.Storage, identities, sessionID)
}

func getSessionID(req *http.Request) string {
	sessionID, err := proxy.TicketSessionIDFromRequest(req)
	if err != nil {
		return ""
	}
	return sessionID
}
