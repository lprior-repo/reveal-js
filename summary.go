package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func CalculateSummary(results []OrganizationResult, duration time.Duration) GlobalSummary {
	summary := GlobalSummary{
		TotalOrgs:        len(results),
		ResourcesByType:  make(map[string]int),
		ProviderUsage:    make(map[string]int),
		WorkspacesByType: make(map[string]int),
		AnalysisDuration: duration,
	}
	
	totalCacheHits := 0
	totalRepos := 0
	
	for _, org := range results {
		summary.TotalRepos += org.Summary.TotalRepos
		summary.TotalTerraformRepos += org.Summary.TerraformRepos
		summary.TotalWorkspaces += org.Summary.TotalWorkspaces
		summary.TotalResources += org.Summary.TotalResources
		
		// Aggregate resource types
		for resourceType, count := range org.Summary.ResourcesByType {
			summary.ResourcesByType[resourceType] += count
		}
		
		// Aggregate provider usage
		for provider, count := range org.Summary.ProviderUsage {
			summary.ProviderUsage[provider] += count
		}
		
		// Aggregate workspace types
		for workspaceType, count := range org.Summary.WorkspacesByType {
			summary.WorkspacesByType[workspaceType] += count
		}
		
		// Calculate cache hit rate
		for _, repo := range org.Repositories {
			totalRepos++
			if repo.CacheHit {
				totalCacheHits++
			}
		}
	}
	
	if totalRepos > 0 {
		summary.CacheHitRate = float64(totalCacheHits) / float64(totalRepos)
	}
	
	return summary
}

func PrintSummary(summary GlobalSummary, outputFile string) {
	fmt.Printf("\n" + strings.Repeat("=", 80) + "\n")
	fmt.Printf("🎯 TERRAFORM CLOUD MIGRATION ANALYSIS COMPLETE\n")
	fmt.Printf(strings.Repeat("=", 80) + "\n\n")
	
	fmt.Printf("📊 OVERVIEW:\n")
	fmt.Printf("  Organizations analyzed: %d\n", summary.TotalOrgs)
	fmt.Printf("  Total repositories: %d\n", summary.TotalRepos)
	fmt.Printf("  Terraform repositories: %d (%.1f%%)\n", 
		summary.TotalTerraformRepos, 
		float64(summary.TotalTerraformRepos)/float64(summary.TotalRepos)*100)
	fmt.Printf("  Total workspaces: %d\n", summary.TotalWorkspaces)
	fmt.Printf("  Total resources: %d\n", summary.TotalResources)
	fmt.Printf("  Analysis duration: %v\n", summary.AnalysisDuration.Round(time.Second))
	fmt.Printf("  Cache hit rate: %.1f%%\n\n", summary.CacheHitRate*100)
	
	// Workspace breakdown
	fmt.Printf("🏗️  WORKSPACES BY TYPE:\n")
	for workspaceType, count := range summary.WorkspacesByType {
		fmt.Printf("  %s: %d\n", workspaceType, count)
	}
	fmt.Printf("\n")
	
	// Top resource types
	fmt.Printf("📦 TOP RESOURCE TYPES:\n")
	resourcePairs := make([]struct{ Type string; Count int }, 0, len(summary.ResourcesByType))
	for resourceType, count := range summary.ResourcesByType {
		resourcePairs = append(resourcePairs, struct{ Type string; Count int }{resourceType, count})
	}
	sort.Slice(resourcePairs, func(i, j int) bool {
		return resourcePairs[i].Count > resourcePairs[j].Count
	})
	
	for i, pair := range resourcePairs {
		if i >= 10 { // Show top 10
			break
		}
		fmt.Printf("  %s: %d\n", pair.Type, pair.Count)
	}
	fmt.Printf("\n")
	
	// Top providers
	fmt.Printf("🔌 TERRAFORM PROVIDER USAGE:\n")
	providerPairs := make([]struct{ Provider string; Count int }, 0, len(summary.ProviderUsage))
	for provider, count := range summary.ProviderUsage {
		providerPairs = append(providerPairs, struct{ Provider string; Count int }{provider, count})
	}
	sort.Slice(providerPairs, func(i, j int) bool {
		return providerPairs[i].Count > providerPairs[j].Count
	})
	
	fmt.Printf("  Total unique providers: %d\n", len(providerPairs))
	for _, pair := range providerPairs {
		fmt.Printf("  %s: %d workspaces\n", pair.Provider, pair.Count)
	}
	fmt.Printf("\n")
	
	fmt.Printf("💾 Output saved to: %s\n", outputFile)
	fmt.Printf(strings.Repeat("=", 80) + "\n")
}