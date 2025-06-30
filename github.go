package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v62/github"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

type githubService struct {
	client         *github.Client
	semaphore      *semaphore.Weighted
	rateLimiter    *time.Ticker
	batchSize      int
	maxRetries     int
	connectionPool *http.Client
	mu             sync.RWMutex
	requestCache   map[string]interface{}
}

func NewGitHubService(token string, concurrency int) GitHubService {
	// Create optimized HTTP client with connection pooling
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 50,
			IdleConnTimeout:     90 * time.Second,
			MaxConnsPerHost:     20,
		},
		Timeout: 30 * time.Second,
	}

	var client *github.Client
	if token != "" {
		client = github.NewClient(httpClient).WithAuthToken(token)
		fmt.Printf("🔐 Using GitHub REST API with authentication and connection pooling\n")
	} else {
		client = github.NewClient(httpClient)
		fmt.Printf("⚠️  No GitHub token - using unauthenticated REST requests\n")
	}

	// Adjust rate limiting based on authentication
	rateLimit := time.Millisecond * 20 // More aggressive for authenticated users
	if token == "" {
		rateLimit = time.Millisecond * 200 // Conservative for unauthenticated
	}

	return &githubService{
		client:         client,
		semaphore:      semaphore.NewWeighted(int64(concurrency * 2)), // Increase semaphore capacity
		rateLimiter:    time.NewTicker(rateLimit),
		batchSize:      50,
		maxRetries:     3,
		connectionPool: httpClient,
		requestCache:   make(map[string]interface{}),
	}
}

func (g *githubService) ListRepositories(ctx context.Context, org string) ([]Repository, error) {
	return ListRepositoriesForOrg(ctx, g.client, g.semaphore, g.rateLimiter, org)
}

func (g *githubService) GetRepositoryContent(ctx context.Context, owner, repo, path string) (*github.RepositoryContent, error) {
	return GetRepoContent(ctx, g.client, g.semaphore, g.rateLimiter, owner, repo, path)
}

func (g *githubService) ListRepositoryContents(ctx context.Context, owner, repo, path string) ([]*github.RepositoryContent, error) {
	return ListRepoContents(ctx, g.client, g.semaphore, g.rateLimiter, owner, repo, path)
}

func (g *githubService) GetRepositoryTree(ctx context.Context, owner, repo, sha string) (*github.Tree, error) {
	return GetRepoTree(ctx, g.client, g.semaphore, g.rateLimiter, owner, repo, sha)
}

func (g *githubService) GetMultipleFileContents(ctx context.Context, owner, repo string, paths []string) (map[string]*github.RepositoryContent, error) {
	return GetMultipleRepoContents(ctx, g.client, g.semaphore, g.rateLimiter, owner, repo, paths, g.batchSize)
}

// Helper function to check if a repository is likely Terraform-related
func IsTerraformRepository(repo Repository) bool {
	name := strings.ToLower(repo.Name)
	desc := strings.ToLower(repo.Description)
	lang := strings.ToLower(repo.Language)

	return strings.Contains(name, "terraform") ||
		strings.Contains(desc, "terraform") ||
		lang == "hcl" ||
		strings.Contains(name, "tf-") ||
		strings.Contains(name, "-tf")
}

// Helper function to check if a repository is Evaporate-related
func IsEvaporateRepository(repo Repository) bool {
	name := strings.ToLower(repo.Name)
	desc := strings.ToLower(repo.Description)

	return strings.Contains(name, "evaporate") ||
		strings.Contains(desc, "evaporate")
}

