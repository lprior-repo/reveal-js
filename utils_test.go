package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// Test min function following Given-When-Then pattern
func TestMin(t *testing.T) {
	t.Run("Given two different numbers When calling min Then should return smaller", func(t *testing.T) {
		// Given
		a, b := 5, 3

		// When
		result := min(a, b)

		// Then
		assert.Equal(t, 3, result)
	})

	t.Run("Given two equal numbers When calling min Then should return either", func(t *testing.T) {
		// Given
		a, b := 5, 5

		// When
		result := min(a, b)

		// Then
		assert.Equal(t, 5, result)
	})

	t.Run("Given negative numbers When calling min Then should return smaller", func(t *testing.T) {
		// Given
		a, b := -2, -8

		// When
		result := min(a, b)

		// Then
		assert.Equal(t, -8, result)
	})
}

// Test max function following Given-When-Then pattern
func TestMax(t *testing.T) {
	t.Run("Given two different numbers When calling max Then should return larger", func(t *testing.T) {
		// Given
		a, b := 5, 3

		// When
		result := max(a, b)

		// Then
		assert.Equal(t, 5, result)
	})

	t.Run("Given two equal numbers When calling max Then should return either", func(t *testing.T) {
		// Given
		a, b := 5, 5

		// When
		result := max(a, b)

		// Then
		assert.Equal(t, 5, result)
	})

	t.Run("Given negative numbers When calling max Then should return larger", func(t *testing.T) {
		// Given
		a, b := -2, -8

		// When
		result := max(a, b)

		// Then
		assert.Equal(t, -2, result)
	})
}

// Test createError function following Given-When-Then pattern
func TestCreateError(t *testing.T) {
	t.Run("Given format string and args When creating error Then should format correctly", func(t *testing.T) {
		// Given
		format := "error processing %s: %d items failed"
		args := []interface{}{"batch1", 5}

		// When
		err := createError(format, args...)

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error processing batch1: 5 items failed")
	})

	t.Run("Given format string without args When creating error Then should return as-is", func(t *testing.T) {
		// Given
		format := "simple error message"

		// When
		err := createError("%s", format)

		// Then
		assert.Error(t, err)
		assert.Equal(t, "simple error message", err.Error())
	})
}

// Test createLoggers function following Given-When-Then pattern
func TestCreateLoggers(t *testing.T) {
	t.Run("Given no parameters When creating loggers Then should return functional loggers", func(t *testing.T) {
		// Given - no parameters needed

		// When
		logInfo, logError := createLoggers()

		// Then
		assert.NotNil(t, logInfo)
		assert.NotNil(t, logError)

		// Test that loggers can be called without panicking
		assert.NotPanics(t, func() {
			logInfo("test info message")
			logError("test error message")
		})
	})
}

// Test BoundedBuffer following Given-When-Then pattern
func TestBoundedBuffer(t *testing.T) {
	t.Run("Given empty buffer When creating Then should initialize correctly", func(t *testing.T) {
		// Given
		maxSize := 3

		// When
		buffer := NewBoundedBuffer(maxSize)

		// Then
		assert.NotNil(t, buffer)
		assert.Equal(t, maxSize, buffer.maxSize)
		assert.Equal(t, 0, buffer.size)
		assert.Equal(t, []string{}, buffer.GetAll())
	})

	t.Run("Given buffer with capacity When writing lines Then should store correctly", func(t *testing.T) {
		// Given
		buffer := NewBoundedBuffer(3)

		// When
		buffer.Write("line1")
		buffer.Write("line2")

		// Then
		result := buffer.GetAll()
		assert.Equal(t, []string{"line1", "line2"}, result)
		assert.Equal(t, 2, buffer.size)
	})

	t.Run("Given buffer at capacity When writing more lines Then should rotate correctly", func(t *testing.T) {
		// Given
		buffer := NewBoundedBuffer(3)
		buffer.Write("line1")
		buffer.Write("line2")
		buffer.Write("line3")

		// When
		buffer.Write("line4")
		buffer.Write("line5")

		// Then
		result := buffer.GetAll()
		assert.Equal(t, []string{"line3", "line4", "line5"}, result)
		assert.Equal(t, 3, buffer.size)
	})

	t.Run("Given buffer with single capacity When writing multiple lines Then should keep only last", func(t *testing.T) {
		// Given
		buffer := NewBoundedBuffer(1)

		// When
		buffer.Write("line1")
		buffer.Write("line2")
		buffer.Write("line3")

		// Then
		result := buffer.GetAll()
		assert.Equal(t, []string{"line3"}, result)
		assert.Equal(t, 1, buffer.size)
	})

	t.Run("Given empty buffer When getting all Then should return empty slice", func(t *testing.T) {
		// Given
		buffer := NewBoundedBuffer(3)

		// When
		result := buffer.GetAll()

		// Then
		assert.Equal(t, []string{}, result)
		assert.Equal(t, 0, len(result))
	})
}

