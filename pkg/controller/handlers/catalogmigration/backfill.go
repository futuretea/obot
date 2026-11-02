package catalogmigration

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/obot-platform/nah/pkg/name"
	"github.com/obot-platform/obot/apiclient/types"
	"github.com/obot-platform/obot/pkg/mcpcatalog"
	v1 "github.com/obot-platform/obot/pkg/storage/apis/obot.obot.ai/v1"
	"github.com/obot-platform/obot/pkg/system"
	"github.com/obot-platform/obot/pkg/utils"
	"github.com/obot-platform/obot/pkg/vmcp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type backfillResult struct {
	entry  string
	reason string
}

// BackfillComponentEntries gives each migrated shared server component that
// references a missing catalog entry an editable entry built from its snapshot,
// so administrators can change its definition and trigger a component update.
// Legacy shared servers had no entry of their own, so the migration recorded
// the server name as the component's entry reference.
//
// It runs on every startup. A component that fails is logged and retried on
// the next startup, because it still references the missing entry.
func BackfillComponentEntries(ctx context.Context, client kclient.Client) error {
	var vmcps v1.VMCPList
	if err := client.List(ctx, &vmcps); err != nil {
		return err
	}

	var instances v1.VMCPInstanceList
	if err := client.List(ctx, &instances); err != nil {
		return err
	}

	var restored, skipped, failed int
	for i := range vmcps.Items {
		target := &vmcps.Items[i]
		if !migratedSharedServer(*target) {
			continue
		}

		componentID := target.Spec.Manifest.Components[0].ID
		result, err := backfillComponentEntry(ctx, client, target, instances.Items)
		switch {
		case err != nil:
			failed++
			slog.Error("Failed to backfill catalog entry for migrated vMCP component", "vmcp", target.Name, "component", componentID, "error", err)
		case result.reason != "":
			skipped++
			slog.Warn("Skipped backfilling catalog entry for migrated vMCP component", "vmcp", target.Name, "component", componentID, "reason", result.reason)
		case result.entry != "":
			restored++
			slog.Info("Backfilled catalog entry for migrated vMCP component", "vmcp", target.Name, "displayName", target.Spec.Manifest.DisplayName, "component", componentID, "entry", result.entry)
		}
	}

	if restored+skipped+failed > 0 {
		slog.Info("Finished backfilling catalog entries for migrated vMCP components", "backfilled", restored, "skipped", skipped, "failed", failed)
	}

	return nil
}

// migratedSharedServer reports whether a vMCP has the shape migrateSharedServer
// creates for a server without a catalog entry. Personal, single-user, and
// composite migrations have other shapes.
func migratedSharedServer(target v1.VMCP) bool {
	if len(target.Spec.Manifest.Components) != 1 {
		return false
	}

	component := target.Spec.Manifest.Components[0]
	slug := target.Spec.LegacySlug

	return target.Spec.UserID == "" &&
		system.IsMCPServerID(slug) &&
		target.Name == migrationName(system.VMCPPrefix, target.Namespace, slug) &&
		component.ID == slug &&
		component.MCPServerCatalogEntryID == slug &&
		!component.ForceSingleUser
}

