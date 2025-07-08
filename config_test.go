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

// Test Config.ValidateAndSetDefaults following Given-When-Then pattern
func TestConfigValidateAndSetDefaults(t *testing.T) {
	t.Run("Given valid config When calling ValidateAndSetDefaults Then should succeed", func(t *testing.T) {
		// Given
		config := &Config{
			Orgs:        []string{"org1", "org2"},
			BatchSize:   50,
			Concurrency: 10,
			TargetDir:   "/tmp/repos",
			GitHubToken: "token123",
		}

		// When
		err := config.ValidateAndSetDefaults()

		// Then
		assert.NoError(t, err)
		assert.Equal(t, []string{"org1", "org2"}, config.Orgs)
		assert.Equal(t, 50, config.BatchSize)
		assert.Equal(t, 10, config.Concurrency)
		assert.Equal(t, "/tmp/repos", config.TargetDir)
	})

	t.Run("Given empty target directory When calling ValidateAndSetDefaults Then should fail", func(t *testing.T) {
		// Given
		config := &Config{
			Orgs:      []string{"org1"},
			TargetDir: "",
		}

		// When
		err := config.ValidateAndSetDefaults()

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "target_dir is required")
	})

	t.Run("Given whitespace-only target directory When calling ValidateAndSetDefaults Then should fail", func(t *testing.T) {
		// Given
		config := &Config{
			Orgs:      []string{"org1"},
			TargetDir: "   ",
		}

		// When
		err := config.ValidateAndSetDefaults()

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "target_dir is required")
	})

	t.Run("Given empty orgs When calling ValidateAndSetDefaults Then should fail", func(t *testing.T) {
		// Given
		config := &Config{
			Orgs:      []string{},
			TargetDir: "/tmp/repos",
		}

		// When
		err := config.ValidateAndSetDefaults()

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one organization must be specified")
	})

	t.Run("Given orgs with whitespace When calling ValidateAndSetDefaults Then should trim whitespace", func(t *testing.T) {
		// Given
		config := &Config{
			Orgs:      []string{"  org1  ", "org2", "  ", "org3  "},
			TargetDir: "/tmp/repos",
		}

		// When
		err := config.ValidateAndSetDefaults()

		// Then
		assert.NoError(t, err)
		assert.Equal(t, []string{"org1", "org2", "org3"}, config.Orgs)
	})

	t.Run("Given zero concurrency When calling ValidateAndSetDefaults Then should set default", func(t *testing.T) {
		// Given
		config := &Config{
			Orgs:        []string{"org1", "org2"},
			TargetDir:   "/tmp/repos",
			Concurrency: 0,
		}

		// When
		err := config.ValidateAndSetDefaults()

		// Then
		assert.NoError(t, err)
		assert.Equal(t, 2, config.Concurrency) // min(25, max(1, len(orgs)))
	})

	t.Run("Given negative concurrency When calling ValidateAndSetDefaults Then should set default", func(t *testing.T) {
		// Given
		config := &Config{
			Orgs:        []string{"org1", "org2"},
			TargetDir:   "/tmp/repos",
			Concurrency: -5,
		}

		// When
		err := config.ValidateAndSetDefaults()

		// Then
		assert.NoError(t, err)
		assert.Equal(t, 2, config.Concurrency)
	})

	t.Run("Given zero batch size When calling ValidateAndSetDefaults Then should set default", func(t *testing.T) {
		// Given
		config := &Config{
			Orgs:      []string{"org1", "org2"},
			TargetDir: "/tmp/repos",
			BatchSize: 0,
		}

		// When
		err := config.ValidateAndSetDefaults()

		// Then
		assert.NoError(t, err)
		assert.Equal(t, 2, config.BatchSize) // min(50, max(1, len(orgs)))
	})

	t.Run("Given large number of orgs When calling ValidateAndSetDefaults Then should cap concurrency and batch size", func(t *testing.T) {
		// Given
		orgs := make([]string, 100)
		for i := 0; i < 100; i++ {
			orgs[i] = fmt.Sprintf("org%d", i)
		}
		config := &Config{
			Orgs:        orgs,
			TargetDir:   "/tmp/repos",
			Concurrency: 0,
			BatchSize:   0,
		}

		// When
		err := config.ValidateAndSetDefaults()

		// Then
		assert.NoError(t, err)
		assert.Equal(t, 25, config.Concurrency) // min(25, max(1, len(orgs)))
		assert.Equal(t, 50, config.BatchSize)   // min(50, max(1, len(orgs)))
	})
}