// Test chunkSlice function following Given-When-Then pattern
func TestChunkSlice(t *testing.T) {
	t.Run("Given slice and chunk size When chunking Then should create correct chunks", func(t *testing.T) {
		// Given
		slice := []string{"a", "b", "c", "d", "e"}
		size := 2

		// When
		result := chunkSlice(slice, size)

		// Then
		expected := [][]string{{"a", "b"}, {"c", "d"}, {"e"}}
		assert.Equal(t, expected, result)
	})

	t.Run("Given empty slice When chunking Then should return empty", func(t *testing.T) {
		// Given
		slice := []string{}
		size := 2

		// When
		result := chunkSlice(slice, size)

		// Then
		assert.Equal(t, [][]string{}, result)
	})

	t.Run("Given slice with size larger than slice When chunking Then should return single chunk", func(t *testing.T) {
		// Given
		slice := []string{"a", "b"}
		size := 5

		// When
		result := chunkSlice(slice, size)

		// Then
		expected := [][]string{{"a", "b"}}
		assert.Equal(t, expected, result)
	})

	t.Run("Given slice with size 1 When chunking Then should return individual chunks", func(t *testing.T) {
		// Given
		slice := []string{"a", "b", "c"}
		size := 1

		// When
		result := chunkSlice(slice, size)

		// Then
		expected := [][]string{{"a"}, {"b"}, {"c"}}
		assert.Equal(t, expected, result)
	})

	t.Run("Given slice with zero size When chunking Then should default to size 1", func(t *testing.T) {
		// Given
		slice := []string{"a", "b", "c"}
		size := 0

		// When
		result := chunkSlice(slice, size)

		// Then
		expected := [][]string{{"a"}, {"b"}, {"c"}}
		assert.Equal(t, expected, result)
	})

	t.Run("Given slice with negative size When chunking Then should default to size 1", func(t *testing.T) {
		// Given
		slice := []string{"a", "b", "c"}
		size := -1

		// When
		result := chunkSlice(slice, size)

		// Then
		expected := [][]string{{"a"}, {"b"}, {"c"}}
		assert.Equal(t, expected, result)
	})

	t.Run("Given slice with perfect division When chunking Then should create equal chunks", func(t *testing.T) {
		// Given
		slice := []string{"a", "b", "c", "d"}
		size := 2

		// When
		result := chunkSlice(slice, size)

		// Then
		expected := [][]string{{"a", "b"}, {"c", "d"}}
		assert.Equal(t, expected, result)
	})
}

