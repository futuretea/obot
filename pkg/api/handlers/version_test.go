package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/obot-platform/obot/pkg/api"
	"github.com/obot-platform/obot/pkg/license"
	"github.com/obot-platform/obot/pkg/storage/scheme"
	"github.com/obot-platform/obot/pkg/upgrade"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type fakeUpgradeStatusReader struct {
	status upgrade.Status
}

func (f fakeUpgradeStatusReader) Status() upgrade.Status {
	return f.status
}

func TestVersionFeatureValuesIncludesHostedAgentsState(t *testing.T) {
	disabled := (&VersionHandler{}).featureValues()
	if enabled := disabled["hostedAgentsEnabled"]; enabled {
		t.Fatal("expected Hosted Agents to be disabled")
	}

	handler := &VersionHandler{}
	handler.HostedAgentsEnabled = true
	enabled := handler.featureValues()
	if hostedAgentsEnabled := enabled["hostedAgentsEnabled"]; !hostedAgentsEnabled {
		t.Fatal("expected Hosted Agents to be enabled")
	}
}

func TestVersionExposesCLIDownloadBaseURL(t *testing.T) {
	gateway := newHandlerTestGateway(t)
	licenseProvider, err := license.NewProvider(t.Context(), gateway, license.Config{})
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name string
		url  string
	}{
		{
			name: "not configured",
		},
		{
			name: "configured",
			url:  "https://downloads.example.com/releases/v0.26.3-uni",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewVersionHandler(VersionHandlerOptions{
				GatewayClient:      gateway,
				StorageClient:      fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
				LicenseProvider:    licenseProvider,
				CLIDownloadBaseURL: tt.url,
			})
			response := httptest.NewRecorder()
			if err := handler.GetVersion(api.Context{
				Request:        httptest.NewRequest(http.MethodGet, "/api/version", nil),
				ResponseWriter: response,
			}); err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result["cliDownloadBaseURL"] != tt.url {
				t.Fatalf("cliDownloadBaseURL = %v, want %q", result["cliDownloadBaseURL"], tt.url)
			}
		})
	}
}

func TestVersionHandlerReadsUpgradeStatus(t *testing.T) {
	want := upgrade.Status{UpgradeAvailable: true, LatestVersion: "v1.2.3"}
	handler := &VersionHandler{}
	handler.UpgradeStatusReader = fakeUpgradeStatusReader{status: want}
	if got := handler.upgradeStatus(); got != want {
		t.Fatalf("upgradeStatus() = %#v, want %#v", got, want)
	}

	handler.UpgradeStatusReader = nil
	if got := handler.upgradeStatus(); got != (upgrade.Status{}) {
		t.Fatalf("upgradeStatus() without reader = %#v, want zero status", got)
	}
}
