package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Global regex patterns for parsing ghorg output
var (
	repoCountPattern = regexp.MustCompile(`(\d+)\s+repos?\s+found|found\s+(\d+)\s+repositor`)
	repoNamePatterns = []*regexp.Regexp{
		regexp.MustCompile(`cloning\s+([^/\s]+/[^/\s]+)`),
		regexp.MustCompile(`successfully cloned\s+([^/\s]+/[^/\s]+)`),
		regexp.MustCompile(`([^/\s]+/[^/\s]+)\s+already exists`),
		regexp.MustCompile(`processing\s+([^/\s]+/[^/\s]+)`),
	}
)

// Math utility functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Error creation utility
func createError(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// Logging utilities
func createLoggers() (LogFunc, LogFunc) {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	logInfo := func(msg string, args ...slog.Attr) {
		logger.LogAttrs(context.Background(), slog.LevelInfo, msg, args...)
	}

	logError := func(msg string, args ...slog.Attr) {
		logger.LogAttrs(context.Background(), slog.LevelError, msg, args...)
	}

	return logInfo, logError
}

// BoundedBuffer implementation for managing command output
func NewBoundedBuffer(maxSize int) *BoundedBuffer {
	return &BoundedBuffer{
		buffer:  make([]string, maxSize),
		maxSize: maxSize,
	}
}

func (bb *BoundedBuffer) Write(line string) {
	bb.mutex.Lock()
	defer bb.mutex.Unlock()

	bb.buffer[bb.tail] = line
	bb.tail = (bb.tail + 1) % bb.maxSize

	if bb.size < bb.maxSize {
		bb.size++
	} else {
		bb.head = (bb.head + 1) % bb.maxSize
	}
}

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

// Generic slice chunking utility
func chunkSlice[T any](slice []T, size int) [][]T {
	if len(slice) == 0 {
		return [][]T{}
	}

	if size <= 0 {
		size = 1
	}

	numChunks := (len(slice) + size - 1) / size
	chunks := make([][]T, 0, numChunks)

	for i := 0; i < len(slice); i += size {
		end := min(i+size, len(slice))
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

// Path expansion utility
func expandPath(path, homeDir string) string {
	if path == "" {
		return ""
	}

	if len(path) >= 2 && path[:2] == "~/" {
		if path == "~/" {
			return homeDir
		}
		if homeDir == "" {
			return "/" + path[2:]
		}
		return homeDir + "/" + path[2:]
	}
	return path
}

// Command building utility
func buildCommand(org, targetDir, token string, concurrency int) []string {
	org = strings.TrimSpace(org)
	targetDir = strings.TrimSpace(targetDir)
	token = strings.TrimSpace(token)

	if concurrency <= 0 {
		concurrency = 1
	}

	base := []string{"clone", org, "--path", targetDir, "--concurrency", strconv.Itoa(concurrency), "--skip-archived"}

	if token != "" {
		return append(base, "--token", token)
	}
	return append(base, "--no-token")
}

// Output parsing utilities
func extractRepoCount(line string) int {
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

func extractRepoName(line string) string {
	lowerLine := strings.ToLower(line)
	for _, pattern := range repoNamePatterns {
		if matches := pattern.FindStringSubmatch(lowerLine); len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

// Batch processing utilities
func createBatches(orgs []string, size int) []Batch {
	if len(orgs) == 0 {
		return []Batch{}
	}

	batchSize := size
	if batchSize <= 0 {
		batchSize = max(1, min(len(orgs), 100)) // Default to reasonable value but allow large batches
	} else {
		batchSize = min(batchSize, len(orgs)) // No hard upper limit - respect user's choice
	}

	chunks := chunkSlice(orgs, batchSize)
	batches := make([]Batch, 0, len(chunks))
	for i, chunk := range chunks {
		batches = append(batches, Batch{
			Orgs:        append([]string(nil), chunk...),
			BatchNumber: i + 1,
		})
	}
	return batches
}

// Error collection utility
func collectAndReportErrors(allErrors []error, logInfo LogFunc) error {
	if len(allErrors) == 0 {
		logInfo("All batches processed successfully",
			slog.Int("error_count", 0),
			slog.String("operation", "execution_successful"))
		return nil
	}

	errorMessages := make([]string, len(allErrors))
	for i, err := range allErrors {
		errorMessages[i] = err.Error()
	}

	return createError("processing completed with %d errors: %s",
		len(allErrors), strings.Join(errorMessages, "; "))
}