// Test expandPath function following Given-When-Then pattern
func TestExpandPath(t *testing.T) {
	t.Run("Given path with tilde and subdirectory When expanding Then should expand correctly", func(t *testing.T) {
		// Given
		path := "~/documents"
		homeDir := "/home/user"

		// When
		result := expandPath(path, homeDir)

		// Then
		assert.Equal(t, "/home/user/documents", result)
	})

	t.Run("Given path with tilde root When expanding Then should expand to home dir", func(t *testing.T) {
		// Given
		path := "~/"
		homeDir := "/home/user"

		// When
		result := expandPath(path, homeDir)

		// Then
		assert.Equal(t, "/home/user", result)
	})

	t.Run("Given absolute path When expanding Then should return unchanged", func(t *testing.T) {
		// Given
		path := "/absolute/path"
		homeDir := "/home/user"

		// When
		result := expandPath(path, homeDir)

		// Then
		assert.Equal(t, "/absolute/path", result)
	})

	t.Run("Given relative path When expanding Then should return unchanged", func(t *testing.T) {
		// Given
		path := "relative/path"
		homeDir := "/home/user"

		// When
		result := expandPath(path, homeDir)

		// Then
		assert.Equal(t, "relative/path", result)
	})

	t.Run("Given path with tilde not at start When expanding Then should return unchanged", func(t *testing.T) {
		// Given
		path := "/path/~/file"
		homeDir := "/home/user"

		// When
		result := expandPath(path, homeDir)

		// Then
		assert.Equal(t, "/path/~/file", result)
	})

	t.Run("Given empty path When expanding Then should return empty", func(t *testing.T) {
		// Given
		path := ""
		homeDir := "/home/user"

		// When
		result := expandPath(path, homeDir)

		// Then
		assert.Equal(t, "", result)
	})

	t.Run("Given tilde path with empty home dir When expanding Then should use root", func(t *testing.T) {
		// Given
		path := "~/documents"
		homeDir := ""

		// When
		result := expandPath(path, homeDir)

		// Then
		assert.Equal(t, "/documents", result)
	})

	t.Run("Given tilde root with empty home dir When expanding Then should use root", func(t *testing.T) {
		// Given
		path := "~/"
		homeDir := ""

		// When
		result := expandPath(path, homeDir)

		// Then
		assert.Equal(t, "/", result)
	})
}

// Test buildCommand function following Given-When-Then pattern
func TestBuildCommand(t *testing.T) {
	t.Run("Given org, dir, token and concurrency When building command Then should create correct command with token", func(t *testing.T) {
		// Given
		org := "myorg"
		targetDir := "/tmp/repos"
		token := "token123"
		concurrency := 5

		// When
		result := buildCommand(org, targetDir, token, concurrency)

		// Then
		expected := []string{"clone", "myorg", "--path", "/tmp/repos", "--concurrency", "5", "--skip-archived", "--token", "token123"}
		assert.Equal(t, expected, result)
	})

	t.Run("Given org, dir, empty token and concurrency When building command Then should create correct command without token", func(t *testing.T) {
		// Given
		org := "myorg"
		targetDir := "/tmp/repos"
		token := ""
		concurrency := 5

		// When
		result := buildCommand(org, targetDir, token, concurrency)

		// Then
		expected := []string{"clone", "myorg", "--path", "/tmp/repos", "--concurrency", "5", "--skip-archived", "--no-token"}
		assert.Equal(t, expected, result)
	})

	t.Run("Given parameters with whitespace When building command Then should trim whitespace", func(t *testing.T) {
		// Given
		org := "  myorg  "
		targetDir := "  /tmp/repos  "
		token := "  token123  "
		concurrency := 5

		// When
		result := buildCommand(org, targetDir, token, concurrency)

		// Then
		expected := []string{"clone", "myorg", "--path", "/tmp/repos", "--concurrency", "5", "--skip-archived", "--token", "token123"}
		assert.Equal(t, expected, result)
	})

	t.Run("Given zero concurrency When building command Then should default to 1", func(t *testing.T) {
		// Given
		org := "myorg"
		targetDir := "/tmp/repos"
		token := "token123"
		concurrency := 0

		// When
		result := buildCommand(org, targetDir, token, concurrency)

		// Then
		expected := []string{"clone", "myorg", "--path", "/tmp/repos", "--concurrency", "1", "--skip-archived", "--token", "token123"}
		assert.Equal(t, expected, result)
	})

	t.Run("Given negative concurrency When building command Then should default to 1", func(t *testing.T) {
		// Given
		org := "myorg"
		targetDir := "/tmp/repos"
		token := "token123"
		concurrency := -5

		// When
		result := buildCommand(org, targetDir, token, concurrency)

		// Then
		expected := []string{"clone", "myorg", "--path", "/tmp/repos", "--concurrency", "1", "--skip-archived", "--token", "token123"}
		assert.Equal(t, expected, result)
	})

	t.Run("Given whitespace-only token When building command Then should use no-token", func(t *testing.T) {
		// Given
		org := "myorg"
		targetDir := "/tmp/repos"
		token := "   "
		concurrency := 5

		// When
		result := buildCommand(org, targetDir, token, concurrency)

		// Then
		expected := []string{"clone", "myorg", "--path", "/tmp/repos", "--concurrency", "5", "--skip-archived", "--no-token"}
		assert.Equal(t, expected, result)
	})
}

