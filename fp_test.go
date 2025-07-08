package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// DRY: Single comprehensive test file covering all functionality

func TestConfigValidation(t *testing.T) {
	t.Run("validateConfig succeeds with valid config", func(t *testing.T) {
		config := &Config{
			Orgs:        []string{"org1", "org2"},
			BatchSize:   50,
			Concurrency: 10,
			TargetDir:   "/tmp/repos",
			GitHubToken: "token123",
		}

		err := validateConfig(config)
		assert.NoError(t, err)
	})

	t.Run("validateConfig fails with empty orgs", func(t *testing.T) {
		config := &Config{
			Orgs:      []string{},
			TargetDir: "/tmp/repos",
		}

		err := validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no valid organizations")
	})

	t.Run("validateConfig fails with empty target dir", func(t *testing.T) {
		config := &Config{
			Orgs:      []string{"org1"},
			TargetDir: "",
		}

		err := validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "target directory cannot be empty")
	})
}

func TestBatchCreation(t *testing.T) {
	t.Run("createBatches creates correct batches", func(t *testing.T) {
		orgs := []string{"org1", "org2", "org3", "org4", "org5"}
		batches := createBatches(orgs, 2)

		assert.Equal(t, 3, len(batches))
		assert.Equal(t, []string{"org1", "org2"}, batches[0].Orgs)
		assert.Equal(t, []string{"org3", "org4"}, batches[1].Orgs)
		assert.Equal(t, []string{"org5"}, batches[2].Orgs)
		assert.Equal(t, 1, batches[0].BatchNumber)
		assert.Equal(t, 2, batches[1].BatchNumber)
		assert.Equal(t, 3, batches[2].BatchNumber)
	})

	t.Run("createBatches handles default batch size", func(t *testing.T) {
		orgs := []string{"org1", "org2"}
		batches := createBatches(orgs, 0) // Should default to 50

		assert.Equal(t, 1, len(batches))
		assert.Equal(t, orgs, batches[0].Orgs)
	})
}

func TestProcessConfig(t *testing.T) {
	t.Run("processConfig succeeds with valid config", func(t *testing.T) {
		config := &Config{
			Orgs:        []string{"org1", "org2", "org3"},
			BatchSize:   2,
			Concurrency: 10,
			TargetDir:   "/tmp/repos",
			GitHubToken: "token123",
		}

		plan, err := processConfig(config)
		require.NoError(t, err)
		assert.Equal(t, 3, plan.TotalOrgs)
		assert.Equal(t, 2, len(plan.Batches))
	})

	t.Run("processConfig fails with invalid config", func(t *testing.T) {
		config := &Config{
			Orgs:      []string{},
			TargetDir: "/tmp/repos",
		}

		_, err := processConfig(config)
		assert.Error(t, err)
	})
}

func TestCommandBuilding(t *testing.T) {
	t.Run("buildCommand creates correct command with token", func(t *testing.T) {
		cmd := buildCommand("myorg", "/tmp/repos", "token123", 5)
		expected := []string{"clone", "myorg", "--path", "/tmp/repos", "--concurrency", "5", "--token", "token123"}
		assert.Equal(t, expected, cmd)
	})

	t.Run("buildCommand creates correct command without token", func(t *testing.T) {
		cmd := buildCommand("myorg", "/tmp/repos", "", 5)
		expected := []string{"clone", "myorg", "--path", "/tmp/repos", "--concurrency", "5", "--no-token"}
		assert.Equal(t, expected, cmd)
	})
}

func TestPathExpansion(t *testing.T) {
	t.Run("expandPath expands home directory", func(t *testing.T) {
		result := expandPath("~/documents", "/home/user")
		assert.Equal(t, "/home/user/documents", result)

		result = expandPath("~/", "/home/user")
		assert.Equal(t, "/home/user", result)
	})

	t.Run("expandPath returns unchanged for non-home paths", func(t *testing.T) {
		result := expandPath("/absolute/path", "/home/user")
		assert.Equal(t, "/absolute/path", result)

		result = expandPath("relative/path", "/home/user")
		assert.Equal(t, "relative/path", result)
	})
}

func TestDirectFunctionUsage(t *testing.T) {
	t.Run("processConfig direct function works", func(t *testing.T) {
		config := &Config{
			Orgs:        []string{"org1", "org2"},
			BatchSize:   2,
			Concurrency: 10,
			TargetDir:   "/tmp/repos",
			GitHubToken: "token123",
		}

		plan, err := processConfig(config)
		require.NoError(t, err)
		assert.Equal(t, 2, plan.TotalOrgs)
	})

	t.Run("buildCommand direct function works", func(t *testing.T) {
		args := buildCommand("myorg", "/tmp/repos", "token123", 5)
		expected := []string{"clone", "myorg", "--path", "/tmp/repos", "--concurrency", "5", "--token", "token123"}
		assert.Equal(t, expected, args)
	})
}

