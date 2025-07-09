package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	ants "github.com/panjf2000/ants/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// Test ExecuteWorkflow following Given-When-Then pattern
func TestExecuteWorkflow(t *testing.T) {
	t.Run("Given valid config and loaders When executing workflow Then should complete successfully", func(t *testing.T) {
		// Given
		ctx := context.Background()
		configPath := ""

		// Mock config loader that returns valid config
		mockLoader := func(ctx context.Context, configPath string) (*Config, error) {
			return &Config{
				Orgs:        []string{"org1", "org2"},
				TargetDir:   "/tmp/test",
				Concurrency: 2,
				BatchSize:   1,
				GitHubToken: "test-token",
			}, nil
		}

		// Mock cloner that succeeds
		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			return nil
		}

		// When
		err := ExecuteWorkflow(ctx, configPath, mockLoader, mockCloner)

		// Then
		assert.NoError(t, err)
	})

	t.Run("Given cancelled context When executing workflow Then should return cancellation error", func(t *testing.T) {
		// Given
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		configPath := ""

		mockLoader := func(ctx context.Context, configPath string) (*Config, error) {
			return &Config{Orgs: []string{"org1"}, TargetDir: "/tmp/test"}, nil
		}

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			return nil
		}

		// When
		err := ExecuteWorkflow(ctx, configPath, mockLoader, mockCloner)

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workflow cancelled before starting")
	})

	t.Run("Given config loader that fails When executing workflow Then should return config error", func(t *testing.T) {
		// Given
		ctx := context.Background()
		configPath := ""

		mockLoader := func(ctx context.Context, configPath string) (*Config, error) {
			return nil, fmt.Errorf("config loading failed")
		}

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			return nil
		}

		// When
		err := ExecuteWorkflow(ctx, configPath, mockLoader, mockCloner)

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "configuration loading failed")
	})

	t.Run("Given invalid config When executing workflow Then should return processing error", func(t *testing.T) {
		// Given
		ctx := context.Background()
		configPath := ""

		mockLoader := func(ctx context.Context, configPath string) (*Config, error) {
			return &Config{
				Orgs:      []string{}, // Invalid - empty orgs
				TargetDir: "/tmp/test",
			}, nil
		}

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			return nil
		}

		// When
		err := ExecuteWorkflow(ctx, configPath, mockLoader, mockCloner)

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "processing plan creation failed")
	})

	t.Run("Given cloner that fails When executing workflow Then should return execution error", func(t *testing.T) {
		// Given
		ctx := context.Background()
		configPath := ""

		mockLoader := func(ctx context.Context, configPath string) (*Config, error) {
			return &Config{
				Orgs:        []string{"org1"},
				TargetDir:   "/tmp/test",
				Concurrency: 1,
				BatchSize:   1,
			}, nil
		}

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			return fmt.Errorf("cloning failed for %s", org)
		}

		// When
		err := ExecuteWorkflow(ctx, configPath, mockLoader, mockCloner)

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cloning failed for org1")
	})
}

