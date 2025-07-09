package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	ants "github.com/panjf2000/ants/v2"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// Workflow execution and orchestration
func ExecuteWorkflow(ctx context.Context, configPath string, loader ConfigLoader, cloner OrgCloner) error {
	logInfo, logError := createLoggers()

	// Initialize metrics tracking
	metrics := initMetrics()
	defer trackExecutionTime(logInfo, "ExecuteWorkflow")()

	// Start periodic metrics reporting
	startMetricsReporter(ctx, logInfo, 5*time.Second)

	select {
	case <-ctx.Done():
		return createError("workflow cancelled before starting: %v", ctx.Err())
	default:
	}

	logInfo("Workflow started",
		slog.String("config_path", configPath),
		slog.String("operation", "workflow_started"))

	config, err := loader(ctx, configPath)
	if err != nil {
		return createError("configuration loading failed: %v", err)
	}

	plan, err := processConfig(config)
	if err != nil {
		return createError("processing plan creation failed: %v", err)
	}

	logInfo("Processing plan created",
		slog.Int("total_orgs", plan.TotalOrgs),
		slog.Int("total_batches", len(plan.Batches)),
		slog.Int("concurrency", config.Concurrency),
		slog.String("operation", "plan_created"))

	err = executePlan(ctx, plan, config, cloner)
	
	// Final metrics report
	active, peak, totalSpawned, totalRepos, processedRepos, remainingRepos, elapsed := metrics.GetFullSnapshot()
	
	if err != nil {
		logError("Plan execution failed",
			slog.String("error", err.Error()),
			slog.String("total_elapsed", elapsed.String()),
			slog.Int("active_goroutines", int(active)),
			slog.Int("peak_goroutines", int(peak)),
			slog.Int64("total_spawned", totalSpawned),
			slog.Int64("total_repos", totalRepos),
			slog.Int64("processed_repos", processedRepos),
			slog.Int64("remaining_repos", remainingRepos),
			slog.String("operation", "plan_execution_failed"))
	} else {
		logInfo("Workflow completed successfully",
			slog.String("total_elapsed", elapsed.String()),
			slog.Int("active_goroutines", int(active)),
			slog.Int("peak_goroutines", int(peak)),
			slog.Int64("total_spawned", totalSpawned),
			slog.Int64("total_repos", totalRepos),
			slog.Int64("processed_repos", processedRepos),
			slog.Int64("remaining_repos", remainingRepos),
			slog.String("operation", "workflow_completed"))
	}

	return err
}

func executePlan(ctx context.Context, plan ProcessingPlan, config *Config, cloner OrgCloner) error {
	logInfo, _ := createLoggers()
	defer trackExecutionTime(logInfo, "executePlan")()

	progressTracker := createProgressTracker(plan.TotalOrgs, len(plan.Batches))
	progressReporter := createProgressReporterWithMetrics(logInfo)

	if len(plan.Batches) == 0 {
		return nil
	}

	pool, err := ants.NewPool(config.Concurrency, ants.WithOptions(ants.Options{
		PreAlloc:    true,
		Nonblocking: false,
	}))
	if err != nil {
		return createError("failed to create worker pool: %v", err)
	}
	defer pool.Release()

	allErrors := processBatches(ctx, plan.Batches, pool, cloner, config, logInfo, &progressTracker, progressReporter)

	return collectAndReportErrors(allErrors, logInfo)
}

func processBatches(ctx context.Context, batches []Batch, pool *ants.Pool, cloner OrgCloner, config *Config, logInfo LogFunc, progressTracker *ProgressTracker, progressReporter ProgressFunc) []error {
	defer trackExecutionTime(logInfo, "processBatches")()
	
	progressCloner := CreateGhorgOrgClonerWithProgress(progressReporter)
	metrics := getCurrentMetrics()

	// Allow unlimited concurrent batch processing
	maxConcurrentBatches := int64(len(batches)) // No limit, process all batches concurrently
	if maxConcurrentBatches < 1 {
		maxConcurrentBatches = 1
	}

	logInfo("Starting batch processing",
		slog.Int("total_batches", len(batches)),
		slog.Int64("max_concurrent_batches", maxConcurrentBatches),
		slog.Int("worker_pool_size", pool.Cap()),
		slog.String("operation", "batch_processing_started"))

	sem := semaphore.NewWeighted(maxConcurrentBatches)
	var result *multierror.Error
	var resultMutex sync.Mutex

	g, gctx := errgroup.WithContext(ctx)

	for _, batch := range batches {
		if len(batch.Orgs) == 0 {
			continue
		}

		currentBatch := batch

		g.Go(func() error {
			// Track goroutine spawn
			metrics.TrackGoroutineSpawn()
			
			if err := sem.Acquire(gctx, 1); err != nil {
				return createError("failed to acquire semaphore for batch %d: %v", currentBatch.BatchNumber, err)
			}
			defer sem.Release(1)

			select {
			case <-gctx.Done():
				return createError("workflow cancelled before batch %d: %v", currentBatch.BatchNumber, gctx.Err())
			default:
			}

			resultMutex.Lock()
			progressTracker.CurrentBatch = currentBatch.BatchNumber
			resultMutex.Unlock()

			batchErrors := processBatch(gctx, currentBatch, pool, progressCloner, config)

			if len(batchErrors) > 0 {
				resultMutex.Lock()
				for _, err := range batchErrors {
					result = multierror.Append(result, err)
				}
				resultMutex.Unlock()
			}

			resultMutex.Lock()
			progressTracker.ProcessedBatches++
			resultMutex.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		resultMutex.Lock()
		result = multierror.Append(result, err)
		resultMutex.Unlock()
	}

	if result != nil {
		return result.Errors
	}
	return []error{}
}

func processBatch(ctx context.Context, batch Batch, pool *ants.Pool, cloner OrgCloner, config *Config) []error {
	logInfo, _ := createLoggers()
	defer trackExecutionTime(logInfo, "processBatch")()
	
	errorChan := make(chan error, len(batch.Orgs))
	var wg sync.WaitGroup
	metrics := getCurrentMetrics()

	logInfo("Processing batch",
		slog.Int("batch_number", batch.BatchNumber),
		slog.Int("org_count", len(batch.Orgs)),
		slog.Int("pool_running", pool.Running()),
		slog.Int("pool_free", pool.Free()),
		slog.String("operation", "batch_processing"))

	for _, org := range batch.Orgs {
		wg.Add(1)
		currentOrg := org // Capture for closure
		
		if err := pool.Submit(func() {
			// Track goroutine spawn for worker pool tasks
			metrics.TrackGoroutineSpawn()
			defer wg.Done()

			start := time.Now()
			if err := cloner(ctx, currentOrg, config.TargetDir, config.GitHubToken, config.Concurrency); err != nil {
				errorChan <- createError("batch %d org %s failed: %v", batch.BatchNumber, currentOrg, err)
			} else {
				logInfo("Organization cloned successfully",
					slog.String("org", currentOrg),
					slog.Int("batch_number", batch.BatchNumber),
					slog.String("duration", time.Since(start).String()),
					slog.String("operation", "org_cloned"))
			}
			
			// Update goroutine count after completion
			metrics.UpdateActiveGoroutines()
		}); err != nil {
			wg.Done()
			errorChan <- createError("failed to submit task for %s: %v", currentOrg, err)
		}
	}

	wg.Wait()
	close(errorChan)

	var allErrors []error
	for err := range errorChan {
		allErrors = append(allErrors, err)
	}

	return allErrors
}
