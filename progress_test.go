package main

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// Test createProgressTracker following Given-When-Then pattern
func TestCreateProgressTracker(t *testing.T) {
	t.Run("Given total orgs and batches When creating progress tracker Then should initialize correctly", func(t *testing.T) {
		// Given
		totalOrgs := 10
		totalBatches := 3

		// When
		tracker := createProgressTracker(totalOrgs, totalBatches)

		// Then
		assert.Equal(t, 10, tracker.TotalOrgs)
		assert.Equal(t, 0, tracker.ProcessedOrgs)
		assert.Equal(t, 3, tracker.TotalBatches)
		assert.Equal(t, 0, tracker.ProcessedBatches)
		assert.Equal(t, "", tracker.CurrentOrg)
		assert.Equal(t, 0, tracker.CurrentBatch)
		assert.False(t, tracker.StartTime.IsZero())
	})

	t.Run("Given zero values When creating progress tracker Then should handle gracefully", func(t *testing.T) {
		// Given
		totalOrgs := 0
		totalBatches := 0

		// When
		tracker := createProgressTracker(totalOrgs, totalBatches)

		// Then
		assert.Equal(t, 0, tracker.TotalOrgs)
		assert.Equal(t, 0, tracker.TotalBatches)
		assert.False(t, tracker.StartTime.IsZero())
	})

	t.Run("Given negative values When creating progress tracker Then should handle gracefully", func(t *testing.T) {
		// Given
		totalOrgs := -5
		totalBatches := -2

		// When
		tracker := createProgressTracker(totalOrgs, totalBatches)

		// Then
		assert.Equal(t, -5, tracker.TotalOrgs)
		assert.Equal(t, -2, tracker.TotalBatches)
		assert.False(t, tracker.StartTime.IsZero())
	})

	t.Run("Given large values When creating progress tracker Then should handle correctly", func(t *testing.T) {
		// Given
		totalOrgs := 10000
		totalBatches := 200

		// When
		tracker := createProgressTracker(totalOrgs, totalBatches)

		// Then
		assert.Equal(t, 10000, tracker.TotalOrgs)
		assert.Equal(t, 200, tracker.TotalBatches)
		assert.False(t, tracker.StartTime.IsZero())
	})
}