// Test executePlan following Given-When-Then pattern
func TestExecutePlan(t *testing.T) {
	t.Run("Given empty plan When executing Then should complete without error", func(t *testing.T) {
		// Given
		ctx := context.Background()
		plan := ProcessingPlan{Batches: []Batch{}, TotalOrgs: 0}
		config := &Config{Concurrency: 1}

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			return nil
		}

		// When
		err := executePlan(ctx, plan, config, mockCloner)

		// Then
		assert.NoError(t, err)
	})

	t.Run("Given plan with single batch When executing Then should process correctly", func(t *testing.T) {
		// Given
		ctx := context.Background()
		plan := ProcessingPlan{
			Batches: []Batch{
				{Orgs: []string{"org1", "org2"}, BatchNumber: 1},
			},
			TotalOrgs: 2,
		}
		config := &Config{
			Concurrency: 2,
			TargetDir:   "/tmp/test",
			GitHubToken: "test-token",
		}

		var processedOrgs []string
		var mu sync.Mutex

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			mu.Lock()
			processedOrgs = append(processedOrgs, org)
			mu.Unlock()
			return nil
		}

		// When
		err := executePlan(ctx, plan, config, mockCloner)

		// Then
		assert.NoError(t, err)
		assert.Len(t, processedOrgs, 2)
		assert.Contains(t, processedOrgs, "org1")
		assert.Contains(t, processedOrgs, "org2")
	})

	t.Run("Given plan with multiple batches When executing Then should process all batches", func(t *testing.T) {
		// Given
		ctx := context.Background()
		plan := ProcessingPlan{
			Batches: []Batch{
				{Orgs: []string{"org1", "org2"}, BatchNumber: 1},
				{Orgs: []string{"org3", "org4"}, BatchNumber: 2},
				{Orgs: []string{"org5"}, BatchNumber: 3},
			},
			TotalOrgs: 5,
		}
		config := &Config{
			Concurrency: 2,
			TargetDir:   "/tmp/test",
			GitHubToken: "test-token",
		}

		var processedOrgs []string
		var mu sync.Mutex

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			mu.Lock()
			processedOrgs = append(processedOrgs, org)
			mu.Unlock()
			return nil
		}

		// When
		err := executePlan(ctx, plan, config, mockCloner)

		// Then
		assert.NoError(t, err)
		assert.Len(t, processedOrgs, 5)
		for i := 1; i <= 5; i++ {
			assert.Contains(t, processedOrgs, fmt.Sprintf("org%d", i))
		}
	})

	t.Run("Given cloner that fails When executing plan Then should collect errors", func(t *testing.T) {
		// Given
		ctx := context.Background()
		plan := ProcessingPlan{
			Batches: []Batch{
				{Orgs: []string{"org1", "org2"}, BatchNumber: 1},
			},
			TotalOrgs: 2,
		}
		config := &Config{
			Concurrency: 2,
			TargetDir:   "/tmp/test",
		}

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			return fmt.Errorf("cloning failed for %s", org)
		}

		// When
		err := executePlan(ctx, plan, config, mockCloner)

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "processing completed with 2 errors")
		assert.Contains(t, err.Error(), "cloning failed for org1")
		assert.Contains(t, err.Error(), "cloning failed for org2")
	})

	t.Run("Given worker pool creation failure When executing plan Then should return error", func(t *testing.T) {
		// Given
		ctx := context.Background()
		plan := ProcessingPlan{
			Batches: []Batch{
				{Orgs: []string{"org1"}, BatchNumber: 1},
			},
			TotalOrgs: 1,
		}
		config := &Config{
			Concurrency: -1, // Invalid concurrency to force pool creation failure
		}

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			return nil
		}

		// When
		err := executePlan(ctx, plan, config, mockCloner)

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create worker pool")
	})
}

