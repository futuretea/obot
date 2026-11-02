package catalogmigration

import (
	"context"
	"errors"
	"testing"

	"github.com/obot-platform/nah/pkg/name"
	"github.com/obot-platform/obot/apiclient/types"
	v1 "github.com/obot-platform/obot/pkg/storage/apis/obot.obot.ai/v1"
	"github.com/obot-platform/obot/pkg/system"
	"github.com/obot-platform/obot/pkg/vmcp"
	"github.com/stretchr/testify/require"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func legacySharedServer(id string) *v1.MCPServer {
	return &v1.MCPServer{
		Name:      system.MCPServerPrefix + id,
		Namespace: "default",
		Spec: v1.MCPServerSpec{
			MCPCatalogID:     "default",
			UserID:           "admin",
			Alias:            "Legacy alias",
			UnsupportedTools: []string{"unsupported"},
			Manifest: types.MCPServerManifest{
				Name:    "Rancher MCP",
				Runtime: types.RuntimeContainerized,
				ContainerizedConfig: &types.ContainerizedRuntimeConfig{
					Image: "example.invalid/mcp:1",
					Port:  8080,
					Path:  "/mcp",
					Args:  []string{"--kubeconfig-paths", "/files/legacy-kube"},
				},
				Config: []types.MCPConfig{
					{Key: "kube", Usage: types.File, Required: true},
					{Key: "LOG_LEVEL", Usage: types.Env, Value: "debug"},
				},
			},
		},
	}
}

// migrateLegacyServers runs the startup migration for shared servers without
// catalog entries and returns the vMCP created for each server.
func migrateLegacyServers(t *testing.T, client kclient.Client, servers ...*v1.MCPServer) []v1.VMCP {
	t.Helper()
	source := make(map[string]map[string]string)
	for _, server := range servers {
		source[server.CredentialContext("")+"/"+server.Name] = map[string]string{"kube": "kubeconfig"}
	}
	require.NoError(t, testHandler(t, source, make(map[string]map[string]string)).MigrateAll(t.Context(), client))
	require.NoError(t, Cleanup(t.Context(), client))

	targets := make([]v1.VMCP, 0, len(servers))
	for _, server := range servers {
		var target v1.VMCP
		require.NoError(t, client.Get(t.Context(), kclient.ObjectKey{Namespace: server.Namespace, Name: migrationName(system.VMCPPrefix, server.Namespace, server.Name)}, &target))
		require.Equal(t, server.Name, target.Spec.Manifest.Components[0].MCPServerCatalogEntryID)
		targets = append(targets, target)
	}
	return targets
}

func backfilledEntryName(scope string, target v1.VMCP) string {
	component := target.Spec.Manifest.Components[0]
	return name.SafeHashConcatName(scope, "rancher-mcp", component.ID)
}

func requireUnchanged(t *testing.T, client kclient.Client, before v1.VMCP) {
	t.Helper()
	var after v1.VMCP
	require.NoError(t, client.Get(t.Context(), kclient.ObjectKeyFromObject(&before), &after))
	require.Equal(t, before.ResourceVersion, after.ResourceVersion)
	require.Equal(t, before.Spec, after.Spec)
}

func TestBackfillComponentEntry(t *testing.T) {
	server := legacySharedServer("legacy")
	client := migrationClientBuilder().WithObjects(testCatalog(), server, testRule("everyone", types.Resource{Type: types.ResourceTypeSelector, ID: "*"})).Build()
	before := migrateLegacyServers(t, client, server)[0]
	require.NotEmpty(t, before.Spec.Manifest.Profiles)

	require.NoError(t, BackfillComponentEntries(t.Context(), client))

	entryName := backfilledEntryName("default", before)
	var entry v1.MCPServerCatalogEntry
	require.NoError(t, client.Get(t.Context(), kclient.ObjectKey{Namespace: "default", Name: entryName}, &entry))
	require.False(t, system.IsMCPServerID(entry.Name))
	require.True(t, entry.Spec.Editable)
	require.Equal(t, "default", entry.Spec.MCPCatalogName)
	require.Empty(t, entry.Spec.PowerUserWorkspaceID)
	require.Empty(t, entry.Spec.SourceURL)
	require.Equal(t, map[string]string{v1.BackfillVMCPAnnotation: before.Name, v1.BackfillComponentAnnotation: server.Name}, entry.Annotations)
	snapshot := before.Spec.Manifest.Components[0].CatalogEntry
	require.Equal(t, snapshot.Manifest, entry.Spec.Manifest)
	require.Equal(t, snapshot.UnsupportedTools, entry.Spec.UnsupportedTools)
	for _, field := range entry.Spec.Manifest.Config {
		require.Empty(t, field.Value)
	}

	var after v1.VMCP
	require.NoError(t, client.Get(t.Context(), kclient.ObjectKeyFromObject(&before), &after))
	expected := before.Spec.DeepCopy()
	expected.Manifest.Components[0].MCPServerCatalogEntryID = entryName
	expected.Manifest.Components[0].SourceDigest = vmcp.SourceDigest(snapshot)
	require.Equal(t, *expected, after.Spec)
	require.False(t, vmcp.NeedsUpdate(after.Spec.Manifest.Components[0], types.MCPServerCatalogEntrySnapshot{Manifest: entry.Spec.Manifest, UnsupportedTools: entry.Spec.UnsupportedTools}))

	require.NoError(t, BackfillComponentEntries(t.Context(), client))
	requireUnchanged(t, client, after)
	var entries v1.MCPServerCatalogEntryList
	require.NoError(t, client.List(t.Context(), &entries))
	require.Len(t, entries.Items, 1)
	require.Equal(t, entry.ResourceVersion, entries.Items[0].ResourceVersion)
}

func TestBackfillComponentEntryInWorkspace(t *testing.T) {
	workspace := &v1.PowerUserWorkspace{Name: system.PowerUserWorkspacePrefix + "workspace", Namespace: "default", Spec: v1.PowerUserWorkspaceSpec{UserID: "owner"}}
	server := legacySharedServer("legacy")
	server.Spec.MCPCatalogID = ""
	server.Spec.PowerUserWorkspaceID = workspace.Name
	client := migrationClientBuilder().WithObjects(workspace, server).Build()
	target := migrateLegacyServers(t, client, server)[0]

	require.NoError(t, BackfillComponentEntries(t.Context(), client))

	var entry v1.MCPServerCatalogEntry
	require.NoError(t, client.Get(t.Context(), kclient.ObjectKey{Namespace: "default", Name: backfilledEntryName(workspace.Name, target)}, &entry))
	require.Equal(t, workspace.Name, entry.Spec.PowerUserWorkspaceID)
	require.Empty(t, entry.Spec.MCPCatalogName)
	require.NoError(t, client.Get(t.Context(), kclient.ObjectKeyFromObject(&target), &target))
	require.Equal(t, entry.Name, target.Spec.Manifest.Components[0].MCPServerCatalogEntryID)
	require.Equal(t, workspace.Name, target.Spec.Manifest.Components[0].MCPCatalogID)
}

func TestBackfillComponentEntryRetainsConnectionComponents(t *testing.T) {
	server := legacySharedServer("legacy")
	instance := &v1.MCPServerInstance{
		Name:      "connection",
		Namespace: "default",
		Spec: v1.MCPServerInstanceSpec{
			UserID:        "1",
			MCPServerName: server.Name,
			Config:        []types.MCPConfig{{Key: "LOG_LEVEL", Value: "info", Usage: types.Env}},
		},
	}
	failVMCPUpdate := false
	client := migrationClientBuilder().WithObjects(testCatalog(), server, instance).WithInterceptorFuncs(interceptor.Funcs{
		Update: func(ctx context.Context, client kclient.WithWatch, obj kclient.Object, opts ...kclient.UpdateOption) error {
			if _, ok := obj.(*v1.VMCP); ok && failVMCPUpdate {
				return errors.New("update failed")
			}
			return client.Update(ctx, obj, opts...)
		},
	}).Build()
	target := migrateLegacyServers(t, client, server)[0]
	var connection v1.VMCPInstance
	require.NoError(t, client.Get(t.Context(), kclient.ObjectKey{Namespace: "default", Name: migrationName(system.VMCPInstancePrefix, "default", instance.Name)}, &connection))
	before := vmcp.ComponentsForInstance(target, connection)[0]
	require.Equal(t, connection.Spec.LegacyComponents[0].Configuration, before.Configuration)
	require.NotEqual(t, target.Spec.Manifest.Components[0].Configuration, before.Configuration)

	// A failed vMCP update leaves the connection rebound to the pending component.
	failVMCPUpdate = true
	require.NoError(t, BackfillComponentEntries(t.Context(), client))
	requireUnchanged(t, client, target)

	failVMCPUpdate = false
	require.NoError(t, BackfillComponentEntries(t.Context(), client))
	require.NoError(t, BackfillComponentEntries(t.Context(), client))

	var updated v1.VMCP
	require.NoError(t, client.Get(t.Context(), kclient.ObjectKeyFromObject(&target), &updated))
	require.NoError(t, client.Get(t.Context(), kclient.ObjectKeyFromObject(&connection), &connection))
	entryName := backfilledEntryName("default", target)
	require.Equal(t, entryName, updated.Spec.Manifest.Components[0].MCPServerCatalogEntryID)
	after := vmcp.ComponentsForInstance(updated, connection)[0]
	expected := before.DeepCopy()
	expected.MCPServerCatalogEntryID = entryName
	expected.SourceDigest = connection.Spec.LegacyComponents[0].SourceDigest
	require.Equal(t, *expected, after)
}

func TestBackfillComponentEntryResumesOrSkipsExistingEntry(t *testing.T) {
	for _, tt := range []struct {
		name      string
		component string
		resumes   bool
	}{
		{name: "same source", component: system.MCPServerPrefix + "legacy", resumes: true},
		{name: "other source", component: system.MCPServerPrefix + "other"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := legacySharedServer("legacy")
			client := migrationClientBuilder().WithObjects(testCatalog(), server).Build()
			target := migrateLegacyServers(t, client, server)[0]
			existing := &v1.MCPServerCatalogEntry{
				Name:        backfilledEntryName("default", target),
				Namespace:   "default",
				Annotations: map[string]string{v1.BackfillVMCPAnnotation: target.Name, v1.BackfillComponentAnnotation: tt.component},
				Spec:        v1.MCPServerCatalogEntrySpec{MCPCatalogName: "default", Editable: true, Manifest: types.MCPServerCatalogEntryManifest{Name: "Existing"}},
			}
			require.NoError(t, client.Create(t.Context(), existing))

			require.NoError(t, BackfillComponentEntries(t.Context(), client))

			var entry v1.MCPServerCatalogEntry
			require.NoError(t, client.Get(t.Context(), kclient.ObjectKeyFromObject(existing), &entry))
			require.Equal(t, existing.ResourceVersion, entry.ResourceVersion)
			if !tt.resumes {
				requireUnchanged(t, client, target)
				return
			}
			var after v1.VMCP
			require.NoError(t, client.Get(t.Context(), kclient.ObjectKeyFromObject(&target), &after))
			require.Equal(t, existing.Name, after.Spec.Manifest.Components[0].MCPServerCatalogEntryID)
		})
	}
}

