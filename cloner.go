package main

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// CloneResult represents the outcome of a cloning operation
type CloneResult struct {
	Org            string
	RepoCount      int
	ProcessedRepos int
	Output         []string
	Success        bool
}

// OutputProcessor handles command output processing
type OutputProcessor func(line string) CloneResult

// DirectoryPreparer handles target directory preparation
type DirectoryPreparer func(ctx context.Context, targetDir string) (string, error)

// CommandExecutor handles command execution
type CommandExecutor func(ctx context.Context, org, expandedDir, token string, concurrency int) (*exec.Cmd, error)

// OutputCapturer handles output capture and processing
type OutputCapturer func(ctx context.Context, cmd *exec.Cmd, org string, progressFunc ProgressFunc) (CloneResult, error)

// Organization cloning functionality - Pure function factory
func CreateGhorgOrgCloner() OrgCloner {
	return CreateGhorgOrgClonerWithProgress(nil)
}

// Create cloner with progress tracking - Pure function factory
func CreateGhorgOrgClonerWithProgress(progressFunc ProgressFunc) OrgCloner {
	// Compose the cloning pipeline using pure functions
	dirPreparer := createDirectoryPreparer()
	cmdExecutor := createCommandExecutor()
	outputCapturer := createOutputCapturer()

	return func(ctx context.Context, org, targetDir, token string, concurrency int) error {
		logInfo, _ := createLoggers()
		defer trackExecutionTime(logInfo, "cloneOrg_"+org)()
		
		start := time.Now()
		
		// Check context cancellation early
		if err := checkContextCancellation(ctx, org); err != nil {
			return err
		}

		// Prepare target directory
		expandedDir, err := dirPreparer(ctx, targetDir)
		if err != nil {
			return err
		}

		// Execute ghorg command
		cmd, err := cmdExecutor(ctx, org, expandedDir, token, concurrency)
		if err != nil {
			return err
		}

		// Capture and process output
		result, err := outputCapturer(ctx, cmd, org, progressFunc)
		if err != nil {
			return err
		}

		// Log success and return result
		err = logCloneCompletion(result, expandedDir)
		
		// Log timing information
		elapsed := time.Since(start)
		logInfo("Clone operation completed",
			slog.String("org", org),
			slog.String("duration", elapsed.String()),
			slog.Int("repo_count", result.RepoCount),
			slog.Int("processed_repos", result.ProcessedRepos),
			slog.Bool("success", result.Success),
			slog.String("operation", "clone_timing"))
		
		return err
	}
}

// Pure function to check context cancellation
func checkContextCancellation(ctx context.Context, org string) error {
	select {
	case <-ctx.Done():
		return createError("clone operation cancelled for org %s: %v", org, ctx.Err())
	default:
		return nil
	}
}

// Pure function factory for directory preparation
func createDirectoryPreparer() DirectoryPreparer {
	return func(ctx context.Context, targetDir string) (string, error) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", createError("failed to get home directory: %v", err)
		}

		expandedDir := expandPath(targetDir, homeDir)

		if err := os.MkdirAll(expandedDir, 0755); err != nil {
			return "", createError("failed to create directory %s: %v", expandedDir, err)
		}

		return expandedDir, nil
	}
}

// Pure function factory for command execution
func createCommandExecutor() CommandExecutor {
	return func(ctx context.Context, org, expandedDir, token string, concurrency int) (*exec.Cmd, error) {
		args := buildCommand(org, expandedDir, token, concurrency)
		cmd := exec.CommandContext(ctx, "ghorg", args...)
		cmd.Env = os.Environ()

		// Don't start the command here - let the output capturer handle it
		return cmd, nil
	}
}

// Pure function factory for output capture
func createOutputCapturer() OutputCapturer {
	return func(ctx context.Context, cmd *exec.Cmd, org string, progressFunc ProgressFunc) (CloneResult, error) {
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return CloneResult{}, createError("failed to create stdout pipe for org %s: %v", org, err)
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			return CloneResult{}, createError("failed to create stderr pipe for org %s: %v", org, err)
		}

		// Start the command after pipes are set up
		if err := cmd.Start(); err != nil {
			return CloneResult{}, createError("failed to start ghorg command for org %s: %v", org, err)
		}

		// Process outputs using functional approach
		result := processCommandOutput(stdout, stderr, org, progressFunc)
		cmdErr := cmd.Wait()

		if cmdErr != nil {
			outputStr := strings.Join(result.Output, "\n")
			return result, createError("ghorg command failed for org %s: %v\nOutput:\n%s", org, cmdErr, outputStr)
		}

		result.Success = true
		return result, nil
	}
}

// Pure function to process command output
func processCommandOutput(stdout, stderr io.Reader, org string, progressFunc ProgressFunc) CloneResult {
	logInfo, _ := createLoggers()
	outputBuffer := NewBoundedBuffer(1000)

	var wg sync.WaitGroup
	var repoCount int
	var processedRepos int

	// Pure function for line processing
	processLine := func(line string) {
		outputBuffer.Write(line)

		// Extract repository count
		if strings.Contains(line, "repos found") {
			if count := extractRepoCount(line); count > 0 {
				repoCount = count
				// Track repository discovery in metrics
				metrics := getCurrentMetrics()
				metrics.TrackRepoDiscovery(count)
				logInfo("Repository count detected",
					slog.String("org", org),
					slog.Int("repo_count", repoCount),
					slog.String("operation", "repo_count_detected"))
			}
		}

		// Track repository processing
		if isRepoProcessingLine(line) {
			if repoName := extractRepoName(line); repoName != "" {
				processedRepos++
				// Track repository processing in metrics
				metrics := getCurrentMetrics()
				metrics.TrackRepoProcessed(1)
				sendProgressUpdate(progressFunc, org, processedRepos, repoCount)
			}
		}
	}

	// Capture output from both streams
	captureStream := func(reader io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			processLine(scanner.Text())
		}
	}

	wg.Add(2)
	go captureStream(stdout)
	go captureStream(stderr)
	wg.Wait()

	// Return immutable result
	return CloneResult{
		Org:            org,
		RepoCount:      repoCount,
		ProcessedRepos: processedRepos,
		Output:         outputBuffer.GetAll(),
		Success:        false, // Will be set by caller
	}
}

// Pure function to check if line indicates repository processing
func isRepoProcessingLine(line string) bool {
	return strings.Contains(line, "successfully cloned") ||
		strings.Contains(line, "already exists") ||
		strings.Contains(line, "cloning")
}

// Pure function to send progress update
func sendProgressUpdate(progressFunc ProgressFunc, org string, processedRepos, repoCount int) {
	if progressFunc != nil && repoCount > 0 {
		update := ProgressUpdate{
			Org:           org,
			CompletedOrgs: processedRepos,
			TotalOrgs:     repoCount,
			Status:        "cloning_repo",
		}
		progressFunc(update)
	}
}

// Pure function to log clone completion
func logCloneCompletion(result CloneResult, expandedDir string) error {
	if !result.Success {
		return createError("clone operation failed for org %s", result.Org)
	}

	logInfo, _ := createLoggers()
	logInfo("Organization cloned successfully",
		slog.String("org", result.Org),
		slog.String("target_dir", expandedDir),
		slog.Int("repo_count", result.RepoCount),
		slog.Int("processed_repos", result.ProcessedRepos),
		slog.String("operation", "clone_completed"))

	return nil
}
