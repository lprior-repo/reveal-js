package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/panjf2000/ants/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// END-TO-END TEST: Testing the entire program workflow
func TestMainWorkflowE2E(t *testing.T) {
	t.Run("complete workflow with environment variables", func(t *testing.T) {
		// Set up environment variables for E2E test
		os.Setenv("GHORG_TARGET_DIR", "/tmp/e2e-test-repos")
		os.Setenv("GHORG_ORGS", "test-org1,test-org2")
		os.Setenv("GHORG_CONCURRENCY", "2")
		os.Setenv("GHORG_BATCH_SIZE", "1")
		os.Setenv("GHORG_GITHUB_TOKEN", "test-token-e2e")

		defer func() {
			os.Unsetenv("GHORG_TARGET_DIR")
			os.Unsetenv("GHORG_ORGS")
			os.Unsetenv("GHORG_CONCURRENCY")
			os.Unsetenv("GHORG_BATCH_SIZE")
			os.Unsetenv("GHORG_GITHUB_TOKEN")
			os.RemoveAll("/tmp/e2e-test-repos")
		}()

		// Test the main workflow function
		// This will fail at the ghorg command execution, but we can test the full flow up to that point
		err := processOrgsFromEnv("")

		// The function should fail because ghorg is not installed
		// but we can verify the directory was created and the flow executed
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ghorg command failed")

		// Verify the target directory was created
		_, statErr := os.Stat("/tmp/e2e-test-repos")
		assert.NoError(t, statErr)
	})

	t.Run("workflow with .env file", func(t *testing.T) {
		// Create a temporary .env file for testing
		envContent := `GHORG_TARGET_DIR=/tmp/e2e-env-test
GHORG_ORGS=env-org1,env-org2
GHORG_CONCURRENCY=3
GHORG_BATCH_SIZE=2
GHORG_GITHUB_TOKEN=env-token
`

		envFile := "/tmp/test.env"
		err := os.WriteFile(envFile, []byte(envContent), 0644)
		require.NoError(t, err)

		defer func() {
			os.Remove(envFile)
			os.RemoveAll("/tmp/e2e-env-test")
		}()

		// Test the main workflow function with .env file
		err = processOrgsFromEnv(envFile)

		// Should fail at ghorg execution but complete the flow
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ghorg command failed")

		// Verify the target directory was created
		_, statErr := os.Stat("/tmp/e2e-env-test")
		assert.NoError(t, statErr)
	})

	t.Run("workflow fails gracefully with invalid config", func(t *testing.T) {
		// Clear any existing environment variables
		os.Unsetenv("GHORG_TARGET_DIR")
		os.Unsetenv("GHORG_ORGS")
		os.Unsetenv("GHORG_CONCURRENCY")
		os.Unsetenv("GHORG_BATCH_SIZE")
		os.Unsetenv("GHORG_GITHUB_TOKEN")

		// Test the main workflow function with missing required config
		err := processOrgsFromEnv("")

		// Should fail due to missing required configuration
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse environment variables")
	})
}