func TestChunkSliceEdgeCases(t *testing.T) {
	t.Run("chunkSlice with empty slice", func(t *testing.T) {
		result := chunkSlice([]string{}, 2)
		assert.Equal(t, [][]string{}, result)
	})

	t.Run("chunkSlice with size larger than slice", func(t *testing.T) {
		input := []string{"a", "b"}
		result := chunkSlice(input, 5)
		expected := [][]string{{"a", "b"}}
		assert.Equal(t, expected, result)
	})

	t.Run("chunkSlice with size 1", func(t *testing.T) {
		input := []string{"a", "b", "c"}
		result := chunkSlice(input, 1)
		expected := [][]string{{"a"}, {"b"}, {"c"}}
		assert.Equal(t, expected, result)
	})

	t.Run("chunkSlice perfect division", func(t *testing.T) {
		input := []string{"a", "b", "c", "d"}
		result := chunkSlice(input, 2)
		expected := [][]string{{"a", "b"}, {"c", "d"}}
		assert.Equal(t, expected, result)
	})
}

func TestPureFunctionProperties(t *testing.T) {
	t.Run("functions are deterministic", func(t *testing.T) {
		// Same inputs should always produce same outputs
		config := &Config{
			Orgs:      []string{"org1", "org2", "org3"},
			BatchSize: 2,
			TargetDir: "/tmp/test",
		}

		plan1, err1 := processConfig(config)
		plan2, err2 := processConfig(config)

		assert.Equal(t, err1, err2)
		assert.Equal(t, plan1, plan2)
	})

	t.Run("functions do not mutate inputs", func(t *testing.T) {
		orgs := []string{"org1", "org2", "org3"}
		originalOrgs := make([]string, len(orgs))
		copy(originalOrgs, orgs)

		// Call function that processes orgs
		batches := createBatches(orgs, 2)

		// Original slice should be unchanged
		assert.Equal(t, originalOrgs, orgs)

		// But we should have batches
		assert.Equal(t, 2, len(batches))
	})

	t.Run("functions create new data structures", func(t *testing.T) {
		orgs := []string{"org1", "org2"}
		batches := createBatches(orgs, 1)

		// Modify returned batch
		batches[0].Orgs[0] = "modified"

		// Original should be unchanged
		assert.Equal(t, "org1", orgs[0])
	})
}

// CONTRACT TESTING: Interface compliance verification

// TABLE-DRIVEN TESTS
func TestConfigValidationTableDriven(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &Config{
				Orgs:        []string{"org1", "org2"},
				BatchSize:   50,
				Concurrency: 10,
				TargetDir:   "/tmp/repos",
				GitHubToken: "token123",
			},
			wantErr: false,
		},
		{
			name: "empty orgs",
			config: &Config{
				Orgs:      []string{},
				TargetDir: "/tmp/repos",
			},
			wantErr: true,
			errMsg:  "no valid organizations",
		},
		{
			name: "empty target dir",
			config: &Config{
				Orgs:      []string{"org1"},
				TargetDir: "",
			},
			wantErr: true,
			errMsg:  "target directory cannot be empty",
		},
		{
			name: "nil orgs",
			config: &Config{
				Orgs:      nil,
				TargetDir: "/tmp/repos",
			},
			wantErr: true,
			errMsg:  "no valid organizations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBatchCreationTableDriven(t *testing.T) {
	tests := []struct {
		name           string
		orgs           []string
		batchSize      int
		expectedLen    int
		expectedBatch0 []string
		expectedBatch1 []string
	}{
		{
			name:           "normal batching",
			orgs:           []string{"org1", "org2", "org3", "org4", "org5"},
			batchSize:      2,
			expectedLen:    3,
			expectedBatch0: []string{"org1", "org2"},
			expectedBatch1: []string{"org3", "org4"},
		},
		{
			name:           "single org",
			orgs:           []string{"org1"},
			batchSize:      5,
			expectedLen:    1,
			expectedBatch0: []string{"org1"},
		},
		{
			name:           "default batch size",
			orgs:           []string{"org1", "org2"},
			batchSize:      0,
			expectedLen:    1,
			expectedBatch0: []string{"org1", "org2"},
		},
		{
			name:           "batch size 1",
			orgs:           []string{"org1", "org2", "org3"},
			batchSize:      1,
			expectedLen:    3,
			expectedBatch0: []string{"org1"},
			expectedBatch1: []string{"org2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batches := createBatches(tt.orgs, tt.batchSize)
			assert.Equal(t, tt.expectedLen, len(batches))
			if len(batches) > 0 {
				assert.Equal(t, tt.expectedBatch0, batches[0].Orgs)
			}
			if len(batches) > 1 && tt.expectedBatch1 != nil {
				assert.Equal(t, tt.expectedBatch1, batches[1].Orgs)
			}
		})
	}
}

