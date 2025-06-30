package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func WriteJSONFile(filename string, data interface{}) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filename, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

func ReadJSONFile(filename string, data interface{}) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(data); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	return nil
}

func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// CountWorkspacesByType counts workspaces by their type
func CountWorkspacesByType(workspaces []WorkspaceInfo) map[string]int {
	counts := make(map[string]int)
	for _, workspace := range workspaces {
		workspaceType := workspace.Name
		if workspaceType == "" {
			workspaceType = "default"
		}
		counts[workspaceType]++
	}
	return counts
}

// CountResourcesByWorkspace counts resources by workspace type
func CountResourcesByWorkspace(workspaces []WorkspaceInfo) map[string]int {
	counts := make(map[string]int)
	for _, workspace := range workspaces {
		workspaceType := workspace.Name
		if workspaceType == "" {
			workspaceType = "default"
		}
		counts[workspaceType] += workspace.ResourceCount
	}
	return counts
}

// HasTfvarsInWorkspaces checks if any workspace has tfvars files
func HasTfvarsInWorkspaces(workspaces []WorkspaceInfo) bool {
	for _, workspace := range workspaces {
		if workspace.HasTfvars {
			return true
		}
	}
	return false
}

// HasBackendInWorkspaces checks if any workspace has backend configuration
func HasBackendInWorkspaces(workspaces []WorkspaceInfo) bool {
	for _, workspace := range workspaces {
		if workspace.HasBackend {
			return true
		}
	}
	return false
}

// GetTopResourceTypes returns top N resource types from workspaces
func GetTopResourceTypes(workspaces []WorkspaceInfo, n int) string {
	typeCounts := make(map[string]int)
	for _, workspace := range workspaces {
		for _, resource := range workspace.Resources {
			typeCounts[resource.Type]++
		}
	}
	
	// Simple implementation - return first N types
	var types []string
	count := 0
	for resourceType := range typeCounts {
		if count >= n {
			break
		}
		types = append(types, resourceType)
		count++
	}
	
	return joinStrings(types, ",")
}

// GetTerraformFilesByWorkspace filters terraform files by workspace
func GetTerraformFilesByWorkspace(files []TerraformFile, workspaceName string) []TerraformFile {
	var filtered []TerraformFile
	for _, file := range files {
		if file.Workspace == workspaceName {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

// GetResourceTypesFromWorkspace gets comma-separated resource types from workspace
func GetResourceTypesFromWorkspace(workspace WorkspaceInfo) string {
	typeSet := make(map[string]bool)
	for _, resource := range workspace.Resources {
		typeSet[resource.Type] = true
	}
	
	var types []string
	for resourceType := range typeSet {
		types = append(types, resourceType)
	}
	
	return joinStrings(types, ",")
}

// GetTerraformFileNames gets comma-separated terraform file names
func GetTerraformFileNames(files []TerraformFile) string {
	var names []string
	for _, file := range files {
		names = append(names, file.Path)
	}
	return joinStrings(names, ",")
}

// InferProviderFromResourceType infers provider name from resource type
func InferProviderFromResourceType(resourceType string) string {
	if len(resourceType) == 0 {
		return "unknown"
	}
	
	// Simple inference based on common prefixes
	switch {
	case startsWith(resourceType, "aws_"):
		return "aws"
	case startsWith(resourceType, "azurerm_"):
		return "azurerm"
	case startsWith(resourceType, "google_"):
		return "google"
	case startsWith(resourceType, "kubernetes_"):
		return "kubernetes"
	case startsWith(resourceType, "helm_"):
		return "helm"
	default:
		return "other"
	}
}

// Helper functions
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// MigrationComplexity represents the complexity of migrating a repository
type MigrationComplexity struct {
	Score               int
	Reason              string
	EstimatedHours      int
	Priority            string
	RequiredActions     []string
	RiskFactors         []string
	RecommendedApproach string
}

// CalculateMigrationComplexity assesses the complexity of migrating a repository
func CalculateMigrationComplexity(repo RepositoryAnalysis) MigrationComplexity {
	score := 1
	var reasons []string
	var actions []string
	var risks []string
	
	// Base complexity on resource count
	if repo.TotalResources > 100 {
		score += 3
		reasons = append(reasons, "high resource count")
		risks = append(risks, "large infrastructure footprint")
	} else if repo.TotalResources > 50 {
		score += 2
		reasons = append(reasons, "moderate resource count")
	}
	
	// Complexity based on workspace count
	if len(repo.Workspaces) > 5 {
		score += 2
		reasons = append(reasons, "multiple workspaces")
		actions = append(actions, "consolidate workspaces")
	}
	
	// Provider diversity complexity
	if len(repo.AllProviders) > 3 {
		score += 2
		reasons = append(reasons, "multiple providers")
		risks = append(risks, "provider version conflicts")
	}
	
	// Determine priority and approach
	priority := "Low"
	approach := "Standard migration"
	
	if score >= 7 {
		priority = "High"
		approach = "Phased migration with careful testing"
	} else if score >= 4 {
		priority = "Medium"
		approach = "Incremental migration"
	}
	
	return MigrationComplexity{
		Score:               score,
		Reason:              joinStrings(reasons, ", "),
		EstimatedHours:      score * 8, // Rough estimate
		Priority:            priority,
		RequiredActions:     actions,
		RiskFactors:         risks,
		RecommendedApproach: approach,
	}
}

// ProviderMigrationComplexity represents provider-specific migration complexity
type ProviderMigrationComplexity struct {
	Level                 string
	RequiresVersionUpdate bool
	HasBreakingChanges    bool
	Notes                 string
	RecommendedVersion    string
}

// AssessProviderMigrationComplexity assesses migration complexity for a specific provider
func AssessProviderMigrationComplexity(providerName string) ProviderMigrationComplexity {
	switch providerName {
	case "aws":
		return ProviderMigrationComplexity{
			Level:                 "Medium",
			RequiresVersionUpdate: true,
			HasBreakingChanges:    true,
			Notes:                 "AWS provider has frequent updates with breaking changes",
			RecommendedVersion:    "~> 5.0",
		}
	case "azurerm":
		return ProviderMigrationComplexity{
			Level:                 "Medium",
			RequiresVersionUpdate: true,
			HasBreakingChanges:    true,
			Notes:                 "Azure provider requires careful version management",
			RecommendedVersion:    "~> 3.0",
		}
	case "google":
		return ProviderMigrationComplexity{
			Level:                 "Low",
			RequiresVersionUpdate: false,
			HasBreakingChanges:    false,
			Notes:                 "Google provider is generally stable",
			RecommendedVersion:    "~> 4.0",
		}
	default:
		return ProviderMigrationComplexity{
			Level:                 "Low",
			RequiresVersionUpdate: false,
			HasBreakingChanges:    false,
			Notes:                 "Standard provider migration",
			RecommendedVersion:    "latest",
		}
	}
}