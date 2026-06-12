package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	gcontext "github.com/obot-platform/obot/pkg/gateway/context"
	"github.com/obot-platform/obot/pkg/gateway/types"
	"github.com/obot-platform/obot/pkg/hash"
	v1 "github.com/obot-platform/obot/pkg/storage/apis/obot.obot.ai/v1"
	"github.com/obot-platform/obot/pkg/system"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const singleSessionProviderLockTimeout = 15 * time.Second

type LogoutAllErr struct{}

func (e LogoutAllErr) Error() string {
	return "logout all is not supported in the current configuration"
}

func (c *Client) DeleteSessionsForUser(ctx context.Context, storageClient kclient.Client, identities []types.Identity, sessionID string) error {
	return c.deleteSessionsForUser(ctx, c.db.WithContext(ctx), storageClient, identities, sessionID)
}

func (c *Client) WithSingleSessionProviderLock(ctx context.Context, providerNamespace, providerName string, fn func(context.Context) error) error {
	db := c.db.WithContext(ctx)
	if db.Name() != "postgres" {
		return fmt.Errorf("single-session provider lock requires postgres: %w", LogoutAllErr{})
	}

	lockKey := providerNamespace + "/" + providerName
	return db.Transaction(func(tx *gorm.DB) error {
		deadline := time.Now().Add(singleSessionProviderLockTimeout)
		for {
			var locked bool
			if err := tx.Raw("SELECT pg_try_advisory_xact_lock(hashtext(?))", lockKey).Scan(&locked).Error; err != nil {
				return fmt.Errorf("acquire single-session provider lock: %w", err)
			}
			if locked {
				return fn(ctx)
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out acquiring single-session provider lock for %s/%s", providerNamespace, providerName)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}
	})
}

func (c *Client) EnforceSingleSession(ctx context.Context, storageClient kclient.Client, providerNamespace, providerName, currentSessionID, user, email string) error {
	if currentSessionID == "" {
		return errors.New("current session ID is required")
	}
	if user == "" {
		return errors.New("provider user is required")
	}
	if email == "" {
		return errors.New("provider email is required")
	}

	db := c.db.WithContext(ctx)
	if db.Name() != "postgres" {
		return fmt.Errorf("single-session enforcement requires postgres: %w", LogoutAllErr{})
	}

	tableName, err := c.singleSessionTableName(ctx, storageClient, providerNamespace, providerName)
	if err != nil {
		return err
	}
	if !c.tableExists(db, tableName) {
		return fmt.Errorf("single-session sessions table %q does not exist", tableName)
	}

	if err := c.requireCurrentSession(ctx, db, tableName, currentSessionID); err != nil {
		return err
	}

	emailHash := hash.String(email)
	userHash := hash.String(user)
	if err := db.Exec(
		"DELETE FROM "+tableName+" WHERE key NOT LIKE ? AND \"user\" = decode(?, 'hex') AND \"email\" = decode(?, 'hex')",
		currentSessionID+"%",
		userHash,
		emailHash,
	).Error; err != nil {
		return fmt.Errorf("delete other sessions for provider %q: %w", providerName, err)
	}

	gcontext.GetLogger(ctx).Infof("single_session_enforcement_result=success provider=%s/%s emailHash=%s userHash=%s", providerNamespace, providerName, emailHash, userHash)
	return nil
}

func (c *Client) DeleteCurrentSession(ctx context.Context, storageClient kclient.Client, providerNamespace, providerName, currentSessionID string) error {
	if currentSessionID == "" {
		return nil
	}
	db := c.db.WithContext(ctx)
	if db.Name() != "postgres" {
		return LogoutAllErr{}
	}

	tableName, err := c.singleSessionTableName(ctx, storageClient, providerNamespace, providerName)
	if err != nil {
		return err
	}
	if !c.tableExists(db, tableName) {
		return nil
	}

	if err := db.Exec("DELETE FROM "+tableName+" WHERE key LIKE ?", currentSessionID+"%").Error; err != nil {
		return fmt.Errorf("delete current session for provider %q: %w", providerName, err)
	}
	return nil
}

func (c *Client) deleteSessionsForUser(ctx context.Context, db *gorm.DB, storageClient kclient.Client, identities []types.Identity, sessionID string) error {
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

		logger.Debugf("deleting sessions: provider=%s emailHash=%s userHash=%s", identity.AuthProviderName, emailHash, userHash)

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
					logger.Infof("deleted sessions: provider=%s emailHash=%s userHash=%s", identity.AuthProviderName, emailHash, userHash)
				}
			}
		}
	}

	return errors.Join(errs...)
}

func (c *Client) tableExists(db *gorm.DB, tableName string) bool {
	return db.Migrator().HasTable(tableName)
}

func (c *Client) singleSessionTableName(ctx context.Context, storageClient kclient.Client, providerNamespace, providerName string) (string, error) {
	if storageClient == nil {
		return "", errors.New("storage client is required")
	}

	var ref v1.ToolReference
	if err := storageClient.Get(ctx, kclient.ObjectKey{Namespace: providerNamespace, Name: providerName}, &ref); err != nil {
		return "", fmt.Errorf("failed to get auth provider %q: %w", providerName, err)
	}
	if ref.Status.Tool == nil {
		return "", fmt.Errorf("auth provider %q has no provider metadata", providerName)
	}
	meta := ref.Status.Tool.Metadata["providerMeta"]
	if meta == "" {
		return "", fmt.Errorf("auth provider %q has no provider metadata", providerName)
	}
	tablePrefix := gjson.Get(meta, "postgresTablePrefix").String()
	if tablePrefix == "" {
		return "", fmt.Errorf("auth provider %q has no postgresTablePrefix", providerName)
	}
	return postgresSessionsTableName(tablePrefix)
}

func postgresSessionsTableName(tablePrefix string) (string, error) {
	if tablePrefix == "" {
		return "", errors.New("postgresTablePrefix is required")
	}
	for i, r := range tablePrefix {
		if i == 0 {
			if r != '_' && !isASCIILetter(r) {
				return "", fmt.Errorf("unsafe postgresTablePrefix %q", tablePrefix)
			}
			continue
		}
		if r != '_' && !isASCIILetter(r) && !isASCIIDigit(r) {
			return "", fmt.Errorf("unsafe postgresTablePrefix %q", tablePrefix)
		}
	}
	return tablePrefix + "sessions", nil
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func (c *Client) requireCurrentSession(ctx context.Context, db *gorm.DB, tableName, currentSessionID string) error {
	var count int64
	if err := db.WithContext(ctx).Raw("SELECT count(*) FROM "+tableName+" WHERE key LIKE ?", currentSessionID+"%").Scan(&count).Error; err != nil {
		return fmt.Errorf("check current session in %q: %w", tableName, err)
	}
	if count == 0 {
		return fmt.Errorf("current session %q does not exist in %q", currentSessionID, tableName)
	}
	return nil
}

func (c *Client) deleteAllSessionsForUser(ctx context.Context, db *gorm.DB, emailHash, userHash, tablePrefix string) error {
	if !c.tableExists(db, tablePrefix+"sessions") {
		gcontext.GetLogger(ctx).Infof("table does not exist: table=%s", tablePrefix+"sessions")
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
		gcontext.GetLogger(ctx).Infof("table does not exist: table=%s", tablePrefix+"sessions")
		return nil
	}

	return db.Exec(
		"DELETE FROM "+tablePrefix+"sessions WHERE key NOT LIKE ? AND \"user\" = decode(?, 'hex') AND \"email\" = decode(?, 'hex')",
		currentSessionID+"%",
		userHash,
		emailHash,
	).Error
}
