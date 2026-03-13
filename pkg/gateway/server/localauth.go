package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	types2 "github.com/obot-platform/obot/apiclient/types"
	"github.com/obot-platform/obot/pkg/api"
	"github.com/obot-platform/obot/pkg/gateway/client"
	"github.com/obot-platform/obot/pkg/gateway/types"
	"github.com/obot-platform/obot/pkg/proxy"
)

type localLoginRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	NewPassword string `json:"newPassword,omitempty"` // used for must-change-on-first-login flow
}

type localLoginResponse struct {
	// MustChange is set when the user must choose a new password before a session is granted.
	// No token or cookie is issued in this case.
	MustChange bool `json:"mustChangePassword,omitempty"`
	// Success is true when login completed and the session cookie was set.
	Success bool `json:"success,omitempty"`
}

type localUserRequest struct {
	Username string      `json:"username"`
	Email    string      `json:"email"`
	Password string      `json:"password"`
	Role     types2.Role `json:"role"`
}

type localPasswordRequest struct {
	CurrentPassword string `json:"currentPassword,omitempty"` // required for self-service; ignored for admin reset
	NewPassword     string `json:"newPassword"`
	MustChange      bool   `json:"mustChangePassword"`
}

// localActionResponse is the standard response body for local-auth mutation endpoints.
type localActionResponse struct {
	LoggedOut bool `json:"loggedOut,omitempty"`
	Updated   bool `json:"updated,omitempty"`
	Unlocked  bool `json:"unlocked,omitempty"`
}

