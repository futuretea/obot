package mcp

import (
	"slices"
	"testing"

	"github.com/obot-platform/obot/apiclient/types"
	v1 "github.com/obot-platform/obot/pkg/storage/apis/obot.obot.ai/v1"
	"github.com/obot-platform/obot/pkg/vmcp"
)

func TestK8sObjects_VMCPComponentFileTemplateUsesMountedPath(t *testing.T) {
	component := types.VMCPComponent{
		ID: "ms1legacy",
		CatalogEntry: types.MCPServerCatalogEntrySnapshot{Manifest: types.MCPServerCatalogEntryManifest{
			Name: "Example", Runtime: types.RuntimeContainerized,
			ContainerizedConfig: &types.ContainerizedRuntimeConfig{
				Image: "example.invalid/mcp:test", Port: 8080, Path: "/mcp",
				Args: []string{"--kubeconfig-paths", "${kube}"},
			},
			Config: []types.MCPConfig{{Key: "kube", Usage: types.File, Required: true}},
		}},
		Configuration: []types.VMCPConfigurationPolicy{{Key: "kube", Policy: types.VMCPConfigurationPolicyFixed}},
	}
	// The vMCP controller builds component servers and their fixed values this way.
	manifest, err := types.MapCatalogEntryToServer(component.CatalogEntry.Manifest, "", true)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Config = vmcp.ComponentConfig(component)
	server := v1.MCPServer{
		Name: "ms1vmcp1example-ms1legacy",
		Spec: v1.MCPServerSpec{Manifest: manifest, VMCPID: "vmcp1example", VMCPComponentID: component.ID},
	}

	config, missing, err := ServerToServerConfig(server, nil, "user-1", "scope", "default", map[string]string{"kube": "synthetic kubeconfig"})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing required configuration %v", missing)
	}

	objs, err := newTestKubernetesBackend(t).k8sObjects(t.Context(), config)
	if err != nil {
		t.Fatalf("k8sObjects() error = %v", err)
	}
	path := "/files/" + server.Name + "-kube"
	container := findContainer(t, findDeployment(t, objs, server.Name), "mcp")
	if !slices.Equal(container.Args, []string{"--kubeconfig-paths", path}) {
		t.Fatalf("container args = %v, want the mounted file path %s", container.Args, path)
	}
	files := findSecret(t, objs, server.Name+"-mcp-files")
	if string(files.Data[server.Name+"-kube"]) != "synthetic kubeconfig" {
		t.Fatal("mounted file does not hold the fixed configuration value")
	}
}
