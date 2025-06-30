package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// WriteCSVReports generates CSV reports from analysis results
func WriteCSVReports(results []OrganizationResult, outputPrefix string) error {
	// Repository summary report
	if err := writeRepositoryReport(results, outputPrefix+"_repositories.csv"); err != nil {
		return fmt.Errorf("failed to write repository report: %w", err)
	}

	// Resource summary report
	if err := writeResourceReport(results, outputPrefix+"_resources.csv"); err != nil {
		return fmt.Errorf("failed to write resource report: %w", err)
	}

	// Provider usage report
	if err := writeProviderReport(results, outputPrefix+"_providers.csv"); err != nil {
		return fmt.Errorf("failed to write provider report: %w", err)
	}

	fmt.Printf("📊 Generated CSV reports with prefix: %s\n", outputPrefix)
	return nil
}

// writeRepositoryReport creates a CSV report of all repositories
func writeRepositoryReport(results []OrganizationResult, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Organization", "Repository", "IsTerraform", "IsEvaporate",
		"TotalResources", "TotalWorkspaces", "Providers", "LastAnalyzed",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, org := range results {
		for _, repo := range org.Repositories {
			record := []string{
				org.Organization,
				repo.Repository.FullName,
				strconv.FormatBool(repo.IsTerraform),
				strconv.FormatBool(repo.IsEvaporate),
				strconv.Itoa(repo.TotalResources),
				strconv.Itoa(len(repo.Workspaces)),
				strings.Join(repo.AllProviders, ";"),
				repo.LastAnalyzed.Format("2006-01-02 15:04:05"),
			}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
	}

	return nil
}

// writeResourceReport creates a CSV report of resource types by organization
func writeResourceReport(results []OrganizationResult, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"Organization", "ResourceType", "Count"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, org := range results {
		for resourceType, count := range org.Summary.ResourcesByType {
			record := []string{
				org.Organization,
				resourceType,
				strconv.Itoa(count),
			}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
	}

	return nil
}

// writeProviderReport creates a CSV report of provider usage
func writeProviderReport(results []OrganizationResult, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"Organization", "Provider", "UsageCount"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, org := range results {
		for provider, count := range org.Summary.ProviderUsage {
			record := []string{
				org.Organization,
				provider,
				strconv.Itoa(count),
			}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
	}

	return nil
}