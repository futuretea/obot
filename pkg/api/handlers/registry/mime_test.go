package registry

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestGuessMimeTypeFromExtension(t *testing.T) {
	for _, tt := range []struct {
		name string
		url  string
		want string
	}{
		{
			name: "png",
			url:  "https://example.com/icon.PNG",
			want: "image/png",
		},
		{
			name: "jpg",
			url:  "https://example.com/icon.jpg",
			want: "image/jpeg",
		},
		{
			name: "jpeg",
			url:  "https://example.com/icon.jpeg",
			want: "image/jpeg",
		},
		{
			name: "svg",
			url:  "/images/icon.svg",
			want: "image/svg+xml",
		},
		{
			name: "webp",
			url:  "https://example.com/icon.webp",
			want: "image/webp",
		},
		{
			name: "empty",
		},
		{
			name: "relative unknown",
			url:  "/images/icon",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := newMimeFetcher().guessMimeType(t.Context(), tt.url); got != tt.want {
				t.Fatalf("guessMimeType(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestGuessMimeTypeDoesNotFetchUnknownIcons(t *testing.T) {
	var calls atomic.Int32
	icons := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "image/png")
	}))
	defer icons.Close()

	fetcher := newMimeFetcher()
	for _, path := range []string{"/icon", "/icon.png?revision=1", "/image.unknown"} {
		if got := fetcher.guessMimeType(t.Context(), icons.URL+path); got != "" {
			t.Errorf("unknown icon %q MIME = %q, want empty", path, got)
		}
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("icon HTTP requests = %d, want 0", got)
	}
}
