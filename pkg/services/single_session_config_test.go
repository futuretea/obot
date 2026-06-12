package services

import (
	"reflect"
	"strings"
	"testing"

	"github.com/obot-platform/obot/pkg/gateway/client"
)

func TestSingleSessionConfig(t *testing.T) {
	tests := []struct {
		name         string
		config       Config
		postgresDSN  string
		wantErr      bool
		wantContains string
	}{
		{
			name: "default disabled",
		},
		{
			name:         "enabled without authentication",
			config:       Config{SingleSessionEnabled: true},
			postgresDSN:  "postgres://example",
			wantErr:      true,
			wantContains: "authentication",
		},
		{
			name:         "enabled without postgres",
			config:       Config{SingleSessionEnabled: true, EnableAuthentication: true},
			wantErr:      true,
			wantContains: "Postgres",
		},
		{
			name:        "enabled with authentication and postgres",
			config:      Config{SingleSessionEnabled: true, EnableAuthentication: true},
			postgresDSN: "postgres://example",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSingleSessionConfig(tt.config, tt.postgresDSN)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.wantContains != "" && !strings.Contains(err.Error(), tt.wantContains) {
					t.Fatalf("expected error containing %q, got %v", tt.wantContains, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestSingleSessionConfigWiresProxyManager(t *testing.T) {
	disabled := newProxyManagerForConfig(nil, nil, &client.Client{}, nil, Config{})
	if proxyManagerSingleSessionEnabled(t, disabled) {
		t.Fatal("single-session enforcer should be disabled by default")
	}

	enabled := newProxyManagerForConfig(nil, nil, &client.Client{}, nil, Config{SingleSessionEnabled: true})
	if !proxyManagerSingleSessionEnabled(t, enabled) {
		t.Fatal("single-session enforcer should be enabled when SingleSessionEnabled is true")
	}
}

func proxyManagerSingleSessionEnabled(t *testing.T, manager any) bool {
	t.Helper()

	value := reflect.ValueOf(manager)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		t.Fatalf("expected non-nil proxy manager pointer, got %T", manager)
	}
	field := value.Elem().FieldByName("singleSession")
	if !field.IsValid() {
		t.Fatal("proxy manager missing singleSession field")
	}
	return !field.IsNil()
}
