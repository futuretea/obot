package mcpcatalog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test .git suffix detection for non-GitHub Git servers
func TestIsGitURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantGit bool
	}{
		// Should detect as Git (has .git suffix)
		{"gitlab with .git", "https://gitlab.com/org/repo.git", true},
		{"bitbucket with .git", "https://bitbucket.org/org/repo.git", true},
		{"enterprise git with .git", "https://git.enterprise.com/org/repo.git", true},
		{"gitea with .git", "https://gitea.io/org/repo.git", true},
		{"github enterprise with .git", "https://github.enterprise.com/org/repo.git", true},
		{"custom domain with .git", "https://code.company.com/team/project.git", true},
		{"with trailing slash", "https://gitlab.com/org/repo.git/", true},
		{"github.com with .git", "https://github.com/org/repo.git", true}, // Has .git suffix

		// Should NOT detect as Git (no .git suffix)
		{"without .git suffix", "https://git.enterprise.com/org/repo", false},
		{"github.com without .git", "https://github.com/org/repo", false},
		{"raw file URL yaml", "https://example.com/catalog.yaml", false},
		{"raw file URL json", "https://cdn.example.com/catalogs/mcp.json", false},
		{"http url with .git", "http://git.example.com/org/repo.git", true},

		// Edge cases
		{"invalid URL", "not-a-url", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isGitURL(tt.url)
			assert.Equal(t, tt.wantGit, result)
		})
	}
}