func TestBackfillComponentEntrySkipsIneligibleComponents(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*testing.T, kclient.Client, *v1.VMCP)
	}{
		{name: "catalog synced", mutate: func(_ *testing.T, _ kclient.Client, target *v1.VMCP) {
			target.Spec.SourceURL = "https://example.com/catalog"
		}},
		{name: "missing runtime", mutate: func(_ *testing.T, _ kclient.Client, target *v1.VMCP) {
			target.Spec.Manifest.Components[0].CatalogEntry.Manifest.Runtime = ""
		}},
		{name: "static configuration", mutate: func(_ *testing.T, _ kclient.Client, target *v1.VMCP) {
			target.Spec.Manifest.Components[0].CatalogEntry.Manifest.Config[0].Static = true
		}},
		{name: "secret binding", mutate: func(_ *testing.T, _ kclient.Client, target *v1.VMCP) {
			target.Spec.Manifest.Components[0].CatalogEntry.Manifest.Config[0].SecretBinding = &types.MCPSecretBinding{Name: "secret", Key: "kube"}
		}},
		{name: "static OAuth", mutate: func(_ *testing.T, _ kclient.Client, target *v1.VMCP) {
			target.Spec.Manifest.Components[0].OAuthCredentialID = "oauth"
		}},
		{name: "missing catalog", mutate: func(_ *testing.T, _ kclient.Client, target *v1.VMCP) {
			target.Spec.Manifest.Components[0].MCPCatalogID = "missing"
		}},
		{name: "personal", mutate: func(_ *testing.T, _ kclient.Client, target *v1.VMCP) {
			target.Spec.UserID = "user"
		}},
		{name: "single user", mutate: func(_ *testing.T, _ kclient.Client, target *v1.VMCP) {
			target.Spec.Manifest.Components[0].ForceSingleUser = true
		}},
		{name: "existing source entry", mutate: func(t *testing.T, client kclient.Client, target *v1.VMCP) {
			require.NoError(t, client.Create(t.Context(), &v1.MCPServerCatalogEntry{Name: target.Spec.Manifest.Components[0].MCPServerCatalogEntryID, Namespace: target.Namespace}))
		}},
		{name: "deleting", mutate: func(t *testing.T, client kclient.Client, target *v1.VMCP) {
			require.NoError(t, client.Update(t.Context(), target))
			require.NoError(t, client.Delete(t.Context(), target))
			require.NoError(t, client.Get(t.Context(), kclient.ObjectKeyFromObject(target), target))
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := legacySharedServer("legacy")
			client := migrationClientBuilder().WithObjects(testCatalog(), server).Build()
			target := migrateLegacyServers(t, client, server)[0]
			tt.mutate(t, client, &target)
			if target.DeletionTimestamp.IsZero() {
				require.NoError(t, client.Update(t.Context(), &target))
			}

			require.NoError(t, BackfillComponentEntries(t.Context(), client))

			requireUnchanged(t, client, target)
			var entries v1.MCPServerCatalogEntryList
			require.NoError(t, client.List(t.Context(), &entries))
			for _, entry := range entries.Items {
				require.Empty(t, entry.Annotations[v1.BackfillVMCPAnnotation])
			}
		})
	}
}

