package main

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	F "github.com/IBM/fp-go/function"
)

// Core types - minimal, no duplication
type Config struct {
	// Orgs specifies the list of GitHub organizations to clone.
	Orgs []string `envconfig:"ORGS" json:"orgs"`
	// BatchSize defines the number of repositories to process in each batch.
	BatchSize int `envconfig:"BATCH_SIZE" default:"50" json:"batch_size"`
	// Concurrency sets the number of concurrent cloning operations.
	Concurrency int `envconfig:"CONCURRENCY" default:"25" json:"concurrency"`
	// TargetDir is the local directory where repositories will be cloned.
	TargetDir string `envconfig:"TARGET_DIR" required:"true" json:"target_dir"`
	// GitHubToken provides a GitHub Personal Access Token for authentication.
	GitHubToken string `envconfig:"GITHUB_TOKEN" json:"github_token"`
}

// Batch represents a group of organizations to be processed together.
type Batch struct {
	// Orgs is the list of organization names in the batch.
	Orgs []string
	// BatchNumber is the identifier for this batch.
	BatchNumber int
}

// ProcessingPlan outlines the entire cloning operation, including all batches.
type ProcessingPlan struct {
	// Batches is the list of all batches to be processed.
	Batches []Batch
	// TotalOrgs is the total number of organizations to be cloned.
	TotalOrgs int
}

// Progress tracking types

// ProgressTracker holds the state of the cloning progress.
type ProgressTracker struct {
	// TotalOrgs is the total number of organizations to be processed.
	TotalOrgs int
	// ProcessedOrgs is the number of organizations that have been successfully cloned.
	ProcessedOrgs int
	// TotalBatches is the total number of batches to be processed.
	TotalBatches int
	// ProcessedBatches is the number of batches that have completed processing.
	ProcessedBatches int
	// StartTime is the time when the cloning process began.
	StartTime time.Time
	// CurrentOrg is the name of the organization currently being processed.
	CurrentOrg string
	// CurrentBatch is the number of the batch currently being processed.
	CurrentBatch int
}

// ProgressUpdate represents a snapshot of the cloning progress at a specific moment.
type ProgressUpdate struct {
	// Org is the name of the organization in the update.
	Org string
	// BatchNumber is the number of the batch being processed.
	BatchNumber int
	// CompletedOrgs is the number of organizations cloned so far.
	CompletedOrgs int
	// TotalOrgs is the total number of organizations to clone.
	TotalOrgs int
	// ElapsedTime is the time elapsed since the start of the process.
	ElapsedTime time.Duration
	// EstimatedETA is the estimated time remaining for completion.
	EstimatedETA time.Duration
	// Status provides a description of the current state (e.g., "processing", "completed").
	Status string
}

// ProgressFunc represents a function that handles progress updates.
type ProgressFunc func(update ProgressUpdate)

// DRY: ProcessingOptions removed - use Config directly

// validateConfig checks if the provided configuration is valid.
// It ensures that there are organizations to process and a target directory is specified.
// This is a pure function that returns an error if validation fails.
func validateConfig(config *Config) error {
	// Use functional approach to validate orgs
	validOrgs := F.Pipe1(config.Orgs, func(orgs []string) []string {
		var valid []string
		for _, org := range orgs {
			if trimmed := strings.TrimSpace(org); trimmed != "" {
				valid = append(valid, trimmed)
			}
		}
		return valid
	})

	if len(validOrgs) == 0 {
		return createError("no valid organizations specified")
	}

	if strings.TrimSpace(config.TargetDir) == "" {
		return createError("target directory cannot be empty")
	}

	// Update config with sanitized orgs
	config.Orgs = validOrgs
	return nil
}

// createBatches divides a list of organizations into smaller batches of a specified size.
// This pure function returns a slice of Batch structs.
func createBatches(orgs []string, size int) []Batch {
	if len(orgs) == 0 {
		return []Batch{}
	}

	// Use functional approach to determine optimal batch size
	batchSize := F.Pipe1(size, func(s int) int {
		if s <= 0 {
			return min(50, len(orgs)) // Don't create batches larger than input
		}
		return min(s, len(orgs)) // Ensure batch size doesn't exceed input size
	})

	return F.Pipe1(chunkSlice(orgs, batchSize), func(chunks [][]string) []Batch {
		batches := make([]Batch, 0, len(chunks))
		for i, chunk := range chunks {
			batches = append(batches, Batch{
				Orgs:        append([]string(nil), chunk...), // Immutable copy
				BatchNumber: i + 1,
			})
		}
		return batches
	})
}