// localLogin handles POST /api/local/login (unauthenticated).
//
// Normal flow:      {username, password}               → 200 {success:true} + cookie
// Must-change step1: {username, password}               → 200 {mustChangePassword:true}
// Must-change step2: {username, password, newPassword}  → 200 {success:true} + cookie
func (s *Server) localLogin(apiContext api.Context) error {
	if !s.loginLimiter.Allow(clientIP(apiContext.Request)) {
		return types2.NewErrHTTP(http.StatusTooManyRequests, "too many login attempts, please try again later")
	}

	var req localLoginRequest
	if err := apiContext.Read(&req); err != nil {
		return types2.NewErrHTTP(http.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		return types2.NewErrHTTP(http.StatusBadRequest, "username and password are required")
	}

	// Must-change completion: verify old creds, change password, then issue session.
	if req.NewPassword != "" {
		user, err := apiContext.GatewayClient.VerifyAndChangePassword(
			apiContext.Context(), req.Username, req.Password, req.NewPassword,
		)
		if err != nil {
			return s.mapLocalAuthError(err, http.StatusBadRequest)
		}
		return s.issueLocalSession(apiContext, user)
	}

	user, mustChange, err := apiContext.GatewayClient.VerifyLocalCredential(apiContext.Context(), req.Username, req.Password)
	if err != nil {
		return s.mapVerifyCredentialError(err)
	}
	if mustChange {
		return apiContext.Write(localLoginResponse{MustChange: true})
	}
	return s.issueLocalSession(apiContext, user)
}

// mapLocalAuthError maps local auth errors to appropriate HTTP responses.
func (s *Server) mapLocalAuthError(err error, defaultStatus int) error {
	if errors.Is(err, client.ErrAccountLocked) {
		return types2.NewErrHTTP(http.StatusTooManyRequests, err.Error())
	}
	return types2.NewErrHTTP(defaultStatus, err.Error())
}

// mapVerifyCredentialError maps credential verification errors to appropriate HTTP responses.
func (s *Server) mapVerifyCredentialError(err error) error {
	if errors.Is(err, client.ErrAccountLocked) {
		return types2.NewErrHTTP(http.StatusTooManyRequests, err.Error())
	}
	if errors.Is(err, client.ErrAccountDisabled) {
		return types2.NewErrHTTP(http.StatusForbidden, "account disabled: contact an administrator")
	}
	return types2.NewErrHTTP(http.StatusUnauthorized, "invalid username or password")
}

// issueLocalSession creates a session token, sets the HttpOnly cookie, and writes a success response.
// The token is NOT included in the response body — the cookie is the only transport.
//
// Session lifetime: 15 min for admin/owner, 30 min for basic users (Dev-38, Gen-13/14).
// Any existing local-auth sessions are invalidated first (single concurrent session, Dev-36).
func (s *Server) issueLocalSession(apiContext api.Context, user *types.User) error {
	// Intentionally best-effort: a leftover token merely shortens the effective
	// session lifetime on next login; failure must not block the current login.
	_ = apiContext.GatewayClient.DeleteTokensByProviderAndUser(
		apiContext.Context(), user.ID,
		client.LocalAuthProviderNamespace, client.LocalAuthProviderName,
	)

	tokenStr, err := apiContext.GatewayClient.CreateSessionToken(
		apiContext.Context(),
		client.LocalAuthProviderNamespace,
		client.LocalAuthProviderName,
		user.Username,
		user.ID,
		false,
	)
	if err != nil {
		return fmt.Errorf("failed to issue auth token: %w", err)
	}

	sessionDuration := 30 * time.Minute
	if user.Role.HasRole(types2.RoleAdmin) || user.Role.HasRole(types2.RoleOwner) {
		// Privileged accounts get a shorter session lifetime (principle of least privilege:
		// the higher the privilege, the shorter the window of exposure if a token is stolen).
		sessionDuration = 15 * time.Minute
	}

	http.SetCookie(apiContext.ResponseWriter, s.localAuthCookie(tokenStr, int(sessionDuration.Seconds())))
	return apiContext.Write(localLoginResponse{Success: true})
}

// localLogout handles POST /api/local/logout (authenticated).
func (s *Server) localLogout(apiContext api.Context) error {
	http.SetCookie(apiContext.ResponseWriter, s.localAuthCookie("", -1))
	if userID := apiContext.UserID(); userID != 0 {
		_ = apiContext.GatewayClient.DeleteTokensByProviderAndUser(
			apiContext.Context(), userID,
			client.LocalAuthProviderNamespace, client.LocalAuthProviderName,
		)
	}
	return apiContext.Write(localActionResponse{LoggedOut: true})
}

// createLocalUser handles POST /api/local/users (admin/owner only).
// Authorization is enforced by the authz rule "POST /api/local/users" in adminAndOwnerRules.
func (s *Server) createLocalUser(apiContext api.Context) error {
	var req localUserRequest
	if err := apiContext.Read(&req); err != nil {
		return types2.NewErrHTTP(http.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		return types2.NewErrHTTP(http.StatusBadRequest, "username and password are required")
	}

	role := req.Role
	if role == types2.RoleUnknown {
		role = types2.RoleBasic
	}

	// ProviderUserID = username — stable and immutable for local accounts.
	identity := &types.Identity{
		AuthProviderName:      client.LocalAuthProviderName,
		AuthProviderNamespace: client.LocalAuthProviderNamespace,
		ProviderUserID:        req.Username,
		ProviderUsername:      req.Username,
		Email:                 req.Email,
	}

	gatewayUser, err := apiContext.GatewayClient.EnsureIdentityWithRole(apiContext.Context(), identity, "", role)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "duplicate key") {
			return types2.NewErrHTTP(http.StatusConflict, fmt.Sprintf("username %q already exists", req.Username))
		}
		return fmt.Errorf("failed to create local user: %w", err)
	}

	if err := apiContext.GatewayClient.CreateLocalCredential(apiContext.Context(), gatewayUser.ID, req.Password); err != nil {
		// Compensating action: remove the orphaned user+identity so the admin can retry.
		if _, err2 := apiContext.GatewayClient.DeleteUser(apiContext.Context(), client.FormatUserID(gatewayUser.ID)); err2 != nil {
			logger.Errorf("failed to rollback orphaned user %d after credential creation failure: %v", gatewayUser.ID, err2)
		}
		return fmt.Errorf("failed to store local credential: %w", err)
	}

	return apiContext.Write(types.ConvertUser(gatewayUser, false, client.LocalAuthProviderName))
}