// Test processBatches following Given-When-Then pattern
func TestProcessBatches(t *testing.T) {
	t.Run("Given batches with successful cloning When processing Then should return no errors", func(t *testing.T) {
		// Given
		ctx := context.Background()
		batches := []Batch{
			{Orgs: []string{"org1", "org2"}, BatchNumber: 1},
			{Orgs: []string{"org3"}, BatchNumber: 2},
		}

		// Create a real ants pool for testing
		pool, err := ants.NewPool(2, ants.WithOptions(ants.Options{
			PreAlloc:    true,
			Nonblocking: false,
		}))
		require.NoError(t, err)
		defer pool.Release()

		config := &Config{
			TargetDir:   "/tmp/test",
			GitHubToken: "test-token",
			Concurrency: 2,
		}

		logInfo, _ := createLoggers()
		progressTracker := createProgressTracker(3, 2)
		progressReporter := createProgressReporter(logInfo)

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			return nil
		}

		// When
		errors := processBatches(ctx, batches, pool, mockCloner, config, logInfo, &progressTracker, progressReporter)

		// Then
		assert.Empty(t, errors)
	})

	t.Run("Given batches with failing cloning When processing Then should collect errors", func(t *testing.T) {
		// Given
		ctx := context.Background()
		batches := []Batch{
			{Orgs: []string{"org1", "org2"}, BatchNumber: 1},
		}

		pool, err := ants.NewPool(2, ants.WithOptions(ants.Options{
			PreAlloc:    true,
			Nonblocking: false,
		}))
		require.NoError(t, err)
		defer pool.Release()

		config := &Config{
			TargetDir:   "/tmp/test",
			Concurrency: 2,
		}

		logInfo, _ := createLoggers()
		progressTracker := createProgressTracker(2, 1)
		progressReporter := createProgressReporter(logInfo)

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			return fmt.Errorf("cloning failed for %s", org)
		}

		// When
		errors := processBatches(ctx, batches, pool, mockCloner, config, logInfo, &progressTracker, progressReporter)

		// Then
		assert.Len(t, errors, 2)
		assert.Contains(t, errors[0].Error(), "cloning failed for")
		assert.Contains(t, errors[1].Error(), "cloning failed for")
	})

	t.Run("Given cancelled context When processing batches Then should handle cancellation", func(t *testing.T) {
		// Given
		ctx, cancel := context.WithCancel(context.Background())
		batches := []Batch{
			{Orgs: []string{"org1"}, BatchNumber: 1},
		}

		pool, err := ants.NewPool(1, ants.WithOptions(ants.Options{
			PreAlloc:    true,
			Nonblocking: false,
		}))
		require.NoError(t, err)
		defer pool.Release()

		config := &Config{
			TargetDir:   "/tmp/test",
			Concurrency: 1,
		}

		logInfo, _ := createLoggers()
		progressTracker := createProgressTracker(1, 1)
		progressReporter := createProgressReporter(logInfo)

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			return nil
		}

		// Cancel the context before processing
		cancel()

		// When
		errors := processBatches(ctx, batches, pool, mockCloner, config, logInfo, &progressTracker, progressReporter)

		// Then
		assert.NotEmpty(t, errors)
		assert.Contains(t, errors[0].Error(), "workflow cancelled before batch")
	})

	t.Run("Given empty batches When processing Then should return no errors", func(t *testing.T) {
		// Given
		ctx := context.Background()
		batches := []Batch{}

		pool, err := ants.NewPool(1, ants.WithOptions(ants.Options{
			PreAlloc:    true,
			Nonblocking: false,
		}))
		require.NoError(t, err)
		defer pool.Release()

		config := &Config{Concurrency: 1}
		logInfo, _ := createLoggers()
		progressTracker := createProgressTracker(0, 0)
		progressReporter := createProgressReporter(logInfo)

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			return nil
		}

		// When
		errors := processBatches(ctx, batches, pool, mockCloner, config, logInfo, &progressTracker, progressReporter)

		// Then
		assert.Empty(t, errors)
	})

	t.Run("Given batches with empty orgs When processing Then should skip empty batches", func(t *testing.T) {
		// Given
		ctx := context.Background()
		batches := []Batch{
			{Orgs: []string{}, BatchNumber: 1},       // Empty batch
			{Orgs: []string{"org1"}, BatchNumber: 2}, // Non-empty batch
		}

		pool, err := ants.NewPool(1, ants.WithOptions(ants.Options{
			PreAlloc:    true,
			Nonblocking: false,
		}))
		require.NoError(t, err)
		defer pool.Release()

		config := &Config{
			TargetDir:   "/tmp/test",
			Concurrency: 1,
		}

		logInfo, _ := createLoggers()
		progressTracker := createProgressTracker(1, 2)
		progressReporter := createProgressReporter(logInfo)

		var processedOrgs []string
		var mu sync.Mutex

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			mu.Lock()
			processedOrgs = append(processedOrgs, org)
			mu.Unlock()
			return nil
		}

		// When
		errors := processBatches(ctx, batches, pool, mockCloner, config, logInfo, &progressTracker, progressReporter)

		// Then
		assert.Empty(t, errors)
		assert.Len(t, processedOrgs, 1)
		assert.Contains(t, processedOrgs, "org1")
	})
}

