package registry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/obot-platform/obot/apiclient/types"
	"github.com/obot-platform/obot/pkg/api"
	"github.com/obot-platform/obot/pkg/api/authz"
	v1 "github.com/obot-platform/obot/pkg/storage/apis/obot.obot.ai/v1"
	"github.com/obot-platform/obot/pkg/system"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

func TestRegistryHandlersDoNotFetchIcons(t *testing.T) {
	for _, source := range []string{"entry", "server"} {
		for _, operation := range []string{"list", "no match", "anonymous", "versions", "version"} {
			t.Run(source+"/"+operation, func(t *testing.T) {
				var calls atomic.Int32
				icons := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls.Add(1)
					w.Header().Set("Content-Type", "image/png")
				}))
				defer icons.Close()
				entry := iconTestEntry("mcpce1icon", "Yunxiao", icons.URL+"/icon")
				server := registryOrdinaryServer("ms1icon", v1.MCPServerSpec{
					MCPCatalogID: system.DefaultCatalog,
					Manifest: types.MCPServerManifest{
						Name:         "Yunxiao",
						Icon:         icons.URL + "/icon",
						Runtime:      types.RuntimeRemote,
						RemoteConfig: &types.RemoteRuntimeConfig{URL: "https://example.com/mcp"},
					},
				})
				storage := newRegistryTestStorage(entry)
				name := entry.Name
				if source == "server" {
					storage = newRegistryTestStorage(server)
					name = server.Name
				}
				h, ctx := registryTestHandlerAndContext(t, storage)
				ctx.Request = httptest.NewRequest(http.MethodGet, "/v0.1/servers?limit=1&search=yunxiao", nil)
				if operation == "no match" {
					ctx.Request = httptest.NewRequest(http.MethodGet, "/v0.1/servers?limit=1&search=missing", nil)
				}
				if operation == "anonymous" {
					ctx.User = &user.DefaultInfo{UID: "anonymous", Groups: []string{authz.UnauthenticatedGroup}}
				}
				ctx.Request.SetPathValue("serverName", "com.example.obot/"+name)
				ctx.Request.SetPathValue("version", "latest")
				response := httptest.NewRecorder()
				ctx.ResponseWriter = response
				var err error
				switch operation {
				case "versions":
					err = h.ListServerVersions(ctx)
				case "version":
					err = h.GetServerVersion(ctx)
				default:
					err = h.ListServers(ctx)
				}
				if err != nil {
					t.Fatal(err)
				}
				if calls.Load() != 0 {
					t.Errorf("icon HTTP requests = %d, want 0", calls.Load())
				}
				if operation == "no match" {
					if got := decodeIconTestList(t, response).Servers; len(got) != 0 {
						t.Fatalf("no-match search returned %d servers", len(got))
					}
					return
				}
				var result types.RegistryServerResponse
				if operation == "version" {
					if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
				} else {
					list := decodeIconTestList(t, response)
					if len(list.Servers) != 1 {
						t.Fatalf("server count = %d, want 1: %s", len(list.Servers), response.Body.String())
					}
					result = list.Servers[0]
				}
				if result.Server.Name != "com.example.obot/"+name || result.Server.Title != "Yunxiao" {
					t.Fatalf("unexpected server identity: %#v", result.Server)
				}
				if len(result.Server.Icons) != 1 || result.Server.Icons[0].Src != icons.URL+"/icon" {
					t.Fatalf("icon URL not preserved: %#v", result.Server.Icons)
				}
				if strings.Contains(response.Body.String(), `"mimeType"`) {
					t.Fatalf("unknown MIME should be omitted: %s", response.Body.String())
				}
				if len(result.Server.Remotes) != 1 || result.Server.Remotes[0].URL != "https://obot.example.com/mcp-connect/"+name {
					t.Fatalf("connection URL changed: %#v", result.Server.Remotes)
				}
			})
		}
	}
}

func TestRegistryIconSearchPreservesPaginationAndAccess(t *testing.T) {
	first := iconTestEntry("mcpce1first", "Yunxiao First", "/icon.png")
	second := iconTestEntry("mcpce1second", "Yunxiao Second", "/icon.svg")
	first.CreationTimestamp = metav1.NewTime(time.Unix(1, 0))
	second.CreationTimestamp = metav1.NewTime(time.Unix(2, 0))
	hidden := iconTestEntry("mcpce1hidden", "Yunxiao Hidden", "/private.png")
	hidden.Spec.MCPCatalogName = "private-catalog"
	h, ctx := registryTestHandlerAndContext(t, newRegistryTestStorage(first, second, hidden))
	for _, tt := range []struct {
		query string
		name  string
		mime  string
		next  string
	}{
		{
			query: "limit=1&search=yunxiao",
			name:  "mcpce1first",
			mime:  "image/png",
			next:  "com.example.obot/mcpce1first",
		},
		{
			query: "limit=1&search=yunxiao&cursor=com.example.obot/mcpce1first",
			name:  "mcpce1second",
			mime:  "image/svg+xml",
		},
	} {
		ctx.Request = httptest.NewRequest(http.MethodGet, "/v0.1/servers?"+tt.query, nil)
		response := httptest.NewRecorder()
		ctx.ResponseWriter = response
		if err := h.ListServers(ctx); err != nil {
			t.Fatal(err)
		}
		list := decodeIconTestList(t, response)
		if len(list.Servers) != 1 || list.Servers[0].Server.Name != "com.example.obot/"+tt.name {
			t.Fatalf("wrong search page for %s: %#v", tt.query, list)
		}
		if len(list.Servers[0].Server.Icons) != 1 || list.Servers[0].Server.Icons[0].MimeType != tt.mime {
			t.Fatalf("known icon type changed: %#v", list.Servers[0].Server.Icons)
		}
		if list.Metadata == nil || list.Metadata.NextCursor != tt.next {
			t.Fatalf("wrong next cursor: %#v, want %q", list.Metadata, tt.next)
		}
	}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0.1/servers", nil)
	ctx.Request.SetPathValue("serverName", "com.example.obot/"+hidden.Name)
	ctx.Request.SetPathValue("version", "latest")
	ctx.ResponseWriter = httptest.NewRecorder()
	for _, lookup := range []func(api.Context) error{h.ListServerVersions, h.GetServerVersion} {
		if err := lookup(ctx); err == nil {
			t.Fatal("private catalog entry was accessible")
		}
	}
}

func iconTestEntry(name, title, icon string) *v1.MCPServerCatalogEntry {
	return &v1.MCPServerCatalogEntry{
		ObjectMeta: registryObjectMeta(name),
		Spec: v1.MCPServerCatalogEntrySpec{
			MCPCatalogName: system.DefaultCatalog,
			Manifest: types.MCPServerCatalogEntryManifest{
				Name:         title,
				Icon:         icon,
				Runtime:      types.RuntimeRemote,
				RemoteConfig: &types.RemoteCatalogConfig{FixedURL: "https://example.com/mcp"},
			},
		},
	}
}

func decodeIconTestList(t *testing.T, response *httptest.ResponseRecorder) types.RegistryServerList {
	t.Helper()
	var result types.RegistryServerList
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