func TestCommandBuildingTableDriven(t *testing.T) {
	tests := []struct {
		name        string
		org         string
		targetDir   string
		token       string
		concurrency int
		expected    []string
	}{
		{
			name:        "with token",
			org:         "myorg",
			targetDir:   "/tmp/repos",
			token:       "token123",
			concurrency: 5,
			expected:    []string{"clone", "myorg", "--path", "/tmp/repos", "--concurrency", "5", "--token", "token123"},
		},
		{
			name:        "without token",
			org:         "myorg",
			targetDir:   "/tmp/repos",
			token:       "",
			concurrency: 5,
			expected:    []string{"clone", "myorg", "--path", "/tmp/repos", "--concurrency", "5", "--no-token"},
		},
		{
			name:        "high concurrency",
			org:         "testorg",
			targetDir:   "/home/user/repos",
			token:       "abc123",
			concurrency: 100,
			expected:    []string{"clone", "testorg", "--path", "/home/user/repos", "--concurrency", "100", "--token", "abc123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := buildCommand(tt.org, tt.targetDir, tt.token, tt.concurrency)
			assert.Equal(t, tt.expected, cmd)
		})
	}
}

func TestPathExpansionTableDriven(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		homeDir  string
		expected string
	}{
		{
			name:     "expand home with path",
			path:     "~/documents",
			homeDir:  "/home/user",
			expected: "/home/user/documents",
		},
		{
			name:     "expand home root",
			path:     "~/",
			homeDir:  "/home/user",
			expected: "/home/user",
		},
		{
			name:     "absolute path unchanged",
			path:     "/absolute/path",
			homeDir:  "/home/user",
			expected: "/absolute/path",
		},
		{
			name:     "relative path unchanged",
			path:     "relative/path",
			homeDir:  "/home/user",
			expected: "relative/path",
		},
		{
			name:     "tilde not at start",
			path:     "/path/~/file",
			homeDir:  "/home/user",
			expected: "/path/~/file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.path, tt.homeDir)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// CONTRACT TESTING: Interface compliance testing with functional types
func TestFunctionalContract(t *testing.T) {
	t.Run("functional implementations satisfy interfaces", func(t *testing.T) {
		// Test that our functional implementations satisfy the function types
		var _ ConfigLoader = CreateEnvConfigLoader()
		var _ OrgCloner = CreateGhorgOrgCloner()

		// Test functional creation
		loader := CreateEnvConfigLoader()
		cloner := CreateGhorgOrgCloner()

		assert.NotNil(t, loader)
		assert.NotNil(t, cloner)
	})
}

// INTEGRATION TESTS
func TestFunctionalIntegration(t *testing.T) {
	t.Run("functional components work together", func(t *testing.T) {
		// Test actual integration without external dependencies
		loader := CreateEnvConfigLoader()
		cloner := CreateGhorgOrgCloner()

		assert.NotNil(t, loader)
		assert.NotNil(t, cloner)

		// Test that the functions can be called (will fail at ghorg command)
		ctx := context.Background()
		_, err := loader(ctx, "")
		assert.Error(t, err) // Expected to fail without env vars
	})
}

// API-BASED TESTS: Testing environment configuration
func TestEnvConfigLoader(t *testing.T) {
	t.Run("loads config from environment", func(t *testing.T) {
		// Set environment variables for testing
		os.Setenv("GHORG_TARGET_DIR", "/tmp/test")
		os.Setenv("GHORG_ORGS", "org1,org2")
		os.Setenv("GHORG_CONCURRENCY", "15")
		os.Setenv("GHORG_BATCH_SIZE", "30")
		os.Setenv("GHORG_GITHUB_TOKEN", "test-token")

		defer func() {
			os.Unsetenv("GHORG_TARGET_DIR")
			os.Unsetenv("GHORG_ORGS")
			os.Unsetenv("GHORG_CONCURRENCY")
			os.Unsetenv("GHORG_BATCH_SIZE")
			os.Unsetenv("GHORG_GITHUB_TOKEN")
		}()

		loader := NewEnvConfigLoader()
		config, err := loader.LoadConfig(context.Background(), "")

		require.NoError(t, err)
		assert.Equal(t, "/tmp/test", config.TargetDir)
		assert.Equal(t, []string{"org1", "org2"}, config.Orgs)
		assert.Equal(t, 15, config.Concurrency)
		assert.Equal(t, 30, config.BatchSize)
		assert.Equal(t, "test-token", config.GitHubToken)
	})

	t.Run("handles missing required environment variables", func(t *testing.T) {
		loader := NewEnvConfigLoader()
		_, err := loader.LoadConfig(context.Background(), "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse environment variables")
	})
}

func TestGhorgOrgCloner(t *testing.T) {
	t.Run("creates target directory", func(t *testing.T) {
		ctx := context.Background()
		cloner := NewGhorgOrgCloner()

		// Use a temporary directory for testing
		tempDir := "/tmp/test-ghorg-cloner"
		defer os.RemoveAll(tempDir)

		// This will fail because ghorg is not installed, but we can test directory creation
		err := cloner.CloneOrg(ctx, "testorg", tempDir, "token", 5)

		// Directory should be created even if command fails
		_, statErr := os.Stat(tempDir)
		assert.NoError(t, statErr)

		// Command should fail since ghorg is not installed
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ghorg command failed")
	})
}

func TestMainValidation(t *testing.T) {
	t.Run("config validation and defaults", func(t *testing.T) {
		config := &Config{
			Orgs:        []string{"org1", "org2"},
			BatchSize:   0,
			Concurrency: 0,
			TargetDir:   "/tmp/test",
			GitHubToken: "token",
		}

		err := config.ValidateAndSetDefaults()

		assert.NoError(t, err)
		assert.Equal(t, 2, config.Concurrency) // Scales with org count
		assert.Equal(t, 2, config.BatchSize)   // Scales with org count
	})

	t.Run("config validation fails appropriately", func(t *testing.T) {
		config := &Config{
			Orgs:      []string{},
			TargetDir: "",
		}

		err := config.ValidateAndSetDefaults()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "target_dir is required")
	})
}

// PROPERTY-BASED TESTS USING RAPID
func TestChunkSlicePropertiesRapid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random slice of strings and chunk size
		input := rapid.SliceOf(rapid.String()).Draw(t, "input")
		size := rapid.IntRange(1, 100).Draw(t, "size")

		chunks := chunkSlice[string](input, size)

		// Property 1: All elements are preserved
		totalElements := 0
		for _, chunk := range chunks {
			totalElements += len(chunk)
		}
		if totalElements != len(input) {
			t.Fatalf("chunkSlice lost elements: input=%d, output=%d", len(input), totalElements)
		}

		// Property 2: No chunk exceeds the specified size (except when size is 0, defaults to 50)
		expectedSize := size
		if size <= 0 {
			expectedSize = 50
		}
		for i, chunk := range chunks {
			if len(chunk) > expectedSize {
				t.Fatalf("chunk %d exceeds size limit: %d > %d", i, len(chunk), expectedSize)
			}
		}

		// Property 3: All chunks except the last should be at maximum size
		if len(chunks) > 1 {
			for i := 0; i < len(chunks)-1; i++ {
				if len(chunks[i]) != expectedSize {
					t.Fatalf("non-final chunk %d is not at max size: %d != %d", i, len(chunks[i]), expectedSize)
				}
			}
		}

		// Property 4: Elements maintain their relative order
		reconstructed := make([]string, 0, len(input))
		for _, chunk := range chunks {
			reconstructed = append(reconstructed, chunk...)
		}
		for i, elem := range input {
			if reconstructed[i] != elem {
				t.Fatalf("element order not preserved at index %d: %s != %s", i, reconstructed[i], elem)
			}
		}
	})
}

