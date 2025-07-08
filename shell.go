package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"

	F "github.com/IBM/fp-go/function"
	"github.com/hashicorp/go-multierror"
	ants "github.com/panjf2000/ants/v2"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// Functional Shell - No methods, only pure functions and side-effect handlers

// ConfigLoader loads configuration from environment or files
type ConfigLoader func(ctx context.Context, configPath string) (*Config, error)

// OrgCloner clones organizations
type OrgCloner func(ctx context.Context, org, targetDir, token string, concurrency int) error

// WorkflowResult represents the outcome of workflow execution
type WorkflowResult struct {
	Success bool
	Errors  []error
}

// Logger configuration as a pure value
type LoggerConfig struct {
	Level  slog.Level
	Output *os.File
}

// createLoggerHandler is a pure function that creates a log handler
func createLoggerHandler(config LoggerConfig) slog.Handler {
	opts := &slog.HandlerOptions{
		Level: config.Level,
	}
	return slog.NewJSONHandler(config.Output, opts)
}

// LogFunc represents a logging function
type LogFunc func(msg string, args ...slog.Attr)

// createLoggers returns logging functions for different levels
func createLoggers() (LogFunc, LogFunc) {
	config := LoggerConfig{Level: slog.LevelInfo, Output: os.Stdout}
	handler := createLoggerHandler(config)
	logger := slog.New(handler)

	logInfo := func(msg string, args ...slog.Attr) {
		logger.LogAttrs(context.Background(), slog.LevelInfo, msg, args...)
	}

	logError := func(msg string, args ...slog.Attr) {
		logger.LogAttrs(context.Background(), slog.LevelError, msg, args...)
	}

	return logInfo, logError
}

// ExecuteWorkflow orchestrates the complete workflow using functional composition
func ExecuteWorkflow(ctx context.Context, configPath string, loader ConfigLoader, cloner OrgCloner) error {
	logInfo, logError := createLoggers()

	return F.Pipe1(ctx, func(ctx context.Context) error {
		logInfo("Starting workflow execution",
			slog.String("config_path", configPath),
			slog.String("operation", "workflow_start"))

		// Early context check
		select {
		case <-ctx.Done():
			logError("Workflow cancelled before starting",
				slog.String("error", ctx.Err().Error()),
				slog.String("operation", "workflow_cancelled"))
			return createError("workflow cancelled before starting: %v", ctx.Err())
		default:
		}

		// Load and validate configuration
		logInfo("Loading configuration",
			slog.String("config_path", configPath),
			slog.String("operation", "config_loading"))

		config, err := loader(ctx, configPath)
		if err != nil {
			logError("Configuration loading failed",
				slog.String("error", err.Error()),
				slog.String("config_path", configPath),
				slog.String("operation", "config_loading_failed"))
			return createError("configuration loading failed: %v", err)
		}

		logInfo("Configuration loaded successfully",
			slog.Int("org_count", len(config.Orgs)),
			slog.Int("concurrency", config.Concurrency),
			slog.Int("batch_size", config.BatchSize),
			slog.String("target_dir", config.TargetDir),
			slog.String("operation", "config_loaded"))

		// Create processing plan with validation
		logInfo("Creating processing plan",
			slog.String("operation", "plan_creation"))

		plan, err := processConfig(config)
		if err != nil {
			logError("Processing plan creation failed",
				slog.String("error", err.Error()),
				slog.String("operation", "plan_creation_failed"))
			return createError("processing plan creation failed: %v", err)
		}

		logInfo("Processing plan created",
			slog.Int("total_orgs", plan.TotalOrgs),
			slog.Int("batch_count", len(plan.Batches)),
			slog.String("operation", "plan_created"))

		// Execute plan with proper error handling
		logInfo("Starting plan execution",
			slog.String("operation", "plan_execution_start"))

		err = executePlan(ctx, plan, config, cloner)
		if err != nil {
			logError("Plan execution failed",
				slog.String("error", err.Error()),
				slog.String("operation", "plan_execution_failed"))
		} else {
			logInfo("Workflow completed successfully",
				slog.String("operation", "workflow_completed"))
		}

		return err
	})
}