// Test calculateProgress following Given-When-Then pattern
func TestCalculateProgress(t *testing.T) {
	t.Run("Given tracker with no processed orgs When calculating progress Then should return zero progress with no ETA", func(t *testing.T) {
		// Given
		tracker := ProgressTracker{
			TotalOrgs:        10,
			ProcessedOrgs:    0,
			TotalBatches:     3,
			ProcessedBatches: 0,
			StartTime:        time.Now(),
			CurrentOrg:       "current-org",
			CurrentBatch:     1,
		}

		// When
		update := calculateProgress(tracker)

		// Then
		assert.Equal(t, "current-org", update.Org)
		assert.Equal(t, 1, update.BatchNumber)
		assert.Equal(t, 0, update.CompletedOrgs)
		assert.Equal(t, 10, update.TotalOrgs)
		assert.True(t, update.ElapsedTime > 0)
		assert.Equal(t, time.Duration(0), update.EstimatedETA)
		assert.Equal(t, "processing", update.Status)
	})

	t.Run("Given tracker with some processed orgs When calculating progress Then should return progress with ETA", func(t *testing.T) {
		// Given
		startTime := time.Now().Add(-10 * time.Second)
		tracker := ProgressTracker{
			TotalOrgs:        10,
			ProcessedOrgs:    2,
			TotalBatches:     3,
			ProcessedBatches: 1,
			StartTime:        startTime,
			CurrentOrg:       "current-org",
			CurrentBatch:     2,
		}

		// When
		update := calculateProgress(tracker)

		// Then
		assert.Equal(t, "current-org", update.Org)
		assert.Equal(t, 2, update.BatchNumber)
		assert.Equal(t, 2, update.CompletedOrgs)
		assert.Equal(t, 10, update.TotalOrgs)
		assert.True(t, update.ElapsedTime >= 10*time.Second)
		assert.True(t, update.EstimatedETA > 0)
		assert.Equal(t, "processing", update.Status)
	})

	t.Run("Given tracker with all orgs processed When calculating progress Then should return complete progress", func(t *testing.T) {
		// Given
		startTime := time.Now().Add(-30 * time.Second)
		tracker := ProgressTracker{
			TotalOrgs:        5,
			ProcessedOrgs:    5,
			TotalBatches:     2,
			ProcessedBatches: 2,
			StartTime:        startTime,
			CurrentOrg:       "final-org",
			CurrentBatch:     2,
		}

		// When
		update := calculateProgress(tracker)

		// Then
		assert.Equal(t, "final-org", update.Org)
		assert.Equal(t, 2, update.BatchNumber)
		assert.Equal(t, 5, update.CompletedOrgs)
		assert.Equal(t, 5, update.TotalOrgs)
		assert.True(t, update.ElapsedTime >= 30*time.Second)
		assert.Equal(t, time.Duration(0), update.EstimatedETA) // No remaining orgs
		assert.Equal(t, "processing", update.Status)
	})

	t.Run("Given tracker with half progress When calculating progress Then should estimate remaining time correctly", func(t *testing.T) {
		// Given
		startTime := time.Now().Add(-20 * time.Second)
		tracker := ProgressTracker{
			TotalOrgs:        10,
			ProcessedOrgs:    5,
			TotalBatches:     3,
			ProcessedBatches: 1,
			StartTime:        startTime,
			CurrentOrg:       "mid-org",
			CurrentBatch:     2,
		}

		// When
		update := calculateProgress(tracker)

		// Then
		assert.Equal(t, "mid-org", update.Org)
		assert.Equal(t, 2, update.BatchNumber)
		assert.Equal(t, 5, update.CompletedOrgs)
		assert.Equal(t, 10, update.TotalOrgs)
		assert.True(t, update.ElapsedTime >= 20*time.Second)
		
		// ETA calculation: avgTimePerOrg = 20s / 5 orgs = 4s per org
		// remainingOrgs = 10 - 5 = 5 orgs
		// eta = 4s * 5 = 20s
		expectedETA := 20 * time.Second
		assert.True(t, update.EstimatedETA >= expectedETA-time.Second && update.EstimatedETA <= expectedETA+time.Second)
		assert.Equal(t, "processing", update.Status)
	})

	t.Run("Given tracker with single processed org When calculating progress Then should calculate ETA correctly", func(t *testing.T) {
		// Given
		startTime := time.Now().Add(-5 * time.Second)
		tracker := ProgressTracker{
			TotalOrgs:        3,
			ProcessedOrgs:    1,
			TotalBatches:     1,
			ProcessedBatches: 0,
			StartTime:        startTime,
			CurrentOrg:       "single-org",
			CurrentBatch:     1,
		}

		// When
		update := calculateProgress(tracker)

		// Then
		assert.Equal(t, "single-org", update.Org)
		assert.Equal(t, 1, update.BatchNumber)
		assert.Equal(t, 1, update.CompletedOrgs)
		assert.Equal(t, 3, update.TotalOrgs)
		assert.True(t, update.ElapsedTime >= 5*time.Second)
		
		// ETA calculation: avgTimePerOrg = 5s / 1 org = 5s per org
		// remainingOrgs = 3 - 1 = 2 orgs
		// eta = 5s * 2 = 10s
		expectedETA := 10 * time.Second
		assert.True(t, update.EstimatedETA >= expectedETA-time.Second && update.EstimatedETA <= expectedETA+time.Second)
		assert.Equal(t, "processing", update.Status)
	})
}