func TestConfigValidationPropertiesRapid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random config with valid fields
		orgs := rapid.SliceOfN(rapid.StringMatching(`^[a-zA-Z0-9_-]+$`), 1, 10).Draw(t, "orgs")
		targetDir := rapid.StringMatching(`^/[a-zA-Z0-9/_-]+$`).Draw(t, "targetDir")
		batchSize := rapid.IntRange(1, 1000).Draw(t, "batchSize")
		concurrency := rapid.IntRange(1, 100).Draw(t, "concurrency")

		config := &Config{
			Orgs:        orgs,
			BatchSize:   batchSize,
			Concurrency: concurrency,
			TargetDir:   targetDir,
			GitHubToken: rapid.String().Draw(t, "token"),
		}

		// Property: Valid configs should always pass validation
		err := validateConfig(config)
		if err != nil {
			t.Fatalf("valid config failed validation: %v", err)
		}
	})
}

func TestConfigValidationFailurePropertiesRapid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate configs that should fail validation
		choice := rapid.IntRange(0, 1).Draw(t, "failureType")

		config := &Config{
			BatchSize:   rapid.IntRange(1, 100).Draw(t, "batchSize"),
			Concurrency: rapid.IntRange(1, 100).Draw(t, "concurrency"),
			GitHubToken: rapid.String().Draw(t, "token"),
		}

		switch choice {
		case 0:
			// Empty orgs should fail
			config.Orgs = []string{}
			config.TargetDir = "/valid/path"
		case 1:
			// Empty target dir should fail
			config.Orgs = rapid.SliceOfN(rapid.StringMatching(`^[a-zA-Z0-9_-]+$`), 1, 10).Draw(t, "orgs")
			config.TargetDir = ""
		}

		// Property: Invalid configs should always fail validation
		err := validateConfig(config)
		if err == nil {
			t.Fatalf("invalid config passed validation: %+v", config)
		}
	})
}

