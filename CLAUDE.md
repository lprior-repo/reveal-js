# Functional Programming & Testing Principles

## Core Functional Programming Principles

### 1. Pure Functions Only
- **No methods allowed**: All functions must be standalone pure functions
- **No side effects**: Functions should not modify external state
- **Deterministic**: Same input always produces same output
- **Immutable data**: Use immutable data structures where possible

### 2. Function Composition
- Use function composition over object-oriented inheritance
- Create higher-order functions that accept and return functions
- Implement dependency injection through function parameters

### 3. Eliminate Imperative Loops
- Replace `for` loops with functional patterns:
  - Use `map` for transformations
  - Use `filter` for conditional selection
  - Use `reduce`/`fold` for aggregation
- Avoid mutating loop counters and accumulators

### 4. Structured Logging
- Use structured JSON logging throughout the codebase
- Create functional logging patterns with LogFunc types
- Avoid method-based loggers in favor of function-based logging

## Testing Methodologies

### 1. Test-Driven Development (TDD)
- Write tests before implementation
- Follow Red-Green-Refactor cycle
- Ensure comprehensive test coverage for all functions

### 2. Acceptance Testing (Given-When-Then)
- Structure acceptance tests using Given-When-Then format
- Test complete user workflows end-to-end
- Focus on business requirements and user stories

### 3. Integration Testing
- Test component interactions
- Verify worker pool integration with ants library
- Test concurrency scenarios and resource management

### 4. Unit Testing
- Test individual pure functions in isolation
- Verify function behavior with various inputs
- Test error handling and edge cases

### 5. Property-Based Testing
- Use `pgregory.net/rapid` for property-based testing
- Test function invariants and properties
- Generate random inputs to discover edge cases

### 6. Mutation Testing
- Test code behavior under modified conditions
- Verify error handling and recovery scenarios
- Test system resilience and fault tolerance

## Concurrency Management

### 1. Ants Worker Pool Integration
- Use `github.com/panjf2000/ants/v2` for goroutine management
- Implement functional interfaces for worker pool operations
- Handle context cancellation and resource cleanup properly

### 2. Context Propagation
- Pass context through all function calls
- Implement proper cancellation handling
- Use context for timeout and deadline management

## Code Structure Requirements

### 1. Functional Architecture
```go
// Functional approach - correct
type ConfigLoader func(ctx context.Context, configPath string) (*Config, error)
type OrgCloner func(ctx context.Context, org, targetDir, token string, concurrency int) error

// Object-oriented approach - avoid
type ProcessingShell struct { ... }
func (s *ProcessingShell) ExecuteWorkflow(...) error
```

### 2. Error Handling
- Use functional error handling patterns
- Return errors as values, not exceptions
- Implement proper error propagation through function chains

### 3. Dependency Injection
- Inject dependencies as function parameters
- Use factory functions to create configured functions
- Avoid global state and singletons

## Testing File Organization

### 1. Test File Structure
- `e2e_test.go`: End-to-end workflow testing
- `fp_test.go`: Functional programming and unit tests
- `coverage_improvement_test.go`: Edge cases and coverage improvement

### 2. Test Categories
- **E2E Tests**: Complete workflow testing with real environment
- **Integration Tests**: Component interaction testing
- **Unit Tests**: Individual function testing
- **Property Tests**: Invariant and property verification
- **Stress Tests**: Concurrency and performance testing

## Build and Verification Commands

### 1. Build Commands
```bash
go build -o ghorg-wrapper
```

### 2. Test Commands
```bash
go test -v ./...
go test -race ./...
go test -cover ./...
```

### 3. Linting and Formatting
```bash
go fmt ./...
go vet ./...
```

## Key Libraries and Dependencies

- **Concurrency**: `github.com/panjf2000/ants/v2`
- **Testing**: `github.com/stretchr/testify`
- **Property Testing**: `pgregory.net/rapid`
- **Functional Programming**: Custom implementations following functional principles
- **Logging**: Go's standard `log/slog` package

## Implementation Notes

- All functions must be pure and free of side effects
- Use functional composition over inheritance
- Implement comprehensive logging for operational visibility
- Follow TDD principles with Given-When-Then acceptance tests
- Use ants worker pools for all concurrent operations
- Maintain immutability and avoid state mutation