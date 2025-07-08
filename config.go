package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Config validation and defaults
func (c *Config) ValidateAndSetDefaults() error {
	c.TargetDir = strings.TrimSpace(c.TargetDir)
	if c.TargetDir == "" {
		return fmt.Errorf("target_dir is required")
	}

	var validOrgs []string
	for _, org := range c.Orgs {
		if trimmed := strings.TrimSpace(org); trimmed != "" {
			validOrgs = append(validOrgs, trimmed)
		}
	}

	if len(validOrgs) == 0 {
		return fmt.Errorf("at least one organization must be specified")
	}

	c.Orgs = validOrgs

	if c.Concurrency <= 0 {
		c.Concurrency = max(1, min(len(c.Orgs), 50)) // Default to reasonable value but allow override
	}

	if c.BatchSize <= 0 {
		c.BatchSize = max(1, min(len(c.Orgs), 100)) // Default to reasonable value but allow override
	}

	return nil
}

// Configuration processing
func processConfig(config *Config) (ProcessingPlan, error) {
	if err := config.ValidateAndSetDefaults(); err != nil {
		return ProcessingPlan{}, err
	}

	batches := createBatches(config.Orgs, config.BatchSize)
	return ProcessingPlan{Batches: batches, TotalOrgs: len(config.Orgs)}, nil
}

// Environment configuration loader
func CreateEnvConfigLoader() ConfigLoader {
	return func(ctx context.Context, envFilePath string) (*Config, error) {
		logInfo, logError := createLoggers()

		logInfo("Starting configuration loading",
			slog.String("env_file_path", envFilePath),
			slog.String("operation", "config_load_start"))

		select {
		case <-ctx.Done():
			logError("Config loading cancelled",
				slog.String("error", ctx.Err().Error()),
				slog.String("operation", "config_load_cancelled"))
			return nil, fmt.Errorf("config loading cancelled: %w", ctx.Err())
		default:
		}

		if envFilePath != "" {
			if err := godotenv.Load(envFilePath); err != nil {
				logError("Failed to load env file",
					slog.String("env_file_path", envFilePath),
					slog.String("error", err.Error()),
					slog.String("operation", "env_file_load_failed"))
				return nil, fmt.Errorf("failed to load .env file %s: %w", envFilePath, err)
			}
		}

		var envConfig EnvConfig
		if err := envconfig.Process("", &envConfig); err != nil {
			logError("Failed to parse environment variables",
				slog.String("error", err.Error()),
				slog.String("operation", "env_parsing_failed"))
			return nil, fmt.Errorf("failed to parse environment variables: %w", err)
		}

		config := convertEnvConfigToConfig(&envConfig)

		if err := config.ValidateAndSetDefaults(); err != nil {
			logError("Configuration validation failed",
				slog.String("error", err.Error()),
				slog.String("operation", "config_validation_failed"))
			return nil, fmt.Errorf("invalid configuration: %w", err)
		}

		return config, nil
	}
}

// Environment config conversion
func convertEnvConfigToConfig(envConfig *EnvConfig) *Config {
	var orgs []string
	for _, org := range envConfig.Orgs {
		trimmed := strings.TrimSpace(org)
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
