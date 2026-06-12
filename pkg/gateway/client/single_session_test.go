package client

import (
	"context"
	"strings"
	"testing"

	v1 "github.com/obot-platform/obot/pkg/storage/apis/obot.obot.ai/v1"
	"github.com/obot-platform/obot/pkg/system"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSingleSessionRejectsNonPostgres(t *testing.T) {
	c := newTestClient(t)

	err := c.EnforceSingleSession(t.Context(), nil, "default", "google-auth-provider", "0123456789", "user", "user@example.com")
	if err == nil {
		t.Fatal("expected non-Postgres error")
	}
	if !strings.Contains(err.Error(), "postgres") {
		t.Fatalf("expected postgres error, got %v", err)
	}
}

func TestSingleSessionRejectsEmptyUserOrEmail(t *testing.T) {
	c := newTestClient(t)

	tests := []struct {
		name  string
		user  string
		email string
		want  string
	}{
		{name: "empty user", email: "user@example.com", want: "user"},
		{name: "empty email", user: "user", want: "email"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.EnforceSingleSession(t.Context(), nil, "default", "google-auth-provider", "0123456789", tt.user, tt.email)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestSingleSessionProviderLockRejectsNonPostgres(t *testing.T) {
	c := newTestClient(t)

	err := c.WithSingleSessionProviderLock(t.Context(), "default", "google-auth-provider", func(context.Context) error {
		t.Fatal("lock callback should not run for non-Postgres DB")
		return nil
	})
	if err == nil {
		t.Fatal("expected non-Postgres lock error")
	}
	if !strings.Contains(err.Error(), "postgres") {
		t.Fatalf("expected postgres error, got %v", err)
	}
}

func TestSingleSessionTableNameRequiresStrictProviderMetadata(t *testing.T) {
	c := newTestClient(t)

	tests := []struct {
		name         string
		objects      []kclient.Object
		wantContains string
		nilStorage   bool
	}{
		{
			name:         "missing storage",
			wantContains: "storage",
			nilStorage:   true,
		},
		{
			name: "missing tool status",
			objects: []kclient.Object{&v1.ToolReference{
				ObjectMeta: metav1.ObjectMeta{Namespace: system.DefaultNamespace, Name: "google-auth-provider"},
			}},
			wantContains: "metadata",
		},
		{
			name: "missing provider metadata",
			objects: []kclient.Object{&v1.ToolReference{
				ObjectMeta: metav1.ObjectMeta{Namespace: system.DefaultNamespace, Name: "google-auth-provider"},
				Status: v1.ToolReferenceStatus{
					Tool: &v1.ToolShortDescription{Metadata: map[string]string{}},
				},
			}},
			wantContains: "metadata",
		},
		{
			name: "missing table prefix",
			objects: []kclient.Object{&v1.ToolReference{
				ObjectMeta: metav1.ObjectMeta{Namespace: system.DefaultNamespace, Name: "google-auth-provider"},
				Status: v1.ToolReferenceStatus{
					Tool: &v1.ToolShortDescription{Metadata: map[string]string{"providerMeta": `{}`}},
				},
			}},
			wantContains: "postgresTablePrefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var storage kclient.Client
			if !tt.nilStorage {
				storage = newSingleSessionStorage(t, tt.objects...)
			}

			_, err := c.singleSessionTableName(t.Context(), storage, system.DefaultNamespace, "google-auth-provider")
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Fatalf("expected error containing %q, got %v", tt.wantContains, err)
			}
		})
	}
}

func TestValidatePostgresTablePrefixRejectsUnsafeIdentifier(t *testing.T) {
	tests := []string{
		"",
		"bad prefix",
		"bad-prefix",
		"bad;prefix",
		`bad"prefix`,
		"1bad",
		"é_prefix",
	}

	for _, prefix := range tests {
		t.Run(prefix, func(t *testing.T) {
			if _, err := postgresSessionsTableName(prefix); err == nil {
				t.Fatalf("expected unsafe prefix %q to be rejected", prefix)
			}
		})
	}
}

func TestValidatePostgresTablePrefixAcceptsSafeIdentifier(t *testing.T) {
	got, err := postgresSessionsTableName("auth_provider_")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "auth_provider_sessions" {
		t.Fatalf("table name mismatch: got %q want %q", got, "auth_provider_sessions")
	}
}

func newSingleSessionStorage(t *testing.T, objects ...kclient.Object) kclient.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
}