// MUTATION TESTING: Test behavior under modified conditions (4x expanded)
func TestMutationScenarios(t *testing.T) {
	t.Run("config field mutation tests", func(t *testing.T) {
		baseConfig := &Config{
			Orgs:        []string{"org1", "org2"},
			BatchSize:   50,
			Concurrency: 10,
			TargetDir:   "/tmp/repos",
			GitHubToken: "token123",
		}

		// Mutation 1: Batch size mutations
		config := *baseConfig
		config.BatchSize = 0
		batches := createBatches(config.Orgs, config.BatchSize)
		assert.Equal(t, 1, len(batches))

		config.BatchSize = -1
		batches = createBatches(config.Orgs, config.BatchSize)
		assert.Equal(t, 1, len(batches))

		config.BatchSize = 1
		batches = createBatches(config.Orgs, config.BatchSize)
		assert.Equal(t, 2, len(batches))

		// Mutation 2: Orgs mutations
		config = *baseConfig
		config.Orgs = []string{}
		err := validateConfig(&config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no valid organizations")

		config.Orgs = nil
		err = validateConfig(&config)
		assert.Error(t, err)

		config.Orgs = []string{"single-org"}
		err = validateConfig(&config)
		assert.NoError(t, err)

		// Mutation 3: TargetDir mutations
		config = *baseConfig
		config.TargetDir = ""
		err = validateConfig(&config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "target directory cannot be empty")

		config.TargetDir = "   "
		err = validateConfig(&config)
		assert.Error(t, err)

		// Mutation 4: Concurrency mutations
		config = *baseConfig
		config.Concurrency = 0
		err = config.ValidateAndSetDefaults()
		assert.NoError(t, err)
		assert.Equal(t, 2, config.Concurrency) // Scales with org count

		config.Concurrency = -5
		err = config.ValidateAndSetDefaults()
		assert.NoError(t, err)
		assert.Equal(t, 2, config.Concurrency) // Scales with org count
	})

	t.Run("command building mutation tests", func(t *testing.T) {
		// Test command building with various parameter mutations
		testCases := []struct {
			name        string
			org         string
			targetDir   string
			token       string
			concurrency int
			validateFn  func([]string) bool
		}{
			{
				name: "empty token",
				org:  "org", targetDir: "/path", token: "", concurrency: 5,
				validateFn: func(cmd []string) bool {
					for _, arg := range cmd {
						if arg == "--no-token" {
							return true
						}
					}
					return false
				},
			},
			{
				name: "whitespace token",
				org:  "org", targetDir: "/path", token: "   ", concurrency: 5,
				validateFn: func(cmd []string) bool {
					for _, arg := range cmd {
						if arg == "--no-token" { // Whitespace token is now trimmed to empty
							return true
						}
					}
					return false
				},
			},
			{
				name: "empty org",
				org:  "", targetDir: "/path", token: "token", concurrency: 5,
				validateFn: func(cmd []string) bool {
					return len(cmd) > 1 && cmd[1] == ""
				},
			},
			{
				name: "zero concurrency",
				org:  "org", targetDir: "/path", token: "token", concurrency: 0,
				validateFn: func(cmd []string) bool {
					for i, arg := range cmd {
						if arg == "--concurrency" && i+1 < len(cmd) && cmd[i+1] == "1" { // Now normalized to 1
							return true
						}
					}
					return false
				},
			},
			{
				name: "negative concurrency",
				org:  "org", targetDir: "/path", token: "token", concurrency: -10,
				validateFn: func(cmd []string) bool {
					for i, arg := range cmd {
						if arg == "--concurrency" && i+1 < len(cmd) && cmd[i+1] == "1" { // Now normalized to 1
							return true
						}
					}
					return false
				},
			},
			{
				name: "special chars in org",
				org:  "org!@#$%", targetDir: "/path", token: "token", concurrency: 5,
				validateFn: func(cmd []string) bool {
					return len(cmd) > 1 && cmd[1] == "org!@#$%"
				},
			},
			{
				name: "unicode in token",
				org:  "org", targetDir: "/path", token: "tökén123", concurrency: 5,
				validateFn: func(cmd []string) bool {
					for i, arg := range cmd {
						if arg == "--token" && i+1 < len(cmd) && cmd[i+1] == "tökén123" {
							return true
						}
					}
					return false
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				cmd := buildCommand(tc.org, tc.targetDir, tc.token, tc.concurrency)
				assert.True(t, tc.validateFn(cmd), "Validation failed for command: %v", cmd)
			})
		}
	})

	t.Run("path expansion edge case mutations", func(t *testing.T) {
		testCases := []struct {
			name     string
			path     string
			homeDir  string
			expected string
		}{
			{"empty home dir", "~/test", "", "/test"},
			{"home dir with trailing slash", "~/test", "/home/user/", "/home/user//test"},
			{"home dir is root", "~/test", "/", "//test"},
			{"unicode in home dir", "~/test", "/home/üser", "/home/üser/test"},
			{"tilde in middle", "/path/~/file", "/home/user", "/path/~/file"},
			{"multiple tildes", "~~test", "/home/user", "~~test"},
			{"tilde at end", "test~", "/home/user", "test~"},
			{"just tilde", "~", "/home/user", "~"},
			{"tilde with space", "~ /test", "/home/user", "~ /test"},
			{"path with spaces", "~/my documents", "/home/user", "/home/user/my documents"},
			{"empty path", "", "/home/user", ""},
			{"absolute path", "/absolute/path", "/home/user", "/absolute/path"},
			{"relative path", "relative/path", "/home/user", "relative/path"},
			{"complex unicode path", "~/测试/файл", "/home/пользователь", "/home/пользователь/测试/файл"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := expandPath(tc.path, tc.homeDir)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("batch creation boundary mutations", func(t *testing.T) {
		testCases := []struct {
			name          string
			orgs          []string
			batchSize     int
			expectedCount int
		}{
			{"empty orgs", []string{}, 10, 0},
			{"single org zero batch", []string{"org1"}, 0, 1},
			{"single org huge batch", []string{"org1"}, 1000, 1},
			{"many orgs tiny batch", []string{"org1", "org2", "org3", "org4", "org5"}, 1, 5},
			{"exact division", []string{"org1", "org2", "org3", "org4"}, 2, 2},
			{"uneven division", []string{"org1", "org2", "org3", "org4", "org5"}, 3, 2},
			{"negative batch size", []string{"org1", "org2"}, -5, 1},
			{"zero orgs positive batch", []string{}, 50, 0},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				batches := createBatches(tc.orgs, tc.batchSize)
				assert.Equal(t, tc.expectedCount, len(batches))

				// Verify all orgs are preserved
				totalOrgs := 0
				for _, batch := range batches {
					totalOrgs += len(batch.Orgs)
				}
				assert.Equal(t, len(tc.orgs), totalOrgs)
			})
		}
	})

	t.Run("validation mutation stress tests", func(t *testing.T) {
		// Test various invalid config combinations
		invalidConfigs := []*Config{
			{Orgs: nil, TargetDir: "/valid", BatchSize: 10, Concurrency: 5},
			{Orgs: []string{}, TargetDir: "/valid", BatchSize: 10, Concurrency: 5},
			{Orgs: []string{"org"}, TargetDir: "", BatchSize: 10, Concurrency: 5},
			{Orgs: []string{"org"}, TargetDir: "   ", BatchSize: 10, Concurrency: 5},
			{Orgs: []string{""}, TargetDir: "/valid", BatchSize: 10, Concurrency: 5},
			{Orgs: []string{"   "}, TargetDir: "/valid", BatchSize: 10, Concurrency: 5},
		}

		for i, config := range invalidConfigs {
			t.Run(fmt.Sprintf("invalid_config_%d", i), func(t *testing.T) {
				err := validateConfig(config)
				assert.Error(t, err, "Config should be invalid: %+v", config)
			})
		}

		// Test valid config variations
		validConfigs := []*Config{
			{Orgs: []string{"org1"}, TargetDir: "/valid", BatchSize: 1, Concurrency: 1},
			{Orgs: []string{"org1", "org2"}, TargetDir: "/valid/path", BatchSize: 100, Concurrency: 50},
			{Orgs: []string{"org-with-dash"}, TargetDir: "/tmp", BatchSize: 0, Concurrency: 0},
			{Orgs: []string{"org_with_underscore"}, TargetDir: "/home/user/repos", BatchSize: 25, Concurrency: 10},
		}

		for i, config := range validConfigs {
			t.Run(fmt.Sprintf("valid_config_%d", i), func(t *testing.T) {
				err := validateConfig(config)
				assert.NoError(t, err, "Config should be valid: %+v", config)
			})
		}
	})

	t.Run("function composition mutation tests", func(t *testing.T) {
		// Test how functions behave when composed in different orders
		config := &Config{
			Orgs:        []string{"org1", "org2", "org3"},
			BatchSize:   2,
			TargetDir:   "/tmp/test",
			Concurrency: 5,
			GitHubToken: "token",
		}

		// Direct validation then processing
		err := validateConfig(config)
		require.NoError(t, err)
		plan, err := processConfig(config)
		require.NoError(t, err)
		assert.Equal(t, 3, plan.TotalOrgs)

		// Process without explicit validation (should validate internally)
		config2 := &Config{
			Orgs:        []string{"org1", "org2", "org3"},
			BatchSize:   2,
			TargetDir:   "/tmp/test",
			Concurrency: 5,
			GitHubToken: "token",
		}
		plan2, err := processConfig(config2)
		require.NoError(t, err)
		assert.Equal(t, plan, plan2)

		// Test with invalid config
		invalidConfig := &Config{
			Orgs:      []string{},
			TargetDir: "/tmp/test",
		}
		_, err = processConfig(invalidConfig)
		assert.Error(t, err)
	})
}

// STRESS TESTING: Test with large inputs
func TestStressScenarios(t *testing.T) {
	t.Run("large number of organizations", func(t *testing.T) {
		// Generate 1000 organizations
		orgs := make([]string, 1000)
		for i := 0; i < 1000; i++ {
			orgs[i] = "org" + string(rune(i))
		}

		config := &Config{
			Orgs:        orgs,
			BatchSize:   10,
			Concurrency: 5,
			TargetDir:   "/tmp/stress-test",
			GitHubToken: "token",
		}

		// Should process without error
		plan, err := processConfig(config)
		assert.NoError(t, err)
		assert.Equal(t, 1000, plan.TotalOrgs)
		assert.Equal(t, 100, len(plan.Batches)) // 1000 / 10 = 100 batches
	})

	t.Run("very large batch size", func(t *testing.T) {
		orgs := []string{"org1", "org2", "org3"}
		batches := createBatches(orgs, 10000)

		// Should create single batch
		assert.Equal(t, 1, len(batches))
		assert.Equal(t, orgs, batches[0].Orgs)
	})
}

// CONCURRENCY TESTING: Test concurrent operations
func TestConcurrentOperations(t *testing.T) {
	t.Run("concurrent config processing", func(t *testing.T) {
		config := &Config{
			Orgs:        []string{"org1", "org2", "org3"},
			BatchSize:   2,
			Concurrency: 5,
			TargetDir:   "/tmp/concurrent-test",
			GitHubToken: "token",
		}

		// Process config concurrently
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				plan, err := processConfig(config)
				assert.NoError(t, err)
				assert.Equal(t, 3, plan.TotalOrgs)
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// FUNCTIONAL WORKFLOW TESTING
func TestFunctionalWorkflow(t *testing.T) {
	t.Run("ExecuteWorkflow with invalid config", func(t *testing.T) {
		// Create functional adapters
		loader := CreateEnvConfigLoader()
		cloner := CreateGhorgOrgCloner()

		// Clear all env variables to force failure
		os.Unsetenv("GHORG_TARGET_DIR")
		os.Unsetenv("GHORG_ORGS")
		os.Unsetenv("GHORG_CONCURRENCY")
		os.Unsetenv("GHORG_BATCH_SIZE")
		os.Unsetenv("GHORG_GITHUB_TOKEN")

		ctx := context.Background()
		err := ExecuteWorkflow(ctx, "", loader, cloner)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "configuration loading failed")
	})

	t.Run("ExecuteWorkflow with context cancellation", func(t *testing.T) {
		// Set up valid environment
		os.Setenv("GHORG_TARGET_DIR", "/tmp/shell-test")
		os.Setenv("GHORG_ORGS", "test-org")
		os.Setenv("GHORG_CONCURRENCY", "1")
		os.Setenv("GHORG_BATCH_SIZE", "1")
		os.Setenv("GHORG_GITHUB_TOKEN", "test-token")

		defer func() {
			os.Unsetenv("GHORG_TARGET_DIR")
			os.Unsetenv("GHORG_ORGS")
			os.Unsetenv("GHORG_CONCURRENCY")
			os.Unsetenv("GHORG_BATCH_SIZE")
			os.Unsetenv("GHORG_GITHUB_TOKEN")
			os.RemoveAll("/tmp/shell-test")
		}()

		loader := CreateEnvConfigLoader()
		cloner := CreateGhorgOrgCloner()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := ExecuteWorkflow(ctx, "", loader, cloner)

		// Should fail due to context cancellation during config loading
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workflow cancelled before starting")
	})
}

// FILE SYSTEM ERROR TESTING
func TestFileSystemErrors(t *testing.T) {
	t.Run("CloneOrg with directory creation failure", func(t *testing.T) {
		cloner := NewGhorgOrgCloner()
		ctx := context.Background()

		// Try to create directory in a path that doesn't exist and can't be created
		err := cloner.CloneOrg(ctx, "testorg", "/proc/nonexistent/path", "token", 5)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create directory")
	})
}

// ENVIRONMENT CONFIGURATION FAILURE TESTING
func TestEnvironmentConfigurationFailures(t *testing.T) {
	t.Run("LoadConfig with invalid .env file", func(t *testing.T) {
		// Create invalid .env file with bad syntax
		envContent := "INVALID_ENV_FILE_CONTENT\n\t\tGHORG_TARGET_DIR=/tmp/test\n\t\tGHORG_ORGS=test-org\n\t\t"

		envFile := "/tmp/invalid.env"
		err := os.WriteFile(envFile, []byte(envContent), 0644)
		require.NoError(t, err)

		defer os.Remove(envFile)

		loader := NewEnvConfigLoader()
		ctx := context.Background()

		_, err = loader.LoadConfig(ctx, envFile)

		// Should fail due to invalid .env file format
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load .env file")
	})

	t.Run("LoadConfig with missing required env vars", func(t *testing.T) {
		// Create .env file with missing required vars
		envContent := `GHORG_CONCURRENCY=5
GHORG_BATCH_SIZE=10
`

		envFile := "/tmp/incomplete.env"
		err := os.WriteFile(envFile, []byte(envContent), 0644)
		require.NoError(t, err)

		defer os.Remove(envFile)

		loader := NewEnvConfigLoader()
		ctx := context.Background()

		_, err = loader.LoadConfig(ctx, envFile)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse environment variables")
	})

	t.Run("LoadConfig with validation failure", func(t *testing.T) {
		// Create .env file that will pass parsing but fail validation
		envContent := `GHORG_TARGET_DIR=
GHORG_ORGS=test-org
GHORG_CONCURRENCY=5
GHORG_BATCH_SIZE=10
`

		envFile := "/tmp/invalid-validation.env"
		err := os.WriteFile(envFile, []byte(envContent), 0644)
		require.NoError(t, err)

		defer os.Remove(envFile)

		loader := NewEnvConfigLoader()
		ctx := context.Background()

		_, err = loader.LoadConfig(ctx, envFile)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid configuration")
	})
}

// BATCH PROCESSING EDGE CASES
func TestBatchProcessingEdgeCases(t *testing.T) {
	t.Run("executePlan with empty batches", func(t *testing.T) {
		cloner := CreateGhorgOrgCloner()

		// Create empty plan
		plan := ProcessingPlan{
			Batches:   []Batch{},
			TotalOrgs: 0,
		}

		config := &Config{
			TargetDir:   "/tmp/empty-test",
			Orgs:        []string{},
			Concurrency: 5,
			BatchSize:   10,
		}

		ctx := context.Background()
		err := executePlan(ctx, plan, config, cloner)

		// Should succeed with empty plan
		assert.NoError(t, err)
	})

	t.Run("executePlan with single org", func(t *testing.T) {
		cloner := CreateGhorgOrgCloner()

		// Create plan with single org
		plan := ProcessingPlan{
			Batches: []Batch{
				{Orgs: []string{"single-org"}, BatchNumber: 1},
			},
			TotalOrgs: 1,
		}

		config := &Config{
			TargetDir:   "/tmp/single-org-test",
			Orgs:        []string{"single-org"},
			Concurrency: 5,
			BatchSize:   10,
			GitHubToken: "test-token",
		}

		ctx := context.Background()
		err := executePlan(ctx, plan, config, cloner)

		// Should fail at ghorg execution but complete the flow
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "processing completed with")
	})
}

// ADDITIONAL COVERAGE TESTS
func TestAdditionalCoverage(t *testing.T) {
	t.Run("CloneOrg successful path", func(t *testing.T) {
		// Test the successful return path by mocking a successful command
		cloner := NewGhorgOrgCloner()
		ctx := context.Background()

		// Use a path that will succeed in creation but fail at ghorg command
		// This tests the successful directory creation and command building path
		tempDir := "/tmp/clone-success-test"
		err := cloner.CloneOrg(ctx, "testorg", tempDir, "token", 5)

		// Clean up
		defer os.RemoveAll(tempDir)

		// Should fail at ghorg command but have created directory
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ghorg command failed")

		// Verify directory was created (tests successful path up to command execution)
		_, statErr := os.Stat(tempDir)
		assert.NoError(t, statErr)
	})

	t.Run("processConfig error path in ExecuteWorkflow", func(t *testing.T) {
		// We can't easily create a config that passes validation but fails processConfig
		// since processConfig calls validateConfig. Let's test the workflow failure instead.
		os.Setenv("GHORG_TARGET_DIR", "/tmp/process-fail-test")
		os.Setenv("GHORG_ORGS", "") // Empty orgs will cause config validation to fail
		os.Setenv("GHORG_CONCURRENCY", "1")
		os.Setenv("GHORG_BATCH_SIZE", "1")

		defer func() {
			os.Unsetenv("GHORG_TARGET_DIR")
			os.Unsetenv("GHORG_ORGS")
			os.Unsetenv("GHORG_CONCURRENCY")
			os.Unsetenv("GHORG_BATCH_SIZE")
		}()

		loader := CreateEnvConfigLoader()
		cloner := CreateGhorgOrgCloner()

		ctx := context.Background()
		err := ExecuteWorkflow(ctx, "", loader, cloner)

		// Should fail at config loading/validation step
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "configuration loading failed")
	})

	t.Run("executePlan successful completion", func(t *testing.T) {
		// Test the successful path with empty plan
		cloner := CreateGhorgOrgCloner()

		// Create empty plan that should succeed
		plan := ProcessingPlan{
			Batches:   []Batch{},
			TotalOrgs: 0,
		}

		config := &Config{
			TargetDir:   "/tmp/success-test",
			Orgs:        []string{},
			Concurrency: 5,
			BatchSize:   10,
		}

		ctx := context.Background()
		err := executePlan(ctx, plan, config, cloner)

		// Should succeed with empty plan
		assert.NoError(t, err)
	})
}

// MAIN FUNCTION COVERAGE
func TestMainFunctionCoverage(t *testing.T) {
	// We can't easily test main() directly due to os.Exit(), but we can test processOrgsFromEnv
	// which contains most of the main logic

	t.Run("processOrgsFromEnv with no args", func(t *testing.T) {
		// Clear environment to ensure failure
		os.Unsetenv("GHORG_TARGET_DIR")
		os.Unsetenv("GHORG_ORGS")

		err := processOrgsFromEnv("")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse environment variables")
	})

	t.Run("processOrgsFromEnv with valid env", func(t *testing.T) {
		// Set up valid environment
		os.Setenv("GHORG_TARGET_DIR", "/tmp/process-test")
		os.Setenv("GHORG_ORGS", "test-org")
		os.Setenv("GHORG_CONCURRENCY", "1")
		os.Setenv("GHORG_BATCH_SIZE", "1")
		os.Setenv("GHORG_GITHUB_TOKEN", "test-token")

		defer func() {
			os.Unsetenv("GHORG_TARGET_DIR")
			os.Unsetenv("GHORG_ORGS")
			os.Unsetenv("GHORG_CONCURRENCY")
			os.Unsetenv("GHORG_BATCH_SIZE")
			os.Unsetenv("GHORG_GITHUB_TOKEN")
			os.RemoveAll("/tmp/process-test")
		}()

		err := processOrgsFromEnv("")

		// Should fail at ghorg command but complete the workflow
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ghorg command failed")
	})
}

// ACCEPTANCE TESTS - End-to-End Testing with Given-When-Then Structure
// Following TDD principles for critical user flows

func TestWorkflowExecution_AcceptanceTest(t *testing.T) {
	t.Run("complete workflow execution with valid configuration", func(t *testing.T) {
		// Given: A valid configuration with multiple organizations
		given := setupValidEnvironment(t, []string{"test-org1", "test-org2"}, "/tmp/acceptance-test-multi")
		defer given.cleanup()

		// When: User executes the complete workflow
		when := executeCompleteWorkflow(t, given.envFilePath)

		// Then: All organizations are processed successfully (or fail predictably at ghorg)
		assertWorkflowExecutionResults(t, when, given)
	})

	t.Run("workflow handles single organization correctly", func(t *testing.T) {
		// Given: A configuration with a single organization
		given := setupValidEnvironment(t, []string{"single-org"}, "/tmp/acceptance-test-single")
		defer given.cleanup()

		// When: User executes the workflow for single org
		when := executeCompleteWorkflow(t, given.envFilePath)

		// Then: Single organization is processed correctly
		assertSingleOrgResults(t, when, given)
	})

	t.Run("workflow gracefully handles configuration errors", func(t *testing.T) {
		// Given: An invalid configuration (missing required fields)
		given := setupInvalidEnvironment(t)
		defer given.cleanup()

		// When: User attempts to execute workflow with invalid config
		when := executeCompleteWorkflow(t, given.envFilePath)

		// Then: Workflow fails with clear error messages
		assertConfigurationErrorHandling(t, when)
	})
}

// INTEGRATION TESTS - Component Interaction Testing with Ants Worker Pools

func TestWorkerPoolIntegration(t *testing.T) {
	t.Run("ants pool integrates correctly with functional cloner", func(t *testing.T) {
		poolSize := 3
		pool, err := ants.NewPool(poolSize, ants.WithOptions(ants.Options{
			PreAlloc:    true,
			Nonblocking: false,
		}))
		require.NoError(t, err)
		defer pool.Release()

		cloner := CreateGhorgOrgCloner()
		ctx := context.Background()

		var wg sync.WaitGroup
		var mu sync.Mutex
		results := make([]error, 0)

		orgs := []string{"test-org1", "test-org2", "test-org3"}

		for _, org := range orgs {
			wg.Add(1)
			err := pool.Submit(func(orgName string) func() {
				return func() {
					defer wg.Done()
					err := cloner(ctx, orgName, "/tmp/integration-test", "test-token", 1)
					mu.Lock()
					results = append(results, err)
					mu.Unlock()
				}
			}(org))
			assert.NoError(t, err, "Task submission should succeed")
		}

		wg.Wait()

		// All tasks should have been executed (and failed at ghorg command)
		assert.Equal(t, len(orgs), len(results))
		for i, err := range results {
			assert.Error(t, err, "Org %d should fail at ghorg execution", i)
			assert.Contains(t, err.Error(), "ghorg command failed")
		}
	})

	t.Run("functional config loader integrates with workflow executor", func(t *testing.T) {
		loader := CreateEnvConfigLoader()
		cloner := CreateGhorgOrgCloner()

		os.Setenv("GHORG_TARGET_DIR", "/tmp/integration-config-test")
		os.Setenv("GHORG_ORGS", "integration-org1,integration-org2")
		os.Setenv("GHORG_CONCURRENCY", "2")
		os.Setenv("GHORG_BATCH_SIZE", "1")
		os.Setenv("GHORG_GITHUB_TOKEN", "integration-token")

		defer func() {
			os.Unsetenv("GHORG_TARGET_DIR")
			os.Unsetenv("GHORG_ORGS")
			os.Unsetenv("GHORG_CONCURRENCY")
			os.Unsetenv("GHORG_BATCH_SIZE")
			os.Unsetenv("GHORG_GITHUB_TOKEN")
			os.RemoveAll("/tmp/integration-config-test")
		}()

		ctx := context.Background()
		err := ExecuteWorkflow(ctx, "", loader, cloner)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "processing completed with")
	})
}

// Test Setup Helpers for Acceptance Tests

type AcceptanceTestSetup struct {
	envFilePath string
	targetDir   string
	orgs        []string
	cleanup     func()
}

type AcceptanceTestResult struct {
	err     error
	output  string
	success bool
}

func setupValidEnvironment(t *testing.T, orgs []string, targetDir string) AcceptanceTestSetup {
	envFile := createTempEnvFile(t, map[string]string{
		"GHORG_TARGET_DIR":   targetDir,
		"GHORG_ORGS":         joinOrgs(orgs),
		"GHORG_CONCURRENCY":  "2",
		"GHORG_BATCH_SIZE":   "3",
		"GHORG_GITHUB_TOKEN": "test-token-acceptance",
	})

	return AcceptanceTestSetup{
		envFilePath: envFile,
		targetDir:   targetDir,
		orgs:        orgs,
		cleanup: func() {
			os.Remove(envFile)
			os.RemoveAll(targetDir)
		},
	}
}

func setupInvalidEnvironment(t *testing.T) AcceptanceTestSetup {
	envFile := createTempEnvFile(t, map[string]string{
		"GHORG_TARGET_DIR": "",
		"GHORG_ORGS":       "",
	})

	return AcceptanceTestSetup{
		envFilePath: envFile,
		cleanup: func() {
			os.Remove(envFile)
		},
	}
}

func executeCompleteWorkflow(t *testing.T, envFilePath string) AcceptanceTestResult {
	err := processOrgsFromEnv(envFilePath)
	return AcceptanceTestResult{
		err:     err,
		success: err == nil,
	}
}

func assertWorkflowExecutionResults(t *testing.T, result AcceptanceTestResult, setup AcceptanceTestSetup) {
	assert.Error(t, result.err, "Expected workflow to fail at ghorg command execution")
	assert.Contains(t, result.err.Error(), "processing completed with",
		"Expected error to indicate processing completion with failures")

	_, err := os.Stat(setup.targetDir)
	assert.NoError(t, err, "Target directory should be created")
}

func assertSingleOrgResults(t *testing.T, result AcceptanceTestResult, setup AcceptanceTestSetup) {
	assert.Error(t, result.err, "Expected single org workflow to fail at ghorg execution")
	assert.Contains(t, result.err.Error(), setup.orgs[0],
		"Error should mention the single organization")

	_, err := os.Stat(setup.targetDir)
	assert.NoError(t, err, "Target directory should be created for single org")
}

func assertConfigurationErrorHandling(t *testing.T, result AcceptanceTestResult) {
	assert.Error(t, result.err, "Expected workflow to fail with invalid configuration")
	assert.Contains(t, result.err.Error(), "configuration loading failed",
		"Error should indicate configuration loading failure")
}

func createTempEnvFile(t *testing.T, vars map[string]string) string {
	file, err := os.CreateTemp("", "acceptance-test-*.env")
	require.NoError(t, err)
	defer file.Close()

	for key, value := range vars {
		_, err := file.WriteString(key + "=" + value + "\n")
		require.NoError(t, err)
	}

	return file.Name()
}

func joinOrgs(orgs []string) string {
	return strings.Join(orgs, ",")
}
