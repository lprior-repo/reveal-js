package main

import (
	"context"
	"time"

	"github.com/google/go-github/v62/github"
)

// Core types
type Repository struct {
	ID            int64     `json:"id"`
	FullName      string    `json:"full_name"`
	Name          string    `json:"name"`
	Organization  string    `json:"organization"`
	Description   string    `json:"description"`
	Language      string    `json:"language"`
	HTMLURL       string    `json:"html_url"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	PushedAt      time.Time `json:"pushed_at"`
	Size          int       `json:"size"`
	IsPrivate     bool      `json:"is_private"`
	IsFork        bool      `json:"is_fork"`
	IsArchived    bool      `json:"is_archived"`
	OpenIssues    int       `json:"open_issues"`
	DefaultBranch string    `json:"default_branch"`
}

type TerraformFile struct {
	Path            string    `json:"path"`
	Content         string    `json:"content"`
	Size            int       `json:"size"`
	LastModified    time.Time `json:"last_modified"`
	Providers       []string  `json:"providers"`
	Resources       []Resource `json:"resources"`
	Variables       []Variable `json:"variables"`
	Outputs         []Output   `json:"outputs"`
	Workspace       string     `json:"workspace"` // dev, stage, prod
}

type Resource struct {
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes"`
	File       string            `json:"file"`
	Line       int               `json:"line"`
	Workspace  string            `json:"workspace"`
}

type Variable struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Default     string `json:"default"`
	File        string `json:"file"`
	Line        int    `json:"line"`
}

type Output struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Value       string `json:"value"`
	Sensitive   bool   `json:"sensitive"`
	File        string `json:"file"`
	Line        int    `json:"line"`
}

type WorkspaceInfo struct {
	Name          string     `json:"name"`
	Path          string     `json:"path"`
	ResourceCount int        `json:"resource_count"`
	Resources     []Resource `json:"resources"`
	Providers     []string   `json:"providers"`
	HasTfvars     bool       `json:"has_tfvars"`
	HasBackend    bool       `json:"has_backend"`
}

type RepositoryAnalysis struct {
	Repository    Repository      `json:"repository"`
	IsTerraform   bool            `json:"is_terraform"`
	IsEvaporate   bool            `json:"is_evaporate"`
	Workspaces    []WorkspaceInfo `json:"workspaces"`
	TotalResources int            `json:"total_resources"`
	AllProviders  []string        `json:"all_providers"`
	TerraformFiles []TerraformFile `json:"terraform_files"`
	LastAnalyzed  time.Time       `json:"last_analyzed"`
	CacheHit      bool            `json:"cache_hit"`
}

type OrganizationResult struct {
	Organization string               `json:"organization"`
	Repositories []RepositoryAnalysis `json:"repositories"`
	Summary      OrgSummary           `json:"summary"`
	AnalyzedAt   time.Time            `json:"analyzed_at"`
}

type OrgSummary struct {
	TotalRepos         int                    `json:"total_repos"`
	TerraformRepos     int                    `json:"terraform_repos"`
	EvaporateRepos     int                    `json:"evaporate_repos"`
	TotalWorkspaces    int                    `json:"total_workspaces"`
	TotalResources     int                    `json:"total_resources"`
	ResourcesByType    map[string]int         `json:"resources_by_type"`
	ProviderUsage      map[string]int         `json:"provider_usage"`
	WorkspacesByType   map[string]int         `json:"workspaces_by_type"` // dev, stage, prod counts
}

type AnalysisResult struct {
	Organizations []OrganizationResult `json:"organizations"`
	Summary       GlobalSummary        `json:"summary"`
	AnalyzedAt    time.Time            `json:"analyzed_at"`
}

type GlobalSummary struct {
	TotalOrgs          int                    `json:"total_orgs"`
	TotalRepos         int                    `json:"total_repos"`
	TotalTerraformRepos int                   `json:"total_terraform_repos"`
	TotalWorkspaces    int                    `json:"total_workspaces"`
	TotalResources     int                    `json:"total_resources"`
	ResourcesByType    map[string]int         `json:"resources_by_type"`
	ProviderUsage      map[string]int         `json:"provider_usage"`
	WorkspacesByType   map[string]int         `json:"workspaces_by_type"`
	AnalysisDuration   time.Duration          `json:"analysis_duration"`
	CacheHitRate       float64                `json:"cache_hit_rate"`
}

// Configuration
type Config struct {
	GitHubToken   string   `json:"github_token"`
	Organizations []string `json:"organizations"`
	OutputFile    string   `json:"output_file"`
	CacheFile     string   `json:"cache_file"`
	Concurrency   int      `json:"concurrency"`
	ForceRefresh  bool     `json:"force_refresh"`
	UseDynamoDB   bool     `json:"use_dynamodb"`
	AWSRegion     string   `json:"aws_region"`
	TableName     string   `json:"table_name"`
}

// Services
type Services struct {
	GitHub   GitHubService
	Cache    CacheService
	Analyzer FileAnalyzer
}

type GitHubService interface {
	ListRepositories(ctx context.Context, org string) ([]Repository, error)
	GetRepositoryContent(ctx context.Context, owner, repo, path string) (*github.RepositoryContent, error)
	ListRepositoryContents(ctx context.Context, owner, repo, path string) ([]*github.RepositoryContent, error)
	GetRepositoryTree(ctx context.Context, owner, repo, sha string) (*github.Tree, error)
	GetMultipleFileContents(ctx context.Context, owner, repo string, paths []string) (map[string]*github.RepositoryContent, error)
}

type CacheService interface {
	Get(key string) (*RepositoryAnalysis, bool)
	Set(key string, analysis *RepositoryAnalysis) error
	Load() error
	Save() error
}

type FileAnalyzer interface {
	AnalyzeFile(file TerraformFile) ([]Resource, []string, error)
	ParseTerraformFile(content, filename string) ([]Resource, []Variable, []Output, []string, error)
	DetectWorkspace(path string) string
	CountResources(files []TerraformFile) map[string]int
}

// Cache types for file storage
type CacheData struct {
	Repositories map[string]*RepositoryAnalysis `json:"repositories"`
	LastUpdated  time.Time                      `json:"last_updated"`
	Version      string                         `json:"version"`
}