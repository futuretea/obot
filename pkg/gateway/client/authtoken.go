package client

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/obot-platform/obot/pkg/gateway/types"
	"github.com/obot-platform/obot/pkg/hash"
	"gorm.io/gorm"
)

const (
	randomTokenLength = 32
	tokenIDLength     = 8
	expirationDur     = 7 * 24 * time.Hour
)

func (c *Client) NewAuthToken(
	ctx context.Context,
	authProviderNamespace,
	authProviderName string,
	authProviderUserID string,
	userID uint,
	tr *types.TokenRequest,
) (*types.AuthToken, error) {
	randBytes := make([]byte, tokenIDLength+randomTokenLength)
	if _, err := rand.Read(randBytes); err != nil {
		return nil, fmt.Errorf("could not generate token id: %w", err)
	}

	id := randBytes[:tokenIDLength]
	token := randBytes[tokenIDLength:]

	tkn := &types.AuthToken{
		ID: fmt.Sprintf("%x", id),
		// Hash the token again for long-term storage
		HashedToken:           hash.String(fmt.Sprintf("%x", token)),
		NoExpiration:          tr.NoExpiration,
		ExpiresAt:             time.Now().Add(expirationDur),
		AuthProviderNamespace: authProviderNamespace,
		AuthProviderName:      authProviderName,
		AuthProviderUserID:    authProviderUserID,
	}
	if tkn.NoExpiration {
		tkn.ExpiresAt = time.Time{}
	}

	return tkn, c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tr != nil {
			tr.Token = publicToken(id, token)
			tr.ExpiresAt = tkn.ExpiresAt

			if err := tx.Updates(tr).Error; err != nil {
				return err
			}
		}

		tkn.UserID = userID

		return tx.Create(tkn).Error
	})
}

func publicToken(id, token []byte) string {
	return fmt.Sprintf("%x:%x", id, token)
}

// CreateSessionToken creates a new AuthToken and returns the public token string
// (format: "id:secret") that the caller can use as a bearer token or set in a cookie.
// Unlike NewAuthToken, this method does not require an existing TokenRequest record.
func (c *Client) CreateSessionToken(
	ctx context.Context,
	authProviderNamespace,
	authProviderName string,
	authProviderUserID string,
	userID uint,
	noExpiration bool,
) (string, error) {
	randBytes := make([]byte, tokenIDLength+randomTokenLength)
	if _, err := rand.Read(randBytes); err != nil {
		return "", fmt.Errorf("could not generate token id: %w", err)
	}

	id := randBytes[:tokenIDLength]
	token := randBytes[tokenIDLength:]
	public := publicToken(id, token)

	tkn := &types.AuthToken{
		ID:                    fmt.Sprintf("%x", id),
		HashedToken:           hash.String(fmt.Sprintf("%x", token)),
		NoExpiration:          noExpiration,
		ExpiresAt:             time.Now().Add(expirationDur),
		AuthProviderNamespace: authProviderNamespace,
		AuthProviderName:      authProviderName,
		AuthProviderUserID:    authProviderUserID,
		UserID:                userID,
	}
	if noExpiration {
		tkn.ExpiresAt = time.Time{}
	}

	if err := c.db.WithContext(ctx).Create(tkn).Error; err != nil {
		return "", fmt.Errorf("failed to create session token: %w", err)
	}

	return public, nil
}

// DeleteTokensByProviderAndUser removes all auth tokens for a given user+provider pair.
// Used by local-auth logout to invalidate all sessions.
func (c *Client) DeleteTokensByProviderAndUser(ctx context.Context, userID uint, providerNamespace, providerName string) error {
	return c.db.WithContext(ctx).
		Where("user_id = ? AND auth_provider_namespace = ? AND auth_provider_name = ?",
			userID, providerNamespace, providerName).
		Delete(&types.AuthToken{}).Error
}
