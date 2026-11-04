package internal

import (
	"testing"

	"github.com/obot-platform/obot/pkg/cli/internal/credentials"
)

func TestEmployeeFacingConstantsUseUniNames(t *testing.T) {
	if TokenEnvVar != "UNI_TOKEN" {
		t.Fatalf("token env var = %q, want UNI_TOKEN", TokenEnvVar)
	}
	if credentials.DefaultService != "uni" {
		t.Fatalf("keyring service = %q, want uni", credentials.DefaultService)
	}
}
