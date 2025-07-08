package main

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	F "github.com/IBM/fp-go/function"
)

// Pre-compiled regex patterns for performance optimization
var (
	// repoCountPattern extracts the total number of repositories found.
	repoCountPattern = regexp.MustCompile(`(\d+)\s+repos?\s+found|found\s+(\d+)\s+repositor`)
	// repoNamePatterns extracts the repository name from various ghorg output lines.
	repoNamePatterns = []*regexp.Regexp{
		regexp.MustCompile(`cloning\s+([^/\s]+/[^/\s]+)`),
		regexp.MustCompile(`successfully cloned\s+([^/\s]+/[^/\s]+)`),
		regexp.MustCompile(`([^/\s]+/[^/\s]+)\s+already exists`),
		regexp.MustCompile(`processing\s+([^/\s]+/[^/\s]+)`),
	}
)

// BoundedBuffer implements a simple circular buffer with a fixed size.
// This is used to store command output without consuming unbounded memory.
type BoundedBuffer struct {
	buffer  []string
	head    int
	tail    int
	size    int
	maxSize int
	mutex   sync.RWMutex
}

// NewBoundedBuffer creates a new bounded buffer with a specified maximum size.
func NewBoundedBuffer(maxSize int) *BoundedBuffer {
	return &BoundedBuffer{
		buffer:  make([]string, maxSize),
		maxSize: maxSize,
	}
}

// Write adds a line to the buffer, overwriting the oldest entry if the buffer is full.
func (bb *BoundedBuffer) Write(line string) {
	bb.mutex.Lock()
	defer bb.mutex.Unlock()

	bb.buffer[bb.tail] = line
	bb.tail = (bb.tail + 1) % bb.maxSize

	if bb.size < bb.maxSize {
		bb.size++
	} else {
		// Buffer is full, move head forward
		bb.head = (bb.head + 1) % bb.maxSize
	}
}

// GetAll returns all lines currently in the buffer in chronological order.
func (bb *BoundedBuffer) GetAll() []string {
	bb.mutex.RLock()
	defer bb.mutex.RUnlock()

	if bb.size == 0 {
		return []string{}
	}

	result := make([]string, bb.size)
	for i := 0; i < bb.size; i++ {
		result[i] = bb.buffer[(bb.head+i)%bb.maxSize]
	}
	return result
}

// Functional adapters - No methods, only pure functions and side-effect handlers

// CreateGhorgOrgCloner creates a functional organization cloner without progress tracking.
// It serves as a simple entry point for the cloning functionality.
func CreateGhorgOrgCloner() OrgCloner {
	return CreateGhorgOrgClonerWithProgress(nil)
}

