package client

import (
	"context"
	"errors"
	"fmt"

	gcontext "github.com/obot-platform/obot/pkg/gateway/context"
	"github.com/obot-platform/obot/pkg/gateway/types"
	"github.com/obot-platform/obot/pkg/hash"
	v1 "github.com/obot-platform/obot/pkg/storage/apis/obot.obot.ai/v1"
	"github.com/obot-platform/obot/pkg/system"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type LogoutAllErr struct{}

func (e LogoutAllErr) Error() string {
	return "logout all is not supported in the current configuration"
}

func (c *Client) DeleteSessionsForUser(ctx context.Context, storageClient kclient.Client, identities []types.Identity, sessionID, localSessionID string) error {
	return c.deleteSessionsForUser(ctx, c.db.WithContext(ctx), storageClient, identities, sessionID, localSessionID)
}

func (c *Client) deleteSessionsForUser(ctx context.Context, db *gorm.DB, storageClient kclient.Client, identities []types.Identity, sessionID, localSessionID string) error {
	// Logout all sessions is only supported when using PostgreSQL.
	if db.Name() != "postgres" {
		return LogoutAllErr{}
	}

	logger := gcontext.GetLogger(ctx)
	var errs []error
	for _, identity := range identities {
		if identity.AuthProviderName == "" || identity.AuthProviderNamespace == "" {
			continue
		}

		// Local-auth sessions are stored in auth_tokens (not the OAuth proxy sessions table).
		// Delete them directly instead of trying to look up a non-existent ToolReference.
		if identity.AuthProviderName == LocalAuthProviderName {
			q := db.Where(
				"user_id = ? AND auth_provider_namespace = ? AND auth_provider_name = ?",
				identity.UserID, identity.AuthProviderNamespace, identity.AuthProviderName,
			)
			if localSessionID != "" {
				// Exclude the current session's token so the user stays logged in.
				q = q.Where("id != ?", localSessionID)
			}
			if err := q.Delete(&types.AuthToken{}).Error; err != nil {
				errs = append(errs, fmt.Errorf("failed to delete local-auth sessions: %w", err))
			} else {
				logger.Info("deleted local-auth sessions", "userID", identity.UserID)
			}
			continue
		}

		var ref v1.ToolReference
		if err := storageClient.Get(ctx, kclient.ObjectKey{Namespace: identity.AuthProviderNamespace, Name: identity.AuthProviderName}, &ref); err != nil {
			errs = append(errs, fmt.Errorf("failed to get auth provider %q: %w", identity.AuthProviderName, err))
			continue
		}

		user := identity.ProviderUserID
		if identity.AuthProviderName == "github-auth-provider" && identity.AuthProviderNamespace == system.DefaultNamespace {
			// The GitHub auth provider stores the username as the user ID in the sessions table.
			// This is because of an annoying quirk of the oauth2-proxy code for GitHub,
			// where we do not know the real user ID until after the user has logged in and the session is created,
			// and we have to manually fetch it from the GitHub API.
			// The oauth2-proxy is only aware of the username, which is why that's in the sessions table.
			user = identity.ProviderUsername
		}

		emailHash := hash.String(identity.Email)
		userHash := hash.String(user)

		logger.Debug("deleting sessions", "provider", identity.AuthProviderName, "emailHash", emailHash, "userHash", userHash)

		if meta, ok := ref.Status.Tool.Metadata["providerMeta"]; ok {
			tablePrefix := gjson.Get(meta, "postgresTablePrefix").String()
			if tablePrefix != "" {
				var err error
				if sessionID != "" {
					err = c.deleteSessionsForUserExceptCurrent(ctx, db, emailHash, userHash, tablePrefix, sessionID)
				} else {
					err = c.deleteAllSessionsForUser(ctx, db, emailHash, userHash, tablePrefix)
				}
				if err != nil {
					errs = append(errs, fmt.Errorf("failed to delete sessions for provider %q: %w", identity.AuthProviderName, err))
				} else {
					logger.Info("deleted sessions", "provider", identity.AuthProviderName, "emailHash", emailHash, "userHash", userHash)
				}
			}
		}
	}

	return errors.Join(errs...)
}

func (c *Client) tableExists(db *gorm.DB, tableName string) bool {
	return db.Migrator().HasTable(tableName)
}

func (c *Client) deleteAllSessionsForUser(ctx context.Context, db *gorm.DB, emailHash, userHash, tablePrefix string) error {
	if !c.tableExists(db, tablePrefix+"sessions") {
		gcontext.GetLogger(ctx).Info("table does not exist", "table", tablePrefix+"sessions")
		return nil
	}

	return db.Exec(
		"DELETE FROM "+tablePrefix+"sessions WHERE \"user\" = decode(?, 'hex') AND \"email\" = decode(?, 'hex')",
		userHash,
		emailHash,
	).Error
}

func (c *Client) deleteSessionsForUserExceptCurrent(ctx context.Context, db *gorm.DB, emailHash, userHash, tablePrefix, currentSessionID string) error {
	if !c.tableExists(db, tablePrefix+"sessions") {
		gcontext.GetLogger(ctx).Info("table does not exist", "table", tablePrefix+"sessions")
		return nil
	}

	return db.Exec(
		"DELETE FROM "+tablePrefix+"sessions WHERE key NOT LIKE ? AND \"user\" = decode(?, 'hex') AND \"email\" = decode(?, 'hex')",
		currentSessionID+"%",
		userHash,
		emailHash,
	).Error
}
