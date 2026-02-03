package mcpcatalog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test GitHub.com URL detection
func TestIsGitHubURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantGitHub bool
	}{
		// Should detect as GitHub (github.com only)
		{"github.com with .git", "https://github.com/org/repo.git", true},
		{"github.com without .git", "https://github.com/org/repo", true},
		{"github.com with branch", "https://github.com/org/repo/tree/main", true},
		{"github.com http", "http://github.com/org/repo.git", true},
		{"github.com trailing slash", "https://github.com/org/repo.git/", true},

		// Should NOT detect as GitHub (other domains)
		{"github enterprise", "https://github.enterprise.com/org/repo.git", false},
		{"github subdomain", "https://git.github.company.com/org/repo.git", false},
		{"gitlab", "https://gitlab.com/org/repo.git", false},
		{"bitbucket", "https://bitbucket.org/org/repo.git", false},
		{"other git", "https://git.enterprise.com/org/repo.git", false},

		// Edge cases
		{"invalid URL", "not-a-url", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isGitHubURL(tt.url)
			assert.Equal(t, tt.wantGitHub, result)
		})
	}
}

func TestReadGitHubCatalog(t *testing.T) {
	tests := []struct {
		name       string
		catalog    string
		wantErr    bool
		numEntries int
	}{
		{
			name:       "valid github url with https and .git suffix",
			catalog:    "https://github.com/obot-platform/test-mcp-catalog.git",
			wantErr:    false,
			numEntries: 3,
		},
		{
			name:       "valid github url without protocol",
			catalog:    "github.com/obot-platform/test-mcp-catalog",
			wantErr:    false,
			numEntries: 3,
		},
		{
			name:       "invalid protocol",
			catalog:    "http://github.com/obot-platform/test-mcp-catalog",
			wantErr:    true,
			numEntries: 0,
		},
		{
			name:       "invalid url format",
			catalog:    "github.com/invalid",
			wantErr:    true,
			numEntries: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := readGitHubCatalog(tt.catalog)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.numEntries, len(entries), "should return the correct number of catalog entries")

			// Verify that each entry has required fields
			for _, entry := range entries {
				// "Test 0" is in a file that should not have been included when reading the catalog.
				assert.NotEqual(t, entry.Name, "Test 0", "should not be the left out entry")

				assert.NotEmpty(t, entry.Name, "Name should not be empty")
				assert.NotEmpty(t, entry.Description, "Description should not be empty")
			}
		})
	}
}