// ListRepositoriesForOrg fetches all repositories for an organization
func ListRepositoriesForOrg(ctx context.Context, client *github.Client, sem *semaphore.Weighted, limiter *time.Ticker, org string) ([]Repository, error) {
	if err := sem.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer sem.Release(1)

	<-limiter.C

	var allRepos []Repository
	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
		Sort:        "updated",
		Direction:   "desc",
		Type:        "all",
	}

	for {
		repos, resp, err := client.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			if rateLimitErr, ok := err.(*github.RateLimitError); ok {
				fmt.Printf("⏰ Rate limit hit, waiting until %v\n", rateLimitErr.Rate.Reset.Time)
				time.Sleep(time.Until(rateLimitErr.Rate.Reset.Time))
				continue
			}
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		for _, repo := range repos {
			allRepos = append(allRepos, Repository{
				ID:            repo.GetID(),
				FullName:      repo.GetFullName(),
				Name:          repo.GetName(),
				Organization:  org,
				Description:   repo.GetDescription(),
				Language:      repo.GetLanguage(),
				HTMLURL:       repo.GetHTMLURL(),
				CreatedAt:     repo.GetCreatedAt().Time,
				UpdatedAt:     repo.GetUpdatedAt().Time,
				PushedAt:      repo.GetPushedAt().Time,
				Size:          repo.GetSize(),
				IsPrivate:     repo.GetPrivate(),
				IsFork:        repo.GetFork(),
				IsArchived:    repo.GetArchived(),
				OpenIssues:    repo.GetOpenIssuesCount(),
				DefaultBranch: repo.GetDefaultBranch(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

// GetRepoContent fetches repository content
func GetRepoContent(ctx context.Context, client *github.Client, sem *semaphore.Weighted, limiter *time.Ticker, owner, repo, path string) (*github.RepositoryContent, error) {
	if err := sem.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer sem.Release(1)

	<-limiter.C

	content, _, _, err := client.Repositories.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
		if rateLimitErr, ok := err.(*github.RateLimitError); ok {
			fmt.Printf("⏰ Rate limit hit, waiting until %v\n", rateLimitErr.Rate.Reset.Time)
			time.Sleep(time.Until(rateLimitErr.Rate.Reset.Time))
			return GetRepoContent(ctx, client, sem, limiter, owner, repo, path)
		}
		return nil, err
	}
	return content, nil
}

// ListRepoContents fetches repository directory contents
func ListRepoContents(ctx context.Context, client *github.Client, sem *semaphore.Weighted, limiter *time.Ticker, owner, repo, path string) ([]*github.RepositoryContent, error) {
	if err := sem.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer sem.Release(1)

	<-limiter.C

	_, contents, _, err := client.Repositories.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
		if rateLimitErr, ok := err.(*github.RateLimitError); ok {
			fmt.Printf("⏰ Rate limit hit, waiting until %v\n", rateLimitErr.Rate.Reset.Time)
			time.Sleep(time.Until(rateLimitErr.Rate.Reset.Time))
			return ListRepoContents(ctx, client, sem, limiter, owner, repo, path)
		}
		return nil, err
	}
	return contents, nil
}

// GetRepoTree fetches repository tree
func GetRepoTree(ctx context.Context, client *github.Client, sem *semaphore.Weighted, limiter *time.Ticker, owner, repo, sha string) (*github.Tree, error) {
	if err := sem.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer sem.Release(1)

	<-limiter.C

	tree, _, err := client.Git.GetTree(ctx, owner, repo, sha, true)
	if err != nil {
		if rateLimitErr, ok := err.(*github.RateLimitError); ok {
			fmt.Printf("⏰ Rate limit hit, waiting until %v\n", rateLimitErr.Rate.Reset.Time)
			time.Sleep(time.Until(rateLimitErr.Rate.Reset.Time))
			return GetRepoTree(ctx, client, sem, limiter, owner, repo, sha)
		}
		return nil, err
	}
	return tree, nil
}

// GetMultipleRepoContents fetches multiple files concurrently with batching
func GetMultipleRepoContents(ctx context.Context, client *github.Client, sem *semaphore.Weighted, limiter *time.Ticker, owner, repo string, paths []string, batchSize int) (map[string]*github.RepositoryContent, error) {
	results := make(map[string]*github.RepositoryContent)
	var mu sync.Mutex
	
	// Process paths in batches to avoid overwhelming the API
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(batchSize)
	
	for _, path := range paths {
		path := path // Capture loop variable
		g.Go(func() error {
			content, err := GetRepoContent(ctx, client, sem, limiter, owner, repo, path)
			if err != nil {
				// Log error but don't fail the entire batch
				fmt.Printf("⚠️  Failed to get content for %s:%s: %v\n", repo, path, err)
			} else if content != nil {
				mu.Lock()
				results[path] = content
				mu.Unlock()
			}
			return nil // Don't propagate errors to avoid failing entire batch
		})
	}
	
	if err := g.Wait(); err != nil {
		return results, err
	}
	
	return results, nil
}