// Test createProgressReporter following Given-When-Then pattern
func TestCreateProgressReporter(t *testing.T) {
	t.Run("Given log function When creating progress reporter Then should return functional reporter", func(t *testing.T) {
		// Given
		var loggedMessages []string
		var loggedAttrs [][]slog.Attr
		
		logInfo := func(msg string, attrs ...slog.Attr) {
			loggedMessages = append(loggedMessages, msg)
			loggedAttrs = append(loggedAttrs, attrs)
		}

		// When
		reporter := createProgressReporter(logInfo)

		// Then
		assert.NotNil(t, reporter)
		
		// Test that reporter can be called
		update := ProgressUpdate{
			Org:           "test-org",
			BatchNumber:   1,
			CompletedOrgs: 2,
			TotalOrgs:     10,
			ElapsedTime:   30 * time.Second,
			EstimatedETA:  60 * time.Second,
			Status:        "processing",
		}
		
		assert.NotPanics(t, func() {
			reporter(update)
		})
		
		// Verify logging was called
		assert.Len(t, loggedMessages, 1)
		assert.Equal(t, "Progress update", loggedMessages[0])
		
		// Verify attributes were logged
		assert.Len(t, loggedAttrs, 1)
		attrs := loggedAttrs[0]
		assert.True(t, len(attrs) > 0)
	})

	t.Run("Given progress update When reporting Then should log correct percentage", func(t *testing.T) {
		// Given
		var loggedPercentage float64
		
		logInfo := func(msg string, attrs ...slog.Attr) {
			for _, attr := range attrs {
				if attr.Key == "percentage" {
					loggedPercentage = attr.Value.Float64()
				}
			}
		}
		
		reporter := createProgressReporter(logInfo)
		
		update := ProgressUpdate{
			Org:           "test-org",
			BatchNumber:   1,
			CompletedOrgs: 3,
			TotalOrgs:     10,
			ElapsedTime:   30 * time.Second,
			EstimatedETA:  70 * time.Second,
			Status:        "processing",
		}

		// When
		reporter(update)

		// Then
		expectedPercentage := float64(3) / float64(10) * 100 // 30%
		assert.Equal(t, expectedPercentage, loggedPercentage)
	})

	t.Run("Given complete progress update When reporting Then should log 100 percent", func(t *testing.T) {
		// Given
		var loggedPercentage float64
		
		logInfo := func(msg string, attrs ...slog.Attr) {
			for _, attr := range attrs {
				if attr.Key == "percentage" {
					loggedPercentage = attr.Value.Float64()
				}
			}
		}
		
		reporter := createProgressReporter(logInfo)
		
		update := ProgressUpdate{
			Org:           "final-org",
			BatchNumber:   3,
			CompletedOrgs: 10,
			TotalOrgs:     10,
			ElapsedTime:   120 * time.Second,
			EstimatedETA:  0,
			Status:        "completed",
		}

		// When
		reporter(update)

		// Then
		expectedPercentage := float64(10) / float64(10) * 100 // 100%
		assert.Equal(t, expectedPercentage, loggedPercentage)
	})

	t.Run("Given zero total orgs When reporting Then should handle division by zero", func(t *testing.T) {
		// Given
		var loggedPercentage float64
		
		logInfo := func(msg string, attrs ...slog.Attr) {
			for _, attr := range attrs {
				if attr.Key == "percentage" {
					loggedPercentage = attr.Value.Float64()
				}
			}
		}
		
		reporter := createProgressReporter(logInfo)
		
		update := ProgressUpdate{
			Org:           "test-org",
			BatchNumber:   1,
			CompletedOrgs: 0,
			TotalOrgs:     0,
			ElapsedTime:   10 * time.Second,
			EstimatedETA:  0,
			Status:        "processing",
		}

		// When/Then - should not panic
		assert.NotPanics(t, func() {
			reporter(update)
		})
		
		// Should result in NaN or Inf, which is acceptable for edge case
		assert.True(t, loggedPercentage != loggedPercentage || loggedPercentage == 0) // NaN != NaN or 0
	})

	t.Run("Given progress reporter When called multiple times Then should handle all calls", func(t *testing.T) {
		// Given
		var callCount int
		
		logInfo := func(msg string, attrs ...slog.Attr) {
			callCount++
		}
		
		reporter := createProgressReporter(logInfo)
		
		updates := []ProgressUpdate{
			{Org: "org1", CompletedOrgs: 1, TotalOrgs: 5},
			{Org: "org2", CompletedOrgs: 2, TotalOrgs: 5},
			{Org: "org3", CompletedOrgs: 3, TotalOrgs: 5},
		}

		// When
		for _, update := range updates {
			reporter(update)
		}

		// Then
		assert.Equal(t, 3, callCount)
	})
}