// Test extractRepoCount function following Given-When-Then pattern
func TestExtractRepoCount(t *testing.T) {
	t.Run("Given line with repo count When extracting Then should return correct count", func(t *testing.T) {
		// Given
		line := "5 repos found for organization"

		// When
		result := extractRepoCount(line)

		// Then
		assert.Equal(t, 5, result)
	})

	t.Run("Given line with different repo count format When extracting Then should return correct count", func(t *testing.T) {
		// Given
		line := "found 12 repositories for organization"

		// When
		result := extractRepoCount(line)

		// Then
		assert.Equal(t, 12, result)
	})

	t.Run("Given line with single repo When extracting Then should return 1", func(t *testing.T) {
		// Given
		line := "1 repo found for organization"

		// When
		result := extractRepoCount(line)

		// Then
		assert.Equal(t, 1, result)
	})

	t.Run("Given line without repo count When extracting Then should return 0", func(t *testing.T) {
		// Given
		line := "some other message without count"

		// When
		result := extractRepoCount(line)

		// Then
		assert.Equal(t, 0, result)
	})

	t.Run("Given line with invalid number When extracting Then should return 0", func(t *testing.T) {
		// Given
		line := "invalid repos found for organization"

		// When
		result := extractRepoCount(line)

		// Then
		assert.Equal(t, 0, result)
	})

	t.Run("Given mixed case line When extracting Then should handle case insensitivity", func(t *testing.T) {
		// Given
		line := "Found 7 Repositories for Organization"

		// When
		result := extractRepoCount(line)

		// Then
		assert.Equal(t, 7, result)
	})
}

// Test extractRepoName function following Given-When-Then pattern
func TestExtractRepoName(t *testing.T) {
	t.Run("Given line with cloning message When extracting Then should return repo name", func(t *testing.T) {
		// Given
		line := "cloning org/repo-name"

		// When
		result := extractRepoName(line)

		// Then
		assert.Equal(t, "org/repo-name", result)
	})

	t.Run("Given line with successfully cloned message When extracting Then should return repo name", func(t *testing.T) {
		// Given
		line := "successfully cloned myorg/awesome-repo"

		// When
		result := extractRepoName(line)

		// Then
		assert.Equal(t, "myorg/awesome-repo", result)
	})

	t.Run("Given line with already exists message When extracting Then should return repo name", func(t *testing.T) {
		// Given
		line := "org/existing-repo already exists"

		// When
		result := extractRepoName(line)

		// Then
		assert.Equal(t, "org/existing-repo", result)
	})

	t.Run("Given line with processing message When extracting Then should return repo name", func(t *testing.T) {
		// Given
		line := "processing testorg/sample-project"

		// When
		result := extractRepoName(line)

		// Then
		assert.Equal(t, "testorg/sample-project", result)
	})

	t.Run("Given line without repo name When extracting Then should return empty string", func(t *testing.T) {
		// Given
		line := "some other message without repo name"

		// When
		result := extractRepoName(line)

		// Then
		assert.Equal(t, "", result)
	})

	t.Run("Given mixed case line When extracting Then should handle case insensitivity", func(t *testing.T) {
		// Given
		line := "Cloning MyOrg/MyRepo"

		// When
		result := extractRepoName(line)

		// Then
		assert.Equal(t, "myorg/myrepo", result)
	})
}

