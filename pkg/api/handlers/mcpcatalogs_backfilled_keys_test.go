package handlers

import (
	"net/http"
	"testing"

	"github.com/obot-platform/obot/apiclient/types"
	"github.com/obot-platform/obot/pkg/mcp"
	v1 "github.com/obot-platform/obot/pkg/storage/apis/obot.obot.ai/v1"
	"github.com/obot-platform/obot/pkg/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestUpdateBackfilledEntryKeepsExistingConfigurationKeys(t *testing.T) {
	stored := types.MCPServerCatalogEntryManifest{
		Name:    "Example",
		Runtime: types.RuntimeContainerized,
		ContainerizedConfig: &types.ContainerizedRuntimeConfig{
			Image: "example.invalid/mcp:test", Port: 8080, Path: "/mcp",
			Args: []string{"--kubeconfig-paths", "/files/legacy-kube"},
		},
		Config: []types.MCPConfig{
			{Key: "kube", Usage: types.File},
			{Key: "views.yaml", Usage: types.File},
		},
	}
	edited := func(required bool) types.MCPServerCatalogEntryManifest {
		manifest := *stored.DeepCopy()
		manifest.ContainerizedConfig.Args = []string{"--kubeconfig-paths", "${kube}", "--views", "${views.yaml}"}
		manifest.Config[0].Required = required
		manifest.Config[1].Required = required
		manifest.Config = append(manifest.Config, types.MCPConfig{Key: "log-level", Usage: types.Env})
		return manifest
	}

	tests := []struct {
		name        string
		annotations map[string]string
		manifest    types.MCPServerCatalogEntryManifest
		wantKeys    []string
		wantErr     string
	}{
		{
			name:        "backfilled entry keeps existing keys and normalizes new ones",
			annotations: map[string]string{v1.BackfillVMCPAnnotation: "vmcp1example"},
			manifest:    edited(true),
			wantKeys:    []string{"kube", "views.yaml", "LOG_LEVEL"},
		},
		{
			name:        "backfilled entry still requires referenced configuration",
			annotations: map[string]string{v1.BackfillVMCPAnnotation: "vmcp1example"},
			manifest:    edited(false),
			wantErr:     `env var "kube" is referenced from a command/args/URL template and must be required=true`,
		},
		{
			name:     "other entries normalize every key",
			manifest: edited(true),
			wantErr:  `template references undeclared env var "kube"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &v1.MCPServerCatalogEntry{
				Name:        "entry1backfilled",
				Namespace:   system.DefaultNamespace,
				Annotations: tt.annotations,
				Spec:        v1.MCPServerCatalogEntrySpec{MCPCatalogName: system.DefaultCatalog, Editable: true, Manifest: *stored.DeepCopy()},
			}
			storage := newFakeStorage(t, &v1.MCPCatalog{Name: system.DefaultCatalog, Namespace: system.DefaultNamespace}, entry)
			handler := &MCPCatalogHandler{mcpBackend: "docker", sessionManager: &mcp.SessionManager{}}
			req, _ := catalogEntryRequest(t, storage, newEnforcementTestGatewayClient(t), http.MethodPut, "/api/mcp-catalogs/"+system.DefaultCatalog+"/entries/"+entry.Name, tt.manifest, map[string]string{"catalog_id": system.DefaultCatalog, "entry_id": entry.Name})

			err := handler.UpdateEntry(req)
			var updated v1.MCPServerCatalogEntry
			require.NoError(t, storage.Get(t.Context(), kclient.ObjectKeyFromObject(entry), &updated))
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, stored, updated.Spec.Manifest)
				return
			}
			require.NoError(t, err)
			var keys []string
			for _, field := range updated.Spec.Manifest.Config {
				keys = append(keys, field.Key)
			}
			assert.Equal(t, tt.wantKeys, keys)
			assert.Equal(t, tt.manifest.ContainerizedConfig.Args, updated.Spec.Manifest.ContainerizedConfig.Args)
		})
	}
}

func TestUpdateBackfilledEntryKeepsStaticValueUnderExistingKey(t *testing.T) {
	gatewayClient := newEnforcementTestGatewayClient(t)
	entry := &v1.MCPServerCatalogEntry{
		Name:        "entry1backfilled",
		Namespace:   system.DefaultNamespace,
		Annotations: map[string]string{v1.BackfillVMCPAnnotation: "vmcp1example"},
		Spec: v1.MCPServerCatalogEntrySpec{MCPCatalogName: system.DefaultCatalog, Editable: true, Manifest: types.MCPServerCatalogEntryManifest{
			Name:      "Example",
			Runtime:   types.RuntimeNPX,
			NPXConfig: &types.NPXRuntimeConfig{Package: "test-server"},
			Config:    []types.MCPConfig{{Key: "kube", Usage: types.File}},
		}},
	}
	storage := newFakeStorage(t, &v1.MCPCatalog{Name: system.DefaultCatalog, Namespace: system.DefaultNamespace}, entry)
	handler := &MCPCatalogHandler{mcpBackend: "docker", sessionManager: &mcp.SessionManager{}}
	update := func(manifest types.MCPServerCatalogEntryManifest) v1.MCPServerCatalogEntry {
		t.Helper()
		req, _ := catalogEntryRequest(t, storage, gatewayClient, http.MethodPut, "/api/mcp-catalogs/"+system.DefaultCatalog+"/entries/"+entry.Name, manifest, map[string]string{"catalog_id": system.DefaultCatalog, "entry_id": entry.Name})
		require.NoError(t, handler.UpdateEntry(req))
		var updated v1.MCPServerCatalogEntry
		require.NoError(t, storage.Get(t.Context(), kclient.ObjectKeyFromObject(entry), &updated))
		return updated
	}

	withValue := *entry.Spec.Manifest.DeepCopy()
	withValue.Config[0].Value = "synthetic kubeconfig"
	updated := update(withValue)
	require.Equal(t, "kube", updated.Spec.Manifest.Config[0].Key)
	require.True(t, updated.Spec.Manifest.Config[0].Static)

	// A static field submitted without a value keeps the value stored under its existing key.
	updated = update(updated.Spec.Manifest)
	values, err := mcp.RevealStaticConfiguration(t.Context(), gatewayClient, entry.Name, updated.Spec.Manifest.StaticConfigurationRevision)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"kube": "synthetic kubeconfig"}, values)
}
