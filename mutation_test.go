package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// Mutation tests for testing code behavior under modified conditions
// These tests verify error handling and recovery scenarios as per CLAUDE.md

// Mutation Test 1: Config validation with corrupted data
func TestConfigValidationMutations(t *testing.T) {
	t.Run("Config mutation - nil pointer safety", func(t *testing.T) {
		// Mutation: Test with nil config
		var config *Config = nil

		// Should panic or handle gracefully
		assert.Panics(t, func() {
			config.ValidateAndSetDefaults()
		})
	})

	t.Run("Config mutation - extreme negative values", func(t *testing.T) {
		// Mutation: Use extreme negative values
		config := &Config{
			Orgs:        []string{"org1"},
			TargetDir:   "/tmp/test",
			Concurrency: -999999,
			BatchSize:   -999999,
		}

		// Should handle gracefully and set defaults
		err := config.ValidateAndSetDefaults()
		assert.NoError(t, err)
		assert.True(t, config.Concurrency > 0)
		assert.True(t, config.BatchSize > 0)
	})

	t.Run("Config mutation - very large values", func(t *testing.T) {
		// Mutation: Use extremely large values
		config := &Config{
			Orgs:        []string{"org1"},
			TargetDir:   "/tmp/test",
			Concurrency: 999999999,
			BatchSize:   999999999,
		}

		// Should handle large values without error (no capping for positive values)
		err := config.ValidateAndSetDefaults()
		assert.NoError(t, err)
		assert.Equal(t, 999999999, config.Concurrency) // Large values are preserved
		assert.Equal(t, 999999999, config.BatchSize)   // Large values are preserved
	})

	t.Run("Config mutation - zero length after trimming", func(t *testing.T) {
		// Mutation: Strings that become empty after trimming
		config := &Config{
			Orgs:      []string{"   ", "\t\n", ""},
			TargetDir: "   \t\n   ",
		}

		// Should fail validation appropriately
		err := config.ValidateAndSetDefaults()
		assert.Error(t, err)
	})
}