// BatchProcessor represents a function that processes a single batch
type BatchProcessor func(ctx context.Context, batch Batch, pool *ants.Pool, cloner OrgCloner, config *Config) []error

// ErrorCollector collects errors from batch processing
type ErrorCollector func([]error) error

// executePlan executes the processing plan using ants worker pool for enhanced concurrency
func executePlan(ctx context.Context, plan ProcessingPlan, config *Config, cloner OrgCloner) error {
	logInfo, logError := createLoggers()

	// Initialize progress tracking
	progressTracker := createProgressTracker(plan.TotalOrgs, len(plan.Batches))
	progressReporter := createProgressReporter(logInfo)

	// Handle empty plan gracefully using functional approach
	if len(plan.Batches) == 0 {
		logInfo("Empty processing plan, nothing to execute",
			slog.String("operation", "empty_plan"))
		return nil
	}

	logInfo("Initializing worker pool",
		slog.Int("pool_size", config.Concurrency),
		slog.String("operation", "pool_init"))

	// Create ants worker pool for efficient resource management
	pool, err := ants.NewPool(config.Concurrency, ants.WithOptions(ants.Options{
		PreAlloc:    true,
		Nonblocking: false,
	}))
	if err != nil {
		logError("Failed to create worker pool",
			slog.String("error", err.Error()),
			slog.Int("pool_size", config.Concurrency),
			slog.String("operation", "pool_creation_failed"))
		return createError("failed to create worker pool: %v", err)
	}
	defer func() {
		pool.Release()
		logInfo("Worker pool released",
			slog.String("operation", "pool_released"))
	}()

	logInfo("Worker pool created successfully",
		slog.Int("pool_size", config.Concurrency),
		slog.String("operation", "pool_created"))

	// Process all batches using functional map/reduce pattern with progress tracking
	allErrors := processBatches(ctx, plan.Batches, pool, cloner, config, logInfo, logError, &progressTracker, progressReporter)

	return collectAndReportErrors(allErrors, logInfo, logError)
}

// processBatches processes all batches with parallel processing and semaphore control
func processBatches(ctx context.Context, batches []Batch, pool *ants.Pool, cloner OrgCloner, config *Config, logInfo, logError LogFunc, progressTracker *ProgressTracker, progressReporter ProgressFunc) []error {
	// Create batch processor function with progress tracking
	processBatch := createBatchProcessor(logInfo, logError, progressTracker, progressReporter)

	// Create progress-aware cloner
	progressCloner := CreateGhorgOrgClonerWithProgress(progressReporter)

	// Calculate optimal concurrency for batch processing (max 3 concurrent batches)
	maxConcurrentBatches := int64(3)
	if len(batches) < 3 {
		maxConcurrentBatches = int64(len(batches))
	}

	// Use semaphore to control concurrent batch processing
	sem := semaphore.NewWeighted(maxConcurrentBatches)
	var result *multierror.Error
	var resultMutex sync.Mutex

	// Use errgroup for coordinated error handling and context cancellation
	g, gctx := errgroup.WithContext(ctx)

	// Process batches in parallel with semaphore control
	for _, batch := range batches {
		// Skip empty batches using filter-like logic
		if len(batch.Orgs) == 0 {
			logInfo("Skipping empty batch",
				slog.Int("batch_number", batch.BatchNumber),
				slog.String("operation", "batch_skipped"))
			continue
		}

		// Capture batch in closure
		currentBatch := batch

		g.Go(func() error {
			// Acquire semaphore slot
			if err := sem.Acquire(gctx, 1); err != nil {
				return createError("failed to acquire semaphore for batch %d: %v", currentBatch.BatchNumber, err)
			}
			defer sem.Release(1)

			// Early context cancellation check
			select {
			case <-gctx.Done():
				logError("Workflow cancelled before batch processing",
					slog.Int("batch_number", currentBatch.BatchNumber),
					slog.String("error", gctx.Err().Error()),
					slog.String("operation", "batch_cancelled"))
				return createError("workflow cancelled before batch %d: %v", currentBatch.BatchNumber, gctx.Err())
			default:
			}

			// Update progress tracker for current batch (thread-safe)
			resultMutex.Lock()
			progressTracker.CurrentBatch = currentBatch.BatchNumber
			resultMutex.Unlock()

			// Process batch and collect errors using progress-aware cloner
			batchErrors := processBatch(gctx, currentBatch, pool, progressCloner, config)

			// Aggregate errors thread-safely
			if len(batchErrors) > 0 {
				resultMutex.Lock()
				for _, err := range batchErrors {
					result = multierror.Append(result, err)
				}
				resultMutex.Unlock()
			}

			// Update batch completion (thread-safe)
			resultMutex.Lock()
			progressTracker.ProcessedBatches++
			resultMutex.Unlock()

			return nil
		})
	}

	// Wait for all batches to complete
	if err := g.Wait(); err != nil {
		resultMutex.Lock()
		result = multierror.Append(result, err)
		resultMutex.Unlock()
	}

	// Convert multierror to slice for compatibility
	if result != nil {
		return result.Errors
	}
	return []error{}
}

