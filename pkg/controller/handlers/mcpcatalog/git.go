package mcpcatalog

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/obot-platform/obot/apiclient/types"
)

var gitToken = os.Getenv("GITHUB_AUTH_TOKEN") // Reuse same token for all Git servers

// isGitURL checks if the URL has .git suffix (for non-GitHub Git servers)
func isGitURL(catalogURL string) bool {
	u, err := url.Parse(catalogURL)
	if err != nil {
		return false
	}
	// Check for .git suffix (standard Git repository indicator)
	return strings.HasSuffix(strings.TrimSuffix(u.Path, "/"), ".git")
}

// readGitCatalog clones a Git repository and reads catalog entries
// This works for GitLab, Bitbucket, Gitea, GitHub Enterprise, and any standard Git server
func readGitCatalog(catalogURL string) ([]types.MCPServerCatalogEntryManifest, error) {
	// Make sure we don't use plain HTTP
	if strings.HasPrefix(catalogURL, "http://") {
		return nil, fmt.Errorf("only HTTPS is supported for Git catalogs")
	}

	// Normalize the URL to ensure HTTPS
	if !strings.HasPrefix(catalogURL, "https://") {
		catalogURL = "https://" + catalogURL
	}

	// Parse URL
	u, err := url.Parse(catalogURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Git URL: %w", err)
	}

	// Parse org/repo from path
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid Git URL format, expected host/org/repo.git")
	}

	// Remove .git suffix from repo name if present
	repo := strings.TrimSuffix(parts[1], ".git")
	org := parts[0]
	branch := "main"
	if len(parts) > 2 {
		branch = strings.Join(parts[2:], "/")
		branch = strings.TrimSuffix(branch, ".git")
		// Validate branch name for security
		if err := validateBranchName(branch); err != nil {
			return nil, fmt.Errorf("invalid branch name: %w", err)
		}
	}

	// Create temporary directory for cloning
	tempDir, err := os.MkdirTemp("", "catalog-clone-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Construct clone URL
	cloneURL := fmt.Sprintf("%s://%s/%s/%s.git", u.Scheme, u.Host, org, repo)

	// Set up clone options
	cloneOptions := &git.CloneOptions{
		URL:           cloneURL,
		Depth:         1,
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch)),
	}

	// Set up git credentials if token is available
	if gitToken != "" {
		cloneOptions.Auth = &githttp.BasicAuth{
			Username: "git", // Username is ignored but required to be non-empty
			Password: gitToken,
		}
	}

	// Clone the repository
	_, err = git.PlainClone(tempDir, false, cloneOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	return readMCPCatalogDirectory(tempDir)
}