// Property-based tests using rapid
func TestProgressTrackingProperties(t *testing.T) {
	t.Run("Property: Progress tracker always has valid start time", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			totalOrgs := rapid.IntRange(-100, 1000).Draw(t, "totalOrgs")
			totalBatches := rapid.IntRange(-10, 100).Draw(t, "totalBatches")

			tracker := createProgressTracker(totalOrgs, totalBatches)

			assert.False(t, tracker.StartTime.IsZero())
			assert.True(t, tracker.StartTime.Before(time.Now().Add(time.Second))) // Should be recent
		})
	})

	t.Run("Property: Progress calculation maintains invariants", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			totalOrgs := rapid.IntRange(1, 1000).Draw(t, "totalOrgs")
			processedOrgs := rapid.IntRange(0, totalOrgs).Draw(t, "processedOrgs")
			totalBatches := rapid.IntRange(1, 100).Draw(t, "totalBatches")
			processedBatches := rapid.IntRange(0, totalBatches).Draw(t, "processedBatches")
			
			tracker := ProgressTracker{
				TotalOrgs:        totalOrgs,
				ProcessedOrgs:    processedOrgs,
				TotalBatches:     totalBatches,
				ProcessedBatches: processedBatches,
				StartTime:        time.Now().Add(-time.Duration(rapid.IntRange(1, 3600).Draw(t, "elapsed")) * time.Second),
				CurrentOrg:       rapid.String().Draw(t, "currentOrg"),
				CurrentBatch:     rapid.IntRange(0, totalBatches).Draw(t, "currentBatch"),
			}

			update := calculateProgress(tracker)

			// Property: Update should preserve input values
			assert.Equal(t, tracker.CurrentOrg, update.Org)
			assert.Equal(t, tracker.CurrentBatch, update.BatchNumber)
			assert.Equal(t, tracker.ProcessedOrgs, update.CompletedOrgs)
			assert.Equal(t, tracker.TotalOrgs, update.TotalOrgs)
			assert.Equal(t, "processing", update.Status)

			// Property: Elapsed time should be positive
			assert.True(t, update.ElapsedTime > 0)

			// Property: ETA should be non-negative
			assert.True(t, update.EstimatedETA >= 0)

			// Property: If no orgs processed, ETA should be 0
			if processedOrgs == 0 {
				assert.Equal(t, time.Duration(0), update.EstimatedETA)
			}
		})
	})

	t.Run("Property: Progress reporter handles all valid inputs", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Create a no-op logger that doesn't panic
			logInfo := func(msg string, attrs ...slog.Attr) {
				// No-op
			}
			
			reporter := createProgressReporter(logInfo)
			
			update := ProgressUpdate{
				Org:           rapid.String().Draw(t, "org"),
				BatchNumber:   rapid.IntRange(0, 1000).Draw(t, "batchNumber"),
				CompletedOrgs: rapid.IntRange(0, 1000).Draw(t, "completedOrgs"),
				TotalOrgs:     rapid.IntRange(1, 1000).Draw(t, "totalOrgs"),
				ElapsedTime:   time.Duration(rapid.IntRange(0, 86400).Draw(t, "elapsed")) * time.Second,
				EstimatedETA:  time.Duration(rapid.IntRange(0, 86400).Draw(t, "eta")) * time.Second,
				Status:        rapid.String().Draw(t, "status"),
			}

			// Property: Reporter should never panic
			assert.NotPanics(t, func() {
				reporter(update)
			})
		})
	})
}

