package main

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Core configuration types
type Config struct {
	Orgs        []string `envconfig:"ORGS" json:"orgs"`
	BatchSize   int      `envconfig:"BATCH_SIZE" default:"50" json:"batch_size"`
	Concurrency int      `envconfig:"CONCURRENCY" default:"25" json:"concurrency"`
	TargetDir   string   `envconfig:"TARGET_DIR" required:"true" json:"target_dir"`
	GitHubToken string   `envconfig:"GITHUB_TOKEN" json:"github_token"`
}

type EnvConfig struct {
	TargetDir   string   `envconfig:"GHORG_TARGET_DIR" required:"true"`
	Orgs        []string `envconfig:"GHORG_ORGS" required:"true"`
	Concurrency int      `envconfig:"GHORG_CONCURRENCY" default:"25"`
	GitHubToken string   `envconfig:"GHORG_GITHUB_TOKEN"`
	BatchSize   int      `envconfig:"GHORG_BATCH_SIZE" default:"50"`
}

// Processing types
type Batch struct {
	Orgs        []string
	BatchNumber int
}

type ProcessingPlan struct {
	Batches   []Batch
	TotalOrgs int
}

// Progress tracking types
type ProgressTracker struct {
	TotalOrgs        int
	ProcessedOrgs    int
	TotalBatches     int
	ProcessedBatches int
	StartTime        time.Time
	CurrentOrg       string
	CurrentBatch     int
}

type ProgressUpdate struct {
	Org           string
	BatchNumber   int
	CompletedOrgs int
	TotalOrgs     int
	ElapsedTime   time.Duration
	EstimatedETA  time.Duration
	Status        string
}

// Functional types
type ProgressFunc func(update ProgressUpdate)
type ConfigLoader func(ctx context.Context, configPath string) (*Config, error)
type OrgCloner func(ctx context.Context, org, targetDir, token string, concurrency int) error
type LogFunc func(msg string, args ...slog.Attr)
type BatchProcessor func(ctx context.Context, batch Batch, pool interface{}, cloner OrgCloner, config *Config) []error

// Buffer type for command output
type BoundedBuffer struct {
	buffer  []string
	head    int
	tail    int
	size    int
	maxSize int
	mutex   sync.RWMutex
}

// Legacy wrapper types for backward compatibility
type GhorgOrgCloner struct {
	cloner OrgCloner
}

type EnvConfigLoader struct {
	loader ConfigLoader
}