// Test createBatches function following Given-When-Then pattern
func TestCreateBatches(t *testing.T) {
	t.Run("Given orgs and batch size When creating batches Then should create correct batches", func(t *testing.T) {
		// Given
		orgs := []string{"org1", "org2", "org3", "org4", "org5"}
		size := 2

		// When
		result := createBatches(orgs, size)

		// Then
		assert.Len(t, result, 3)
		assert.Equal(t, []string{"org1", "org2"}, result[0].Orgs)
		assert.Equal(t, []string{"org3", "org4"}, result[1].Orgs)
		assert.Equal(t, []string{"org5"}, result[2].Orgs)
		assert.Equal(t, 1, result[0].BatchNumber)
		assert.Equal(t, 2, result[1].BatchNumber)
		assert.Equal(t, 3, result[2].BatchNumber)
	})

	t.Run("Given empty orgs When creating batches Then should return empty", func(t *testing.T) {
		// Given
		orgs := []string{}
		size := 2

		// When
		result := createBatches(orgs, size)

		// Then
		assert.Equal(t, []Batch{}, result)
	})

	t.Run("Given orgs with zero size When creating batches Then should default to min(50, len(orgs))", func(t *testing.T) {
		// Given
		orgs := []string{"org1", "org2"}
		size := 0

		// When
		result := createBatches(orgs, size)

		// Then
		assert.Len(t, result, 1)
		assert.Equal(t, []string{"org1", "org2"}, result[0].Orgs)
	})

	t.Run("Given orgs with negative size When creating batches Then should default to min(50, len(orgs))", func(t *testing.T) {
		// Given
		orgs := []string{"org1", "org2", "org3"}
		size := -1

		// When
		result := createBatches(orgs, size)

		// Then
		assert.Len(t, result, 1)
		assert.Equal(t, []string{"org1", "org2", "org3"}, result[0].Orgs)
	})

	t.Run("Given large batch size When creating batches Then should cap at orgs length", func(t *testing.T) {
		// Given
		orgs := []string{"org1", "org2", "org3"}
		size := 100

		// When
		result := createBatches(orgs, size)

		// Then
		assert.Len(t, result, 1)
		assert.Equal(t, []string{"org1", "org2", "org3"}, result[0].Orgs)
	})

	t.Run("Given many orgs with default size When creating batches Then should cap at 50", func(t *testing.T) {
		// Given
		orgs := make([]string, 100)
		for i := 0; i < 100; i++ {
			orgs[i] = fmt.Sprintf("org%d", i)
		}
		size := 0

		// When
		result := createBatches(orgs, size)

		// Then
		assert.Len(t, result, 2) // 100 orgs / 50 batch size = 2 batches
		assert.Len(t, result[0].Orgs, 50)
		assert.Len(t, result[1].Orgs, 50)
	})
}

// Test collectAndReportErrors function following Given-When-Then pattern
func TestCollectAndReportErrors(t *testing.T) {
	t.Run("Given no errors When collecting Then should return nil", func(t *testing.T) {
		// Given
		allErrors := []error{}
		logInfo, _ := createLoggers()

		// When
		result := collectAndReportErrors(allErrors, logInfo)

		// Then
		assert.NoError(t, result)
	})

	t.Run("Given single error When collecting Then should return formatted error", func(t *testing.T) {
		// Given
		allErrors := []error{fmt.Errorf("test error")}
		logInfo, _ := createLoggers()

		// When
		result := collectAndReportErrors(allErrors, logInfo)

		// Then
		assert.Error(t, result)
		assert.Contains(t, result.Error(), "processing completed with 1 errors")
		assert.Contains(t, result.Error(), "test error")
	})

	t.Run("Given multiple errors When collecting Then should return formatted error with all messages", func(t *testing.T) {
		// Given
		allErrors := []error{
			fmt.Errorf("first error"),
			fmt.Errorf("second error"),
			fmt.Errorf("third error"),
		}
		logInfo, _ := createLoggers()

		// When
		result := collectAndReportErrors(allErrors, logInfo)

		// Then
		assert.Error(t, result)
		assert.Contains(t, result.Error(), "processing completed with 3 errors")
		assert.Contains(t, result.Error(), "first error")
		assert.Contains(t, result.Error(), "second error")
		assert.Contains(t, result.Error(), "third error")
	})
}