// changeOwnPassword handles PUT /api/local/me/password.
// Requires a valid current password; always clears MustChange on success.
func (s *Server) changeOwnPassword(apiContext api.Context) error {
	hasLocal, err := apiContext.GatewayClient.HasLocalCredential(apiContext.Context(), apiContext.UserID())
	if err != nil {
		return fmt.Errorf("failed to check local credential: %w", err)
	}
	if !hasLocal {
		return types2.NewErrHTTP(http.StatusBadRequest, "this account does not use local authentication")
	}

	var req localPasswordRequest
	if err := apiContext.Read(&req); err != nil {
		return types2.NewErrHTTP(http.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.CurrentPassword) == "" {
		return types2.NewErrHTTP(http.StatusBadRequest, "currentPassword is required")
	}
	if strings.TrimSpace(req.NewPassword) == "" {
		return types2.NewErrHTTP(http.StatusBadRequest, "newPassword is required")
	}

	user, err := apiContext.GatewayClient.UserByID(apiContext.Context(), client.FormatUserID(apiContext.UserID()))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return types2.NewErrNotFound("user %d not found", apiContext.UserID())
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	if _, _, err := apiContext.GatewayClient.VerifyLocalCredential(apiContext.Context(), user.Username, req.CurrentPassword); err != nil {
		return s.mapLocalAuthError(err, http.StatusUnauthorized)
	}

	if err := apiContext.GatewayClient.UpdateLocalPassword(apiContext.Context(), user.ID, req.NewPassword, false); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	return apiContext.Write(localActionResponse{Updated: true})
}

// updateLocalPassword handles PUT /api/local/users/{user_id}/password (admin/owner only).
// Authorization is enforced by the authz rule "PUT /api/local/users/{user_id}/password" in adminAndOwnerRules.
func (s *Server) updateLocalPassword(apiContext api.Context) error {
	user, err := requireAdminTarget(apiContext)
	if err != nil {
		return err
	}

	var req localPasswordRequest
	if err := apiContext.Read(&req); err != nil {
		return types2.NewErrHTTP(http.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.NewPassword) == "" {
		return types2.NewErrHTTP(http.StatusBadRequest, "newPassword is required")
	}

	if err := apiContext.GatewayClient.UpdateLocalPassword(apiContext.Context(), user.ID, req.NewPassword, req.MustChange); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	return apiContext.Write(localActionResponse{Updated: true})
}

// unlockLocalUser handles POST /api/local/users/{user_id}/unlock (admin/owner only).
// It clears both the inactivity-disabled flag and any active lockout.
// Authorization is enforced by the authz rule "POST /api/local/users/{user_id}/unlock" in adminAndOwnerRules.
func (s *Server) unlockLocalUser(apiContext api.Context) error {
	user, err := requireAdminTarget(apiContext)
	if err != nil {
		return err
	}
	if err := apiContext.GatewayClient.UnlockLocalUser(apiContext.Context(), user.ID); err != nil {
		return fmt.Errorf("failed to unlock user: %w", err)
	}
	return apiContext.Write(localActionResponse{Unlocked: true})
}

// localAuthStatus handles GET /api/local/status (unauthenticated).
func (s *Server) localAuthStatus(apiContext api.Context) error {
	return apiContext.Write(map[string]bool{"enabled": s.localAuthEnabled})
}

// localAuthCookie builds a cookie for the local-auth session token.
func (s *Server) localAuthCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     proxy.ObotAccessTokenCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   strings.HasPrefix(s.uiURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	}
}

// requireAdminTarget resolves the target user from the {user_id} path parameter.
// Admin/owner authorization is already enforced by the authz rule layer;
// this function only handles the user lookup and 404 handling.
func requireAdminTarget(apiContext api.Context) (*types.User, error) {
	userID := apiContext.PathValue("user_id")
	if userID == "" {
		return nil, types2.NewErrHTTP(http.StatusBadRequest, "user_id path parameter is required")
	}

	user, err := apiContext.GatewayClient.UserByID(apiContext.Context(), userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, types2.NewErrNotFound("user %s not found", userID)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// disableInactiveLocalUsersLoop disables local accounts inactive beyond 30 days (Dev-7).
func (s *Server) disableInactiveLocalUsersLoop(ctx context.Context) {
	const (
		inactiveDuration = 30 * 24 * time.Hour // 30 days
		pollInterval     = 24 * time.Hour
	)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.gatewayClient.DisableInactiveLocalUsers(ctx, inactiveDuration); err != nil {
				slog.ErrorContext(ctx, "failed to disable inactive local users", "error", err)
			}
		}
	}
}