// Test processConfig following Given-When-Then pattern
func TestProcessConfig(t *testing.T) {
	t.Run("Given valid config When calling processConfig Then should create processing plan", func(t *testing.T) {
		// Given
		config := &Config{
			Orgs:        []string{"org1", "org2", "org3"},
			BatchSize:   2,
			Concurrency: 5,
			TargetDir:   "/tmp/repos",
		}

		// When
		plan, err := processConfig(config)

		// Then
		assert.NoError(t, err)
		assert.Equal(t, 3, plan.TotalOrgs)
		assert.Len(t, plan.Batches, 2)
		assert.Equal(t, []string{"org1", "org2"}, plan.Batches[0].Orgs)
		assert.Equal(t, []string{"org3"}, plan.Batches[1].Orgs)
		assert.Equal(t, 1, plan.Batches[0].BatchNumber)
		assert.Equal(t, 2, plan.Batches[1].BatchNumber)
	})

	t.Run("Given invalid config When calling processConfig Then should fail", func(t *testing.T) {
		// Given
		config := &Config{
			Orgs:      []string{},
			TargetDir: "/tmp/repos",
		}

		// When
		_, err := processConfig(config)

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one organization must be specified")
	})
}

// Test CreateEnvConfigLoader following Given-When-Then pattern
func TestCreateEnvConfigLoader(t *testing.T) {
	t.Run("Given valid env file When calling config loader Then should load config", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		envFile := filepath.Join(tmpDir, ".env")
		envContent := `GHORG_TARGET_DIR=/tmp/test
GHORG_ORGS=org1,org2
GHORG_CONCURRENCY=10
GHORG_BATCH_SIZE=25
GHORG_GITHUB_TOKEN=test-token`

		require.NoError(t, os.WriteFile(envFile, []byte(envContent), 0644))

		ctx := context.Background()
		loader := CreateEnvConfigLoader()

		// When
		config, err := loader(ctx, envFile)

		// Then
		assert.NoError(t, err)
		assert.Equal(t, "/tmp/test", config.TargetDir)
		assert.Equal(t, []string{"org1", "org2"}, config.Orgs)
		assert.Equal(t, 10, config.Concurrency)
		assert.Equal(t, 25, config.BatchSize)
		assert.Equal(t, "test-token", config.GitHubToken)
	})

	t.Run("Given cancelled context When calling config loader Then should return error", func(t *testing.T) {
		// Given
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		loader := CreateEnvConfigLoader()

		// When
		_, err := loader(ctx, "")

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config loading cancelled")
	})

	t.Run("Given context with timeout When calling config loader Then should respect timeout", func(t *testing.T) {
		// Given
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(2 * time.Nanosecond) // Ensure timeout
		loader := CreateEnvConfigLoader()

		// When
		_, err := loader(ctx, "")

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config loading cancelled")
	})

	t.Run("Given non-existent env file When calling config loader Then should return error", func(t *testing.T) {
		// Given
		ctx := context.Background()
		loader := CreateEnvConfigLoader()

		// When
		_, err := loader(ctx, "/non/existent/file")

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load .env file")
	})

	t.Run("Given env without required fields When calling config loader Then should return error", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		envFile := filepath.Join(tmpDir, ".env")
		envContent := `GHORG_CONCURRENCY=10` // Missing required GHORG_TARGET_DIR and GHORG_ORGS

		require.NoError(t, os.WriteFile(envFile, []byte(envContent), 0644))

		// Clear required env variables to ensure they fail
		originalTargetDir := os.Getenv("GHORG_TARGET_DIR")
		originalOrgs := os.Getenv("GHORG_ORGS")
		defer func() {
			if originalTargetDir != "" {
				os.Setenv("GHORG_TARGET_DIR", originalTargetDir)
			} else {
				os.Unsetenv("GHORG_TARGET_DIR")
			}
			if originalOrgs != "" {
				os.Setenv("GHORG_ORGS", originalOrgs)
			} else {
				os.Unsetenv("GHORG_ORGS")
			}
		}()
		os.Unsetenv("GHORG_TARGET_DIR")
		os.Unsetenv("GHORG_ORGS")

		ctx := context.Background()
		loader := CreateEnvConfigLoader()

		// When
		_, err := loader(ctx, envFile)

		// Then
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "failed to parse environment variables")
		}
	})

	t.Run("Given empty env file path When calling config loader Then should use env variables", func(t *testing.T) {
		// Given
		originalEnvs := map[string]string{
			"GHORG_TARGET_DIR":   os.Getenv("GHORG_TARGET_DIR"),
			"GHORG_ORGS":         os.Getenv("GHORG_ORGS"),
			"GHORG_CONCURRENCY":  os.Getenv("GHORG_CONCURRENCY"),
			"GHORG_BATCH_SIZE":   os.Getenv("GHORG_BATCH_SIZE"),
			"GHORG_GITHUB_TOKEN": os.Getenv("GHORG_GITHUB_TOKEN"),
		}

		// Set test environment variables
		os.Setenv("GHORG_TARGET_DIR", "/tmp/test")
		os.Setenv("GHORG_ORGS", "org1,org2")
		os.Setenv("GHORG_CONCURRENCY", "5")
		os.Setenv("GHORG_BATCH_SIZE", "10")
		os.Setenv("GHORG_GITHUB_TOKEN", "test-token")

		// Restore original environment after test
		defer func() {
			for key, value := range originalEnvs {
				if value == "" {
					os.Unsetenv(key)
				} else {
					os.Setenv(key, value)
				}
			}
		}()

		ctx := context.Background()
		loader := CreateEnvConfigLoader()

		// When
		config, err := loader(ctx, "")

		// Then
		assert.NoError(t, err)
		assert.Equal(t, "/tmp/test", config.TargetDir)
		assert.Equal(t, []string{"org1", "org2"}, config.Orgs)
		assert.Equal(t, 5, config.Concurrency)
		assert.Equal(t, 10, config.BatchSize)
		assert.Equal(t, "test-token", config.GitHubToken)
	})
}