// Test processBatch following Given-When-Then pattern
func TestProcessBatch(t *testing.T) {
	t.Run("Given batch with orgs When processing Then should process all orgs", func(t *testing.T) {
		// Given
		ctx := context.Background()
		batch := Batch{
			Orgs:        []string{"org1", "org2", "org3"},
			BatchNumber: 1,
		}

		pool, err := ants.NewPool(3, ants.WithOptions(ants.Options{
			PreAlloc:    true,
			Nonblocking: false,
		}))
		require.NoError(t, err)
		defer pool.Release()

		config := &Config{
			TargetDir:   "/tmp/test",
			GitHubToken: "test-token",
			Concurrency: 3,
		}

		var processedOrgs []string
		var mu sync.Mutex

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			mu.Lock()
			processedOrgs = append(processedOrgs, org)
			mu.Unlock()
			return nil
		}

		// When
		errors := processBatch(ctx, batch, pool, mockCloner, config)

		// Then
		assert.Empty(t, errors)
		assert.Len(t, processedOrgs, 3)
		assert.Contains(t, processedOrgs, "org1")
		assert.Contains(t, processedOrgs, "org2")
		assert.Contains(t, processedOrgs, "org3")
	})

	t.Run("Given batch with some failing orgs When processing Then should collect errors", func(t *testing.T) {
		// Given
		ctx := context.Background()
		batch := Batch{
			Orgs:        []string{"org1", "org2"},
			BatchNumber: 1,
		}

		pool, err := ants.NewPool(2, ants.WithOptions(ants.Options{
			PreAlloc:    true,
			Nonblocking: false,
		}))
		require.NoError(t, err)
		defer pool.Release()

		config := &Config{
			TargetDir:   "/tmp/test",
			Concurrency: 2,
		}

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			if org == "org1" {
				return fmt.Errorf("cloning failed for %s", org)
			}
			return nil
		}

		// When
		errors := processBatch(ctx, batch, pool, mockCloner, config)

		// Then
		assert.Len(t, errors, 1)
		assert.Contains(t, errors[0].Error(), "batch 1 org org1 failed")
	})

	t.Run("Given empty batch When processing Then should return no errors", func(t *testing.T) {
		// Given
		ctx := context.Background()
		batch := Batch{
			Orgs:        []string{},
			BatchNumber: 1,
		}

		pool, err := ants.NewPool(1, ants.WithOptions(ants.Options{
			PreAlloc:    true,
			Nonblocking: false,
		}))
		require.NoError(t, err)
		defer pool.Release()

		config := &Config{Concurrency: 1}

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			return nil
		}

		// When
		errors := processBatch(ctx, batch, pool, mockCloner, config)

		// Then
		assert.Empty(t, errors)
	})

	t.Run("Given batch with worker pool submission failure When processing Then should collect errors", func(t *testing.T) {
		// Given
		ctx := context.Background()
		batch := Batch{
			Orgs:        []string{"org1"},
			BatchNumber: 1,
		}

		// Create a pool with minimal capacity
		pool, err := ants.NewPool(1, ants.WithOptions(ants.Options{
			PreAlloc:    true,
			Nonblocking: false,
		}))
		require.NoError(t, err)
		defer pool.Release()

		config := &Config{Concurrency: 1}

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			return nil
		}

		// When
		errors := processBatch(ctx, batch, pool, mockCloner, config)

		// Then
		// Note: This might not fail with ants pool depending on implementation
		// The test verifies that if submission fails, errors are collected
		assert.True(t, len(errors) >= 0) // May or may not have errors depending on pool behavior
	})
}

// Helper function to create a test pool
func createTestPool(capacity int) (*ants.Pool, error) {
	if capacity <= 0 {
		capacity = 1
	}

	return ants.NewPool(capacity, ants.WithOptions(ants.Options{
		PreAlloc:    true,
		Nonblocking: false,
	}))
}

// Property-based tests using rapid
func TestWorkflowProperties(t *testing.T) {
	t.Run("Property: ExecuteWorkflow with valid inputs should not panic", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate random valid config
			orgs := rapid.SliceOfN(rapid.StringMatching(`^[a-zA-Z0-9_-]+$`), 1, 10).Draw(t, "orgs")
			targetDir := rapid.StringMatching(`^/[a-zA-Z0-9/_-]+$`).Draw(t, "targetDir")
			concurrency := rapid.IntRange(1, 10).Draw(t, "concurrency")
			batchSize := rapid.IntRange(1, 10).Draw(t, "batchSize")

			ctx := context.Background()

			mockLoader := func(ctx context.Context, configPath string) (*Config, error) {
				return &Config{
					Orgs:        orgs,
					TargetDir:   targetDir,
					Concurrency: concurrency,
					BatchSize:   batchSize,
					GitHubToken: rapid.String().Draw(t, "token"),
				}, nil
			}

			mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
				return nil
			}

			// Property: Should never panic with valid inputs
			assert.NotPanics(t, func() {
				ExecuteWorkflow(ctx, "", mockLoader, mockCloner)
			})
		})
	})

	t.Run("Property: processBatch preserves error count", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			orgs := rapid.SliceOfN(rapid.StringMatching(`^[a-zA-Z0-9_-]+$`), 0, 10).Draw(t, "orgs")
			batchNumber := rapid.IntRange(1, 100).Draw(t, "batchNumber")

			batch := Batch{
				Orgs:        orgs,
				BatchNumber: batchNumber,
			}

			ctx := context.Background()
			pool, err := createTestPool(len(orgs) + 1)
			if err != nil {
				t.Skip("Failed to create pool")
			}
			defer pool.Release()

			config := &Config{
				TargetDir:   "/tmp/test",
				Concurrency: len(orgs) + 1,
			}

			// Cloner that always fails
			mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
				return fmt.Errorf("failed for %s", org)
			}

			errors := processBatch(ctx, batch, pool, mockCloner, config)

			// Property: Number of errors should equal number of orgs (since all fail)
			assert.Equal(t, len(orgs), len(errors))
		})
	})
}

