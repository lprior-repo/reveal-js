package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func LoadConfig() (*Config, error) {
	config := &Config{
		Organizations: []string{"hashicorp", "terraform-aws-modules"},
		OutputFile:    "terraform-analysis.json",
		CacheFile:     "terraform-cache.json",
		Concurrency:   20, // Increased default concurrency for better performance
		ForceRefresh:  false,
		UseDynamoDB:   false,
		AWSRegion:     "us-east-1",
		TableName:     "terraform-analyzer-cache",
	}

	// Load from environment variables and .env file
	envVars := loadEnvFile()

	if token := getEnvValue(envVars, "GITHUB_TOKEN"); token != "" {
		config.GitHubToken = token
	} else if token := getEnvValue(envVars, "GH_TOKEN"); token != "" {
		config.GitHubToken = token
	}

	if region := getEnvValue(envVars, "AWS_REGION"); region != "" {
		config.AWSRegion = region
	}
	if table := getEnvValue(envVars, "TABLE_NAME"); table != "" {
		config.TableName = table
	}
	if output := getEnvValue(envVars, "OUTPUT_FILE"); output != "" {
		config.OutputFile = output
	}
	if cache := getEnvValue(envVars, "CACHE_FILE"); cache != "" {
		config.CacheFile = cache
	}
	if refresh := getEnvValue(envVars, "FORCE_REFRESH"); refresh == "true" {
		config.ForceRefresh = true
	}
	if useDynamo := getEnvValue(envVars, "USE_DYNAMODB"); useDynamo == "true" {
		config.UseDynamoDB = true
	}
	if concurrency := getEnvValue(envVars, "CONCURRENCY"); concurrency != "" {
		if c, err := strconv.Atoi(concurrency); err == nil && c > 0 {
			config.Concurrency = c
		}
	}

	// Override with command line arguments
	if len(os.Args) > 1 {
		config.Organizations = os.Args[1:]
	}

	return config, nil
}

func loadEnvFile() map[string]string {
	envVars := make(map[string]string)
	file, err := os.Open(".env")
	if err != nil {
		return envVars
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(strings.Trim(parts[1], "\"'"))
			envVars[key] = value
		}
	}
	return envVars
}

func getEnvValue(envVars map[string]string, key string) string {
	if value, exists := envVars[key]; exists {
		return value
	}
	return os.Getenv(key)
}