func TestBackfillComponentEntryContinuesAfterFailure(t *testing.T) {
	failing, working := legacySharedServer("failing"), legacySharedServer("working")
	fail := false
	client := migrationClientBuilder().WithObjects(testCatalog(), failing, working).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, client kclient.WithWatch, obj kclient.Object, opts ...kclient.CreateOption) error {
			if fail && obj.GetAnnotations()[v1.BackfillComponentAnnotation] == failing.Name {
				return errors.New("create failed")
			}
			return client.Create(ctx, obj, opts...)
		},
	}).Build()
	targets := migrateLegacyServers(t, client, failing, working)

	fail = true
	require.NoError(t, BackfillComponentEntries(t.Context(), client))
	requireUnchanged(t, client, targets[0])
	var updated v1.VMCP
	require.NoError(t, client.Get(t.Context(), kclient.ObjectKeyFromObject(&targets[1]), &updated))
	require.Equal(t, backfilledEntryName("default", targets[1]), updated.Spec.Manifest.Components[0].MCPServerCatalogEntryID)

	fail = false
	require.NoError(t, BackfillComponentEntries(t.Context(), client))
	require.NoError(t, client.Get(t.Context(), kclient.ObjectKeyFromObject(&targets[0]), &updated))
	require.Equal(t, backfilledEntryName("default", targets[0]), updated.Spec.Manifest.Components[0].MCPServerCatalogEntryID)
}
