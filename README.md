# GitHub Organization Cloner Wrapper

A fast, concurrent wrapper around the [ghorg](https://github.com/gabrie30/ghorg) CLI tool for cloning multiple GitHub organizations efficiently.

## Features

- **High-performance concurrent processing** - Clone multiple organizations with configurable concurrency (up to 50+)
- **Intelligent batch processing** - Configurable batch sizes (default: 50 repos at a time)
- **GitHub PAT support** - Authentication for private repositories

## Quick Start

### 1. Prerequisites

Install ghorg CLI tool:

```bash
# Installs the ghorg command-line tool, which is essential for the wrapper to function.
go install github.com/gabrie30/ghorg@latest
```

### 2. Build the wrapper

```bash
# Compiles the Go source code into an executable binary named 'ghorg-wrapper'.
make build
```

### 3. Create configuration

Create a `.env` file:

```bash
# Required: Specifies the destination directory for cloned repositories.
GHORG_TARGET_DIR=/tmp/my-repos
# Required: Defines a comma-separated list of GitHub organizations to clone.
GHORG_ORGS=hashicorp
# Optional: Sets the number of concurrent cloning operations to 50 for faster performance.
GHORG_CONCURRENCY=50
# Optional: Configures the wrapper to process 50 repositories at a time in each batch.
GHORG_BATCH_SIZE=50
# Optional: Provides a GitHub Personal Access Token for authenticating with GitHub to access private repositories.
GHORG_GITHUB_TOKEN=ghp_your_token_here
```

### 4. Run the wrapper

With .env file:

```bash
# Executes the wrapper, loading its configuration from the '.env' file in the current directory.
./ghorg-wrapper .env
```

With environment variables:

```bash
# Runs the wrapper by passing configuration as environment variables directly in the command line.
GHORG_TARGET_DIR="/tmp/repos" GHORG_ORGS="hashicorp" ./ghorg-wrapper
```

## Configuration Options

| Environment Variable | Required | Default | Description                                  |
| -------------------- | -------- | ------- | -------------------------------------------- |
| `GHORG_TARGET_DIR`   | Yes      | -       | Directory where repositories will be cloned  |
| `GHORG_ORGS`         | Yes      | -       | Comma-separated list of GitHub organizations |
| `GHORG_CONCURRENCY`  | No       | 25      | Number of concurrent clones                  |
| `GHORG_BATCH_SIZE`   | No       | 50      | Number of repositories per batch             |
| `GHORG_GITHUB_TOKEN` | No       | -       | GitHub Personal Access Token                 |

## Make Commands

```bash
# Essential commands
# Builds the application binary.
make build
# Runs all tests to ensure code quality and correctness.
make test
# Builds and runs the application, loading configuration from a .env file.
make run

# Development
# Executes a full development workflow, including dependency installation, formatting, and testing.
make dev
# Generates a test coverage report to measure the extent of testing.
make coverage
# Removes build artifacts to clean the project directory.
make clean

# Show all available commands
# Displays a list of all available Make commands for easy reference.
make help
```

## Architecture

This application follows the **Functional Core, Imperative Shell** pattern:

### Pure Core (No Side Effects)

- `planProcessing()` - Calculates batching strategy
- `createProcessingOptions()` - Transforms configuration
- `validateProcessingOptions()` - Validates inputs

### Imperative Shell (Side Effects)

- `EnvConfigLoader()` - Reads environment variables
- `GhorgOrgCloner()` - Executes ghorg CLI commands
- `ProcessingShell()` - Orchestrates Action → Calculation → Action flow

## Examples

### Clone HashiCorp repositories (289 repos tested successfully!)

```bash
# This command creates a '.env' file with the configuration needed to clone all public repositories from the 'hashicorp' organization.
cat > .env << EOF
# Specifies the target directory for all cloned repositories.
GHORG_TARGET_DIR=/tmp/hashicorp-repos
# Defines the GitHub organization whose repositories will be cloned.
GHORG_ORGS=hashicorp
# Sets the number of concurrent cloning operations to 50 for maximum speed.
GHORG_CONCURRENCY=50
# Configures the wrapper to handle 50 repositories in each batch.
GHORG_BATCH_SIZE=50
# Provides a GitHub Personal Access Token for authentication if private repositories are included.
GHORG_GITHUB_TOKEN=ghp_your_token_here
EOF

# This command executes the wrapper to begin cloning with the settings defined in the newly created '.env' file.
./ghorg-wrapper .env
```

### High-performance setup for large organizations

```bash
# This command runs the wrapper with a high-performance configuration suitable for very large organizations like 'kubernetes'.
GHORG_TARGET_DIR="/tmp/large-org" \
GHORG_ORGS="kubernetes" \
GHORG_BATCH_SIZE=50 \
GHORG_CONCURRENCY=50 \
GHORG_GITHUB_TOKEN="ghp_your_token" \
./ghorg-wrapper
```

### Multiple organizations

```bash
# This command demonstrates how to clone repositories from multiple organizations ('hashicorp', 'kubernetes', 'docker') in a single execution.
GHORG_TARGET_DIR="/tmp/multi-org" \
GHORG_ORGS="hashicorp,kubernetes,docker" \
GHORG_CONCURRENCY=25 \
./ghorg-wrapper
```

## Development

### Prerequisites

- Go 1.21+
- ghorg CLI tool installed
- Make (optional, but recommended)

### Testing

Comprehensive test suite with 81% coverage:

```bash
# Run all tests
make test

# Run with coverage report
make coverage
```

**Test Types Included:**

- Property-based testing with `pgregory.net/rapid`
- Mutation testing with `go-mutesting`
- Contract testing for interface compliance
- Table-driven unit tests
- Integration tests
- End-to-end tests
