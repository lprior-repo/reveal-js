package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

func main() {
	startTime := time.Now()

	fmt.Printf("🚀 Terraform Cloud Migration Analyzer\n")
	fmt.Printf("📖 Loading configuration...\n")

	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if config.GitHubToken == "" {
		fmt.Printf("⚠️  No GitHub token found. Add GITHUB_TOKEN to .env file\n")
		fmt.Printf("⚠️  Using unauthenticated requests with strict rate limits\n")
	} else {
		fmt.Printf("🔐 GitHub token loaded\n")
	}

	fmt.Printf("📋 Organizations to analyze: %v\n", config.Organizations)

	// Create services
	githubService := NewGitHubService(config.GitHubToken, config.Concurrency)
	cacheService := NewFileCache(config.CacheFile)
	fileAnalyzer := NewFileAnalyzer()

	services := &Services{
		GitHub:   githubService,
		Cache:    cacheService,
		Analyzer: fileAnalyzer,
	}

	// Analyze all organizations
	var results []OrganizationResult
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(len(config.Organizations))

	for _, org := range config.Organizations {
		org := org
		g.Go(func() error {
			result, err := AnalyzeOrganization(ctx, org, services, config)
			if err != nil {
				log.Printf("❌ Error analyzing %s: %v", org, err)
				return nil
			}

			mu.Lock()
			results = append(results, *result)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Printf("❌ Some analyses failed")
	}

	if len(results) == 0 {
		log.Fatal("No organizations were successfully analyzed")
	}

	summary := CalculateSummary(results, time.Since(startTime))
	finalResult := AnalysisResult{
		Organizations: results,
		Summary:       summary,
		AnalyzedAt:    time.Now(),
	}

	if err := WriteJSONFile(config.OutputFile, finalResult); err != nil {
		log.Fatalf("Failed to write output: %v", err)
	}

	// Generate CSV reports
	outputPrefix := strings.TrimSuffix(config.OutputFile, ".json")
	if err := WriteCSVReports(results, outputPrefix); err != nil {
		log.Printf("⚠️  Failed to write CSV reports: %v", err)
	}

	PrintSummary(summary, config.OutputFile)

	// Optional: Update DynamoDB if configured
	if config.UseDynamoDB {
		if err := UpdateDynamoDB(ctx, config, results); err != nil {
			log.Printf("⚠️  Failed to update DynamoDB: %v", err)
		}
	}
}