package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	F "github.com/IBM/fp-go/function"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// EnvConfig represents environment-based configuration
type EnvConfig struct {
	TargetDir   string   `envconfig:"GHORG_TARGET_DIR" required:"true"`
	Orgs        []string `envconfig:"GHORG_ORGS" required:"true"`
	Concurrency int      `envconfig:"GHORG_CONCURRENCY" default:"25"`
	GitHubToken string   `envconfig:"GHORG_GITHUB_TOKEN"`
	BatchSize   int      `envconfig:"GHORG_BATCH_SIZE" default:"50"`
}

// CreateEnvConfigLoader creates a functional environment-based config loader
func CreateEnvConfigLoader() ConfigLoader {
	return func(ctx context.Context, envFilePath string) (*Config, error) {
		logInfo, logError := createLoggers()

		// Direct implementation without Pipe1 due to type mismatch
		logInfo("Starting configuration loading",
			slog.String("env_file_path", envFilePath),
			slog.String("operation", "config_load_start"))

		// Check context cancellation
		select {
		case <-ctx.Done():
			logError("Config loading cancelled",
				slog.String("error", ctx.Err().Error()),
				slog.String("operation", "config_load_cancelled"))
			return nil, fmt.Errorf("config loading cancelled: %w", ctx.Err())
		default:
		}

		// Load .env file if path is provided using functional approach
		envLoadResult := F.Pipe1(envFilePath, func(path string) error {
			if path == "" {
				logInfo("No env file path provided, using system environment",
					slog.String("operation", "env_file_skipped"))
				return nil
			}

			logInfo("Loading env file",
				slog.String("env_file_path", path),
				slog.String("operation", "env_file_loading"))

			if err := godotenv.Load(path); err != nil {
				logError("Failed to load env file",
					slog.String("env_file_path", path),
					slog.String("error", err.Error()),
					slog.String("operation", "env_file_load_failed"))
				return fmt.Errorf("failed to load .env file %s: %w", path, err)
			}

			logInfo("Env file loaded successfully",
				slog.String("env_file_path", path),
				slog.String("operation", "env_file_loaded"))
			return nil
		})

		if envLoadResult != nil {
			return nil, envLoadResult
		}

		// Parse environment variables into EnvConfig using functional approach
		logInfo("Parsing environment variables",
			slog.String("operation", "env_parsing"))

		var envConfig EnvConfig
		if err := envconfig.Process("", &envConfig); err != nil {
			logError("Failed to parse environment variables",
				slog.String("error", err.Error()),
				slog.String("operation", "env_parsing_failed"))
			return nil, fmt.Errorf("failed to parse environment variables: %w", err)
		}

		logInfo("Environment variables parsed successfully",
			slog.Int("org_count", len(envConfig.Orgs)),
			slog.String("target_dir", envConfig.TargetDir),
			slog.Int("concurrency", envConfig.Concurrency),
			slog.Int("batch_size", envConfig.BatchSize),
			slog.String("operation", "env_parsed"))

		// Convert to main Config struct using pure function
		config := convertEnvConfigToConfig(&envConfig)

		// Validate and set defaults using pure function
		if err := config.ValidateAndSetDefaults(); err != nil {
			logError("Configuration validation failed",
				slog.String("error", err.Error()),
				slog.String("operation", "config_validation_failed"))
			return nil, fmt.Errorf("invalid configuration: %w", err)
		}

		logInfo("Configuration validated and defaults set",
			slog.Int("final_org_count", len(config.Orgs)),
			slog.Int("final_concurrency", config.Concurrency),
			slog.Int("final_batch_size", config.BatchSize),
			slog.String("operation", "config_validated"))

		return config, nil
	}
}

// Legacy wrapper for backwards compatibility
type EnvConfigLoader struct {
	loader ConfigLoader
}

// NewEnvConfigLoader creates a new environment-based config loader
func NewEnvConfigLoader() *EnvConfigLoader {
	return &EnvConfigLoader{
		loader: CreateEnvConfigLoader(),
	}
}

// LoadConfig loads configuration from environment variables (side effect)
func (l *EnvConfigLoader) LoadConfig(ctx context.Context, envFilePath string) (*Config, error) {
	return l.loader(ctx, envFilePath)
}

// convertEnvConfigToConfig converts EnvConfig to Config (pure transformation)
func convertEnvConfigToConfig(envConfig *EnvConfig) *Config {
	// Pure function to process and flatten org strings
	var orgs []string
	for _, org := range envConfig.Orgs {
		trimmed := F.Pipe1(org, strings.TrimSpace)
		if strings.Contains(trimmed, ",") {
			parts := strings.Split(trimmed, ",")
			for _, part := range parts {
				if cleaned := strings.TrimSpace(part); cleaned != "" {
					orgs = append(orgs, cleaned)
				}
			}
		} else if trimmed != "" {
			orgs = append(orgs, trimmed)
		}
	}

	return &Config{
		TargetDir:   envConfig.TargetDir,
		Orgs:        orgs,
		Concurrency: envConfig.Concurrency,
		GitHubToken: envConfig.GitHubToken,
		BatchSize:   envConfig.BatchSize,
	}
}
