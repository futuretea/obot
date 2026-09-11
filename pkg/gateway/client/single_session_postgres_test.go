package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	gatewaydb "github.com/obot-platform/obot/pkg/gateway/db"
	"github.com/obot-platform/obot/pkg/hash"
	"github.com/obot-platform/obot/pkg/system"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSingleSessionPostgresCleanup(t *testing.T) {
	c, gormDB := newPostgresSingleSessionClient(t)

	prefix := fmt.Sprintf("ss_test_%d_", time.Now().UnixNano())
	tableName := createPostgresSessionsTable(t, gormDB, prefix)

	userHash := hash.String("user-1")
	emailHash := hash.String("user-1@example.com")
	insertSession := func(key, user, email string) {
		t.Helper()
		if err := gormDB.Exec(
			"INSERT INTO "+tableName+" (key, value, expires_at, \"user\", email) VALUES (?, decode('00', 'hex'), now(), decode(?, 'hex'), decode(?, 'hex'))",
			key,
			user,
			email,
		).Error; err != nil {
			t.Fatalf("insert session %s: %v", key, err)
		}
	}

	insertSession("current-prefix-suffix", userHash, emailHash)
	insertSession("old-session", userHash, emailHash)
	insertSession("different-user", hash.String("user-2"), emailHash)
	insertSession("different-email", userHash, hash.String("other@example.com"))

	storage := newSingleSessionStorage(t, singleSessionAuthProvider("google-auth-provider", prefix))

	if err := c.EnforceSingleSession(t.Context(), storage, system.DefaultNamespace, "google-auth-provider", "current-prefix", "user-1", "user-1@example.com"); err != nil {
		t.Fatalf("enforce single session: %v", err)
	}

	var keys []string
	if err := gormDB.Raw("SELECT key FROM " + tableName + " ORDER BY key").Scan(&keys).Error; err != nil {
		t.Fatalf("select remaining sessions: %v", err)
	}
	want := []string{"current-prefix-suffix", "different-email", "different-user"}
	if !slices.Equal(keys, want) {
		t.Fatalf("remaining sessions mismatch: got %v want %v", keys, want)
	}
}

func TestSingleSessionPostgresGitHubUsesPreferredUsername(t *testing.T) {
	c, gormDB := newPostgresSingleSessionClient(t)

	prefix := fmt.Sprintf("ss_github_test_%d_", time.Now().UnixNano())
	tableName := createPostgresSessionsTable(t, gormDB, prefix)

	usernameHash := hash.String("octocat")
	emailHash := hash.String("octocat@example.com")
	userIDHash := hash.String("github-user-id")
	insertSession := func(key, user string) {
		t.Helper()
		if err := gormDB.Exec(
			"INSERT INTO "+tableName+" (key, value, expires_at, \"user\", email) VALUES (?, decode('00', 'hex'), now(), decode(?, 'hex'), decode(?, 'hex'))",
			key,
			user,
			emailHash,
		).Error; err != nil {
			t.Fatalf("insert session %s: %v", key, err)
		}
	}
	insertSession("current-prefix-suffix", usernameHash)
	insertSession("old-username-session", usernameHash)
	insertSession("wrong-user-id-session", userIDHash)

	storage := newSingleSessionStorage(t, singleSessionAuthProvider("github-auth-provider", prefix))
	if err := c.EnforceSingleSession(t.Context(), storage, system.DefaultNamespace, "github-auth-provider", "current-prefix", "octocat", "octocat@example.com"); err != nil {
		t.Fatalf("enforce single session: %v", err)
	}

	var keys []string
	if err := gormDB.Raw("SELECT key FROM " + tableName + " ORDER BY key").Scan(&keys).Error; err != nil {
		t.Fatalf("select remaining sessions: %v", err)
	}
	want := []string{"current-prefix-suffix", "wrong-user-id-session"}
	if !slices.Equal(keys, want) {
		t.Fatalf("remaining sessions mismatch: got %v want %v", keys, want)
	}
}

func TestSingleSessionPostgresStrictPreflight(t *testing.T) {
	c, gormDB := newPostgresSingleSessionClient(t)

	t.Run("missing sessions table", func(t *testing.T) {
		prefix := fmt.Sprintf("ss_missing_table_%d_", time.Now().UnixNano())
		storage := newSingleSessionStorage(t, singleSessionAuthProvider("google-auth-provider", prefix))

		err := c.EnforceSingleSession(t.Context(), storage, system.DefaultNamespace, "google-auth-provider", "current-prefix", "user-1", "user-1@example.com")
		if err == nil || !strings.Contains(err.Error(), "does not exist") {
			t.Fatalf("expected missing table error, got %v", err)
		}
	})

	t.Run("missing current session", func(t *testing.T) {
		prefix := fmt.Sprintf("ss_missing_current_%d_", time.Now().UnixNano())
		createPostgresSessionsTable(t, gormDB, prefix)
		storage := newSingleSessionStorage(t, singleSessionAuthProvider("google-auth-provider", prefix))

		err := c.EnforceSingleSession(t.Context(), storage, system.DefaultNamespace, "google-auth-provider", "current-prefix", "user-1", "user-1@example.com")
		if err == nil || !strings.Contains(err.Error(), "current session") {
			t.Fatalf("expected missing current session error, got %v", err)
		}
	})
}

func TestSingleSessionPostgresProviderLockReleasesAfterCanceledCallback(t *testing.T) {
	c, _ := newPostgresSingleSessionClient(t)

	err := c.WithSingleSessionProviderLock(t.Context(), system.DefaultNamespace, "google-auth-provider", func(ctx context.Context) error {
		return context.Canceled
	})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected callback cancellation, got %v", err)
	}

	err = c.WithSingleSessionProviderLock(t.Context(), system.DefaultNamespace, "google-auth-provider", func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("lock should be released after canceled callback: %v", err)
	}
}

func newPostgresSingleSessionClient(t *testing.T) (*Client, *gorm.DB) {
	t.Helper()

	dsn := os.Getenv("OBOT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("OBOT_TEST_POSTGRES_DSN is not set")
	}

	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	db, err := gatewaydb.New(gormDB, sqlDB, false)
	if err != nil {
		t.Fatalf("create gateway db: %v", err)
	}
	return &Client{db: db}, gormDB
}

func createPostgresSessionsTable(t *testing.T, gormDB *gorm.DB, prefix string) string {
	t.Helper()

	tableName, err := postgresSessionsTableName(prefix)
	if err != nil {
		t.Fatalf("sessions table name: %v", err)
	}
	if err := gormDB.Exec("CREATE TABLE " + tableName + " (key text PRIMARY KEY, value bytea, expires_at timestamptz, \"user\" bytea, email bytea)").Error; err != nil {
		t.Fatalf("create sessions table: %v", err)
	}
	t.Cleanup(func() {
		_ = gormDB.Exec("DROP TABLE IF EXISTS " + tableName).Error
	})
	return tableName
}
