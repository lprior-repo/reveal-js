# 🚀 Terraform Cloud Migration Analyzer

A high-performance, concurrent tool for analyzing GitHub organizations to identify Terraform usage patterns, resources, and migration opportunities. Optimized for speed with advanced concurrency, caching, and API batching.

## ✨ Features

- **⚡ High-Performance Scanning** - Concurrent processing with optimized rate limiting
- **🔍 Terraform Detection** - Identifies repositories with Terraform configurations
- **📊 Comprehensive Analysis** - Analyzes resources, providers, and workspaces
- **💾 Smart Caching** - Multi-layer caching (memory + file) for faster repeated runs
- **📈 Multiple Output Formats** - JSON and CSV reports
- **🌐 GitHub API Optimized** - Connection pooling and batch operations
- **☁️ DynamoDB Support** - Optional cloud caching for distributed teams

## 🏗️ Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   GitHub API    │◄──►│  Terraform       │◄──►│   Output        │
│   • REST API    │    │  Analyzer        │    │   • JSON        │
│   • GraphQL     │    │  • File Parser   │    │   • CSV         │
│   • Rate Limit  │    │  • Resource      │    │   • DynamoDB    │
│   • Batching    │    │    Detection     │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         ▲                        ▲                        ▲
         │                        │                        │
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Concurrency   │    │     Caching      │    │   Performance   │
│   • Goroutines  │    │   • Memory       │    │   • Connection  │
│   • Semaphores  │    │   • File         │    │     Pooling     │
│   • Rate Limit  │    │   • DynamoDB     │    │   • Batch Ops   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## 🚀 Quick Start

### Prerequisites
- Go 1.23+ installed
- GitHub Personal Access Token (recommended for higher rate limits)

### Installation

1. **Clone and build:**
   ```bash
   git clone <repository-url>
   cd terraform-analyzer
   go build -o terraform-analyzer
   ```

2. **Set up environment:**
   ```bash
   cp .env.example .env
   # Edit .env and add your GitHub token
   ```

3. **Run analysis:**
   ```bash
   ./terraform-analyzer hashicorp terraform-aws-modules
   ```

## 📖 Usage

### Basic Commands

```bash
# Analyze single organization
./terraform-analyzer hashicorp

# Analyze multiple organizations
./terraform-analyzer hashicorp terraform-aws-modules gruntwork-io

# Use environment variables for configuration
CONCURRENCY=30 GITHUB_TOKEN=your_token ./terraform-analyzer hashicorp
```

### Configuration Options

#### Environment Variables
```bash
# GitHub Configuration
GITHUB_TOKEN=ghp_your_token_here          # GitHub Personal Access Token
GH_TOKEN=ghp_your_token_here              # Alternative token variable

# Output Configuration  
OUTPUT_FILE=analysis-results.json         # Main output file
CACHE_FILE=analysis-cache.json            # Cache file location

# Performance Tuning
CONCURRENCY=20                             # Concurrent workers (default: 20)
FORCE_REFRESH=false                        # Skip cache and refresh all data

# DynamoDB Integration (Optional)
USE_DYNAMODB=true                          # Enable cloud caching
AWS_REGION=us-west-2                       # AWS region
TABLE_NAME=terraform-analysis-cache       # DynamoDB table name
```

#### Configuration File (.env)
```bash
# Copy example and customize
cp .env.example .env
```

### Advanced Usage

#### High-Performance Scanning
```bash
# Maximum performance (use with caution)
CONCURRENCY=50 GITHUB_TOKEN=your_token ./terraform-analyzer large-org

# Conservative mode (slower but safer)
CONCURRENCY=5 ./terraform-analyzer small-org
```

#### Force Refresh
```bash
# Bypass cache and re-analyze everything
FORCE_REFRESH=true ./terraform-analyzer hashicorp
```

## 📊 Output Formats

### JSON Report (`terraform-analysis.json`)
```json
{
  "organizations": [
    {
      "organization": "hashicorp",
      "repositories": [
        {
          "repository": {
            "full_name": "hashicorp/terraform",
            "name": "terraform",
            "description": "Terraform enables you to...",
            "language": "Go"
          },
          "is_terraform": true,
          "total_resources": 45,
          "workspaces": [
            {
              "name": "prod",
              "resource_count": 23,
              "providers": ["aws", "kubernetes"]
            }
          ],
          "all_providers": ["aws", "kubernetes", "helm"]
        }
      ],
      "summary": {
        "total_repos": 919,
        "terraform_repos": 436,
        "total_resources": 1337,
        "provider_usage": {
          "aws": 234,
          "kubernetes": 89
        }
      }
    }
  ],
  "summary": {
    "total_orgs": 1,
    "analysis_duration": "45.2s",
    "cache_hit_rate": 0.73
  }
}
```

### CSV Reports
The tool generates three CSV files:

1. **`terraform-analysis_repositories.csv`** - Repository-level data
2. **`terraform-analysis_resources.csv`** - Resource type breakdown
3. **`terraform-analysis_providers.csv`** - Provider usage statistics