// Test convertEnvConfigToConfig following Given-When-Then pattern
func TestConvertEnvConfigToConfig(t *testing.T) {
	t.Run("Given simple env config When converting Then should create correct config", func(t *testing.T) {
		// Given
		envConfig := &EnvConfig{
			TargetDir:   "/tmp/test",
			Orgs:        []string{"org1", "org2"},
			Concurrency: 10,
			GitHubToken: "test-token",
			BatchSize:   25,
		}

		// When
		config := convertEnvConfigToConfig(envConfig)

		// Then
		assert.Equal(t, "/tmp/test", config.TargetDir)
		assert.Equal(t, []string{"org1", "org2"}, config.Orgs)
		assert.Equal(t, 10, config.Concurrency)
		assert.Equal(t, "test-token", config.GitHubToken)
		assert.Equal(t, 25, config.BatchSize)
	})

	t.Run("Given env config with comma-separated orgs When converting Then should split properly", func(t *testing.T) {
		// Given
		envConfig := &EnvConfig{
			TargetDir: "/tmp/test",
			Orgs:      []string{"org1,org2,org3"},
		}

		// When
		config := convertEnvConfigToConfig(envConfig)

		// Then
		assert.Equal(t, []string{"org1", "org2", "org3"}, config.Orgs)
	})

	t.Run("Given env config with mixed formats When converting Then should handle correctly", func(t *testing.T) {
		// Given
		envConfig := &EnvConfig{
			TargetDir: "/tmp/test",
			Orgs:      []string{"org1", "org2,org3", "  org4  ", ""},
		}

		// When
		config := convertEnvConfigToConfig(envConfig)

		// Then
		assert.Equal(t, []string{"org1", "org2", "org3", "org4"}, config.Orgs)
	})

	t.Run("Given env config with whitespace in comma-separated orgs When converting Then should trim whitespace", func(t *testing.T) {
		// Given
		envConfig := &EnvConfig{
			TargetDir: "/tmp/test",
			Orgs:      []string{"  org1  ,  org2  ,  org3  "},
		}

		// When
		config := convertEnvConfigToConfig(envConfig)

		// Then
		assert.Equal(t, []string{"org1", "org2", "org3"}, config.Orgs)
	})

	t.Run("Given env config with empty orgs When converting Then should handle gracefully", func(t *testing.T) {
		// Given
		envConfig := &EnvConfig{
			TargetDir: "/tmp/test",
			Orgs:      []string{"", "  ", ",,,"},
		}

		// When
		config := convertEnvConfigToConfig(envConfig)

		// Then
		assert.Equal(t, []string{}, config.Orgs)
	})
}