// CreateGhorgOrgClonerWithProgress creates a functional organization cloner that accepts a progress callback function.
// This allows for real-time progress reporting during the cloning process.
func CreateGhorgOrgClonerWithProgress(progressFunc ProgressFunc) OrgCloner {
	return func(ctx context.Context, org, targetDir, token string, concurrency int) error {
		logInfo, logError := createLoggers()

		return F.Pipe1(ctx, func(ctx context.Context) error {
			logInfo("Starting organization clone operation",
				slog.String("org", org),
				slog.String("target_dir", targetDir),
				slog.Int("concurrency", concurrency),
				slog.String("operation", "clone_start"))

			// Check cancellation
			select {
			case <-ctx.Done():
				logError("Clone operation cancelled",
					slog.String("org", org),
					slog.String("error", ctx.Err().Error()),
					slog.String("operation", "clone_cancelled"))
				return createError("clone operation cancelled for org %s: %v", org, ctx.Err())
			default:
			}

			// Get home directory and expand path
			logInfo("Getting home directory",
				slog.String("org", org),
				slog.String("operation", "home_dir_lookup"))

			homeDir, err := os.UserHomeDir()
			if err != nil {
				logError("Failed to get home directory",
					slog.String("org", org),
					slog.String("error", err.Error()),
					slog.String("operation", "home_dir_failed"))
				return createError("failed to get home directory: %v", err)
			}

			expandedDir := expandPath(targetDir, homeDir)

			logInfo("Path expanded",
				slog.String("org", org),
				slog.String("original_path", targetDir),
				slog.String("expanded_path", expandedDir),
				slog.String("operation", "path_expanded"))

			// Create directory
			logInfo("Creating target directory",
				slog.String("org", org),
				slog.String("directory", expandedDir),
				slog.String("operation", "dir_creation"))

			if err := os.MkdirAll(expandedDir, 0755); err != nil {
				logError("Failed to create directory",
					slog.String("org", org),
					slog.String("directory", expandedDir),
					slog.String("error", err.Error()),
					slog.String("operation", "dir_creation_failed"))
				return createError("failed to create directory %s: %v", expandedDir, err)
			}

			logInfo("Directory created successfully",
				slog.String("org", org),
				slog.String("directory", expandedDir),
				slog.String("operation", "dir_created"))

			// Build and execute command
			args := buildCommand(org, expandedDir, token, concurrency)

			logInfo("Executing ghorg command",
				slog.String("org", org),
				slog.Any("command_args", args),
				slog.String("operation", "command_exec"))

			cmd := exec.CommandContext(ctx, "ghorg", args...)
			cmd.Env = os.Environ()

			// Set up pipes for real-time output capture
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				logError("Failed to create stdout pipe",
					slog.String("org", org),
					slog.String("error", err.Error()),
					slog.String("operation", "stdout_pipe_failed"))
				return createError("failed to create stdout pipe for org %s: %v", org, err)
			}

			stderr, err := cmd.StderrPipe()
			if err != nil {
				logError("Failed to create stderr pipe",
					slog.String("org", org),
					slog.String("error", err.Error()),
					slog.String("operation", "stderr_pipe_failed"))
				return createError("failed to create stderr pipe for org %s: %v", org, err)
			}

			// Start the command
			if err := cmd.Start(); err != nil {
				logError("Failed to start ghorg command",
					slog.String("org", org),
					slog.String("error", err.Error()),
					slog.Any("command_args", args),
					slog.String("operation", "command_start_failed"))
				return createError("failed to start ghorg command for org %s: %v", org, err)
			}

			logInfo("Ghorg command started, monitoring progress",
				slog.String("org", org),
				slog.Int("pid", cmd.Process.Pid),
				slog.String("operation", "command_started"))

			// Capture output in real-time using goroutines with bounded buffer
			var wg sync.WaitGroup
			// Use bounded buffer to prevent unbounded memory growth (max 1000 lines)
			outputBuffer := NewBoundedBuffer(1000)
			var repoCount int
			var processedRepos int

			// Function to capture and log output lines with progress tracking
			captureOutput := func(reader io.Reader, outputType string) {
				defer wg.Done()
				scanner := bufio.NewScanner(reader)
				for scanner.Scan() {
					line := scanner.Text()

					// Thread-safe output collection using bounded buffer
					outputBuffer.Write(line) // Will overwrite old data if buffer is full

					// Parse ghorg output for progress information
					if strings.Contains(line, "repos found") {
						if count := extractRepoCount(line); count > 0 {
							repoCount = count
							logInfo("Repository count detected",
								slog.String("org", org),
								slog.Int("repo_count", repoCount),
								slog.String("operation", "repo_count_detected"))
						}
					}

					// Track individual repo completion
					if strings.Contains(line, "successfully cloned") ||
						strings.Contains(line, "already exists") ||
						strings.Contains(line, "cloning") {
						if repoName := extractRepoName(line); repoName != "" {
							processedRepos++

							// Report repo-level progress if progress function is provided
							if progressFunc != nil && repoCount > 0 {
								update := ProgressUpdate{
									Org:           org,
									CompletedOrgs: processedRepos,
									TotalOrgs:     repoCount,
									Status:        "cloning_repo",
								}
								progressFunc(update)
							}

							logInfo("Repository progress",
								slog.String("org", org),
								slog.String("repo", repoName),
								slog.Int("processed_repos", processedRepos),
								slog.Int("total_repos", repoCount),
								slog.Float64("repo_percentage", float64(processedRepos)/float64(max(repoCount, 1))*100),
								slog.String("operation", "repo_progress"))
						}
					}

					// Real-time progress logging
					logInfo("Ghorg command output",
						slog.String("org", org),
						slog.String("output_type", outputType),
						slog.String("line", line),
						slog.String("operation", "command_progress"))
				}
				if err := scanner.Err(); err != nil {
					logError("Error reading command output",
						slog.String("org", org),
						slog.String("output_type", outputType),
						slog.String("error", err.Error()),
						slog.String("operation", "output_read_error"))
				}
			}

			// Start output capture goroutines
			wg.Add(2)
			go captureOutput(stdout, "stdout")
			go captureOutput(stderr, "stderr")

			// Wait for command completion
			cmdErr := cmd.Wait()

			// Wait for all output to be captured
			wg.Wait()

			// Collect all output from bounded buffer
			allOutput := outputBuffer.GetAll()

			if cmdErr != nil {
				logError("Ghorg command failed",
					slog.String("org", org),
					slog.String("error", cmdErr.Error()),
					slog.Any("command_args", args),
					slog.Int("output_lines", len(allOutput)),
					slog.String("operation", "command_failed"))

				// Include output in error for debugging
				outputStr := ""
				if len(allOutput) > 0 {
					for _, line := range allOutput {
						outputStr += line + "\n"
					}
				}
				return createError("ghorg command failed for org %s: %v\nOutput:\n%s", org, cmdErr, outputStr)
			}

			logInfo("Ghorg command completed successfully",
				slog.String("org", org),
				slog.Int("output_lines", len(allOutput)),
				slog.String("operation", "command_completed"))

			logInfo("Organization cloned successfully",
				slog.String("org", org),
				slog.String("target_dir", expandedDir),
				slog.String("operation", "clone_completed"))

			return nil
		})
	}
}

