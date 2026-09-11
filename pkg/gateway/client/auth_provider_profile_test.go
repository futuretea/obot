package client

import (
	"testing"

	"github.com/obot-platform/obot/pkg/gateway/types"
)

func TestApplyAuthProviderProfileKeycloak(t *testing.T) {
	user := &types.User{}
	identity := &types.Identity{}

	applyAuthProviderProfile("keycloak-auth-provider", map[string]any{
		"picture": "https://idp.example/avatar.png",
		"name":    "Ada Lovelace",
	}, user, identity)

	if user.IconURL != "https://idp.example/avatar.png" || identity.IconURL != "https://idp.example/avatar.png" {
		t.Fatalf("keycloak picture: user=%q identity=%q", user.IconURL, identity.IconURL)
	}
	if user.DisplayName != "Ada Lovelace" {
		t.Fatalf("keycloak name: got %q", user.DisplayName)
	}
}

func TestApplyAuthProviderProfileKeycloakMissingFields(t *testing.T) {
	user := &types.User{IconURL: "keep", DisplayName: "keep-name"}
	identity := &types.Identity{IconURL: "keep"}

	applyAuthProviderProfile("keycloak-auth-provider", map[string]any{}, user, identity)

	if user.IconURL != "keep" || identity.IconURL != "keep" {
		t.Fatalf("missing fields should not clear icons: user=%q identity=%q", user.IconURL, identity.IconURL)
	}
	if user.DisplayName != "keep-name" {
		t.Fatalf("missing fields should not clear name: got %q", user.DisplayName)
	}
}

func TestApplyAuthProviderProfileDoesNotChangeEntraOrGoogle(t *testing.T) {
	user := &types.User{}
	identity := &types.Identity{}
	applyAuthProviderProfile("google-auth-provider", map[string]any{
		"picture": "https://google.example/p.png",
		"name":    "G User",
	}, user, identity)
	if user.IconURL != "https://google.example/p.png" || user.DisplayName != "G User" {
		t.Fatalf("google case drifted: %+v", user)
	}

	user = &types.User{}
	identity = &types.Identity{}
	applyAuthProviderProfile("entra-auth-provider", map[string]any{
		"icon_url": "https://entra.example/p.png",
		"name":     "E User",
	}, user, identity)
	if user.IconURL != "https://entra.example/p.png" || user.DisplayName != "E User" {
		t.Fatalf("entra case drifted: %+v", user)
	}
}
