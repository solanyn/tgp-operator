# Development Guide

This guide covers local development setup and common workflows for the TGP Operator.

## Prerequisites

- Go 1.24+
- Docker
- kubectl
- Helm 3.x
- make

## Quick Start

```bash
# Clone the repository
git clone https://github.com/solanyn/tgp-operator.git
cd tgp-operator

# Download dependencies
make deps

# Run all checks (format, vet, lint, test)
make check

# Build the binary
make build

# Run locally against your kubeconfig
make run
```

## Development Workflow

### Available Make Targets

Run `make help` to see all available targets:

```bash
make help
```

### Common Commands

```bash
# Download dependencies and tidy go.mod
make deps

# Generate code (CRDs, deepcopy methods)
make generate

# Format code
make fmt

# Run linter
make lint

# Fix linting issues automatically
make lint-fix

# Run all tests
make test

# Run only unit tests
make test-unit

# Run integration tests (requires kubebuilder-assets)
make test-integration

# Build manager binary
make build

# Run locally
make run

# Full development cycle (generate, format, vet, test, build)
make dev

# Clean build artifacts
make clean
```

### Docker Development

```bash
# Build Docker image
make docker-build

# Push Docker image
make docker-push
```

### Helm Chart Development

```bash
# Generate CRDs for Helm chart
make chart-crd

# Lint Helm chart
make chart-lint

# Package Helm chart
make chart-package
```

## Code Generation

The project uses Go's built-in tool management with `go run` commands. All tools are defined in `tools/tools.go` and their versions are managed through `go.mod`.

### Generating CRDs and DeepCopy Methods

```bash
# This generates both CRDs and deepcopy methods
make generate
```

This runs:
- `go run sigs.k8s.io/controller-tools/cmd/controller-gen object paths="./pkg/api/..."`
- `go run sigs.k8s.io/controller-tools/cmd/controller-gen crd paths="./pkg/api/..." output:crd:artifacts:config=config/crd/bases`

## Testing

### Unit Tests

```bash
make test-unit
```

### Integration Tests

Integration tests require kubebuilder test environment:

```bash
# Install test environment (one time setup)
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
setup-envtest use 1.28.0

# Run integration tests
make test-integration
```

### Test Coverage

```bash
make coverage
```

This generates `coverage.html` which you can open in a browser.

## Linting and Formatting

The project uses:
- `go fmt` for code formatting
- `golangci-lint` for comprehensive linting
- `go vet` for static analysis

```bash
# Format code
make fmt

# Run linter
make lint

# Fix auto-fixable issues
make lint-fix
```

## Security Scanning

```bash
make security
```

This runs both `gosec` and `trivy` security scanners.

## Project Structure

```
.
├── cmd/manager/          # Main application entry point
├── pkg/
│   ├── api/v1/          # API types and CRDs
│   ├── config/          # Operator configuration
│   ├── controllers/     # Kubernetes controllers
│   ├── imagefactory/    # Talos image factory client
│   ├── pricing/         # GPU pricing cache
│   ├── providers/       # Cloud provider implementations
│   ├── validation/      # Configuration validation
│   └── webhooks/        # Admission webhooks
├── config/              # Kubernetes manifests
├── chart/               # Helm chart
├── test/integration/    # Integration tests
├── tools/               # Tool dependencies
└── Makefile            # Build automation
```

## Adding New Dependencies

### Runtime Dependencies

```bash
go get github.com/example/package@v1.2.3
go mod tidy
```

### Development Tools

1. Add the tool import to `tools/tools.go`:
   ```go
   _ "github.com/example/tool/cmd/tool"
   ```

2. Add the tool to go.mod:
   ```bash
   go get github.com/example/tool/cmd/tool@latest
   ```

3. Use the tool in Makefile with `go run`:
   ```makefile
   my-target:
       go run github.com/example/tool/cmd/tool
   ```

## CI/CD

The project uses GitHub Actions for CI/CD with the following workflows:

- **Go Tests**: Runs unit and integration tests
- **Lint**: Runs Go linting, Helm validation, and GitHub Actions linting
- **Security Scan**: Runs security scanners
- **Build and Push Images**: Builds and pushes Docker images and Helm charts
- **Release**: Creates GitHub releases

All workflows use the same `make` commands as local development.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run `make check` to ensure all checks pass
5. Commit your changes (follow conventional commit format)
6. Push to your fork
7. Create a pull request

### Commit Messages

Use conventional commit format:
- `feat:` for new features
- `fix:` for bug fixes
- `docs:` for documentation changes
- `refactor:` for code refactoring
- `test:` for adding tests
- `chore:` for maintenance tasks

## Debugging

### Running Locally

```bash
# Run against your current kubeconfig context
make run
```

### Debug with Delve

```bash
# Build with debug symbols
go build -gcflags="all=-N -l" -o bin/manager ./cmd/manager

# Run with delve
dlv exec bin/manager
```

### Logs

The operator uses structured logging with different log levels:

```bash
# Run with debug logging
make run -- --log-level=debug
```

## Architecture

The TGP Operator follows the Kubernetes controller pattern:

1. **Controllers** watch for changes to custom resources
2. **Providers** implement cloud-specific GPU provisioning logic  
3. **Image Factory** generates Talos Linux images with required extensions
4. **Validation** ensures configuration correctness
5. **Pricing Cache** optimizes cost-based instance selection

See the [Architecture Decision Records](docs/adr/) for detailed design decisions.