func TestBatchCreationPropertiesRapid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random input
		orgs := rapid.SliceOf(rapid.String()).Draw(t, "orgs")
		batchSize := rapid.IntRange(0, 200).Draw(t, "batchSize")

		batches := createBatches(orgs, batchSize)

		// Property 1: All orgs are included
		totalOrgs := 0
		for _, batch := range batches {
			totalOrgs += len(batch.Orgs)
		}
		if totalOrgs != len(orgs) {
			t.Fatalf("batch creation lost orgs: input=%d, output=%d", len(orgs), totalOrgs)
		}

		// Property 2: Batch numbers are sequential starting from 1
		for i, batch := range batches {
			if batch.BatchNumber != i+1 {
				t.Fatalf("batch number incorrect: expected %d, got %d", i+1, batch.BatchNumber)
			}
		}

		// Property 3: No mutation of input slice
		originalOrgs := rapid.SliceOf(rapid.String()).Draw(t, "originalOrgs")
		copy(originalOrgs, orgs)
		createBatches(originalOrgs, batchSize)
		for i, org := range orgs {
			if i < len(originalOrgs) && originalOrgs[i] != org {
				t.Fatalf("input slice was mutated")
			}
		}
	})
}

func TestCommandBuildingPropertiesRapid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random command parameters
		org := rapid.String().Draw(t, "org")
		targetDir := rapid.String().Draw(t, "targetDir")
		token := rapid.String().Draw(t, "token")
		concurrency := rapid.IntRange(1, 1000).Draw(t, "concurrency")

		cmd := buildCommand(org, targetDir, token, concurrency)

		// Property 1: Command always starts with "clone"
		if len(cmd) == 0 || cmd[0] != "clone" {
			t.Fatalf("command doesn't start with 'clone': %v", cmd)
		}

		// Property 2: Org is always second parameter (after trimming)
		expectedOrg := strings.TrimSpace(org)
		if len(cmd) < 2 || cmd[1] != expectedOrg {
			t.Fatalf("org not in second position, expected %q, got %q in: %v", expectedOrg, cmd[1], cmd)
		}

		// Property 3: Contains token or no-token flag
		hasToken := false
		hasNoToken := false
		for _, arg := range cmd {
			if arg == "--token" {
				hasToken = true
			}
			if arg == "--no-token" {
				hasNoToken = true
			}
		}

		trimmedToken := strings.TrimSpace(token)
		if trimmedToken == "" && !hasNoToken {
			t.Fatalf("empty/whitespace token should result in --no-token flag: %v", cmd)
		}
		if trimmedToken != "" && !hasToken {
			t.Fatalf("non-empty token should result in --token flag: %v", cmd)
		}
		if hasToken && hasNoToken {
			t.Fatalf("command has both --token and --no-token: %v", cmd)
		}

		// Property 4: Contains path parameter (after trimming)
		expectedTargetDir := strings.TrimSpace(targetDir)
		hasPath := false
		for i := 0; i < len(cmd)-1; i++ {
			if cmd[i] == "--path" && cmd[i+1] == expectedTargetDir {
				hasPath = true
				break
			}
		}
		if !hasPath {
			t.Fatalf("command missing --path parameter with value %q: %v", expectedTargetDir, cmd)
		}
	})
}