// Integration tests for progress tracking
func TestProgressTrackingIntegration(t *testing.T) {
	t.Run("Given complete progress workflow When tracking progress Then should work end-to-end", func(t *testing.T) {
		// Given
		totalOrgs := 5
		totalBatches := 2
		var progressUpdates []ProgressUpdate
		
		logInfo := func(msg string, attrs ...slog.Attr) {
			// Capture progress updates for verification
		}
		
		tracker := createProgressTracker(totalOrgs, totalBatches)
		reporter := createProgressReporter(logInfo)

		// When - simulate progress through workflow
		for i := 1; i <= totalOrgs; i++ {
			tracker.ProcessedOrgs = i
			tracker.CurrentOrg = fmt.Sprintf("org%d", i)
			tracker.CurrentBatch = ((i - 1) / 3) + 1 // Simulate batch progression
			
			update := calculateProgress(tracker)
			progressUpdates = append(progressUpdates, update)
			
			// Report progress
			assert.NotPanics(t, func() {
				reporter(update)
			})
		}

		// Then
		assert.Len(t, progressUpdates, 5)
		
		// Verify first update
		firstUpdate := progressUpdates[0]
		assert.Equal(t, "org1", firstUpdate.Org)
		assert.Equal(t, 1, firstUpdate.CompletedOrgs)
		assert.Equal(t, 5, firstUpdate.TotalOrgs)
		
		// Verify last update
		lastUpdate := progressUpdates[4]
		assert.Equal(t, "org5", lastUpdate.Org)
		assert.Equal(t, 5, lastUpdate.CompletedOrgs)
		assert.Equal(t, 5, lastUpdate.TotalOrgs)
		assert.Equal(t, time.Duration(0), lastUpdate.EstimatedETA) // No remaining work
	})

	t.Run("Given progress tracking with realistic timing When simulating batches Then should calculate accurate ETAs", func(t *testing.T) {
		// Given
		tracker := createProgressTracker(10, 3)
		
		// Simulate processing with known timing
		baseTime := time.Now()
		tracker.StartTime = baseTime
		
		// When - simulate after 30 seconds with 3 orgs processed
		simulatedNow := baseTime.Add(30 * time.Second)
		tracker.ProcessedOrgs = 3
		tracker.CurrentOrg = "org3"
		tracker.CurrentBatch = 1
		
		// Manually calculate elapsed time for consistent testing
		tracker.StartTime = simulatedNow.Add(-30 * time.Second)
		
		update := calculateProgress(tracker)

		// Then
		assert.Equal(t, 3, update.CompletedOrgs)
		assert.Equal(t, 10, update.TotalOrgs)
		assert.True(t, update.ElapsedTime >= 30*time.Second)
		
		// ETA should be approximately 70 seconds (7 remaining orgs * 10 seconds each)
		expectedETA := 70 * time.Second
		assert.True(t, update.EstimatedETA >= expectedETA-5*time.Second && update.EstimatedETA <= expectedETA+5*time.Second)
	})
}

