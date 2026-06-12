package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type singleSessionClient interface {
	WithSingleSessionProviderLock(ctx context.Context, providerNamespace, providerName string, fn func(context.Context) error) error
	EnforceSingleSession(ctx context.Context, storage kclient.Client, providerNamespace, providerName, currentSessionID, user, email string) error
	DeleteCurrentSession(ctx context.Context, storage kclient.Client, providerNamespace, providerName, currentSessionID string) error
}

type singleSessionEnforcer struct {
	client     singleSessionClient
	storage    kclient.Client
	httpClient *http.Client
}

func newSingleSessionEnforcer(client singleSessionClient, storage kclient.Client) *singleSessionEnforcer {
	return &singleSessionEnforcer{
		client:     client,
		storage:    storage,
		httpClient: http.DefaultClient,
	}
}

func (e *singleSessionEnforcer) enabled() bool {
	return e != nil && e.client != nil
}

func (p *Proxy) serveHTTPWithSingleSession(w http.ResponseWriter, r *http.Request, enforcer *singleSessionEnforcer) {
	if !enforcer.enabled() {
		p.serveHTTP(w, r)
		return
	}
	if r.URL.Path != "/oauth2/callback" {
		p.serveHTTP(w, r)
		return
	}

	if err := enforcer.client.WithSingleSessionProviderLock(r.Context(), p.namespace, p.name, func(ctx context.Context) error {
		req := r.WithContext(ctx)
		captured := captureCallbackResponse(p.proxy, req)
		sessionID, err := p.enforceSingleSessionCallback(ctx, req, captured, enforcer)
		if err != nil {
			if sessionID != "" {
				if deleteErr := enforcer.client.DeleteCurrentSession(ctx, enforcer.storage, p.namespace, p.name, sessionID); deleteErr != nil {
					log.Warnf("single_session_enforcement_result=failure provider=%s/%s reason=%v delete_current_session_error=%v", p.namespace, p.name, err, deleteErr)
				}
			}
			writeSingleSessionFailure(w)
			return nil
		}
		captured.WriteTo(w)
		return nil
	}); err != nil {
		log.Warnf("single_session_enforcement_result=failure provider=%s/%s reason=%v", p.namespace, p.name, err)
		writeSingleSessionFailure(w)
	}
}

func (p *Proxy) enforceSingleSessionCallback(ctx context.Context, r *http.Request, response *callbackResponse, enforcer *singleSessionEnforcer) (string, error) {
	cookieHeader, ticketValue, err := singleSessionCookieHeader(response.Header())
	if err != nil {
		return "", err
	}

	sessionID, err := TicketSessionID(ticketValue)
	if err != nil {
		return "", err
	}

	state, err := p.loadStateWithCookie(ctx, r, cookieHeader, enforcer.httpClient)
	if err != nil {
		return sessionID, err
	}

	userName := state.User
	if p.name == "github-auth-provider" {
		userName = state.PreferredUsername
	}
	if userName == "" {
		return sessionID, errors.New("provider state user is required")
	}
	if state.Email == "" {
		return sessionID, errors.New("provider state email is required")
	}

	if err := enforcer.client.EnforceSingleSession(ctx, enforcer.storage, p.namespace, p.name, sessionID, userName, state.Email); err != nil {
		return sessionID, err
	}

	log.Infof("single_session_enforcement_result=success provider=%s/%s", p.namespace, p.name)
	return sessionID, nil
}

func singleSessionCookieHeader(header http.Header) (string, string, error) {
	var cookies []string
	var ticketValue string
	for _, cookie := range (&http.Response{Header: header}).Cookies() {
		switch cookie.Name {
		case ObotAccessTokenCookie:
			ticketValue = cookie.Value
			cookies = append(cookies, cookie.Name+"="+cookie.Value)
		case ObotAccessTokenCookieZero:
			cookies = append(cookies, cookie.Name+"="+cookie.Value)
		}
	}
	if ticketValue == "" {
		return "", "", errors.New("callback response did not set obot access token cookie")
	}
	return strings.Join(cookies, "; "), ticketValue, nil
}

func (p *Proxy) loadStateWithCookie(ctx context.Context, r *http.Request, cookieHeader string, client *http.Client) (serializableState, error) {
	header := r.Header.Clone()
	header.Set("Cookie", cookieHeader)
	sr := serializableRequest{
		Method: r.Method,
		URL:    r.URL.String(),
		Header: header,
	}

	srJSON, err := json.Marshal(sr)
	if err != nil {
		return serializableState{}, err
	}

	stateRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url+"/obot-get-state", strings.NewReader(string(srJSON)))
	if err != nil {
		return serializableState{}, err
	}

	if client == nil {
		client = http.DefaultClient
	}
	stateResponse, err := client.Do(stateRequest)
	if err != nil {
		return serializableState{}, err
	}
	defer stateResponse.Body.Close()

	if stateResponse.StatusCode < http.StatusOK || stateResponse.StatusCode >= http.StatusMultipleChoices {
		return serializableState{}, fmt.Errorf("failed to load auth provider state: status %d", stateResponse.StatusCode)
	}

	var state serializableState
	if err := json.NewDecoder(stateResponse.Body).Decode(&state); err != nil {
		return serializableState{}, fmt.Errorf("decode auth provider state: %w", err)
	}
	return state, nil
}

func writeSingleSessionFailure(w http.ResponseWriter) {
	ClearObotAccessTokenCookies(w)
	http.Error(w, "single session enforcement failed", http.StatusServiceUnavailable)
}
