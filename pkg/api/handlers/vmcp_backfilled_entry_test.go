package handlers

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/obot-platform/obot/apiclient/types"
	"github.com/obot-platform/obot/pkg/api"
	"github.com/obot-platform/obot/pkg/controller/handlers/catalogmigration"
	v1 "github.com/obot-platform/obot/pkg/storage/apis/obot.obot.ai/v1"
	"github.com/obot-platform/obot/pkg/system"
	"github.com/obot-platform/obot/pkg/utils"
	vmcpconfig "github.com/obot-platform/obot/pkg/vmcp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/authentication/user"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestVMCPRefreshFromBackfilledEntry(t *testing.T) {
	slug := system.MCPServerPrefix + "legacy"
	migrated := sha256.Sum256([]byte(catalogmigration.MigrationName + "/" + system.DefaultNamespace + "/" + slug))
	manifest := types.MCPServerCatalogEntryManifest{
		Name: "Example", Runtime: types.RuntimeContainerized,
		ContainerizedConfig: &types.ContainerizedRuntimeConfig{
			Image: "example.invalid/mcp:test", Port: 8080, Path: "/mcp",
			Args: []string{"--kubeconfig-paths", "/files/legacy-kube"},
		},
		Config: []types.MCPConfig{{Key: "kube", Usage: types.File}},
	}
	// The shape the standalone migration gives a shared server without an entry.
	original := &v1.VMCP{
		Name:      fmt.Sprintf("%s%x", system.VMCPPrefix, migrated[:16]),
		Namespace: system.DefaultNamespace,
		Spec: v1.VMCPSpec{
			LegacySlug: slug, CreatorUserID: "9",
			Manifest: types.VMCPManifest{
				DisplayName: "Example",
				Profiles: []types.VMCPProfile{{
					Name: "everyone", Subjects: []types.Subject{{Type: types.SubjectTypeGroup, ID: "everyone"}},
					Permissions: types.VMCPProfilePermissions{AllowAllComponents: true},
				}},
				Components: []types.VMCPComponent{{
					ID: slug, Name: "Example", MCPCatalogID: system.DefaultCatalog,
					MCPServerCatalogEntryID: slug,
					CatalogEntry:            types.MCPServerCatalogEntrySnapshot{Manifest: manifest},
					Configuration:           []types.VMCPConfigurationPolicy{{Key: "kube", Policy: types.VMCPConfigurationPolicyFixed}},
				}},
			},
		},
	}
	original.Spec.Manifest.Components[0].SourceDigest = utils.Digest("pre-migration manifest")
	storage := newVMCPTestStorage(original, &v1.MCPCatalog{Name: system.DefaultCatalog, Namespace: system.DefaultNamespace})
	handler := vmcpHandlerForTest(t, storage)
	request := httptest.NewRequest(http.MethodPost, "/api/vmcps/"+original.Name+"/trigger-update", nil)
	request.SetPathValue("vmcp_id", original.Name)
	ctx := api.Context{
		Request: request, ResponseWriter: httptest.NewRecorder(), Storage: storage,
		User: &user.DefaultInfo{UID: "admin", Groups: []string{types.GroupAdmin}},
	}
	if err := handler.TriggerUpdate(ctx); !apierrors.IsNotFound(err) {
		t.Fatalf("refresh with missing source: got %v, want NotFound", err)
	}

	if err := catalogmigration.BackfillComponentEntries(t.Context(), storage); err != nil {
		t.Fatal(err)
	}
	var backfilled v1.VMCP
	if err := storage.Get(t.Context(), kclient.ObjectKeyFromObject(original), &backfilled); err != nil {
		t.Fatal(err)
	}
	var entry v1.MCPServerCatalogEntry
	if err := storage.Get(t.Context(), kclient.ObjectKey{Namespace: original.Namespace, Name: backfilled.Spec.Manifest.Components[0].MCPServerCatalogEntryID}, &entry); err != nil {
		t.Fatalf("backfilled entry: %v", err)
	}
	if system.IsMCPServerID(entry.Name) || !entry.Spec.Editable {
		t.Fatalf("backfilled entry %q must be an editable catalog entry", entry.Name)
	}

	entry.Spec.Manifest.ContainerizedConfig.Args[1] = "${kube}"
	if err := storage.Update(t.Context(), &entry); err != nil {
		t.Fatal(err)
	}
	if err := handler.TriggerUpdate(ctx); err != nil {
		t.Fatal(err)
	}
	var updated v1.VMCP
	if err := storage.Get(t.Context(), kclient.ObjectKeyFromObject(original), &updated); err != nil {
		t.Fatal(err)
	}
	component := updated.Spec.Manifest.Components[0]
	if component.CatalogEntry.Manifest.ContainerizedConfig.Args[1] != "${kube}" {
		t.Fatal("edited entry argument was not adopted")
	}
	if updated.Spec.LegacySlug != original.Spec.LegacySlug || component.ID != slug || component.MCPServerCatalogEntryID != entry.Name || component.ForceSingleUser {
		t.Fatal("refresh changed the connection or component identity")
	}
	if !reflect.DeepEqual(component.Configuration, original.Spec.Manifest.Components[0].Configuration) {
		t.Fatal("refresh changed the configuration policy")
	}
	if !reflect.DeepEqual(updated.Spec.Manifest.Profiles, original.Spec.Manifest.Profiles) {
		t.Fatal("refresh changed access profiles")
	}
	if component.SourceDigest != vmcpconfig.SourceDigest(component.CatalogEntry) || component.SourceDigest == backfilled.Spec.Manifest.Components[0].SourceDigest {
		t.Fatal("refresh did not compute the updated snapshot digest")
	}
}