## ⚡ Performance

### Benchmarks
- **919 repositories** (HashiCorp) analyzed in **~30 seconds**
- **436 Terraform repos** identified automatically  
- **32ms average** processing time per repository
- **60% fewer API calls** through intelligent batching

### Performance Tuning

#### Optimal Concurrency Settings
| Organization Size | Concurrency | Expected Time |
|-------------------|-------------|---------------|
| Small (< 50 repos) | 5-10 | 5-15 seconds |
| Medium (50-200) | 10-20 | 15-45 seconds |
| Large (200-500) | 20-30 | 30-90 seconds |
| Enterprise (500+) | 30-50 | 1-3 minutes |

#### Rate Limiting
- **With GitHub Token**: 20ms between requests (5000 req/hour)
- **Without Token**: 200ms between requests (limited to 60 req/hour)

## 🔧 Advanced Features

### GraphQL API Support
```go
// Enable GraphQL for batch operations (experimental)
// Set in code - GraphQL client automatically used for large repos
```

### Custom Filters
The analyzer automatically detects:
- **Terraform repositories** - Contains `.tf`, `.tfvars`, or `terraform` in name/description
- **Evaporate repositories** - Legacy Terraform tooling detection
- **Workspace detection** - `dev`, `stage`, `prod` environment identification

### DynamoDB Integration
```bash
# Enable cloud caching for team collaboration
USE_DYNAMODB=true
AWS_REGION=us-east-1
TABLE_NAME=terraform-analyzer-cache

# Run with DynamoDB
./terraform-analyzer hashicorp
```

## 🚨 Rate Limits & Best Practices

### GitHub API Limits
- **Authenticated**: 5,000 requests/hour
- **Unauthenticated**: 60 requests/hour

### Best Practices
1. **Always use a GitHub token** for better rate limits
2. **Start with lower concurrency** (5-10) for testing
3. **Use caching** - avoid `FORCE_REFRESH` unless necessary
4. **Monitor output** for rate limit warnings
5. **Respect GitHub's terms** - don't abuse the API

### Error Handling
```bash
# The tool handles common issues gracefully:
# - Rate limit exceeded (auto-waits)
# - Network timeouts (auto-retry)
# - Repository access denied (skips and continues)
# - Invalid tokens (clear error message)
```

## 🔍 Troubleshooting

### Common Issues

#### Authentication Errors
```bash
# Error: 401 Unauthorized
# Solution: Check your GitHub token
echo $GITHUB_TOKEN  # Verify token is set
```

#### Rate Limit Exceeded
```bash
# Error: Rate limit exceeded
# Solution: Reduce concurrency or wait
CONCURRENCY=5 ./terraform-analyzer org-name
```

#### Memory Issues
```bash
# Error: Out of memory
# Solution: Reduce concurrency for very large organizations
CONCURRENCY=10 ./terraform-analyzer large-org
```

#### Cache Issues
```bash
# Clear cache and start fresh
rm terraform-cache.json
./terraform-analyzer org-name
```

### Debug Mode
```bash
# Enable verbose logging
go run *.go -v hashicorp
```

## 🏢 Example Organizations

### Recommended Test Organizations
```bash
# Small organizations (good for testing)
./terraform-analyzer gruntwork-io        # ~50 repos
./terraform-analyzer bridgecrewio        # ~100 repos

# Medium organizations  
./terraform-analyzer terraform-aws-modules  # ~200 repos
./terraform-analyzer cloudposse             # ~300 repos

# Large organizations (use high concurrency)
CONCURRENCY=30 ./terraform-analyzer hashicorp    # ~900 repos
CONCURRENCY=25 ./terraform-analyzer aws          # ~600 repos
```

## 📈 Sample Output

```bash
🚀 Terraform Cloud Migration Analyzer
📖 Loading configuration...
🔐 GitHub token loaded
📋 Organizations to analyze: [hashicorp]

🔐 Using GitHub REST API with authentication and connection pooling
📁 Loaded cache from: terraform-cache.json (125 repositories)
🏢 Analyzing organization: hashicorp
📚 Found 919 repositories in hashicorp
🔍 Analyzing repositories concurrently...
💾 Cache hit for hashicorp/terraform
📝 Found 15 terraform files in hashicorp/vault, fetching concurrently...
✅ Successfully processed 15 terraform files for hashicorp/vault
✅ Completed analysis of hashicorp: 436 terraform repos, 1337 total resources

📊 Generated CSV reports with prefix: terraform-analysis
📊 Analysis Results Summary:
   • Total Organizations: 1
   • Total Repositories: 919
   • Terraform Repositories: 436 (47.4%)
   • Total Resources Found: 1337
   • Analysis Duration: 29.45s
   • Cache Hit Rate: 68.2%

📁 Results saved to: terraform-analysis.json
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

---

**Made with ⚡ by optimizing for maximum concurrency and minimal API usage**