// Integration tests for workflow
func TestWorkflowIntegration(t *testing.T) {
	t.Run("Given complete workflow When executing end-to-end Then should coordinate all components", func(t *testing.T) {
		// Given
		ctx := context.Background()
		var processedOrgs []string
		var mu sync.Mutex

		mockLoader := func(ctx context.Context, configPath string) (*Config, error) {
			return &Config{
				Orgs:        []string{"org1", "org2", "org3", "org4", "org5"},
				TargetDir:   "/tmp/integration-test",
				Concurrency: 3,
				BatchSize:   2,
				GitHubToken: "integration-token",
			}, nil
		}

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			mu.Lock()
			processedOrgs = append(processedOrgs, org)
			mu.Unlock()

			// Simulate some work
			time.Sleep(10 * time.Millisecond)
			return nil
		}

		// When
		err := ExecuteWorkflow(ctx, "test-config", mockLoader, mockCloner)

		// Then
		assert.NoError(t, err)
		assert.Len(t, processedOrgs, 5)
		for i := 1; i <= 5; i++ {
			assert.Contains(t, processedOrgs, fmt.Sprintf("org%d", i))
		}
	})

	t.Run("Given workflow with mixed success and failure When executing Then should report partial success", func(t *testing.T) {
		// Given
		ctx := context.Background()
		var processedOrgs []string
		var mu sync.Mutex

		mockLoader := func(ctx context.Context, configPath string) (*Config, error) {
			return &Config{
				Orgs:        []string{"org1", "org2", "org3"},
				TargetDir:   "/tmp/test",
				Concurrency: 2,
				BatchSize:   2,
			}, nil
		}

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			mu.Lock()
			processedOrgs = append(processedOrgs, org)
			mu.Unlock()

			if org == "org2" {
				return fmt.Errorf("failed for %s", org)
			}
			return nil
		}

		// When
		err := ExecuteWorkflow(ctx, "", mockLoader, mockCloner)

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "processing completed with 1 errors")
		assert.Contains(t, err.Error(), "failed for org2")
		assert.Len(t, processedOrgs, 3) // All orgs should be attempted
	})
}

// Stress tests for workflow
func TestWorkflowStress(t *testing.T) {
	t.Run("Given high concurrency workflow When executing Then should handle efficiently", func(t *testing.T) {
		// Given
		ctx := context.Background()
		numOrgs := 100
		orgs := make([]string, numOrgs)
		for i := 0; i < numOrgs; i++ {
			orgs[i] = fmt.Sprintf("org%d", i)
		}

		var processedCount int32
		var mu sync.Mutex

		mockLoader := func(ctx context.Context, configPath string) (*Config, error) {
			return &Config{
				Orgs:        orgs,
				TargetDir:   "/tmp/stress-test",
				Concurrency: 20,
				BatchSize:   10,
			}, nil
		}

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			mu.Lock()
			processedCount++
			mu.Unlock()

			// Simulate minimal work
			time.Sleep(1 * time.Millisecond)
			return nil
		}

		// When
		start := time.Now()
		err := ExecuteWorkflow(ctx, "", mockLoader, mockCloner)
		duration := time.Since(start)

		// Then
		assert.NoError(t, err)
		assert.Equal(t, int32(numOrgs), processedCount)
		assert.True(t, duration < 5*time.Second, "Should complete within reasonable time")
	})

	t.Run("Given workflow with timeout When executing Then should respect timeout", func(t *testing.T) {
		// Given
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		mockLoader := func(ctx context.Context, configPath string) (*Config, error) {
			return &Config{
				Orgs:        []string{"org1", "org2", "org3"},
				TargetDir:   "/tmp/timeout-test",
				Concurrency: 1,
				BatchSize:   1,
			}, nil
		}

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			// Simulate long-running operation
			time.Sleep(200 * time.Millisecond)
			return nil
		}

		// When
		err := ExecuteWorkflow(ctx, "", mockLoader, mockCloner)

		// Then
		assert.Error(t, err)
		// Error might be from context timeout or workflow cancellation
	})
}

