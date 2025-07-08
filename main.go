package main

import (
	"context"
	"fmt"
	"os"

	ants "github.com/panjf2000/ants/v2"
)

// Legacy functions for test compatibility
func NewGhorgOrgCloner() *GhorgOrgCloner {
	return &GhorgOrgCloner{cloner: CreateGhorgOrgCloner()}
}

func NewEnvConfigLoader() *EnvConfigLoader {
	return &EnvConfigLoader{loader: CreateEnvConfigLoader()}
}

func (l *EnvConfigLoader) LoadConfig(ctx context.Context, envFilePath string) (*Config, error) {
	return l.loader(ctx, envFilePath)
}

func (c *GhorgOrgCloner) CloneOrg(ctx context.Context, org, targetDir, token string, concurrency int) error {
	return c.cloner(ctx, org, targetDir, token, concurrency)
}

func processOrgsFromEnv(envFilePath string) error {
	ctx := context.Background()
	return ExecuteWorkflow(ctx, envFilePath, CreateEnvConfigLoader(), CreateGhorgOrgCloner())
}

func validateConfig(config *Config) error {
	return config.ValidateAndSetDefaults()
}

func createBatchProcessor(logInfo, logError LogFunc, progressTracker *ProgressTracker, progressReporter ProgressFunc) BatchProcessor {
	return func(ctx context.Context, batch Batch, pool interface{}, cloner OrgCloner, config *Config) []error {
		if poolPtr, ok := pool.(*ants.Pool); ok {
			return processBatch(ctx, batch, poolPtr, cloner, config)
		}
		return []error{createError("invalid pool type")}
	}
}

func main() {
	var envFilePath string
	if len(os.Args) > 1 {
		envFilePath = os.Args[1]
	}

	ctx := context.Background()
	err := ExecuteWorkflow(ctx, envFilePath, CreateEnvConfigLoader(), CreateGhorgOrgCloner())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
