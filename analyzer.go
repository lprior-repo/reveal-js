package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v62/github"
	"golang.org/x/sync/errgroup"
)

type fileAnalyzer struct{}

func NewFileAnalyzer() FileAnalyzer {
	return &fileAnalyzer{}
}

func (f *fileAnalyzer) DetectWorkspace(path string) string {
	path = strings.ToLower(path)
	
	// Check for workspace folders
	if strings.Contains(path, "/dev/") || strings.Contains(path, "/development/") {
		return "dev"
	}
	if strings.Contains(path, "/stage/") || strings.Contains(path, "/staging/") {
		return "stage"
	}
	if strings.Contains(path, "/prod/") || strings.Contains(path, "/production/") {
		return "prod"
	}
	
	// Check for workspace files
	if strings.Contains(path, "dev.tfvars") || strings.Contains(path, "development.tfvars") {
		return "dev"
	}
	if strings.Contains(path, "stage.tfvars") || strings.Contains(path, "staging.tfvars") {
		return "stage"
	}
	if strings.Contains(path, "prod.tfvars") || strings.Contains(path, "production.tfvars") {
		return "prod"
	}
	
	return "default"
}

func (f *fileAnalyzer) ParseTerraformFile(content, filename string) ([]Resource, []Variable, []Output, []string, error) {
	var resources []Resource
	var variables []Variable
	var outputs []Output
	var providers []string
	
	lines := strings.Split(content, "\n")
	
	// Resource regex patterns
	resourcePattern := regexp.MustCompile(`^\s*resource\s+"([^"]+)"\s+"([^"]+)"\s*{`)
	providerPattern := regexp.MustCompile(`^\s*provider\s+"([^"]+)"\s*{`)
	variablePattern := regexp.MustCompile(`^\s*variable\s+"([^"]+)"\s*{`)
	outputPattern := regexp.MustCompile(`^\s*output\s+"([^"]+)"\s*{`)
	
	workspace := f.DetectWorkspace(filename)
	
	for lineNum, line := range lines {
		// Parse resources
		if matches := resourcePattern.FindStringSubmatch(line); len(matches) == 3 {
			resources = append(resources, Resource{
				Type:      matches[1],
				Name:      matches[2],
				File:      filename,
				Line:      lineNum + 1,
				Workspace: workspace,
			})
		}
		
		// Parse providers
		if matches := providerPattern.FindStringSubmatch(line); len(matches) == 2 {
			provider := matches[1]
			if !contains(providers, provider) {
				providers = append(providers, provider)
			}
		}
		
		// Parse variables
		if matches := variablePattern.FindStringSubmatch(line); len(matches) == 2 {
			variables = append(variables, Variable{
				Name: matches[1],
				File: filename,
				Line: lineNum + 1,
			})
		}
		
		// Parse outputs
		if matches := outputPattern.FindStringSubmatch(line); len(matches) == 2 {
			outputs = append(outputs, Output{
				Name: matches[1],
				File: filename,
				Line: lineNum + 1,
			})
		}
	}
	
	return resources, variables, outputs, providers, nil
}

func (f *fileAnalyzer) AnalyzeFile(file TerraformFile) ([]Resource, []string, error) {
	resources, _, _, providers, err := f.ParseTerraformFile(file.Content, file.Path)
	return resources, providers, err
}

func (f *fileAnalyzer) CountResources(files []TerraformFile) map[string]int {
	counts := make(map[string]int)
	
	for _, file := range files {
		resources, _, err := f.AnalyzeFile(file)
		if err != nil {
			continue
		}
		
		for _, resource := range resources {
			counts[resource.Type]++
		}
	}
	
	return counts
}

// AnalyzeRepository performs comprehensive analysis of a repository
func AnalyzeRepository(ctx context.Context, repo Repository, services *Services, config *Config) (*RepositoryAnalysis, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s", repo.Organization, repo.Name)
	if !config.ForceRefresh {
		if cached, hit := services.Cache.Get(cacheKey); hit {
			fmt.Printf("💾 Cache hit for %s\n", repo.FullName)
			return cached, nil
		}
	}

	fmt.Printf("🔍 Analyzing %s...\n", repo.FullName)
	
	analysis := &RepositoryAnalysis{
		Repository:     repo,
		IsTerraform:    IsTerraformRepository(repo),
		IsEvaporate:    IsEvaporateRepository(repo),
		Workspaces:     []WorkspaceInfo{},
		TerraformFiles: []TerraformFile{},
		AllProviders:   []string{},
		LastAnalyzed:   time.Now(),
		CacheHit:       false,
	}

	// Skip non-Terraform repos
	if !analysis.IsTerraform && !analysis.IsEvaporate {
		services.Cache.Set(cacheKey, analysis)
		return analysis, nil
	}

	// Get repository tree to find all terraform files
	tree, err := services.GitHub.GetRepositoryTree(ctx, repo.Organization, repo.Name, repo.DefaultBranch)
	if err != nil {
		log.Printf("Failed to get tree for %s: %v", repo.FullName, err)
		services.Cache.Set(cacheKey, analysis)
		return analysis, nil
	}

	// Find and analyze terraform files
	terraformFiles, err := services.Analyzer.(*fileAnalyzer).findTerraformFiles(ctx, tree, repo, services.GitHub)
	if err != nil {
		log.Printf("Failed to analyze files for %s: %v", repo.FullName, err)
	}

	analysis.TerraformFiles = terraformFiles
	
	// Group files by workspace and analyze
	workspaces := services.Analyzer.(*fileAnalyzer).groupByWorkspace(terraformFiles)
	analysis.Workspaces = workspaces
	
	// Calculate totals
	totalResources := 0
	allProviders := make(map[string]bool)
	
	for _, workspace := range workspaces {
		totalResources += workspace.ResourceCount
		for _, provider := range workspace.Providers {
			allProviders[provider] = true
		}
	}
	
	analysis.TotalResources = totalResources
	for provider := range allProviders {
		analysis.AllProviders = append(analysis.AllProviders, provider)
	}

	// Cache the result
	services.Cache.Set(cacheKey, analysis)
	
	return analysis, nil
}