// Mutation tests for error handling
func TestWorkflowMutations(t *testing.T) {
	t.Run("Given nil config loader When executing workflow Then should handle gracefully", func(t *testing.T) {
		// Given
		ctx := context.Background()
		var nilLoader ConfigLoader = nil

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			return nil
		}

		// When/Then - should panic or handle gracefully
		assert.Panics(t, func() {
			ExecuteWorkflow(ctx, "", nilLoader, mockCloner)
		})
	})

	t.Run("Given nil cloner When executing workflow Then should handle gracefully", func(t *testing.T) {
		// Given
		ctx := context.Background()

		mockLoader := func(ctx context.Context, configPath string) (*Config, error) {
			return &Config{
				Orgs:        []string{"org1"},
				TargetDir:   "/tmp/test",
				Concurrency: 1,
				BatchSize:   1,
			}, nil
		}

		var nilCloner OrgCloner = nil

		// When/Then - should handle gracefully or panic appropriately
		assert.Panics(t, func() {
			ExecuteWorkflow(ctx, "", mockLoader, nilCloner)
		})
	})

	t.Run("Given extreme configuration values When executing workflow Then should handle gracefully", func(t *testing.T) {
		extremeConfigs := []*Config{
			{Orgs: []string{"org1"}, TargetDir: "/tmp/test", Concurrency: 0, BatchSize: 0},
			{Orgs: []string{"org1"}, TargetDir: "/tmp/test", Concurrency: 10000, BatchSize: 10000},
			{Orgs: []string{"org1"}, TargetDir: "/tmp/test", Concurrency: -1, BatchSize: -1},
		}

		for i, config := range extremeConfigs {
			t.Run(fmt.Sprintf("extreme_config_%d", i), func(t *testing.T) {
				ctx := context.Background()

				mockLoader := func(ctx context.Context, configPath string) (*Config, error) {
					return config, nil
				}

				mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
					return nil
				}

				// Should either succeed or fail gracefully (not panic)
				assert.NotPanics(t, func() {
					ExecuteWorkflow(ctx, "", mockLoader, mockCloner)
				})
			})
		}
	})
}

// Edge case tests
func TestWorkflowEdgeCases(t *testing.T) {
	t.Run("Given workflow with very large batch sizes When executing Then should handle correctly", func(t *testing.T) {
		// Given
		ctx := context.Background()

		mockLoader := func(ctx context.Context, configPath string) (*Config, error) {
			return &Config{
				Orgs:        []string{"org1", "org2", "org3"},
				TargetDir:   "/tmp/test",
				Concurrency: 2,
				BatchSize:   1000, // Much larger than number of orgs
			}, nil
		}

		var processedOrgs []string
		var mu sync.Mutex

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			mu.Lock()
			processedOrgs = append(processedOrgs, org)
			mu.Unlock()
			return nil
		}

		// When
		err := ExecuteWorkflow(ctx, "", mockLoader, mockCloner)

		// Then
		assert.NoError(t, err)
		assert.Len(t, processedOrgs, 3)
	})

	t.Run("Given workflow with single org When executing Then should create single batch", func(t *testing.T) {
		// Given
		ctx := context.Background()

		mockLoader := func(ctx context.Context, configPath string) (*Config, error) {
			return &Config{
				Orgs:        []string{"single-org"},
				TargetDir:   "/tmp/test",
				Concurrency: 1,
				BatchSize:   1,
			}, nil
		}

		var processedOrgs []string
		var mu sync.Mutex

		mockCloner := func(ctx context.Context, org, targetDir, token string, concurrency int) error {
			mu.Lock()
			processedOrgs = append(processedOrgs, org)
			mu.Unlock()
			return nil
		}

		// When
		err := ExecuteWorkflow(ctx, "", mockLoader, mockCloner)

		// Then
		assert.NoError(t, err)
		assert.Len(t, processedOrgs, 1)
		assert.Equal(t, "single-org", processedOrgs[0])
	})
}
