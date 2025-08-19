# Contributing to Bucket Synchronisation Service

Thank you for your interest in contributing to the bucket synchronisation service! This document provides guidelines and information for contributors.

## Table of Contents
- [Development Setup](#development-setup)
- [Code Standards](#code-standards)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Issue Reporting](#issue-reporting)
- [Security](#security)

## Development Setup

### Prerequisites
- Go 1.25 or later
- Git
- Make
- Docker (optional, for containerized development)

### Getting Started
1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/your-username/bucketsyncd.git
   cd bucketsyncd
   ```

3. Install development dependencies:
   ```bash
   # Install pre-commit hooks
   pip install pre-commit
   pre-commit install

   # Install Go tools
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
   ```

4. Run the test suite to ensure everything works:
   ```bash
   make test
   ```

### Development Workflow
1. Create a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make your changes following the [code standards](#code-standards)

3. Run tests and linting:
   ```bash
   make test
   make lint
   ```

4. Commit your changes with a descriptive commit message

5. Push to your fork and create a pull request

## Code Standards

### Go Code Style
- Follow standard Go formatting (`gofmt`)
- Use `golangci-lint` for linting
- Maintain test coverage above 60%
- Write clear, documented code
- Use meaningful variable and function names

### Commit Messages
Follow the conventional commit format:
```
type(scope): description

- feat: new features
- fix: bug fixes
- docs: documentation changes
- style: formatting changes
- refactor: code restructuring
- test: adding or updating tests
- chore: maintenance tasks
```

Example:
```
feat(inbound): add support for SQS message processing
fix(config): handle missing configuration files gracefully
docs(readme): update installation instructions
```

### Code Organization
- Keep functions small and focused
- Group related functionality in appropriate files
- Use interfaces for external dependencies
- Handle errors explicitly
- Log appropriately (use structured logging)

## Testing

### Test Requirements
- Unit tests for all public functions
- Integration tests for complex workflows
- Minimum 60% code coverage
- Tests should be deterministic and isolated

### Running Tests
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests with race detection
go test -race ./...

# Run linting
golangci-lint run

# Run security scanning
gosec ./...
```

### Test Structure
```go
func TestFunctionName(t *testing.T) {
    // Arrange
    input := "test input"
    expected := "expected output"
    
    // Act
    result := FunctionName(input)
    
    // Assert
    if result != expected {
        t.Errorf("Expected %s, got %s", expected, result)
    }
}
```

## Building

### Local Build
```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Build specific platform
GOOS=linux GOARCH=amd64 go build -o build/bucketsyncd
```

### Docker Build
```bash
# Build Docker image
docker build -t bucketsyncd .

# Multi-arch build
docker buildx build --platform linux/amd64,linux/arm64 -t bucketsyncd .
```

## Submitting Changes

### Pull Request Process
1. Ensure your code follows all standards and passes all tests
2. Update documentation if necessary
3. Add or update tests for new functionality
4. Fill out the pull request template completely
5. Request review from maintainers

### Pull Request Requirements
- [ ] Tests pass locally and in CI
- [ ] Code coverage is maintained or improved
- [ ] Documentation is updated if needed
- [ ] Commit messages follow conventional format
- [ ] No merge conflicts with master branch
- [ ] Security scan passes (no new vulnerabilities)

### Review Process
1. Automated checks must pass (CI/CD pipeline)
2. At least one maintainer review required
3. Address all review feedback
4. Maintain clean commit history (squash if necessary)

## Issue Reporting

### Bug Reports
Include the following information:
- Go version and OS
- bucketsyncd version
- Configuration details (sanitized)
- Steps to reproduce
- Expected vs actual behavior
- Relevant log output

### Feature Requests
- Clear description of the proposed feature
- Use case and business justification
- Proposed implementation approach (if applicable)
- Backward compatibility considerations

### Security Issues
**Do not open public issues for security vulnerabilities.**
Report security issues privately to the maintainers.

## Security

### Security Guidelines
- Never commit secrets, tokens, or passwords
- Use environment variables for sensitive configuration
- Validate all input data
- Follow secure coding practices
- Keep dependencies updated

### Dependency Management
- Regularly update Go modules
- Review security advisories
- Use `go mod tidy` to clean up dependencies
- Pin versions for reproducible builds

## Development Tools

### Useful Make Targets
```bash
make build          # Build binary
make test           # Run tests
make lint           # Run linter
make fmt            # Format code
make clean          # Clean build artifacts
make docker         # Build Docker image
make install-tools  # Install development tools
```

### IDE Setup
Recommended VS Code extensions:
- Go (by Google)
- golangci-lint
- Test Explorer for Go
- Git Lens

### Debugging
Use delve for debugging:
```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug with delve
dlv debug -- -c config.yaml
```

## Resources

- [Go Documentation](https://golang.org/doc/)
- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Project Issues](https://github.com/rossigee/bucketsyncd/issues)

## License

By contributing to this project, you agree that your contributions will be licensed under the MIT License.

## Getting Help

- Open an issue for bugs or feature requests
- Check existing issues and documentation first
- Be respectful and constructive in all interactions

Thank you for contributing!