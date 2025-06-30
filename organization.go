package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

func AnalyzeOrganization(ctx context.Context, org string, services *Services, config *Config) (*OrganizationResult, error) {
	fmt.Printf("🏢 Analyzing organization: %s\n", org)
	
	// Get all repositories for the organization
	repos, err := services.GitHub.ListRepositories(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories for %s: %w", org, err)
	}
	
	fmt.Printf("📚 Found %d repositories in %s\n", len(repos), org)
	
	// Analyze repositories concurrently
	var analyses []RepositoryAnalysis
	var mu sync.Mutex
	
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(config.Concurrency)
	
	for _, repo := range repos {
		repo := repo // Capture loop variable
		g.Go(func() error {
			analysis, err := AnalyzeRepository(ctx, repo, services, config)
			if err != nil {
				log.Printf("❌ Failed to analyze %s: %v", repo.FullName, err)
				return nil // Don't fail the entire organization for one repo
			}
			
			mu.Lock()
			analyses = append(analyses, *analysis)
			mu.Unlock()
			
			return nil
		})
	}
	
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("failed to analyze repositories: %w", err)
	}
	
	// Calculate organization summary
	summary := calculateOrgSummary(analyses)
	
	result := &OrganizationResult{
		Organization: org,
		Repositories: analyses,
		Summary:      summary,
		AnalyzedAt:   time.Now(),
	}
	
	fmt.Printf("✅ Completed analysis of %s: %d terraform repos, %d total resources\n", 
		org, summary.TerraformRepos, summary.TotalResources)
	
	return result, nil
}

func calculateOrgSummary(analyses []RepositoryAnalysis) OrgSummary {
	summary := OrgSummary{
		TotalRepos:       len(analyses),
		ResourcesByType:  make(map[string]int),
		ProviderUsage:    make(map[string]int),
		WorkspacesByType: make(map[string]int),
	}
	
	for _, analysis := range analyses {
		if analysis.IsTerraform {
			summary.TerraformRepos++
		}
		if analysis.IsEvaporate {
			summary.EvaporateRepos++
		}
		
		summary.TotalResources += analysis.TotalResources
		summary.TotalWorkspaces += len(analysis.Workspaces)
		
		// Count resources by type
		for _, workspace := range analysis.Workspaces {
			// Count workspace types
			summary.WorkspacesByType[workspace.Name]++
			
			for _, resource := range workspace.Resources {
				summary.ResourcesByType[resource.Type]++
			}
			
			// Count provider usage
			for _, provider := range workspace.Providers {
				summary.ProviderUsage[provider]++
			}
		}
	}
	
	return summary
}