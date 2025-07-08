package main

import (
	"log/slog"
	"time"
)

// Progress tracking functionality
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

func calculateProgress(tracker ProgressTracker) ProgressUpdate {
	elapsed := time.Since(tracker.StartTime)

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
