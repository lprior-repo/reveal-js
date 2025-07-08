package main

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// DRY: Config moved to fp.go to avoid duplication

// ValidateAndSetDefaults validates configuration and sets optimal defaults
func (c *Config) ValidateAndSetDefaults() error {
	// Sanitize and validate target directory
	c.TargetDir = strings.TrimSpace(c.TargetDir)
	if c.TargetDir == "" {
		return fmt.Errorf("target_dir is required")
	}

	// Validate organizations (this also sanitizes them)
	if len(c.Orgs) == 0 {
		return fmt.Errorf("at least one organization must be specified")
	}

	// Set optimal defaults based on workload
	if c.Concurrency <= 0 {
		c.Concurrency = min(25, max(1, len(c.Orgs))) // Scale with org count
	}

	if c.BatchSize <= 0 {
		c.BatchSize = min(50, max(1, len(c.Orgs))) // Scale with org count
	}

	return nil
}

// Helper function for maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// processOrgsFromEnv orchestrates the workflow using functional architecture with env config
func processOrgsFromEnv(envFilePath string) error {
	// Create functional adapters (side effect implementations)
	configLoader := CreateEnvConfigLoader()
	orgCloner := CreateGhorgOrgCloner()

	// Execute workflow using functional composition (Action -> Calculation -> Action)
	ctx := context.Background()
	return ExecuteWorkflow(ctx, envFilePath, configLoader, orgCloner)
}

func main() {
	var envFilePath string

	// Check if .env file path is provided as argument
	if len(os.Args) > 1 {
		envFilePath = os.Args[1]
	}

	err := processOrgsFromEnv(envFilePath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