// createBatchProcessor creates a function to process a single batch with progress tracking
func createBatchProcessor(logInfo, logError LogFunc, progressTracker *ProgressTracker, progressReporter ProgressFunc) BatchProcessor {
	return func(ctx context.Context, batch Batch, pool *ants.Pool, cloner OrgCloner, config *Config) []error {
		logInfo("Starting batch processing",
			slog.Int("batch_number", batch.BatchNumber),
			slog.Int("org_count", len(batch.Orgs)),
			slog.String("operation", "batch_start"))

		// Use channel-based error collection for better performance
		errorChan := make(chan error, len(batch.Orgs))
		var wg sync.WaitGroup

		// Create org processor function using functional composition with progress tracking
		processOrg := createOrgProcessorWithChannels(logInfo, logError, errorChan, progressTracker, progressReporter)

		// Submit tasks for organizations, collecting submission results
		successfulSubmissions := 0
		for _, org := range batch.Orgs {
			wg.Add(1)
			if err := pool.Submit(processOrg(ctx, org, batch.BatchNumber, cloner, config, &wg)); err != nil {
				wg.Done()
				logError("Failed to submit task to pool",
					slog.String("org", org),
					slog.Int("batch_number", batch.BatchNumber),
					slog.String("error", err.Error()),
					slog.String("operation", "task_submit_failed"))
				// Send error to channel instead of using mutex
				errorChan <- createError("failed to submit task for %s: %v", org, err)
			} else {
				successfulSubmissions++
			}
		}

		logInfo("Tasks submitted to worker pool",
			slog.Int("batch_number", batch.BatchNumber),
			slog.Int("successful_submissions", successfulSubmissions),
			slog.Int("total_orgs", len(batch.Orgs)),
			slog.String("operation", "tasks_submitted"))

		// Wait for current batch completion
		wg.Wait()

		// Close error channel and collect all errors
		close(errorChan)
		var allErrors []error
		for err := range errorChan {
			allErrors = append(allErrors, err)
		}

		logInfo("Batch processing completed",
			slog.Int("batch_number", batch.BatchNumber),
			slog.Int("org_count", len(batch.Orgs)),
			slog.Int("error_count", len(allErrors)),
			slog.String("operation", "batch_completed"))

		return allErrors
	}
}

