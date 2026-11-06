package registry

import (
	"context"
	"strings"
)

type mimeFetcher struct{}

func newMimeFetcher() *mimeFetcher {
	return &mimeFetcher{}
}

func (*mimeFetcher) guessMimeType(_ context.Context, iconURL string) string {
	lower := strings.ToLower(iconURL)
	if strings.HasSuffix(lower, ".png") {
		return "image/png"
	}
	if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
		return "image/jpeg"
	}
	if strings.HasSuffix(lower, ".svg") {
		return "image/svg+xml"
	}
	if strings.HasSuffix(lower, ".webp") {
		return "image/webp"
	}

	return ""
}