func (f *fileAnalyzer) findTerraformFiles(ctx context.Context, tree *github.Tree, repo Repository, githubSvc GitHubService) ([]TerraformFile, error) {
	// First, collect all terraform file paths
	var terraformPaths []string
	for _, entry := range tree.Entries {
		if entry.GetType() == "blob" && f.isTerraformFile(entry.GetPath()) {
			terraformPaths = append(terraformPaths, entry.GetPath())
		}
	}
	
	if len(terraformPaths) == 0 {
		return []TerraformFile{}, nil
	}
	
	fmt.Printf("📝 Found %d terraform files in %s, fetching concurrently...\n", len(terraformPaths), repo.FullName)
	
	// Use batch processing to fetch multiple files concurrently
	contents, err := githubSvc.GetMultipleFileContents(ctx, repo.Organization, repo.Name, terraformPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file contents: %w", err)
	}
	
	// Process files concurrently
	var files []TerraformFile
	var mu sync.Mutex
	
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(20) // Limit concurrent file processing
	
	for path, content := range contents {
		path, content := path, content // Capture loop variables
		g.Go(func() error {
			if content == nil {
				return nil
			}
			
			fileContent, err := content.GetContent()
			if err != nil {
				log.Printf("Failed to decode content for %s:%s: %v", repo.FullName, path, err)
				return nil
			}
			
			// Parse the file
			resources, variables, outputs, providers, err := f.ParseTerraformFile(fileContent, path)
			if err != nil {
				log.Printf("Failed to parse %s:%s: %v", repo.FullName, path, err)
				return nil
			}
			
			terraformFile := TerraformFile{
				Path:         path,
				Content:      fileContent,
				Size:         content.GetSize(),
				LastModified: time.Now(),
				Resources:    resources,
				Variables:    variables,
				Outputs:      outputs,
				Providers:    providers,
				Workspace:    f.DetectWorkspace(path),
			}
			
			mu.Lock()
			files = append(files, terraformFile)
			mu.Unlock()
			
			return nil
		})
	}
	
	if err := g.Wait(); err != nil {
		return nil, err
	}
	
	fmt.Printf("✅ Successfully processed %d terraform files for %s\n", len(files), repo.FullName)
	return files, nil
}

func (f *fileAnalyzer) isTerraformFile(path string) bool {
	return strings.HasSuffix(path, ".tf") ||
		strings.HasSuffix(path, ".tfvars") ||
		strings.HasSuffix(path, ".hcl")
}

func (f *fileAnalyzer) groupByWorkspace(files []TerraformFile) []WorkspaceInfo {
	workspaceMap := make(map[string]*WorkspaceInfo)
	
	for _, file := range files {
		workspace := file.Workspace
		if workspace == "" {
			workspace = "default"
		}
		
		if _, exists := workspaceMap[workspace]; !exists {
			workspaceMap[workspace] = &WorkspaceInfo{
				Name:      workspace,
				Path:      filepath.Dir(file.Path),
				Resources: []Resource{},
				Providers: []string{},
			}
		}
		
		ws := workspaceMap[workspace]
		ws.Resources = append(ws.Resources, file.Resources...)
		ws.ResourceCount += len(file.Resources)
		
		// Add providers
		for _, provider := range file.Providers {
			if !contains(ws.Providers, provider) {
				ws.Providers = append(ws.Providers, provider)
			}
		}
		
		// Check for tfvars and backend files
		if strings.HasSuffix(file.Path, ".tfvars") {
			ws.HasTfvars = true
		}
		if strings.Contains(file.Path, "backend") || strings.Contains(file.Content, "backend") {
			ws.HasBackend = true
		}
	}
	
	var workspaces []WorkspaceInfo
	for _, ws := range workspaceMap {
		workspaces = append(workspaces, *ws)
	}
	
	return workspaces
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}