func TestPathExpansionPropertiesRapid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		homeDir := rapid.StringMatching(`^/[a-zA-Z0-9/_-]*$`).Draw(t, "homeDir")
		path := rapid.String().Draw(t, "path")

		result := expandPath(path, homeDir)

		// Property 1: Tilde expansion works correctly
		if len(path) >= 2 && path[:2] == "~/" {
			if path == "~/" {
				if result != homeDir {
					t.Fatalf("~/ not expanded to homeDir: %s != %s", result, homeDir)
				}
			} else {
				expected := homeDir + "/" + path[2:]
				if result != expected {
					t.Fatalf("tilde expansion incorrect: %s != %s", result, expected)
				}
			}
		} else {
			// Property 2: Non-tilde paths remain unchanged
			if result != path {
				t.Fatalf("non-tilde path was modified: %s != %s", result, path)
			}
		}
	})
}

func TestProcessConfigPropertiesRapid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate valid config
		orgs := rapid.SliceOfN(rapid.StringMatching(`^[a-zA-Z0-9_-]+$`), 1, 100).Draw(t, "orgs")
		batchSize := rapid.IntRange(1, 50).Draw(t, "batchSize")
		config := &Config{
			Orgs:        orgs,
			BatchSize:   batchSize,
			Concurrency: rapid.IntRange(1, 100).Draw(t, "concurrency"),
			TargetDir:   rapid.StringMatching(`^/[a-zA-Z0-9/_-]+$`).Draw(t, "targetDir"),
			GitHubToken: rapid.String().Draw(t, "token"),
		}

		plan, err := processConfig(config)

		// Property 1: Valid config should not error
		if err != nil {
			t.Fatalf("processConfig failed on valid config: %v", err)
		}

		// Property 2: Total orgs should match input
		if plan.TotalOrgs != len(orgs) {
			t.Fatalf("total orgs mismatch: %d != %d", plan.TotalOrgs, len(orgs))
		}

		// Property 3: All orgs should be in batches
		totalInBatches := 0
		for _, batch := range plan.Batches {
			totalInBatches += len(batch.Orgs)
		}
		if totalInBatches != len(orgs) {
			t.Fatalf("orgs in batches mismatch: %d != %d", totalInBatches, len(orgs))
		}

		// Property 4: Expected number of batches
		expectedBatches := (len(orgs) + batchSize - 1) / batchSize
		if len(plan.Batches) != expectedBatches {
			t.Fatalf("unexpected number of batches: %d != %d", len(plan.Batches), expectedBatches)
		}
	})
}

// PROPERTY-BASED TESTS (legacy for comparison)
func TestChunkSliceProperties(t *testing.T) {
	t.Run("chunk slice maintains all elements", func(t *testing.T) {
		testCases := []struct {
			input []string
			size  int
		}{
			{[]string{"a", "b", "c", "d", "e"}, 2},
			{[]string{"1", "2", "3"}, 1},
			{[]string{"x", "y"}, 5},
			{[]string{"single"}, 1},
		}

		for _, tc := range testCases {
			chunks := chunkSlice(tc.input, tc.size)

			// Count total elements in all chunks
			total := 0
			for _, chunk := range chunks {
				total += len(chunk)
			}

			assert.Equal(t, len(tc.input), total, "chunkSlice should preserve all elements")
		}
	})
}

// CONTEXT CANCELLATION TESTS
func TestContextCancellation(t *testing.T) {
	t.Run("config loading with cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		loader := NewEnvConfigLoader()
		_, err := loader.LoadConfig(ctx, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config loading cancelled")
	})

	t.Run("clone org with cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		cloner := NewGhorgOrgCloner()
		err := cloner.CloneOrg(ctx, "testorg", "/tmp/test", "token", 5)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "clone operation cancelled")
	})
}