// Property-based tests using rapid
func TestUtilityFunctionProperties(t *testing.T) {
	t.Run("Property: chunkSlice preserves all elements", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate random slice and chunk size
			input := rapid.SliceOf(rapid.String()).Draw(t, "input")
			size := rapid.IntRange(1, 100).Draw(t, "size")

			chunks := chunkSlice(input, size)

			// Property: All elements are preserved
			totalElements := 0
			for _, chunk := range chunks {
				totalElements += len(chunk)
			}
			assert.Equal(t, len(input), totalElements)

			// Property: Elements maintain their relative order
			reconstructed := make([]string, 0, len(input))
			for _, chunk := range chunks {
				reconstructed = append(reconstructed, chunk...)
			}
			assert.Equal(t, input, reconstructed)
		})
	})

	t.Run("Property: buildCommand always starts with clone", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			org := rapid.String().Draw(t, "org")
			targetDir := rapid.String().Draw(t, "targetDir")
			token := rapid.String().Draw(t, "token")
			concurrency := rapid.IntRange(1, 1000).Draw(t, "concurrency")

			cmd := buildCommand(org, targetDir, token, concurrency)

			assert.True(t, len(cmd) > 0)
			assert.Equal(t, "clone", cmd[0])
		})
	})

	t.Run("Property: expandPath handles tilde consistently", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			homeDir := rapid.StringMatching(`^/[a-zA-Z0-9/_-]*$`).Draw(t, "homeDir")
			path := rapid.String().Draw(t, "path")

			result := expandPath(path, homeDir)

			// Property: Tilde expansion works correctly
			if len(path) >= 2 && path[:2] == "~/" {
				if path == "~/" {
					assert.Equal(t, homeDir, result)
				} else {
					expected := homeDir + "/" + path[2:]
					assert.Equal(t, expected, result)
				}
			} else {
				// Non-tilde paths remain unchanged
				assert.Equal(t, path, result)
			}
		})
	})

	t.Run("Property: min and max are pure functions", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			a := rapid.Int().Draw(t, "a")
			b := rapid.Int().Draw(t, "b")

			minResult := min(a, b)
			maxResult := max(a, b)

			// Property: min returns the smaller value
			assert.True(t, minResult <= a && minResult <= b)
			assert.True(t, minResult == a || minResult == b)

			// Property: max returns the larger value
			assert.True(t, maxResult >= a && maxResult >= b)
			assert.True(t, maxResult == a || maxResult == b)

			// Property: min and max are deterministic
			assert.Equal(t, minResult, min(a, b))
			assert.Equal(t, maxResult, max(a, b))
		})
	})
}

// Integration tests for utility functions
func TestUtilityFunctionIntegration(t *testing.T) {
	t.Run("Given complete batch processing workflow When using utilities Then should work together", func(t *testing.T) {
		// Given
		orgs := []string{"org1", "org2", "org3", "org4", "org5"}
		batchSize := 2
		targetDir := "~/repos"
		homeDir := "/home/user"
		token := "test-token"
		concurrency := 5

		// When
		batches := createBatches(orgs, batchSize)
		expandedDir := expandPath(targetDir, homeDir)
		cmd := buildCommand(orgs[0], expandedDir, token, concurrency)

		// Then
		assert.Len(t, batches, 3)
		assert.Equal(t, "/home/user/repos", expandedDir)
		assert.Contains(t, cmd, "clone")
		assert.Contains(t, cmd, "org1")
		assert.Contains(t, cmd, expandedDir)
		assert.Contains(t, cmd, "test-token")
	})
}

// Stress tests for utility functions
func TestUtilityFunctionStress(t *testing.T) {
	t.Run("Given large input When chunking Then should handle efficiently", func(t *testing.T) {
		// Given
		largeSlice := make([]string, 10000)
		for i := 0; i < 10000; i++ {
			largeSlice[i] = fmt.Sprintf("item%d", i)
		}

		// When
		start := time.Now()
		chunks := chunkSlice(largeSlice, 100)
		duration := time.Since(start)

		// Then
		assert.Len(t, chunks, 100)
		assert.True(t, duration < 100*time.Millisecond, "Should be efficient")
	})

	t.Run("Given many concurrent buffer operations When writing Then should handle thread safety", func(t *testing.T) {
		// Given
		buffer := NewBoundedBuffer(1000)
		done := make(chan bool)
		numWorkers := 10

		// When
		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				for j := 0; j < 100; j++ {
					buffer.Write(fmt.Sprintf("worker%d-line%d", workerID, j))
				}
				done <- true
			}(i)
		}

		// Wait for all workers
		for i := 0; i < numWorkers; i++ {
			<-done
		}

		// Then
		result := buffer.GetAll()
		assert.Len(t, result, 1000) // Should have exactly 1000 items (buffer size)
	})
}