func backfillComponentEntry(ctx context.Context, client kclient.Client, target *v1.VMCP, instances []v1.VMCPInstance) (backfillResult, error) {
	component := target.Spec.Manifest.Components[0]

	if err := client.Get(ctx, kclient.ObjectKey{Namespace: target.Namespace, Name: component.MCPServerCatalogEntryID}, &v1.MCPServerCatalogEntry{}); err == nil {
		return backfillResult{}, nil
	} else if !apierrors.IsNotFound(err) {
		return backfillResult{}, err
	}

	if reason := backfillSkipReason(*target, component); reason != "" {
		return backfillResult{reason: reason}, nil
	}

	entry := v1.MCPServerCatalogEntry{
		Namespace: target.Namespace,
		Annotations: map[string]string{
			v1.BackfillVMCPAnnotation:      target.Name,
			v1.BackfillComponentAnnotation: component.ID,
		},
		Spec: v1.MCPServerCatalogEntrySpec{
			Editable:         true,
			Manifest:         *component.CatalogEntry.Manifest.DeepCopy(),
			UnsupportedTools: slices.Clone(component.CatalogEntry.UnsupportedTools),
		},
	}

	scope := component.MCPCatalogID
	if err := client.Get(ctx, kclient.ObjectKey{Namespace: target.Namespace, Name: scope}, &v1.MCPCatalog{}); err == nil {
		entry.Spec.MCPCatalogName = scope
	} else if !apierrors.IsNotFound(err) {
		return backfillResult{}, err
	} else if !system.IsPowerUserWorkspaceID(scope) {
		return backfillResult{reason: fmt.Sprintf("catalog %q not found", scope)}, nil
	} else if err := client.Get(ctx, kclient.ObjectKey{Namespace: target.Namespace, Name: scope}, &v1.PowerUserWorkspace{}); apierrors.IsNotFound(err) {
		return backfillResult{reason: fmt.Sprintf("workspace %q not found", scope)}, nil
	} else if err != nil {
		return backfillResult{}, err
	} else {
		entry.Spec.PowerUserWorkspaceID = scope
	}

	entry.Name = name.SafeHashConcatName(scope, mcpcatalog.SanitizeName(entry.Spec.Manifest.Name), component.ID)

	var existing v1.MCPServerCatalogEntry
	if err := client.Get(ctx, kclient.ObjectKeyFromObject(&entry), &existing); err == nil {
		// A previous startup created the entry but did not update the vMCP.
		if existing.Annotations[v1.BackfillVMCPAnnotation] != target.Name || existing.Annotations[v1.BackfillComponentAnnotation] != component.ID || !existing.DeletionTimestamp.IsZero() {
			return backfillResult{reason: fmt.Sprintf("catalog entry %q already exists for another source", entry.Name)}, nil
		}
	} else if !apierrors.IsNotFound(err) {
		return backfillResult{}, err
	} else if err := client.Create(ctx, &entry); err != nil {
		return backfillResult{}, fmt.Errorf("create catalog entry %q: %w", entry.Name, err)
	}

	previous := component
	updated := &target.Spec.Manifest.Components[0]
	updated.MCPServerCatalogEntryID = entry.Name
	// The migration stored the digest of the server manifest before fixed values
	// were removed; the new entry matches the snapshot, so it must not report drift.
	updated.SourceDigest = vmcp.SourceDigest(updated.CatalogEntry)

	// Connections are rebound before the vMCP changes, so a failed vMCP update
	// leaves the vMCP selectable for the next startup to finish.
	if err := rebindLegacyComponents(ctx, client, *target, previous, *updated, instances); err != nil {
		return backfillResult{}, err
	}
	if err := client.Update(ctx, target); err != nil {
		return backfillResult{}, fmt.Errorf("update vMCP component reference to %q: %w", entry.Name, err)
	}

	return backfillResult{entry: entry.Name}, nil
}

// rebindLegacyComponents keeps migrated connections on their own component
// snapshots, which vmcp.ComponentsForInstance binds by a digest of the vMCP
// component, until an administrator updates the component.
func rebindLegacyComponents(ctx context.Context, client kclient.Client, target v1.VMCP, previous, component types.VMCPComponent, instances []v1.VMCPInstance) error {
	staticHash := target.Spec.ComponentStaticConfigurationHashes[component.ID]
	previousDigest := utils.Digest([]any{previous, staticHash})
	digest := utils.Digest([]any{component, staticHash})

	for i := range instances {
		instance := &instances[i]
		if instance.Namespace != target.Namespace || instance.Spec.Manifest.VMCPID != target.Name {
			continue
		}

		var changed bool
		for j := range instance.Spec.LegacyComponents {
			legacy := &instance.Spec.LegacyComponents[j]
			if legacy.ID == component.ID && legacy.SourceDigest == previousDigest {
				legacy.MCPServerCatalogEntryID = component.MCPServerCatalogEntryID
				legacy.SourceDigest = digest
				changed = true
			}
		}
		if !changed {
			continue
		}
		if err := client.Update(ctx, instance); err != nil {
			return fmt.Errorf("update vMCP instance %q: %w", instance.Name, err)
		}
	}

	return nil
}

// backfillSkipReason rejects snapshots whose values would live under the
// missing entry's own credentials, which a new entry cannot provide.
func backfillSkipReason(target v1.VMCP, component types.VMCPComponent) string {
	switch {
	case !target.DeletionTimestamp.IsZero():
		return "vMCP is being deleted"
	case target.Spec.SourceURL != "":
		return "vMCP is synced from a catalog source"
	case component.CatalogEntry.Manifest.Runtime == "":
		// Catalog entries without a runtime are deleted as soon as they are created.
		return "missing runtime"
	case component.CatalogEntry.Manifest.Runtime == types.RuntimeComposite:
		return "composite runtime"
	case component.OAuthCredentialID != "" || vmcp.StaticOAuthCredentialReference(component.CatalogEntry.Manifest, component.ID) != "":
		return "requires static OAuth credentials"
	}

	for _, field := range component.CatalogEntry.Manifest.Config {
		if field.Static {
			return fmt.Sprintf("configuration %q is static", field.Key)
		}
		if field.SecretBinding != nil {
			return fmt.Sprintf("configuration %q has a secret binding", field.Key)
		}
	}

	return ""
}