// Helper function for minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Robust generic function to chunk a slice with input validation
func chunkSlice[T any](slice []T, size int) [][]T {
	if len(slice) == 0 {
		return [][]T{}
	}

	// Ensure size is positive
	if size <= 0 {
		size = 1
	}

	// Pre-allocate with exact capacity for efficiency
	numChunks := (len(slice) + size - 1) / size
	chunks := make([][]T, 0, numChunks)

	for i := 0; i < len(slice); i += size {
		end := min(i+size, len(slice))
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

// DRY: Single processing function combining validation and batching
// processConfig validates the configuration and creates a processing plan from it.
// This pure function returns a ProcessingPlan or an error if the config is invalid.
func processConfig(config *Config) (ProcessingPlan, error) {
	if err := validateConfig(config); err != nil {
		return ProcessingPlan{}, err
	}

	batches := createBatches(config.Orgs, config.BatchSize)
	return ProcessingPlan{Batches: batches, TotalOrgs: len(config.Orgs)}, nil
}

// Robust command builder with input validation and sanitization
// buildCommand constructs the command-line arguments for the ghorg CLI tool.
// This pure function returns a slice of strings representing the command and its arguments.
func buildCommand(org, targetDir, token string, concurrency int) []string {
	// Sanitize inputs
	org = strings.TrimSpace(org)
	targetDir = strings.TrimSpace(targetDir)
	token = strings.TrimSpace(token)

	// Ensure positive concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	// Build command with validated inputs
	base := []string{"clone", org, "--path", targetDir, "--concurrency", strconv.Itoa(concurrency), "--skip-archived"}

	return F.Pipe1(token, func(t string) []string {
		if t != "" {
			return append(base, "--token", t)
		}
		return append(base, "--no-token")
	})
}

// Robust path expansion with input validation
// expandPath expands the tilde (~) in a file path to the user's home directory.
// This pure function returns the expanded path as a string.
func expandPath(path, homeDir string) string {
	// Handle edge cases
	if path == "" {
		return ""
	}

	// Only expand if path starts with ~/
	if len(path) >= 2 && path[:2] == "~/" {
		// Handle special case of just ~/
		if path == "~/" {
			return homeDir
		}
		// Expand with proper path joining
		return F.Pipe1(homeDir, func(home string) string {
			if home == "" {
				return "/" + path[2:] // Fallback to root if no home
			}
			return home + "/" + path[2:]
		})
	}
	return path
}

// Progress tracking functions

// createProgressTracker creates a new progress tracker.
// This pure function initializes a ProgressTracker with the total number of orgs and batches.
func createProgressTracker(totalOrgs, totalBatches int) ProgressTracker {
	return ProgressTracker{
		TotalOrgs:        totalOrgs,
		ProcessedOrgs:    0,
		TotalBatches:     totalBatches,
		ProcessedBatches: 0,
		StartTime:        time.Now(),
		CurrentOrg:       "",
		CurrentBatch:     0,
	}
}

// calculateProgress computes progress statistics.
// This pure function takes a ProgressTracker and returns a ProgressUpdate with calculated metrics like ETA.
func calculateProgress(tracker ProgressTracker) ProgressUpdate {
	elapsed := time.Since(tracker.StartTime)

	// Calculate ETA based on org completion rate
	var eta time.Duration
	if tracker.ProcessedOrgs > 0 {
		avgTimePerOrg := elapsed / time.Duration(tracker.ProcessedOrgs)
		remainingOrgs := tracker.TotalOrgs - tracker.ProcessedOrgs
		eta = avgTimePerOrg * time.Duration(remainingOrgs)
	}

	return ProgressUpdate{
		Org:           tracker.CurrentOrg,
		BatchNumber:   tracker.CurrentBatch,
		CompletedOrgs: tracker.ProcessedOrgs,
		TotalOrgs:     tracker.TotalOrgs,
		ElapsedTime:   elapsed,
		EstimatedETA:  eta,
		Status:        "processing",
	}
}

// createProgressReporter creates a functional progress reporter.
// This function returns a ProgressFunc that logs progress updates using the provided logger.
func createProgressReporter(logInfo LogFunc) ProgressFunc {
	return func(update ProgressUpdate) {
		percentage := float64(update.CompletedOrgs) / float64(update.TotalOrgs) * 100

		logInfo("Progress update",
			slog.String("org", update.Org),
			slog.Int("batch_number", update.BatchNumber),
			slog.Int("completed_orgs", update.CompletedOrgs),
			slog.Int("total_orgs", update.TotalOrgs),
			slog.Float64("percentage", percentage),
			slog.Duration("elapsed_time", update.ElapsedTime),
			slog.Duration("estimated_eta", update.EstimatedETA),
			slog.String("status", update.Status),
			slog.String("operation", "progress_update"))
	}
}

// DRY: Single error creation function
// createError formats and returns a new error.
func createError(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
