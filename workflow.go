package main

import (
	"context"
	"log/slog"
	"sync"

	"github.com/hashicorp/go-multierror"
	ants "github.com/panjf2000/ants/v2"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// Workflow execution and orchestration
func ExecuteWorkflow(ctx context.Context, configPath string, loader ConfigLoader, cloner OrgCloner) error {
	logInfo, logError := createLoggers()

	select {
	case <-ctx.Done():
		return createError("workflow cancelled before starting: %v", ctx.Err())
	default:
	}

	config, err := loader(ctx, configPath)
	if err != nil {
		return createError("configuration loading failed: %v", err)
	}

	plan, err := processConfig(config)
	if err != nil {
		return createError("processing plan creation failed: %v", err)
	}

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
}

func executePlan(ctx context.Context, plan ProcessingPlan, config *Config, cloner OrgCloner) error {
	logInfo, _ := createLoggers()

	progressTracker := createProgressTracker(plan.TotalOrgs, len(plan.Batches))
	progressReporter := createProgressReporter(logInfo)

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
	progressCloner := CreateGhorgOrgClonerWithProgress(progressReporter)

	// Allow concurrent batch processing up to reasonable limits
	maxConcurrentBatches := int64(min(len(batches), max(10, len(batches)/2))) // Scale with batch count
	if maxConcurrentBatches < 1 {
		maxConcurrentBatches = 1
	}

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
	errorChan := make(chan error, len(batch.Orgs))
	var wg sync.WaitGroup

	for _, org := range batch.Orgs {
		wg.Add(1)
		if err := pool.Submit(func() {
			defer wg.Done()

			if err := cloner(ctx, org, config.TargetDir, config.GitHubToken, config.Concurrency); err != nil {
				errorChan <- createError("batch %d org %s failed: %v", batch.BatchNumber, org, err)
			}
		}); err != nil {
			wg.Done()
			errorChan <- createError("failed to submit task for %s: %v", org, err)
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