// Mutation Test 2: Environment config loading with various corruptions
func TestEnvConfigLoaderMutations(t *testing.T) {
	t.Run("EnvLoader mutation - malformed env file", func(t *testing.T) {
		// Mutation: Create malformed .env file
		tmpDir := t.TempDir()
		envFile := filepath.Join(tmpDir, ".env")
		malformedContent := `GHORG_TARGET_DIR=/tmp/test
GHORG_ORGS=org1,org2
INVALID_LINE_WITHOUT_EQUALS_OR_VALUE
GHORG_CONCURRENCY
=INVALID_KEY_STARTING_WITH_EQUALS
GHORG_BATCH_SIZE=invalid_number_value
MULTI
LINE
VALUE`

		require.NoError(t, os.WriteFile(envFile, []byte(malformedContent), 0644))

		ctx := context.Background()
		loader := CreateEnvConfigLoader()

		// Should handle malformed file gracefully
		_, err := loader(ctx, envFile)

		// May succeed or fail depending on how env parser handles malformed data
		// The key is that it shouldn't panic
		_ = err // Use err to avoid unused variable warning
		assert.NotPanics(t, func() {
			loader(ctx, envFile)
		})
	})

	t.Run("EnvLoader mutation - invalid numeric values", func(t *testing.T) {
		// Mutation: Invalid numeric values in env
		tmpDir := t.TempDir()
		envFile := filepath.Join(tmpDir, ".env")
		invalidContent := `GHORG_TARGET_DIR=/tmp/test
GHORG_ORGS=org1,org2
GHORG_CONCURRENCY=not_a_number
GHORG_BATCH_SIZE=3.14159`

		require.NoError(t, os.WriteFile(envFile, []byte(invalidContent), 0644))

		ctx := context.Background()
		loader := CreateEnvConfigLoader()

		// Should handle invalid numbers gracefully (likely use defaults)
		config, err := loader(ctx, envFile)

		if err == nil {
			// If parsing succeeded, numeric fields should have valid defaults
			assert.True(t, config.Concurrency >= 0)
			assert.True(t, config.BatchSize >= 0)
		}
	})

	t.Run("EnvLoader mutation - context deadline exceeded", func(t *testing.T) {
		// Mutation: Very short context deadline
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
		defer cancel()

		loader := CreateEnvConfigLoader()

		// Should handle expired context
		_, err := loader(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config loading cancelled")
	})
}

// Mutation Test 3: Utility function mutations
func TestUtilityMutationTesting(t *testing.T) {
	t.Run("BoundedBuffer mutation - concurrent access corruption", func(t *testing.T) {
		// Mutation: Concurrent access without proper synchronization test
		buffer := NewBoundedBuffer(10)

		// Simulate concurrent writes (buffer should handle this safely)
		done := make(chan bool)
		for i := 0; i < 5; i++ {
			go func(id int) {
				for j := 0; j < 100; j++ {
					buffer.Write(fmt.Sprintf("goroutine%d-line%d", id, j))
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 5; i++ {
			<-done
		}

		// Buffer should still be in valid state
		result := buffer.GetAll()
		assert.True(t, len(result) <= 10) // Should not exceed capacity
	})

	t.Run("ChunkSlice mutation - integer overflow scenarios", func(t *testing.T) {
		// Mutation: Test with values that could cause integer overflow
		largeSlice := make([]string, 1000000)
		for i := range largeSlice {
			largeSlice[i] = fmt.Sprintf("item%d", i)
		}

		// Should handle large slices without overflow
		result := chunkSlice(largeSlice, 100000)

		// Verify no data loss occurred
		totalItems := 0
		for _, chunk := range result {
			totalItems += len(chunk)
		}
		assert.Equal(t, len(largeSlice), totalItems)
	})

	t.Run("PathExpansion mutation - malicious path inputs", func(t *testing.T) {
		// Mutation: Test with potentially malicious paths
		maliciousPaths := []string{
			"~/../../../etc/passwd",
			"~/../../../../",
			"~/../.ssh/id_rsa",
			"~/../../root",
			"~/../../..",
		}

		for _, path := range maliciousPaths {
			// Should handle malicious paths safely
			result := expandPath(path, "/home/user")

			// Key test: function should not panic and should return deterministic result
			assert.NotPanics(t, func() {
				expandPath(path, "/home/user")
			})

			// Result should be predictable (just string manipulation)
			if strings.HasPrefix(path, "~/") {
				expected := "/home/user" + path[1:]
				assert.Equal(t, expected, result)
			} else {
				assert.Equal(t, path, result)
			}
		}
	})

	t.Run("CommandBuilding mutation - injection attempts", func(t *testing.T) {
		// Mutation: Test with command injection attempts
		injectionAttempts := []struct {
			org, targetDir, token string
			concurrency           int
		}{
			{"org; rm -rf /", "/tmp/test", "token", 1},
			{"org", "/tmp/test; cat /etc/passwd", "token", 1},
			{"org", "/tmp/test", "token; whoami", 1},
			{"org`whoami`", "/tmp/test", "token", 1},
			{"org$(whoami)", "/tmp/test", "token", 1},
		}

		for _, attempt := range injectionAttempts {
			// Should build command safely without executing injection
			result := buildCommand(attempt.org, attempt.targetDir, attempt.token, attempt.concurrency)

			// Should not panic
			assert.NotPanics(t, func() {
				buildCommand(attempt.org, attempt.targetDir, attempt.token, attempt.concurrency)
			})

			// Result should be a valid command slice
			assert.True(t, len(result) > 0)
			assert.Equal(t, "clone", result[0])
		}
	})
}

// Mutation Test 4: Progress tracking mutations
func TestProgressMutationTesting(t *testing.T) {
	t.Run("ProgressTracker mutation - time manipulation", func(t *testing.T) {
		// Mutation: Manipulate time to test edge cases
		tracker := ProgressTracker{
			TotalOrgs:     10,
			ProcessedOrgs: 5,
			StartTime:     time.Now().Add(-24 * time.Hour), // Very old start time
		}

		// Should handle extreme time differences
		update := calculateProgress(tracker)

		// Should not panic or produce invalid results
		assert.True(t, update.ElapsedTime > 0)
		assert.True(t, update.EstimatedETA >= 0)
		assert.Equal(t, 5, update.CompletedOrgs)
		assert.Equal(t, 10, update.TotalOrgs)
	})

	t.Run("ProgressTracker mutation - division by zero scenarios", func(t *testing.T) {
		// Mutation: Create scenarios that could cause division by zero
		trackers := []ProgressTracker{
			{TotalOrgs: 0, ProcessedOrgs: 0, StartTime: time.Now()},
			{TotalOrgs: 10, ProcessedOrgs: 0, StartTime: time.Now()},
			{TotalOrgs: 0, ProcessedOrgs: 5, StartTime: time.Now()}, // More processed than total
		}

		for i, tracker := range trackers {
			t.Run(fmt.Sprintf("scenario_%d", i), func(t *testing.T) {
				// Should not panic with edge case values
				assert.NotPanics(t, func() {
					calculateProgress(tracker)
				})

				update := calculateProgress(tracker)
				// Should produce valid results even with edge cases
				assert.True(t, update.EstimatedETA >= 0)
			})
		}
	})
}

// Mutation Test 5: Workflow execution mutations
func TestWorkflowExecutionMutations(t *testing.T) {
	t.Run("Workflow mutation - resource exhaustion", func(t *testing.T) {
		// Mutation: Test with resource-exhausting configuration
		config := &Config{
			Orgs:        make([]string, 10000), // Very large number of orgs
			TargetDir:   "/tmp/test",
			Concurrency: 10000, // Very high concurrency
			BatchSize:   10000, // Very large batch size
		}

		// Fill orgs
		for i := 0; i < 10000; i++ {
			config.Orgs[i] = fmt.Sprintf("org%d", i)
		}

		// Should handle resource exhaustion gracefully
		err := config.ValidateAndSetDefaults()
		assert.NoError(t, err)

		// Should cap values to reasonable limits
		assert.True(t, config.Concurrency <= 25)
		assert.True(t, config.BatchSize <= 50)
	})

	t.Run("Workflow mutation - nil function parameters", func(t *testing.T) {
		// Mutation: Test with nil function parameters
		ctx := context.Background()

		// Should handle nil functions appropriately (panic or error)
		assert.Panics(t, func() {
			ExecuteWorkflow(ctx, "", nil, CreateGhorgOrgCloner())
		})

		assert.Panics(t, func() {
			ExecuteWorkflow(ctx, "", CreateEnvConfigLoader(), nil)
		})
	})
}

// Mutation Test 6: Property-based mutation testing
func TestMutationProperties(t *testing.T) {
	t.Run("Property mutation: Functions maintain invariants under corruption", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate random valid inputs
			orgs := rapid.SliceOfN(rapid.String().Filter(func(s string) bool {
				return len(strings.TrimSpace(s)) > 0
			}), 1, 100).Draw(t, "orgs")

			// Count valid orgs before mutation
			originalValidCount := 0
			for _, org := range orgs {
				if strings.TrimSpace(org) != "" {
					originalValidCount++
				}
			}

			// Mutate one random element to empty/whitespace
			if len(orgs) > 1 {
				corruptIndex := rapid.IntRange(0, len(orgs)-1).Draw(t, "corruptIndex")
				orgs[corruptIndex] = "   " // Corrupt with whitespace
			}

			batchSize := rapid.IntRange(1, 1000).Draw(t, "batchSize")

			// Function should handle corrupted input gracefully
			batches := createBatches(orgs, batchSize)

			// Property: createBatches doesn't filter - it preserves all input
			// (filtering happens at config validation level)
			actualOrgs := 0
			for _, batch := range batches {
				actualOrgs += len(batch.Orgs)
			}

			// Should preserve all orgs (including corrupted ones)
			assert.Equal(t, len(orgs), actualOrgs)
		})
	})

	t.Run("Property mutation: Path expansion safety under malformed input", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate potentially malformed paths
			path := rapid.String().Draw(t, "path")
			homeDir := rapid.String().Draw(t, "homeDir")

			// Function should never panic regardless of input
			assert.NotPanics(t, func() {
				expandPath(path, homeDir)
			})

			// Result should be deterministic
			result1 := expandPath(path, homeDir)
			result2 := expandPath(path, homeDir)
			assert.Equal(t, result1, result2)
		})
	})
}

