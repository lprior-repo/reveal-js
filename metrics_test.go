package main

import (
	"context"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test metrics initialization following Given-When-Then pattern
func TestMetricsInitialization(t *testing.T) {
	t.Run("Given fresh metrics instance When initializing Then should set start time", func(t *testing.T) {
		// Given
		globalMetrics = nil
		before := time.Now()

		// When
		metrics := initMetrics()

		// Then
		after := time.Now()
		assert.NotNil(t, metrics)
		assert.True(t, metrics.StartTime.After(before) || metrics.StartTime.Equal(before))
		assert.True(t, metrics.StartTime.Before(after) || metrics.StartTime.Equal(after))
		assert.Equal(t, int32(0), metrics.ActiveGoroutines)
		assert.Equal(t, int32(0), metrics.PeakGoroutines)
		assert.Equal(t, int64(0), metrics.TotalSpawned)
	})

	t.Run("Given existing metrics instance When getting current metrics Then should return same instance", func(t *testing.T) {
		// Given
		first := initMetrics()
		startTime := first.StartTime

		// When
		second := getCurrentMetrics()

		// Then
		assert.Equal(t, first, second)
		assert.Equal(t, startTime, second.StartTime)
	})
}

// Test goroutine tracking following Given-When-Then pattern
func TestGoroutineTracking(t *testing.T) {
	t.Run("Given metrics instance When tracking goroutine spawn Then should increment counters", func(t *testing.T) {
		// Given
		metrics := initMetrics()
		initialGoroutines := runtime.NumGoroutine()

		// When
		metrics.TrackGoroutineSpawn()

		// Then
		assert.Equal(t, int64(1), metrics.TotalSpawned)
		assert.Equal(t, int32(initialGoroutines), metrics.ActiveGoroutines)
		assert.GreaterOrEqual(t, metrics.PeakGoroutines, int32(initialGoroutines))
	})

	t.Run("Given metrics instance When tracking multiple spawns Then should update peak correctly", func(t *testing.T) {
		// Given
		metrics := initMetrics()
		initialPeak := metrics.PeakGoroutines

		// When
		for i := 0; i < 5; i++ {
			metrics.TrackGoroutineSpawn()
		}

		// Then
		assert.Equal(t, int64(5), metrics.TotalSpawned)
		assert.GreaterOrEqual(t, metrics.PeakGoroutines, initialPeak)
	})

	t.Run("Given metrics instance When updating active goroutines Then should reflect current count", func(t *testing.T) {
		// Given
		metrics := initMetrics()
		currentCount := int32(runtime.NumGoroutine())

		// When
		metrics.UpdateActiveGoroutines()

		// Then
		assert.Equal(t, currentCount, metrics.ActiveGoroutines)
	})
}

// Test metrics snapshot following Given-When-Then pattern
func TestMetricsSnapshot(t *testing.T) {
	t.Run("Given metrics with tracked data When getting snapshot Then should return current values", func(t *testing.T) {
		// Given
		metrics := initMetrics()
		metrics.TrackGoroutineSpawn()
		metrics.TrackGoroutineSpawn()
		time.Sleep(10 * time.Millisecond) // Ensure some elapsed time

		// When
		active, peak, total, elapsed := metrics.GetSnapshot()

		// Then
		assert.Equal(t, int64(2), total)
		assert.GreaterOrEqual(t, peak, int32(0))
		assert.GreaterOrEqual(t, active, int32(0))
		assert.Greater(t, elapsed, time.Duration(0))
	})
}

// Test metrics reporter following Given-When-Then pattern
func TestMetricsReporter(t *testing.T) {
	t.Run("Given metrics instance When creating reporter Then should log metrics", func(t *testing.T) {
		// Given
		metrics := initMetrics()
		metrics.TrackGoroutineSpawn()
		
		logCalled := false
		mockLogInfo := func(msg string, args ...slog.Attr) {
			logCalled = true
			assert.Equal(t, "Runtime Metrics", msg)
			assert.True(t, len(args) > 0)
		}

		// When
		reporter := createMetricsReporter(mockLogInfo)
		reporter()

		// Then
		assert.True(t, logCalled)
	})

	t.Run("Given nil logger When creating reporter Then should not panic", func(t *testing.T) {
		// Given
		_ = initMetrics()

		// When & Then
		assert.NotPanics(t, func() {
			reporter := createMetricsReporter(nil)
			reporter()
		})
	})
}

// Test periodic metrics reporting following Given-When-Then pattern
func TestPeriodicMetricsReporting(t *testing.T) {
	t.Run("Given context and interval When starting reporter Then should report periodically", func(t *testing.T) {
		// Given
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		
		reportCount := 0
		mockLogInfo := func(msg string, args ...slog.Attr) {
			if msg == "Runtime Metrics" {
				reportCount++
			}
		}

		// When
		startMetricsReporter(ctx, mockLogInfo, 50*time.Millisecond)
		time.Sleep(150 * time.Millisecond) // Allow for 2-3 reports
		cancel()
		time.Sleep(50 * time.Millisecond) // Allow for cleanup

		// Then
		assert.GreaterOrEqual(t, reportCount, 2)
	})

	t.Run("Given zero interval When starting reporter Then should use default interval", func(t *testing.T) {
		// Given
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		
		reportCount := 0
		mockLogInfo := func(msg string, args ...slog.Attr) {
			if msg == "Runtime Metrics" {
				reportCount++
			}
		}

		// When
		startMetricsReporter(ctx, mockLogInfo, 0) // Should default to 5 seconds
		time.Sleep(100 * time.Millisecond) // Short wait
		cancel()

		// Then - Should not report in short time with default interval
		assert.Equal(t, 0, reportCount)
	})
}

// Test progress reporter with metrics following Given-When-Then pattern
func TestProgressReporterWithMetrics(t *testing.T) {
	t.Run("Given progress update When reporting with metrics Then should include runtime data", func(t *testing.T) {
		// Given
		metrics := initMetrics()
		metrics.TrackGoroutineSpawn()
		
		var loggedUpdate ProgressUpdate
		mockLogInfo := func(msg string, args ...slog.Attr) {
			if msg == "Progress Update" {
				// Verify that goroutine metrics are included
				for _, attr := range args {
					switch attr.Key {
					case "active_goroutines":
						assert.Greater(t, attr.Value.Int64(), int64(0))
					case "total_spawned":
						assert.Equal(t, int64(1), attr.Value.Int64())
					}
				}
			}
		}

		progressReporter := createProgressReporterWithMetrics(mockLogInfo)
		update := ProgressUpdate{
			Org:         "test-org",
			BatchNumber: 1,
			Status:      "processing",
		}

		// When
		progressReporter(update)

		// Then - Verification is in the mock function
		assert.Equal(t, "test-org", loggedUpdate.Org)
	})
}

// Test execution time tracking following Given-When-Then pattern
func TestExecutionTimeTracking(t *testing.T) {
	t.Run("Given operation When tracking execution time Then should log duration", func(t *testing.T) {
		// Given
		logCalled := false
		mockLogInfo := func(msg string, args ...slog.Attr) {
			if msg == "Operation Completed" {
				logCalled = true
				// Verify duration is logged
				for _, attr := range args {
					if attr.Key == "duration" {
						assert.NotEmpty(t, attr.Value.String())
					}
					if attr.Key == "operation" {
						assert.Equal(t, "test_operation", attr.Value.String())
					}
				}
			}
		}

		// When
		cleanup := trackExecutionTime(mockLogInfo, "test_operation")
		time.Sleep(10 * time.Millisecond) // Ensure some execution time
		cleanup()

		// Then
		assert.True(t, logCalled)
	})

	t.Run("Given nil logger When tracking execution time Then should not panic", func(t *testing.T) {
		// Given & When & Then
		assert.NotPanics(t, func() {
			cleanup := trackExecutionTime(nil, "test_operation")
			cleanup()
		})
	})
}

// Test concurrent metrics access following Given-When-Then pattern
func TestConcurrentMetricsAccess(t *testing.T) {
	t.Run("Given metrics instance When accessing concurrently Then should be thread-safe", func(t *testing.T) {
		// Given
		metrics := initMetrics()
		done := make(chan bool)
		
		// When - Multiple goroutines accessing metrics
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				
				for j := 0; j < 100; j++ {
					metrics.TrackGoroutineSpawn()
					metrics.UpdateActiveGoroutines()
					metrics.GetSnapshot()
				}
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		// Then
		_, _, total, _ := metrics.GetSnapshot()
		assert.Equal(t, int64(1000), total)
	})
}

// Test metrics integration with workflow following Given-When-Then pattern
func TestMetricsIntegration(t *testing.T) {
	t.Run("Given workflow execution When metrics are tracked Then should provide comprehensive timing", func(t *testing.T) {
		// Given
		globalMetrics = nil // Reset for clean test
		
		timingReports := make(map[string]bool)
		mockLogInfo := func(msg string, args ...slog.Attr) {
			if msg == "Operation Completed" {
				for _, attr := range args {
					if attr.Key == "operation" {
						timingReports[attr.Value.String()] = true
					}
				}
			}
		}

		// When
		cleanup1 := trackExecutionTime(mockLogInfo, "operation1")
		cleanup2 := trackExecutionTime(mockLogInfo, "operation2")
		time.Sleep(10 * time.Millisecond)
		cleanup1()
		cleanup2()

		// Then
		assert.True(t, timingReports["operation1"])
		assert.True(t, timingReports["operation2"])
	})
}