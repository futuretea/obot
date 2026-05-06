package skillrepository

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/chroot"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitfs "github.com/go-git/go-git/v5/storage/filesystem"
)

var errGitRepoTooLarge = errors.New("repository too large")

type repositoryFetcherSelector struct {
	github *githubRepositoryFetcher
	git    *gitRepositoryFetcher
}

type gitRepositoryFetcher struct {
	token             string
	maxRepoSizeMB     int
	maxExtractedFiles int
}

type sizeLimitedFS struct {
	billy.Filesystem
	written  atomic.Int64
	maxBytes int64
}

type sizeLimitedFile struct {
	billy.File
	fs *sizeLimitedFS
}

func newRepositoryFetcher() repositoryFetcher {
	return &repositoryFetcherSelector{
		github: newGitHubRepositoryFetcher(),
		git:    newGitRepositoryFetcher(),
	}
}

func newGitRepositoryFetcher() *gitRepositoryFetcher {
	return &gitRepositoryFetcher{
		token:             os.Getenv("GITHUB_AUTH_TOKEN"),
		maxRepoSizeMB:     maxRepoSizeMB,
		maxExtractedFiles: maxExtractedFiles,
	}
}

func (f *repositoryFetcherSelector) Fetch(ctx context.Context, repoURL, ref string) (*fetchedRepository, error) {
	if isGitHubRepositoryURL(repoURL) {
		return f.github.Fetch(ctx, repoURL, ref)
	}
	return f.git.Fetch(ctx, repoURL, ref)
}

func (f *repositoryFetcherSelector) MaterializeCommit(ctx context.Context, repoURL, commitSHA string) (*fetchedRepository, error) {
	if isGitHubRepositoryURL(repoURL) {
		return f.github.MaterializeCommit(ctx, repoURL, commitSHA)
	}
	return f.git.MaterializeCommit(ctx, repoURL, commitSHA)
}

func (f *gitRepositoryFetcher) Fetch(ctx context.Context, repoURL, ref string) (*fetchedRepository, error) {
	cloneURL, err := parseGitRepositoryURL(repoURL)
	if err != nil {
		return nil, err
	}

	return f.fetchCloneURL(ctx, cloneURL, ref)
}

func (f *gitRepositoryFetcher) MaterializeCommit(ctx context.Context, repoURL, commitSHA string) (*fetchedRepository, error) {
	if commitSHA == "" {
		return nil, fmt.Errorf("commit SHA is required")
	}

	cloneURL, err := parseGitRepositoryURL(repoURL)
	if err != nil {
		return nil, err
	}

	return f.fetchCloneURL(ctx, cloneURL, commitSHA)
}

func (f *gitRepositoryFetcher) fetchCloneURL(ctx context.Context, cloneURL, ref string) (*fetchedRepository, error) {
	workspace, err := os.MkdirTemp("", "skill-repository-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp workspace: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(workspace)
	}

	repoRoot := filepath.Join(workspace, "repo")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to create repository checkout directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "git"), 0o755); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to create repository storage directory: %w", err)
	}

	limitedFS := &sizeLimitedFS{
		Filesystem: osfs.New(workspace),
		maxBytes:   int64(f.maxRepoSizeMB) * 1024 * 1024,
	}
	storer := gitfs.NewStorage(chroot.New(limitedFS, "git"), cache.NewObjectLRUDefault())
	worktreeFS := chroot.New(limitedFS, "repo")

	repository, err := git.CloneContext(ctx, storer, worktreeFS, f.cloneOptions(cloneURL))
	if err != nil {
		cleanup()
		if errors.Is(err, errGitRepoTooLarge) {
			return nil, fmt.Errorf("repository is too large (limit: %d MB)", f.maxRepoSizeMB)
		}
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	commitSHA, err := checkoutAndResolveGitRef(repository, ref)
	if err != nil {
		cleanup()
		return nil, err
	}

	if err := enforceGitWorktreeFileLimit(repoRoot, f.maxExtractedFiles); err != nil {
		cleanup()
		return nil, err
	}

	return &fetchedRepository{
		RepoRoot:  repoRoot,
		CommitSHA: commitSHA,
		cleanup:   cleanup,
	}, nil
}

