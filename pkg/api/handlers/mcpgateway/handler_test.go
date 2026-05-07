package mcpgateway

import (
	"net/http"
	"testing"

	"github.com/obot-platform/obot/apiclient/types"
)

func TestIsMissingNanobotCredentialConfig(t *testing.T) {
	err := types.NewErrHTTP(http.StatusBadRequest, "missing required config: NANOBOT_ENV_FILE")

	if !isMissingNanobotCredentialConfig(err) {
		t.Fatal("expected NANOBOT_ENV_FILE bad request to be treated as missing nanobot credential config")
	}
}

func TestIsMissingNanobotCredentialConfigRejectsOtherBadRequests(t *testing.T) {
	err := types.NewErrHTTP(http.StatusBadRequest, "missing required config: API_TOKEN")

	if isMissingNanobotCredentialConfig(err) {
		t.Fatal("expected unrelated missing config to be ignored")
	}
}

func TestIsMissingNanobotCredentialConfigRejectsOtherStatuses(t *testing.T) {
	err := types.NewErrHTTP(http.StatusUnauthorized, "missing required config: NANOBOT_ENV_FILE")

	if isMissingNanobotCredentialConfig(err) {
		t.Fatal("expected non-bad-request status to be ignored")
	}
}