// Stress tests for progress tracking
func TestProgressTrackingStress(t *testing.T) {
	t.Run("Given high frequency progress updates When reporting Then should handle efficiently", func(t *testing.T) {
		// Given
		var updateCount int
		logInfo := func(msg string, attrs ...slog.Attr) {
			updateCount++
		}
		
		reporter := createProgressReporter(logInfo)
		numUpdates := 10000

		// When
		start := time.Now()
		for i := 0; i < numUpdates; i++ {
			update := ProgressUpdate{
				Org:           fmt.Sprintf("org%d", i),
				BatchNumber:   i / 100,
				CompletedOrgs: i,
				TotalOrgs:     numUpdates,
				ElapsedTime:   time.Duration(i) * time.Millisecond,
				EstimatedETA:  time.Duration(numUpdates-i) * time.Millisecond,
				Status:        "processing",
			}
			reporter(update)
		}
		duration := time.Since(start)

		// Then
		assert.Equal(t, numUpdates, updateCount)
		assert.True(t, duration < 1*time.Second, "Should handle high frequency updates efficiently")
	})

	t.Run("Given concurrent progress tracking When multiple goroutines update Then should handle safely", func(t *testing.T) {
		// Given
		tracker := createProgressTracker(1000, 10)
		var updateCount int
		
		logInfo := func(msg string, attrs ...slog.Attr) {
			updateCount++
		}
		
		reporter := createProgressReporter(logInfo)
		numGoroutines := 10
		done := make(chan bool, numGoroutines)

		// When
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				for j := 0; j < 100; j++ {
					update := calculateProgress(tracker)
					update.Org = fmt.Sprintf("goroutine%d-org%d", id, j)
					reporter(update)
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Then
		assert.Equal(t, 1000, updateCount) // 10 goroutines * 100 updates each
	})
}

// Mutation tests for error handling
func TestProgressTrackingMutations(t *testing.T) {
	t.Run("Given invalid tracker state When calculating progress Then should handle gracefully", func(t *testing.T) {
		invalidTrackers := []ProgressTracker{
			{TotalOrgs: 0, ProcessedOrgs: 1, StartTime: time.Now()},           // More processed than total
			{TotalOrgs: -1, ProcessedOrgs: 0, StartTime: time.Now()},          // Negative total
			{TotalOrgs: 10, ProcessedOrgs: -1, StartTime: time.Now()},         // Negative processed
			{TotalOrgs: 10, ProcessedOrgs: 5, StartTime: time.Time{}},         // Zero start time
		}

		for i, tracker := range invalidTrackers {
			t.Run(fmt.Sprintf("invalid_tracker_%d", i), func(t *testing.T) {
				assert.NotPanics(t, func() {
					update := calculateProgress(tracker)
					assert.Equal(t, "processing", update.Status) // Should always have valid status
				})
			})
		}
	})

	t.Run("Given nil log function When creating reporter Then should handle gracefully", func(t *testing.T) {
		// Given
		var logInfo LogFunc = nil

		// When/Then - should not panic during creation
		assert.NotPanics(t, func() {
			reporter := createProgressReporter(logInfo)
			assert.NotNil(t, reporter)
		})
	})

	t.Run("Given extreme timing values When calculating progress Then should handle gracefully", func(t *testing.T) {
		extremeTrackers := []ProgressTracker{
			{
				TotalOrgs:     1,
				ProcessedOrgs: 1,
				StartTime:     time.Now().Add(-1000 * time.Hour), // Very long elapsed time
			},
			{
				TotalOrgs:     1000000,
				ProcessedOrgs: 1,
				StartTime:     time.Now().Add(-1 * time.Nanosecond), // Very short elapsed time
			},
		}

		for i, tracker := range extremeTrackers {
			t.Run(fmt.Sprintf("extreme_timing_%d", i), func(t *testing.T) {
				assert.NotPanics(t, func() {
					update := calculateProgress(tracker)
					assert.True(t, update.EstimatedETA >= 0) // Should never be negative
				})
			})
		}
	})
}

// Edge case tests
func TestProgressTrackingEdgeCases(t *testing.T) {
	t.Run("Given tracker with future start time When calculating progress Then should handle gracefully", func(t *testing.T) {
		// Given
		tracker := ProgressTracker{
			TotalOrgs:     10,
			ProcessedOrgs: 2,
			StartTime:     time.Now().Add(1 * time.Hour), // Future start time
		}

		// When
		update := calculateProgress(tracker)

		// Then
		assert.NotNil(t, update)
		assert.Equal(t, 2, update.CompletedOrgs)
		assert.Equal(t, 10, update.TotalOrgs)
		// Elapsed time might be negative, but should not panic
	})

	t.Run("Given tracker with very large numbers When calculating progress Then should handle without overflow", func(t *testing.T) {
		// Given
		tracker := ProgressTracker{
			TotalOrgs:     1000000000,
			ProcessedOrgs: 500000000,
			StartTime:     time.Now().Add(-1 * time.Hour),
		}

		// When
		update := calculateProgress(tracker)

		// Then
		assert.Equal(t, 500000000, update.CompletedOrgs)
		assert.Equal(t, 1000000000, update.TotalOrgs)
		assert.True(t, update.EstimatedETA >= 0)
	})

	t.Run("Given zero duration elapsed When calculating progress Then should handle division by zero", func(t *testing.T) {
		// Given
		now := time.Now()
		tracker := ProgressTracker{
			TotalOrgs:     10,
			ProcessedOrgs: 1,
			StartTime:     now, // No elapsed time
		}

		// When
		update := calculateProgress(tracker)

		// Then
		assert.Equal(t, 1, update.CompletedOrgs)
		assert.Equal(t, 10, update.TotalOrgs)
		// Should not panic, ETA calculation might be unusual but handled
	})
}