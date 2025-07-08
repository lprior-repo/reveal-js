package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// Unit Tests for Organization Cloning Functionality

func TestCreateGhorgOrgCloner(t *testing.T) {
	t.Run("Given no progress function, When creating cloner, Then returns functional cloner", func(t *testing.T) {
		// Given - no progress function

		// When - creating cloner
		cloner := CreateGhorgOrgCloner()

		// Then - returns functional cloner
		assert.NotNil(t, cloner)
	})
}

func TestCreateGhorgOrgClonerWithProgress(t *testing.T) {
	t.Run("Given progress function, When creating cloner, Then returns cloner with progress tracking", func(t *testing.T) {
		// Given - progress function
		var progressUpdates []ProgressUpdate
		progressFunc := func(update ProgressUpdate) {
			progressUpdates = append(progressUpdates, update)
		}

		// When - creating cloner with progress
		cloner := CreateGhorgOrgClonerWithProgress(progressFunc)

		// Then - returns functional cloner
		assert.NotNil(t, cloner)
	})
}

func TestOrgClonerContextCancellation(t *testing.T) {
	t.Run("Given cancelled context, When cloning org, Then returns cancellation error", func(t *testing.T) {
		// Given - cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		cloner := CreateGhorgOrgCloner()

		// When - attempting to clone with cancelled context
		err := cloner(ctx, "test-org", "/tmp/test", "token", 1)

		// Then - returns cancellation error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cancelled")
	})
}

func TestOrgClonerDirectoryCreation(t *testing.T) {
	t.Run("Given valid parameters, When cloning org, Then creates target directory", func(t *testing.T) {
		// Given - valid parameters and clean test environment
		ctx := context.Background()
		targetDir := "/tmp/test-cloner-dir-creation"
		defer os.RemoveAll(targetDir)

		cloner := CreateGhorgOrgCloner()

		// When - cloning (will fail at ghorg command but should create directory)
		cloner(ctx, "test-org", targetDir, "", 1)

		// Then - target directory should be created
		_, err := os.Stat(targetDir)
		assert.NoError(t, err, "Target directory should be created")
	})
}

func TestOrgClonerPathExpansion(t *testing.T) {
	t.Run("Given tilde path, When cloning org, Then expands path correctly", func(t *testing.T) {
		// Given - tilde path
		ctx := context.Background()
		homeDir, _ := os.UserHomeDir()

		cloner := CreateGhorgOrgCloner()

		// When - cloning with tilde path (will fail at ghorg but should expand path)
		_ = cloner(ctx, "test-org", "~/test-expansion", "", 1)

		// Then - may succeed or fail depending on ghorg command
		// Verify the expanded path was attempted to be created
		expandedPath := homeDir + "/test-expansion"
		defer os.RemoveAll(expandedPath)
		_, statErr := os.Stat(expandedPath)
		assert.NoError(t, statErr, "Expanded path should be created")
	})
}

// Property-Based Tests for Cloner Functions

func TestOrgClonerProperties(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random valid organization names and paths
		org := rapid.StringMatching(`^[a-zA-Z0-9_-]+$`).Draw(t, "org")
		if len(org) == 0 {
			org = "test-org" // Ensure non-empty
		}

		concurrency := rapid.IntRange(1, 10).Draw(t, "concurrency")

		// Given - random valid inputs
		ctx := context.Background()
		targetDir := "/tmp/rapid-test-" + org
		defer os.RemoveAll(targetDir)

		cloner := CreateGhorgOrgCloner()

		// When - cloning with random inputs
		err := cloner(ctx, org, targetDir, "", concurrency)

		// Then - should always create target directory regardless of ghorg failure
		_, statErr := os.Stat(targetDir)
		assert.NoError(t, statErr, "Target directory should always be created")

		// Property: Error should contain org name if ghorg fails
		if err != nil {
			assert.Contains(t, err.Error(), org, "Error should reference the organization name")
		}
	})
}

// Integration Tests with Progress Tracking

func TestOrgClonerWithProgressIntegration(t *testing.T) {
	t.Run("Given progress function, When cloning fails, Then no progress updates sent", func(t *testing.T) {
		// Given - progress function that captures updates
		var progressUpdates []ProgressUpdate
		progressFunc := func(update ProgressUpdate) {
			progressUpdates = append(progressUpdates, update)
		}

		ctx := context.Background()
		targetDir := "/tmp/test-progress-integration"
		defer os.RemoveAll(targetDir)

		cloner := CreateGhorgOrgClonerWithProgress(progressFunc)

		// When - cloning (will fail due to no real org)
		err := cloner(ctx, "non-existent-test-org", targetDir, "", 1)

		// Then - should have error and no progress updates (since no repos found)
		assert.Error(t, err)
		assert.Len(t, progressUpdates, 0, "Should not send progress updates when no repos are found")
	})
}

// Mutation Tests - Testing Error Handling

func TestOrgClonerErrorHandling(t *testing.T) {
	t.Run("Given invalid target directory, When cloning, Then returns directory creation error", func(t *testing.T) {
		// Given - invalid target directory (e.g., file exists with same name)
		ctx := context.Background()
		invalidPath := "/dev/null/impossible-directory"

		cloner := CreateGhorgOrgCloner()

		// When - attempting to clone to invalid path
		err := cloner(ctx, "test-org", invalidPath, "", 1)

		// Then - returns directory creation error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create directory")
	})
}

// Stress Tests for Concurrency

func TestOrgClonerConcurrencyStress(t *testing.T) {
	t.Run("Given high concurrency value, When cloning, Then handles gracefully", func(t *testing.T) {
		// Given - very high concurrency value
		ctx := context.Background()
		targetDir := "/tmp/test-high-concurrency"
		defer os.RemoveAll(targetDir)

		cloner := CreateGhorgOrgCloner()

		// When - cloning with high concurrency
		err := cloner(ctx, "test-org", targetDir, "", 1000)

		// Then - should handle gracefully (will fail at ghorg but not panic)
		assert.Error(t, err) // Expected to fail at ghorg command
		assert.NotPanics(t, func() {
			cloner(ctx, "test-org", targetDir, "", 1000)
		}, "Should not panic with high concurrency")
	})
}

// Timeout Tests

func TestOrgClonerTimeout(t *testing.T) {
	t.Run("Given context with timeout, When cloning takes too long, Then respects timeout", func(t *testing.T) {
		// Given - context with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		targetDir := "/tmp/test-timeout"
		defer os.RemoveAll(targetDir)

		cloner := CreateGhorgOrgCloner()

		// When - attempting to clone with short timeout
		err := cloner(ctx, "test-org", targetDir, "", 1)

		// Then - should respect timeout
		assert.Error(t, err)
		// The error might be timeout, cancellation, or directory creation error depending on timing
		errorStr := err.Error()
		timeoutOrCancellation := strings.Contains(errorStr, "timeout") ||
			strings.Contains(errorStr, "cancelled") ||
			strings.Contains(errorStr, "context deadline exceeded") ||
			strings.Contains(errorStr, "failed to create directory") ||
			strings.Contains(errorStr, "signal: killed") ||
			strings.Contains(errorStr, "ghorg command failed")
		assert.True(t, timeoutOrCancellation, "Error should be related to timeout, cancellation, or directory creation: %s", errorStr)
	})
}