// Property-based tests using rapid
func TestConfigValidationProperties(t *testing.T) {
	t.Run("Property: Valid config should always validate successfully", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate valid config
			orgs := rapid.SliceOfN(rapid.String().Filter(func(s string) bool {
				return strings.TrimSpace(s) != ""
			}), 1, 10).Draw(t, "orgs")

			targetDir := rapid.String().Filter(func(s string) bool {
				return strings.TrimSpace(s) != ""
			}).Draw(t, "targetDir")

			concurrency := rapid.IntRange(1, 100).Draw(t, "concurrency")
			batchSize := rapid.IntRange(1, 100).Draw(t, "batchSize")

			config := &Config{
				Orgs:        orgs,
				TargetDir:   targetDir,
				Concurrency: concurrency,
				BatchSize:   batchSize,
			}

			err := config.ValidateAndSetDefaults()
			assert.NoError(t, err)
		})
	})

	t.Run("Property: Config with empty target dir should always fail", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			orgs := rapid.SliceOfN(rapid.String().Filter(func(s string) bool {
				return strings.TrimSpace(s) != ""
			}), 1, 10).Draw(t, "orgs")

			config := &Config{
				Orgs:      orgs,
				TargetDir: "",
			}

			err := config.ValidateAndSetDefaults()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "target_dir is required")
		})
	})

	t.Run("Property: Config with no valid orgs should always fail", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate orgs that are all empty or whitespace
			orgs := rapid.SliceOfN(rapid.String().Filter(func(s string) bool {
				return strings.TrimSpace(s) == ""
			}), 0, 10).Draw(t, "orgs")

			targetDir := rapid.String().Filter(func(s string) bool {
				return strings.TrimSpace(s) != ""
			}).Draw(t, "targetDir")

			config := &Config{
				Orgs:      orgs,
				TargetDir: targetDir,
			}

			err := config.ValidateAndSetDefaults()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "at least one organization must be specified")
		})
	})
}

// Integration tests for config loading
func TestConfigLoadingIntegration(t *testing.T) {
	t.Run("Given complete workflow When loading config Then should handle all steps", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		envFile := filepath.Join(tmpDir, ".env")
		envContent := `GHORG_TARGET_DIR=/tmp/test
GHORG_ORGS=org1,org2
GHORG_CONCURRENCY=10
GHORG_BATCH_SIZE=25
GHORG_GITHUB_TOKEN=test-token`

		require.NoError(t, os.WriteFile(envFile, []byte(envContent), 0644))

		ctx := context.Background()
		loader := CreateEnvConfigLoader()

		// When
		config, err := loader(ctx, envFile)
		require.NoError(t, err)

		plan, err := processConfig(config)

		// Then
		assert.NoError(t, err)
		assert.Equal(t, "/tmp/test", config.TargetDir)
		assert.Equal(t, []string{"org1", "org2"}, config.Orgs)
		assert.Equal(t, 10, config.Concurrency)
		assert.Equal(t, 25, config.BatchSize)
		assert.Equal(t, "test-token", config.GitHubToken)

		// Verify processing plan
		assert.Equal(t, 2, plan.TotalOrgs)
		assert.Len(t, plan.Batches, 1)
		assert.Equal(t, []string{"org1", "org2"}, plan.Batches[0].Orgs)
	})
}