// Legacy wrapper for backwards compatibility

// GhorgOrgCloner provides a struct-based wrapper around the functional OrgCloner.
// This is maintained for compatibility with any older code that uses a struct-based approach.
type GhorgOrgCloner struct {
	cloner OrgCloner
}

// NewGhorgOrgCloner creates a new instance of the legacy GhorgOrgCloner wrapper.
func NewGhorgOrgCloner() *GhorgOrgCloner {
	return &GhorgOrgCloner{
		cloner: CreateGhorgOrgCloner(),
	}
}

// CloneOrg executes the cloning operation for the legacy wrapper.
func (c *GhorgOrgCloner) CloneOrg(ctx context.Context, org, targetDir, token string, concurrency int) error {
	return c.cloner(ctx, org, targetDir, token, concurrency)
}

// Utility functions for parsing ghorg output

// extractRepoCount extracts the repository count from a line of ghorg output using a pre-compiled regex.
func extractRepoCount(line string) int {
	// Use pre-compiled regex pattern for performance
	matches := repoCountPattern.FindStringSubmatch(strings.ToLower(line))

	if len(matches) > 1 {
		for _, match := range matches[1:] {
			if match != "" {
				if count, err := strconv.Atoi(match); err == nil {
					return count
				}
			}
		}
	}
	return 0
}

// extractRepoName extracts the repository name from a line of ghorg output using pre-compiled regex patterns.
func extractRepoName(line string) string {
	// Use pre-compiled regex patterns for performance
	lowerLine := strings.ToLower(line)
	for _, pattern := range repoNamePatterns {
		if matches := pattern.FindStringSubmatch(lowerLine); len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}
