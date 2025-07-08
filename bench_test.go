package main

import (
	"context"
	"os"
	"testing"
)

// Benchmark the core workflow functions
func BenchmarkCreateBatches(b *testing.B) {
	orgs := []string{"org1", "org2", "org3", "org4", "org5", "org6", "org7", "org8", "org9", "org10"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		createBatches(orgs, 3)
	}
}

func BenchmarkValidateConfig(b *testing.B) {
	config := &Config{
		Orgs:        []string{"org1", "org2", "org3", "org4", "org5"},
		BatchSize:   50,
		Concurrency: 10,
		TargetDir:   "/tmp/repos",
		GitHubToken: "token123",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a copy to avoid mutation
		testConfig := *config
		validateConfig(&testConfig)
	}
}

func BenchmarkProcessConfig(b *testing.B) {
	config := &Config{
		Orgs:        []string{"org1", "org2", "org3", "org4", "org5"},
		BatchSize:   2,
		Concurrency: 10,
		TargetDir:   "/tmp/repos",
		GitHubToken: "token123",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a copy to avoid mutation
		testConfig := *config
		processConfig(&testConfig)
	}
}

func BenchmarkBuildCommand(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildCommand("test-org", "/tmp/test", "test-token", 5)
	}
}

func BenchmarkExpandPath(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expandPath("~/projects/test", "/home/user")
	}
}

func BenchmarkConfigLoading(b *testing.B) {
	// Set up environment for testing
	os.Setenv("GHORG_TARGET_DIR", "/tmp/bench-test")
	os.Setenv("GHORG_ORGS", "org1,org2,org3,org4,org5")
	os.Setenv("GHORG_CONCURRENCY", "10")
	os.Setenv("GHORG_BATCH_SIZE", "2")
	os.Setenv("GHORG_GITHUB_TOKEN", "test-token")

	defer func() {
		os.Unsetenv("GHORG_TARGET_DIR")
		os.Unsetenv("GHORG_ORGS")
		os.Unsetenv("GHORG_CONCURRENCY")
		os.Unsetenv("GHORG_BATCH_SIZE")
		os.Unsetenv("GHORG_GITHUB_TOKEN")
	}()

	loader := CreateEnvConfigLoader()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		loader(ctx, "")
	}
}

func BenchmarkWorkflowExecution(b *testing.B) {
	// Set up environment for testing
	os.Setenv("GHORG_TARGET_DIR", "/tmp/bench-workflow")
	os.Setenv("GHORG_ORGS", "org1,org2")
	os.Setenv("GHORG_CONCURRENCY", "2")
	os.Setenv("GHORG_BATCH_SIZE", "1")
	os.Setenv("GHORG_GITHUB_TOKEN", "test-token")

	defer func() {
		os.Unsetenv("GHORG_TARGET_DIR")
		os.Unsetenv("GHORG_ORGS")
		os.Unsetenv("GHORG_CONCURRENCY")
		os.Unsetenv("GHORG_BATCH_SIZE")
		os.Unsetenv("GHORG_GITHUB_TOKEN")
		os.RemoveAll("/tmp/bench-workflow")
	}()

	configLoader := CreateEnvConfigLoader()
	orgCloner := CreateGhorgOrgCloner()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This will fail at ghorg command but we can measure everything up to that point
		ExecuteWorkflow(ctx, "", configLoader, orgCloner)
	}
}
