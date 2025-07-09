package main

import (
	"context"
	"log/slog"
	"runtime"
	"sync/atomic"
	"time"
)

// Global metrics instance
var globalMetrics *RuntimeMetrics

// Initialize metrics tracking
func initMetrics() *RuntimeMetrics {
	if globalMetrics == nil {
		globalMetrics = &RuntimeMetrics{
			StartTime: time.Now(),
		}
	}
	return globalMetrics
}

// Get current metrics
func getCurrentMetrics() *RuntimeMetrics {
	if globalMetrics == nil {
		return initMetrics()
	}
	return globalMetrics
}

// Track goroutine spawn (call this when creating goroutines)
func (rm *RuntimeMetrics) TrackGoroutineSpawn() {
	atomic.AddInt64(&rm.TotalSpawned, 1)
	current := int32(runtime.NumGoroutine())
	atomic.StoreInt32(&rm.ActiveGoroutines, current)
	
	// Update peak if current is higher
	for {
		peak := atomic.LoadInt32(&rm.PeakGoroutines)
		if current <= peak {
			break
		}
		if atomic.CompareAndSwapInt32(&rm.PeakGoroutines, peak, current) {
			break
		}
	}
}

// Update active goroutine count
func (rm *RuntimeMetrics) UpdateActiveGoroutines() {
	current := int32(runtime.NumGoroutine())
	atomic.StoreInt32(&rm.ActiveGoroutines, current)
}

// Get metrics snapshot
func (rm *RuntimeMetrics) GetSnapshot() (int32, int32, int64, time.Duration) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	active := atomic.LoadInt32(&rm.ActiveGoroutines)
	peak := atomic.LoadInt32(&rm.PeakGoroutines)
	total := atomic.LoadInt64(&rm.TotalSpawned)
	elapsed := time.Since(rm.StartTime)
	
	return active, peak, total, elapsed
}

// Get full metrics snapshot including repository counts
func (rm *RuntimeMetrics) GetFullSnapshot() (int32, int32, int64, int64, int64, int64, time.Duration) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	active := atomic.LoadInt32(&rm.ActiveGoroutines)
	peak := atomic.LoadInt32(&rm.PeakGoroutines)
	totalSpawned := atomic.LoadInt64(&rm.TotalSpawned)
	totalRepos := atomic.LoadInt64(&rm.TotalRepos)
	processedRepos := atomic.LoadInt64(&rm.ProcessedRepos)
	remainingRepos := atomic.LoadInt64(&rm.RemainingRepos)
	elapsed := time.Since(rm.StartTime)
	
	return active, peak, totalSpawned, totalRepos, processedRepos, remainingRepos, elapsed
}

// Track repository discovery
func (rm *RuntimeMetrics) TrackRepoDiscovery(repoCount int) {
	atomic.AddInt64(&rm.TotalRepos, int64(repoCount))
	atomic.AddInt64(&rm.RemainingRepos, int64(repoCount))
}

// Track repository processing
func (rm *RuntimeMetrics) TrackRepoProcessed(count int) {
	atomic.AddInt64(&rm.ProcessedRepos, int64(count))
	atomic.AddInt64(&rm.RemainingRepos, -int64(count))
}

// Create metrics reporter function
func createMetricsReporter(logInfo LogFunc) func() {
	return func() {
		metrics := getCurrentMetrics()
		active, peak, totalSpawned, totalRepos, processedRepos, remainingRepos, elapsed := metrics.GetFullSnapshot()
		
		if logInfo != nil {
			logInfo("Runtime Metrics",
				slog.String("elapsed_time", elapsed.String()),
				slog.Int("active_goroutines", int(active)),
				slog.Int("peak_goroutines", int(peak)),
				slog.Int64("total_spawned", totalSpawned),
				slog.Int64("total_repos", totalRepos),
				slog.Int64("processed_repos", processedRepos),
				slog.Int64("remaining_repos", remainingRepos),
				slog.String("operation", "metrics_report"))
		}
	}
}

// Start periodic metrics reporting
func startMetricsReporter(ctx context.Context, logInfo LogFunc, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second // Default to 5 seconds
	}
	
	reporter := createMetricsReporter(logInfo)
	ticker := time.NewTicker(interval)
	
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				// Final metrics report
				reporter()
				return
			case <-ticker.C:
				reporter()
			}
		}
	}()
}

// Enhanced progress reporter with metrics
func createProgressReporterWithMetrics(logInfo LogFunc) ProgressFunc {
	return func(update ProgressUpdate) {
		metrics := getCurrentMetrics()
		active, peak, totalSpawned, totalRepos, processedRepos, remainingRepos, elapsed := metrics.GetFullSnapshot()
		
		// Update the progress update with metrics
		update.ActiveGoroutines = active
		update.PeakGoroutines = peak
		update.TotalSpawned = totalSpawned
		update.TotalRepos = totalRepos
		update.ProcessedRepos = processedRepos
		update.RemainingRepos = remainingRepos
		update.ElapsedTime = elapsed
		
		if logInfo != nil {
			logInfo("Progress Update",
				slog.String("org", update.Org),
				slog.Int("batch_number", update.BatchNumber),
				slog.Int("completed_orgs", update.CompletedOrgs),
				slog.Int("total_orgs", update.TotalOrgs),
				slog.String("status", update.Status),
				slog.String("elapsed_time", elapsed.String()),
				slog.String("estimated_eta", update.EstimatedETA.String()),
				slog.Int("active_goroutines", int(active)),
				slog.Int("peak_goroutines", int(peak)),
				slog.Int64("total_spawned", totalSpawned),
				slog.Int64("total_repos", totalRepos),
				slog.Int64("processed_repos", processedRepos),
				slog.Int64("remaining_repos", remainingRepos),
				slog.String("operation", "progress_with_metrics"))
		}
	}
}

// Track execution time for a function
func trackExecutionTime(logInfo LogFunc, operation string) func() {
	start := time.Now()
	return func() {
		elapsed := time.Since(start)
		metrics := getCurrentMetrics()
		active, peak, totalSpawned, totalRepos, processedRepos, remainingRepos, _ := metrics.GetFullSnapshot()
		
		if logInfo != nil {
			logInfo("Operation Completed",
				slog.String("operation", operation),
				slog.String("duration", elapsed.String()),
				slog.Int("active_goroutines", int(active)),
				slog.Int("peak_goroutines", int(peak)),
				slog.Int64("total_spawned", totalSpawned),
				slog.Int64("total_repos", totalRepos),
				slog.Int64("processed_repos", processedRepos),
				slog.Int64("remaining_repos", remainingRepos),
				slog.String("operation_type", "timing_report"))
		}
	}
}