// Mutation Test 7: Error injection and fault tolerance
func TestErrorInjectionMutations(t *testing.T) {
	t.Run("Error injection - simulated system failures", func(t *testing.T) {
		// Mutation: Inject errors at various points to test resilience

		// Test config loading with simulated file system errors
		ctx := context.Background()
		loader := CreateEnvConfigLoader()

		// Test with non-existent file (simulated FS error)
		_, err := loader(ctx, "/definitely/does/not/exist/file.env")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load .env file")

		// Test with permission denied scenario (create file without read permission)
		tmpDir := t.TempDir()
		restrictedFile := filepath.Join(tmpDir, "restricted.env")
		require.NoError(t, os.WriteFile(restrictedFile, []byte("GHORG_TARGET_DIR=/tmp/test\nGHORG_ORGS=org1"), 0000))

		_, err = loader(ctx, restrictedFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load .env file")
	})

	t.Run("Error injection - memory pressure simulation", func(t *testing.T) {
		// Mutation: Create conditions that simulate memory pressure

		// Create very large string slices to test memory handling
		largeData := make([]string, 100000)
		for i := range largeData {
			largeData[i] = strings.Repeat("x", 1000) // Large strings
		}

		// Functions should handle large data without panic
		assert.NotPanics(t, func() {
			chunkSlice(largeData, 1000)
		})

		// Test BoundedBuffer with rapid insertions
		buffer := NewBoundedBuffer(1000)
		assert.NotPanics(t, func() {
			for i := 0; i < 100000; i++ {
				buffer.Write(fmt.Sprintf("line%d-%s", i, strings.Repeat("x", 100)))
			}
		})
	})
}

// Mutation Test 8: Concurrency and race condition testing
func TestConcurrencyMutations(t *testing.T) {
	t.Run("Concurrency mutation - race condition detection", func(t *testing.T) {
		// Mutation: Create scenarios likely to trigger race conditions

		numGoroutines := 100
		iterations := 1000

		// Test BoundedBuffer concurrent access
		buffer := NewBoundedBuffer(100)
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				for j := 0; j < iterations; j++ {
					buffer.Write(fmt.Sprintf("goroutine%d-iter%d", id, j))

					// Randomly read as well to create read/write contention
					if j%10 == 0 {
						buffer.GetAll()
					}
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Buffer should be in consistent state
		result := buffer.GetAll()
		assert.True(t, len(result) <= 100)
	})

	t.Run("Concurrency mutation - progress tracking race conditions", func(t *testing.T) {
		// Mutation: Test progress tracking under concurrent updates
		tracker := createProgressTracker(1000, 10)

		// Mock progress reporter that could be called concurrently
		var reportCount int
		progressReporter := func(update ProgressUpdate) {
			reportCount++
			// Simulate work
			time.Sleep(1 * time.Microsecond)
		}

		// Simulate concurrent progress reporting
		numReporters := 50
		done := make(chan bool, numReporters)

		for i := 0; i < numReporters; i++ {
			go func(id int) {
				for j := 0; j < 20; j++ {
					update := calculateProgress(tracker)
					update.Org = fmt.Sprintf("org%d-%d", id, j)
					progressReporter(update)
				}
				done <- true
			}(i)
		}

		// Wait for all reporters
		for i := 0; i < numReporters; i++ {
			<-done
		}

		// Should have processed all reports without panic
		assert.True(t, reportCount > 0)
	})
}

// Mutation Test 9: Input validation bypass attempts
func TestInputValidationMutations(t *testing.T) {
	t.Run("Validation mutation - bypass attempts", func(t *testing.T) {
		// Mutation: Attempt to bypass validation with crafted inputs

		bypassAttempts := []*Config{
			// Attempt 1: Unicode whitespace that might not be caught by strings.TrimSpace
			{
				Orgs:      []string{"\u00A0", "\u2000", "\u2001"}, // Non-breaking spaces
				TargetDir: "\u00A0\u2000\u2001",
			},
			// Attempt 2: Very long strings that might cause buffer overflows
			{
				Orgs:      []string{strings.Repeat("a", 100000)},
				TargetDir: strings.Repeat("b", 100000),
			},
			// Attempt 3: Null bytes that might interfere with string processing
			{
				Orgs:      []string{"org\x00hidden"},
				TargetDir: "/tmp/test\x00hidden",
			},
		}

		for i, config := range bypassAttempts {
			t.Run(fmt.Sprintf("bypass_attempt_%d", i), func(t *testing.T) {
				// Validation should not panic
				assert.NotPanics(t, func() {
					config.ValidateAndSetDefaults()
				})

				// Should either succeed with cleaned input or fail appropriately
				err := config.ValidateAndSetDefaults()
				if err == nil {
					// If validation passed, values should be reasonable
					assert.True(t, len(config.TargetDir) > 0)
					if len(config.Orgs) > 0 {
						for _, org := range config.Orgs {
							assert.True(t, len(org) > 0)
						}
					}
				}
			})
		}
	})
}