// createOrgProcessor creates a function to process a single organization with progress tracking
func createOrgProcessor(logInfo, logError LogFunc, mu *sync.Mutex, allErrors *[]error, progressTracker *ProgressTracker, progressReporter ProgressFunc) func(context.Context, string, int, OrgCloner, *Config, *sync.WaitGroup) func() {
	return func(ctx context.Context, orgName string, batchNum int, cloner OrgCloner, config *Config, wg *sync.WaitGroup) func() {
		return func() {
			defer wg.Done()

			// Update progress tracker for current org
			progressTracker.CurrentOrg = orgName

			logInfo("Starting organization processing",
				slog.String("org", orgName),
				slog.Int("batch_number", batchNum),
				slog.String("operation", "org_start"))

			// Report org start progress
			progress := calculateProgress(*progressTracker)
			progress.Status = "org_start"
			progressReporter(progress)

			// Context cancellation check within goroutine
			select {
			case <-ctx.Done():
				logError("Organization processing cancelled",
					slog.String("org", orgName),
					slog.Int("batch_number", batchNum),
					slog.String("error", ctx.Err().Error()),
					slog.String("operation", "org_cancelled"))
				mu.Lock()
				*allErrors = append(*allErrors, createError("cancelled processing %s in batch %d: %v", orgName, batchNum, ctx.Err()))
				mu.Unlock()
				return
			default:
			}

			// Execute organization cloning with timeout protection
			if err := cloner(ctx, orgName, config.TargetDir, config.GitHubToken, config.Concurrency); err != nil {
				logError("Organization processing failed",
					slog.String("org", orgName),
					slog.Int("batch_number", batchNum),
					slog.String("error", err.Error()),
					slog.String("operation", "org_failed"))
				mu.Lock()
				*allErrors = append(*allErrors, createError("batch %d org %s failed: %v", batchNum, orgName, err))
				mu.Unlock()
			} else {
				// Update progress on successful completion
				progressTracker.ProcessedOrgs++

				// Report org completion progress
				progress := calculateProgress(*progressTracker)
				progress.Status = "org_completed"
				progressReporter(progress)

				logInfo("Organization processed successfully",
					slog.String("org", orgName),
					slog.Int("batch_number", batchNum),
					slog.String("operation", "org_completed"))
			}
		}
	}
}

// createOrgProcessorWithChannels creates a function to process a single organization with channel-based error collection
func createOrgProcessorWithChannels(logInfo, logError LogFunc, errorChan chan<- error, progressTracker *ProgressTracker, progressReporter ProgressFunc) func(context.Context, string, int, OrgCloner, *Config, *sync.WaitGroup) func() {
	return func(ctx context.Context, orgName string, batchNum int, cloner OrgCloner, config *Config, wg *sync.WaitGroup) func() {
		return func() {
			defer wg.Done()

			// Update progress tracker for current org
			progressTracker.CurrentOrg = orgName

			logInfo("Starting organization processing",
				slog.String("org", orgName),
				slog.Int("batch_number", batchNum),
				slog.String("operation", "org_start"))

			// Report org start progress
			progress := calculateProgress(*progressTracker)
			progress.Status = "org_start"
			progressReporter(progress)

			// Context cancellation check within goroutine
			select {
			case <-ctx.Done():
				logError("Organization processing cancelled",
					slog.String("org", orgName),
					slog.Int("batch_number", batchNum),
					slog.String("error", ctx.Err().Error()),
					slog.String("operation", "org_cancelled"))
				// Send error to channel instead of using mutex
				errorChan <- createError("cancelled processing %s in batch %d: %v", orgName, batchNum, ctx.Err())
				return
			default:
			}

			// Execute organization cloning with timeout protection
			if err := cloner(ctx, orgName, config.TargetDir, config.GitHubToken, config.Concurrency); err != nil {
				logError("Organization processing failed",
					slog.String("org", orgName),
					slog.Int("batch_number", batchNum),
					slog.String("error", err.Error()),
					slog.String("operation", "org_failed"))
				// Send error to channel instead of using mutex
				errorChan <- createError("batch %d org %s failed: %v", batchNum, orgName, err)
			} else {
				// Update progress on successful completion
				progressTracker.ProcessedOrgs++

				// Report org completion progress
				progress := calculateProgress(*progressTracker)
				progress.Status = "org_completed"
				progressReporter(progress)

				logInfo("Organization processed successfully",
					slog.String("org", orgName),
					slog.Int("batch_number", batchNum),
					slog.String("operation", "org_completed"))
			}
		}
	}
}

// collectAndReportErrors collects all errors and reports them functionally
func collectAndReportErrors(allErrors []error, logInfo, logError LogFunc) error {
	if len(allErrors) == 0 {
		logInfo("All batches processed successfully",
			slog.Int("error_count", 0),
			slog.String("operation", "execution_successful"))
		return nil
	}

	logError("Execution completed with errors",
		slog.Int("error_count", len(allErrors)),
		slog.String("operation", "execution_completed_with_errors"))

	// Map errors to strings and join them
	errorMessages := make([]string, len(allErrors))
	for i, err := range allErrors {
		errorMessages[i] = err.Error()
	}

	return createError("processing completed with %d errors: %s",
		len(allErrors), strings.Join(errorMessages, "; "))
}