// Mutation tests for error handling
func TestUtilityFunctionMutations(t *testing.T) {
	t.Run("Given regex patterns When parsing invalid input Then should handle gracefully", func(t *testing.T) {
		// Test regex pattern resilience
		testCases := []string{
			"",
			"invalid line",
			"123 invalid format",
			"found abc repositories", // non-numeric
			"cloning invalid/repo/name/format",
		}

		for _, testCase := range testCases {
			t.Run(fmt.Sprintf("input_%s", testCase), func(t *testing.T) {
				// These should not panic
				assert.NotPanics(t, func() {
					extractRepoCount(testCase)
					extractRepoName(testCase)
				})
			})
		}
	})

	t.Run("Given nil inputs When calling functions Then should handle gracefully", func(t *testing.T) {
		// Test nil safety
		assert.NotPanics(t, func() {
			createBatches(nil, 1)
		})

		assert.NotPanics(t, func() {
			collectAndReportErrors(nil, nil)
		})
	})

	t.Run("Given extreme values When calling functions Then should handle gracefully", func(t *testing.T) {
		// Test extreme values
		assert.NotPanics(t, func() {
			min(int(^uint(0)>>1), -int(^uint(0)>>1)-1) // Max int, min int
			max(int(^uint(0)>>1), -int(^uint(0)>>1)-1)
		})

		assert.NotPanics(t, func() {
			buildCommand("", "", "", 0)
			buildCommand("org", "dir", "token", -1000)
		})
	})
}

// Edge case tests
func TestUtilityFunctionEdgeCases(t *testing.T) {
	t.Run("Given regex compilation When starting Then should have valid patterns", func(t *testing.T) {
		// Test that global regex patterns are valid
		assert.NotNil(t, repoCountPattern)
		assert.NotNil(t, repoNamePatterns)
		assert.True(t, len(repoNamePatterns) > 0)

		// Test patterns can be used
		assert.NotPanics(t, func() {
			repoCountPattern.FindStringSubmatch("5 repos found")
			for _, pattern := range repoNamePatterns {
				pattern.FindStringSubmatch("cloning org/repo")
			}
		})
	})

	t.Run("Given buffer with zero capacity When creating Then should handle gracefully", func(t *testing.T) {
		// Given
		buffer := NewBoundedBuffer(0)

		// When/Then - should not panic
		assert.NotPanics(t, func() {
			buffer.Write("test")
			buffer.GetAll()
		})
	})

	t.Run("Given special characters in paths When expanding Then should handle correctly", func(t *testing.T) {
		testCases := []struct {
			path, homeDir, expected string
		}{
			{"~/path with spaces", "/home/user", "/home/user/path with spaces"},
			{"~/path/with/unicode/测试", "/home/user", "/home/user/path/with/unicode/测试"},
			{"~/", "/home/user with spaces", "/home/user with spaces"},
			{"~/../relative", "/home/user", "/home/user/../relative"},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("path_%s", tc.path), func(t *testing.T) {
				result := expandPath(tc.path, tc.homeDir)
				assert.Equal(t, tc.expected, result)
			})
		}
	})
}

// Table-driven tests for comprehensive coverage
func TestUtilityFunctionTableDriven(t *testing.T) {
	t.Run("buildCommand table-driven tests", func(t *testing.T) {
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
				expected:    []string{"clone", "myorg", "--path", "/tmp/repos", "--concurrency", "5", "--skip-archived", "--token", "token123"},
			},
			{
				name:        "without token",
				org:         "myorg",
				targetDir:   "/tmp/repos",
				token:       "",
				concurrency: 5,
				expected:    []string{"clone", "myorg", "--path", "/tmp/repos", "--concurrency", "5", "--skip-archived", "--no-token"},
			},
			{
				name:        "high concurrency",
				org:         "testorg",
				targetDir:   "/home/user/repos",
				token:       "abc123",
				concurrency: 100,
				expected:    []string{"clone", "testorg", "--path", "/home/user/repos", "--concurrency", "100", "--skip-archived", "--token", "abc123"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := buildCommand(tt.org, tt.targetDir, tt.token, tt.concurrency)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("expandPath table-driven tests", func(t *testing.T) {
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
	})
}