func (f *gitRepositoryFetcher) cloneOptions(cloneURL string) *git.CloneOptions {
	options := &git.CloneOptions{URL: cloneURL}
	if f.token != "" {
		options.Auth = &githttp.BasicAuth{
			Username: "x-access-token",
			Password: f.token,
		}
	}
	return options
}

func checkoutAndResolveGitRef(repository *git.Repository, ref string) (string, error) {
	if ref == "" {
		head, err := repository.Head()
		if err != nil {
			return "", fmt.Errorf("failed to resolve repository HEAD: %w", err)
		}
		return head.Hash().String(), nil
	}

	hash, err := resolveGitRevision(repository, ref)
	if err != nil {
		return "", fmt.Errorf("failed to resolve ref %q: %w", ref, err)
	}

	worktree, err := repository.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to open repository worktree: %w", err)
	}
	if err := worktree.Checkout(&git.CheckoutOptions{Hash: *hash, Force: true}); err != nil {
		return "", fmt.Errorf("failed to checkout ref %q: %w", ref, err)
	}

	return hash.String(), nil
}

func resolveGitRevision(repository *git.Repository, ref string) (*plumbing.Hash, error) {
	candidates := []plumbing.Revision{
		plumbing.Revision(ref),
		plumbing.Revision("refs/heads/" + ref),
		plumbing.Revision("refs/remotes/origin/" + ref),
		plumbing.Revision("refs/tags/" + ref),
	}

	var lastErr error
	for _, candidate := range candidates {
		hash, err := repository.ResolveRevision(candidate)
		if err == nil {
			return hash, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = plumbing.ErrReferenceNotFound
	}
	return nil, lastErr
}

func enforceGitWorktreeFileLimit(repoRoot string, maxFiles int) error {
	var fileCount int
	err := filepath.WalkDir(repoRoot, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}

		fileCount++
		if fileCount > maxFiles {
			return fmt.Errorf("repository checkout exceeds maximum file count of %d", maxFiles)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to inspect repository checkout: %w", err)
	}

	return nil
}

func parseHTTPSRepositoryURL(repoURL string) (*url.URL, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("repository URL must use HTTPS")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("repository URL must include a host")
	}
	if u.User != nil {
		return nil, fmt.Errorf("repository URL must not include credentials")
	}
	return u, nil
}

func parseGitRepositoryURL(repoURL string) (string, error) {
	u, err := parseHTTPSRepositoryURL(repoURL)
	if err != nil {
		return "", err
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("repository URL must be of the form https://{host}/{owner}/{repo}.git")
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("repository URL path contains an invalid segment")
		}
	}

	repoPath := strings.Join(parts, "/")
	if !strings.HasSuffix(repoPath, ".git") {
		return "", fmt.Errorf("repository URL path must end with .git for non-GitHub hosts")
	}

	cloneURL := *u
	cloneURL.Path = "/" + repoPath
	cloneURL.RawPath = ""
	cloneURL.RawQuery = ""
	cloneURL.Fragment = ""
	return cloneURL.String(), nil
}

func isGitHubRepositoryURL(repoURL string) bool {
	u, err := url.Parse(repoURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, "github.com")
}

func (fs *sizeLimitedFS) Create(filename string) (billy.File, error) {
	f, err := fs.Filesystem.Create(filename)
	if err != nil {
		return nil, err
	}
	return &sizeLimitedFile{File: f, fs: fs}, nil
}

func (fs *sizeLimitedFS) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	f, err := fs.Filesystem.OpenFile(filename, flag, perm)
	if err != nil {
		return nil, err
	}
	return &sizeLimitedFile{File: f, fs: fs}, nil
}

func (fs *sizeLimitedFS) TempFile(dir, prefix string) (billy.File, error) {
	f, err := fs.Filesystem.TempFile(dir, prefix)
	if err != nil {
		return nil, err
	}
	return &sizeLimitedFile{File: f, fs: fs}, nil
}

func (f *sizeLimitedFile) Write(p []byte) (int, error) {
	if f.fs.written.Add(int64(len(p))) > f.fs.maxBytes {
		return 0, errGitRepoTooLarge
	}
	return f.File.Write(p)
}
