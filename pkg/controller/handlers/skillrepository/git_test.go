package skillrepository

import (
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGitRepositoryFetcherUsesGitHubAuthToken(t *testing.T) {
	t.Setenv("GITHUB_AUTH_TOKEN", "test-token")

	f := newGitRepositoryFetcher()

	assert.Equal(t, "test-token", f.token)
}

func TestGitRepositoryFetcherCloneOptions(t *testing.T) {
	t.Run("sets basic auth when token configured", func(t *testing.T) {
		f := &gitRepositoryFetcher{token: "test-token"}

		options := f.cloneOptions("https://git.example.com/acme/skills.git")

		assert.Equal(t, "https://git.example.com/acme/skills.git", options.URL)
		auth, ok := options.Auth.(*githttp.BasicAuth)
		require.True(t, ok)
		assert.Equal(t, "x-access-token", auth.Username)
		assert.Equal(t, "test-token", auth.Password)
	})

	t.Run("leaves auth empty without token", func(t *testing.T) {
		f := &gitRepositoryFetcher{}

		options := f.cloneOptions("https://git.example.com/acme/skills.git")

		assert.Equal(t, "https://git.example.com/acme/skills.git", options.URL)
		assert.Nil(t, options.Auth)
	})
}