// Stress tests for concurrent config loading
func TestConfigLoadingStress(t *testing.T) {
	t.Run("Given concurrent config loading When multiple goroutines load config Then should handle gracefully", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		envFile := filepath.Join(tmpDir, ".env")
		envContent := `GHORG_TARGET_DIR=/tmp/stress-test
GHORG_ORGS=org1,org2
GHORG_CONCURRENCY=10
GHORG_BATCH_SIZE=5`

		require.NoError(t, os.WriteFile(envFile, []byte(envContent), 0644))

		loader := CreateEnvConfigLoader()
		numGoroutines := 10
		results := make(chan error, numGoroutines)

		// When
		for i := 0; i < numGoroutines; i++ {
			go func() {
				ctx := context.Background()
				_, err := loader(ctx, envFile)
				results <- err
			}()
		}

		// Then
		for i := 0; i < numGoroutines; i++ {
			err := <-results
			assert.NoError(t, err)
		}
	})
}

// Mutation tests for error handling
func TestConfigMutationTests(t *testing.T) {
	t.Run("Given corrupted env file When loading config Then should handle gracefully", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		envFile := filepath.Join(tmpDir, ".env")
		corruptedContent := `GHORG_TARGET_DIR=/tmp/test
GHORG_ORGS=org1,org2
INVALID_LINE_WITHOUT_EQUALS
GHORG_CONCURRENCY=not-a-number`

		require.NoError(t, os.WriteFile(envFile, []byte(corruptedContent), 0644))

		ctx := context.Background()
		loader := CreateEnvConfigLoader()

		// When
		_, err := loader(ctx, envFile)

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse environment variables")
	})

	t.Run("Given file permission error When loading config Then should handle gracefully", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		envFile := filepath.Join(tmpDir, ".env")
		envContent := `GHORG_TARGET_DIR=/tmp/test
GHORG_ORGS=org1,org2`

		require.NoError(t, os.WriteFile(envFile, []byte(envContent), 0644))
		require.NoError(t, os.Chmod(envFile, 0000)) // Remove all permissions

		ctx := context.Background()
		loader := CreateEnvConfigLoader()

		// When
		_, err := loader(ctx, envFile)

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load .env file")
	})
}

// Edge case tests
func TestConfigEdgeCases(t *testing.T) {
	t.Run("Given config with maximum values When validating Then should handle correctly", func(t *testing.T) {
		// Given
		orgs := make([]string, 1000)
		for i := 0; i < 1000; i++ {
			orgs[i] = fmt.Sprintf("org%d", i)
		}

		config := &Config{
			Orgs:        orgs,
			TargetDir:   "/tmp/massive-test",
			Concurrency: 1000,
			BatchSize:   1000,
		}

		// When
		err := config.ValidateAndSetDefaults()

		// Then
		assert.NoError(t, err)
		assert.Equal(t, 25, config.Concurrency) // Should be capped at 25
		assert.Equal(t, 50, config.BatchSize)   // Should be capped at 50
	})

	t.Run("Given config with unicode org names When validating Then should handle correctly", func(t *testing.T) {
		// Given
		config := &Config{
			Orgs:      []string{"组织", "संगठन", "منظمة", "組織"},
			TargetDir: "/tmp/unicode-test",
		}

		// When
		err := config.ValidateAndSetDefaults()

		// Then
		assert.NoError(t, err)
		assert.Equal(t, []string{"组织", "संगठन", "منظمة", "組織"}, config.Orgs)
	})
}