// ENVIRONMENT VARIABLE EDGE CASES
func TestEnvironmentVariableEdgeCases(t *testing.T) {
	t.Run("comma-separated orgs conversion", func(t *testing.T) {
		envConfig := &EnvConfig{
			TargetDir:   "/tmp/test",
			Orgs:        []string{"org1,org2,org3", "org4"},
			Concurrency: 5,
			BatchSize:   10,
		}

		config := convertEnvConfigToConfig(envConfig)

		assert.Equal(t, []string{"org1", "org2", "org3", "org4"}, config.Orgs)
	})

	t.Run("comma-separated orgs with whitespace", func(t *testing.T) {
		envConfig := &EnvConfig{
			TargetDir:   "/tmp/test",
			Orgs:        []string{" org1 , org2 , org3 ", "  org4  "},
			Concurrency: 5,
			BatchSize:   10,
		}

		config := convertEnvConfigToConfig(envConfig)

		assert.Equal(t, []string{"org1", "org2", "org3", "org4"}, config.Orgs)
	})

	t.Run("comma-separated orgs with empty parts", func(t *testing.T) {
		envConfig := &EnvConfig{
			TargetDir:   "/tmp/test",
			Orgs:        []string{"org1,,org2", ",org3,", ""},
			Concurrency: 5,
			BatchSize:   10,
		}

		config := convertEnvConfigToConfig(envConfig)

		assert.Equal(t, []string{"org1", "org2", "org3"}, config.Orgs)
	})

	t.Run("empty org strings", func(t *testing.T) {
		envConfig := &EnvConfig{
			TargetDir:   "/tmp/test",
			Orgs:        []string{"", "   ", "org1"},
			Concurrency: 5,
			BatchSize:   10,
		}

		config := convertEnvConfigToConfig(envConfig)

		assert.Equal(t, []string{"org1"}, config.Orgs)
	})
}

// VALIDATION EDGE CASES
func TestValidationEdgeCases(t *testing.T) {
	t.Run("config validation with whitespace-only target dir", func(t *testing.T) {
		config := &Config{
			Orgs:      []string{"org1"},
			TargetDir: "   ",
		}

		err := config.ValidateAndSetDefaults()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "target_dir is required")
	})

	t.Run("config validation with empty orgs after validation", func(t *testing.T) {
		config := &Config{
			Orgs:      []string{},
			TargetDir: "/tmp/test",
		}

		err := config.ValidateAndSetDefaults()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one organization")
	})

	t.Run("config validation sets correct defaults", func(t *testing.T) {
		config := &Config{
			Orgs:        []string{"org1", "org2"},
			TargetDir:   "/tmp/test",
			Concurrency: 0,
			BatchSize:   0,
		}

		err := config.ValidateAndSetDefaults()
		assert.NoError(t, err)
		assert.Equal(t, 2, config.Concurrency) // Scales with org count
		assert.Equal(t, 2, config.BatchSize)   // Scales with org count
	})

	t.Run("config validation with negative values", func(t *testing.T) {
		config := &Config{
			Orgs:        []string{"org1", "org2"},
			TargetDir:   "/tmp/test",
			Concurrency: -10,
			BatchSize:   -5,
		}

		err := config.ValidateAndSetDefaults()
		assert.NoError(t, err)
		assert.Equal(t, 2, config.Concurrency) // Scales with org count
		assert.Equal(t, 2, config.BatchSize)   // Scales with org count
	})
}

// MAIN FUNCTION TESTING
func TestMainFunction(t *testing.T) {
	t.Run("main function with command line args", func(t *testing.T) {
		// Create a temporary .env file
		envContent := `GHORG_TARGET_DIR=/tmp/main-test
GHORG_ORGS=test-org
GHORG_CONCURRENCY=1
GHORG_BATCH_SIZE=1
GHORG_GITHUB_TOKEN=test-token
`

		envFile := "/tmp/main-test.env"
		err := os.WriteFile(envFile, []byte(envContent), 0644)
		require.NoError(t, err)

		defer func() {
			os.Remove(envFile)
			os.RemoveAll("/tmp/main-test")
		}()

		// Test processOrgsFromEnv with the env file
		err = processOrgsFromEnv(envFile)

		// Should fail at ghorg execution but complete the flow
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ghorg command failed")

		// Verify the target directory was created
		_, statErr := os.Stat("/tmp/main-test")
		assert.NoError(t, statErr)
	})
}

// ENHANCED UNIT TESTS - Pure Function Testing
// Focus on isolated testing of pure functions with comprehensive edge cases

