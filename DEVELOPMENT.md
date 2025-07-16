# Development Guide

## Development Setup

### Prerequisites

- [mise](https://mise.jdx.dev/) for tool version management
- Docker for container builds
- System packages: jq, yq, kubeconform, yamllint, markdownlint-cli, shellcheck, gh

### Environment Setup

```bash
git clone https://github.com/solanyn/tgp-operator
cd tgp-operator

# Install tool versions (go, task, kubectl, helm)
mise install

# Setup development tools (golangci-lint, controller-gen, etc.)
./scripts/setup-dev-tools.sh

# Initialize development environment
task setup
```

### Installation Guide

1. **Install mise**:

   ```bash
   # macOS
   brew install mise

   # Linux
   curl https://mise.jdx.dev/install.sh | sh
   ```

2. **Install system packages**:

   ```bash
   # macOS
   brew install jq yq kubeconform yamllint markdownlint-cli shellcheck gh docker

   # Ubuntu/Debian
   apt install jq docker.io shellcheck
   npm install -g markdownlint-cli

   # Arch Linux
   pacman -S jq docker shellcheck
   ```

3. **Setup environment**:

   ```bash
   mise install
   ./scripts/setup-dev-tools.sh
   task setup
   ```

## Tool Management with mise

This project uses [mise](https://mise.jdx.dev/) for consistent tool version management across development environments and CI/CD.

### Core Tools (via mise)

- **Go** - Programming language and toolchain
- **Task** - Task runner (replaces Make)
- **kubectl** - Kubernetes CLI
- **helm** - Kubernetes package manager

### Development Tools (via Go install)

These tools are installed automatically via `./scripts/setup-dev-tools.sh` or `task setup`.

**Important**: All `go install` commands must be run within the mise environment to ensure they use the correct Go version and install to the right GOPATH.

```bash
# Always activate mise environment first
eval "$(mise env)"

# Code generation and building
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

# Linting and formatting
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install mvdan.cc/gofumpt@latest
go install golang.org/x/tools/cmd/goimports@latest

# Security scanning
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Testing tools
go install github.com/onsi/ginkgo/v2/ginkgo@latest
```

**Path Management**: The Taskfile automatically activates the mise environment, ensuring all tools use the correct versions. When running commands directly, always use `mise exec` or activate the environment with `eval "$(mise env)"`.

### System Tools (via package manager)

Install these via your system package manager:

```bash
# macOS (Homebrew)
brew install yamllint yamlfmt markdownlint-cli shellcheck actionlint

# Ubuntu/Debian
apt install yamllint shellcheck
npm install -g markdownlint-cli

# Arch Linux
pacman -S yamllint shellcheck
```

### Tool Usage Patterns

#### Local Development

```bash
# Run tools directly (mise manages PATH)
golangci-lint run ./...
gosec ./...
yamllint .

# Run via task (recommended)
task lint:go
task security:gosec
task lint:yaml
```

#### CI/CD Integration

GitHub Actions uses the same tools but installs them differently:

- **Go tools**: Installed via `go install` in workflow steps
- **System tools**: Installed via GitHub Actions marketplace actions
- **mise tools**: Not used in CI (GitHub runners use different approach)

### Tool Configuration Files

| Tool          | Configuration File | Purpose                       |
| ------------- | ------------------ | ----------------------------- |
| golangci-lint | `.golangci.yml`    | Go linting rules and settings |
| yamllint      | `.yamllint.yml`    | YAML linting rules            |
| gosec         | None               | Uses default security rules   |
| gofumpt       | None               | Stricter version of gofmt     |
| goimports     | None               | Auto-manages Go imports       |

### Troubleshooting Tools

**Tool not found in PATH:**

```bash
# Reload mise environment
mise install
eval "$(mise env)"

# Check tool location and versions
mise where go
mise where task
which golangci-lint
which setup-envtest

# Manually install missing Go tools (within mise environment)
eval "$(mise env)" && ./scripts/setup-dev-tools.sh
```

**Environment variable issues (KUBEBUILDER_ASSETS, etc.):**

```bash
# Check if setup-envtest is properly installed
which setup-envtest
setup-envtest list

# Manually set envtest environment
export KUBEBUILDER_ASSETS="$(setup-envtest use 1.28.0 -p path)"
echo $KUBEBUILDER_ASSETS

# Run task with proper environment
eval "$(mise env)" && task test:integration
```

**Version mismatches:**

```bash
# Check current versions
mise current
go version
golangci-lint version

# Update all tools
mise upgrade

# Reinstall Go tools with correct version
eval "$(mise env)" && ./scripts/setup-dev-tools.sh
```

**Task execution issues:**

```bash
# Always run tasks through mise environment
mise exec -- task build
mise exec -- task test

# Or activate environment first
eval "$(mise env)"
task build
task test
```

## Task Commands

The task runner is aligned with our CI/CD workflows for consistency between local development and GitHub Actions.

### Quick Commands

- `task help` - Show common development workflows
- `task build` - Build the operator binary
- `task test` - Run all tests (unit + integration)
- `task lint` - Run all linters
- `task lint:fix` - Auto-fix all formatting issues
- `task check` - Quick pre-commit check (lint + unit tests)
- `task fix` - Quick fix for common issues

### CI Simulation

- `task ci:local` - Run full CI suite locally (matches GitHub Actions)
- `task ci:pr` - Run PR checks before pushing

### Development

- `task setup` - Complete development environment setup
- `task dev:build` - Build manager binary
- `task dev:generate` - Generate Go code (deepcopy methods)
- `task dev:clean` - Clean build artifacts

### Testing

- `task test:unit` - Run unit tests
- `task test:integration` - Run all integration tests (envtest + Talos Docker)
- `task test:validate-providers` - Validate cloud provider credentials (no instance launches)
- `task test:e2e` - Run true e2e tests against real cloud providers
- `task test:all` - Run all safe tests (unit + integration)

### Linting & Formatting

- `task lint:all` - Run all linting checks
- `task lint:fix-all` - Auto-fix all formatting issues
- `task lint:go` - Go-specific linting and formatting
- `task lint:yaml` - YAML linting and formatting
- `task lint:markdown` - Markdown linting

### Security

- `task security` - Run all security scans
- `task security:gosec` - Go security scanner (code vulnerabilities)
- `task security:trivy-code` - Scan code and dependencies for vulnerabilities
- `task security:trivy-container` - Scan container for vulnerabilities

### Container & Deployment

- `task docker:build` - Build rootless container image
- `task docker:push` - Push container to registry
- `task deploy:local` - Deploy to local Talos cluster
- `task deploy:talos` - Deploy to existing Talos cluster
- `task chart:template` - Generate Helm templates
- `task chart:validate` - Validate Helm chart and manifests
- `task chart:push-oci` - Push chart as OCI artifact to GHCR

### Release

- `task release:next-version` - Show next semantic version
- `task release:preview-changelog` - Preview changelog for next release
- `task release:release` - Create full automated release
- `task release:auto-release` - Trigger GitHub Actions release workflow

## Observability

The operator exposes Prometheus metrics on `:8080/metrics` for production monitoring:

### Key Metrics

- `tgp_operator_gpu_requests_total` - GPU request counts by provider/type/phase
- `tgp_operator_instance_launch_duration_seconds` - Instance launch times
- `tgp_operator_instances_active` - Current active instances
- `tgp_operator_instance_hourly_cost_usd` - Cost tracking per instance
- `tgp_operator_provider_requests_total` - Provider API success/error rates
- `tgp_operator_health_checks_total` - Instance health monitoring
- `tgp_operator_idle_timeouts_total` - Cost optimization effectiveness

### Accessing Metrics

```bash
# Port forward to local development
kubectl port-forward -n tgp-system deployment/tgp-operator 8080:8080

# View metrics
curl http://localhost:8080/metrics | grep tgp_operator
```

## Provider Validation

Validate cloud provider credentials before deployment:

```bash
# Export API keys
export VAST_API_KEY=your_vast_key
export RUNPOD_API_KEY=your_runpod_key
export LAMBDA_LABS_API_KEY=your_lambda_key
export PAPERSPACE_API_KEY=your_paperspace_key

# Validate connectivity (no instance launches)
task test:validate-providers
```

## Testing Strategy

- **Unit tests** - Test individual components with mocks
- **Integration tests** - Test controller logic (envtest) + operator workflow (Docker Talos + mocked providers)
- **Provider validation** - Test real API connectivity without launching instances
- **E2E tests** - Test against real cloud providers (cost involved, requires credentials)

## Releases

### Conventional Commit Types

- `feat:` → minor version bump (0.1.0 → 0.2.0)
- `fix:` → patch version bump (0.1.0 → 0.1.1)
- `feat!:` or `BREAKING CHANGE:` → major version bump (0.1.0 → 1.0.0)

### Release Methods

**Method 1: Automated GitHub Release (Recommended)**

```bash
# Check what would be released
task release:preview-changelog

# Trigger automated release via GitHub Actions
task release:auto-release

# Monitor progress
gh run list -w release.yml
```

**Method 2: Manual GitHub Release**

- Go to GitHub → Actions → Release workflow
- Click "Run workflow"
- Choose version type: auto/patch/minor/major
- Release is built and published automatically

**Method 3: Local Release**

```bash
# Full local release (requires Docker login)
task release:release
```

### What Gets Released

- ✅ **Container image** → `ghcr.io/solanyn/tgp-operator:X.Y.Z` and `ghcr.io/solanyn/tgp-operator:vX.Y.Z`
- ✅ **Helm chart** → `oci://ghcr.io/solanyn/charts/tgp-operator:X.Y.Z`
- ✅ **GitHub release** with auto-generated changelog
- ✅ **Git tag** with semantic version

### Checking Release Status

```bash
task release:next-version       # Show next version number
git log --oneline $(git describe --tags --abbrev=0)..HEAD  # Commits since last release
```

## Architecture

The operator consists of:

- **Controller** - Reconciles GPURequest custom resources
- **Provider clients** - Interface with cloud provider APIs
- **Pricing cache** - Tracks real-time pricing for cost optimization
- **Node lifecycle** - Manages instance provisioning and Kubernetes integration

## Contributing

1. Follow conventional commits format
2. Run `task ci` before submitting PRs
3. Add tests for new functionality
4. Update documentation as needed

## CI/CD Workflows

Our CI/CD pipeline is structured into modular workflows that mirror local development tasks.

### Workflow Structure

1. **lint.yml** - All code quality checks

   - Go linting (golangci-lint)
   - Go formatting (gofumpt, goimports)
   - YAML linting
   - Markdown linting
   - Helm chart linting
   - GitHub Actions linting

2. **test-go.yml** - Go-specific testing

   - Unit tests with coverage
   - Integration tests with envtest

3. **test-e2e.yml** - End-to-end testing

   - Mock provider tests with Docker-based Talos
   - Real provider tests (main branch only, requires secrets)

4. **security-scan.yml** - Security analysis

   - Go security scanning (gosec)
   - Code and dependency vulnerability scanning (Trivy)
   - Container vulnerability scanning (Trivy)

5. **build-and-push-images.yml** - Container management

   - Multi-platform builds (amd64, arm64)
   - Push to GitHub Container Registry

6. **ci.yml** - PR orchestrator

   - Runs all checks in parallel where possible
   - Ensures all tests pass before merge

7. **main.yml** - Main branch workflow
   - Runs full test suite
   - Builds and pushes images
   - Checks for pending releases

### Local/CI Parity

Tasks are designed to match CI workflows:

- `task lint` = lint.yml
- `task test` = test-go.yml
- `task test:e2e` = test-e2e.yml
- `task security` = security-scan.yml
- `task docker:build` = build-and-push-images.yml
- `task ci:local` = ci.yml behavior

### Dependency Management

- Renovate automatically updates dependencies
- Auto-merges minor/patch updates after CI passes
- Requires manual approval for major updates and Kubernetes packages