func TestPureFunctionIsolation(t *testing.T) {
	t.Run("validateConfig is pure and deterministic", func(t *testing.T) {
		config1 := &Config{
			Orgs:        []string{"org1", "org2"},
			BatchSize:   50,
			Concurrency: 10,
			TargetDir:   "/tmp/test",
			GitHubToken: "token",
		}

		config2 := &Config{
			Orgs:        []string{"org1", "org2"},
			BatchSize:   50,
			Concurrency: 10,
			TargetDir:   "/tmp/test",
			GitHubToken: "token",
		}

		// Multiple calls should produce identical results
		err1 := validateConfig(config1)
		err2 := validateConfig(config2)

		assert.Equal(t, err1, err2, "Pure function should be deterministic")
		assert.NoError(t, err1)
		assert.NoError(t, err2)

		// Original configs should be unchanged (immutability test)
		assert.Equal(t, []string{"org1", "org2"}, config1.Orgs)
		assert.Equal(t, []string{"org1", "org2"}, config2.Orgs)
	})

	t.Run("processConfig maintains purity", func(t *testing.T) {
		originalConfig := &Config{
			Orgs:        []string{"pure1", "pure2", "pure3"},
			BatchSize:   2,
			Concurrency: 5,
			TargetDir:   "/tmp/pure",
			GitHubToken: "pure-token",
		}

		configCopy := &Config{
			Orgs:        append([]string(nil), originalConfig.Orgs...),
			BatchSize:   originalConfig.BatchSize,
			Concurrency: originalConfig.Concurrency,
			TargetDir:   originalConfig.TargetDir,
			GitHubToken: originalConfig.GitHubToken,
		}

		plan, err := processConfig(originalConfig)
		assert.NoError(t, err)
		assert.Equal(t, 3, plan.TotalOrgs)

		// Original config should be unchanged
		assert.Equal(t, configCopy.Orgs, originalConfig.Orgs)
		assert.Equal(t, configCopy.BatchSize, originalConfig.BatchSize)
	})

	t.Run("createBatches is pure and doesn't mutate input", func(t *testing.T) {
		originalOrgs := []string{"batch1", "batch2", "batch3", "batch4"}
		orgsCopy := append([]string(nil), originalOrgs...)

		batches := createBatches(originalOrgs, 2)

		// Function should not mutate input
		assert.Equal(t, orgsCopy, originalOrgs)

		// Output should be correct
		assert.Equal(t, 2, len(batches))
		assert.Equal(t, []string{"batch1", "batch2"}, batches[0].Orgs)
		assert.Equal(t, []string{"batch3", "batch4"}, batches[1].Orgs)
	})

	t.Run("buildCommand is pure and deterministic", func(t *testing.T) {
		cmd1 := buildCommand("test-org", "/tmp/test", "token123", 5)
		cmd2 := buildCommand("test-org", "/tmp/test", "token123", 5)

		assert.Equal(t, cmd1, cmd2, "Pure function should be deterministic")

		// Test immutability - modifying result shouldn't affect future calls
		cmd1[0] = "modified"
		cmd3 := buildCommand("test-org", "/tmp/test", "token123", 5)
		assert.Equal(t, "clone", cmd3[0], "Function should return fresh results")
	})
}

func TestFunctionalComposition(t *testing.T) {
	t.Run("functions compose correctly", func(t *testing.T) {
		config := &Config{
			Orgs:        []string{"comp1", "comp2", "comp3", "comp4", "comp5"},
			BatchSize:   2,
			Concurrency: 3,
			TargetDir:   "/tmp/compose",
			GitHubToken: "compose-token",
		}

		// Step 1: Validate
		err := validateConfig(config)
		assert.NoError(t, err)

		// Step 2: Process (includes validation)
		plan, err := processConfig(config)
		assert.NoError(t, err)

		// Step 3: Verify composition results
		assert.Equal(t, 5, plan.TotalOrgs)
		assert.Equal(t, 3, len(plan.Batches)) // 5 orgs / 2 batch size = 3 batches

		// Test that createBatches can be called independently
		directBatches := createBatches(config.Orgs, config.BatchSize)
		assert.Equal(t, plan.Batches, directBatches, "Direct and composed calls should match")
	})

	t.Run("error propagation through composition", func(t *testing.T) {
		invalidConfigs := []*Config{
			{Orgs: []string{}, TargetDir: "/tmp/test"},         // Empty orgs
			{Orgs: []string{"org"}, TargetDir: ""},             // Empty target dir
			{Orgs: []string{"  ", ""}, TargetDir: "/tmp/test"}, // Whitespace orgs
		}

		for i, config := range invalidConfigs {
			t.Run(fmt.Sprintf("invalid_config_%d", i), func(t *testing.T) {
				// Validation should fail
				err := validateConfig(config)
				assert.Error(t, err)

				// Processing should also fail with same error propagation
				_, processErr := processConfig(config)
				assert.Error(t, processErr)
				assert.Contains(t, processErr.Error(), err.Error(),
					"Error should propagate through composition")
			})
		}
	})
}
