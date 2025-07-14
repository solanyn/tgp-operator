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

## Task Commands

### Development
- `task setup` - Complete development environment setup (git hooks + test env)
- `task dev:build` - Build manager binary
- `task dev:generate` - Generate Go code (deepcopy methods)
- `task dev:clean` - Clean build artifacts

### Testing
- `task test:unit` - Run unit tests
- `task test:integration` - Run all integration tests (envtest + Talos Docker)
- `task test:e2e` - Run true e2e tests against real cloud providers
- `task test:all` - Run all safe tests (unit + integration)

### Linting & Formatting
- `task lint:all` - Run all linting checks
- `task lint:fix-all` - Auto-fix all formatting issues
- `task lint:go` - Go-specific linting and formatting
- `task lint:yaml` - YAML linting and formatting
- `task lint:markdown` - Markdown linting

### Container & Deployment
- `task docker:build` - Build rootless container image
- `task docker:push` - Push container to registry
- `task docker:push-release` - Push with semantic version tags
- `task chart:template` - Generate Helm templates
- `task chart:validate` - Validate Helm chart and manifests
- `task chart:push-oci` - Push chart as OCI artifact to GHCR

### Release
- `task release:next-version` - Show next semantic version
- `task release:preview-changelog` - Preview changelog for next release
- `task release:release` - Create full automated release
- `task release:auto-release` - Trigger GitHub Actions release workflow

### Workflows
- `task ci` - Full CI workflow (deps, build, test, lint)
- `task deploy` - Build container and generate chart

## Testing Strategy

- **Unit tests** - Test individual components with mocks
- **Integration tests** - Test controller logic (envtest) + operator workflow (Docker Talos + mocked providers)
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

### PR Checks
- Runs on all pull requests
- Full linting, testing, chart validation
- Security scanning

### Main Branch
- Runs on pushes to main
- Auto-detects if release needed
- Shows preview of what would be released

### Dependency Management
- Renovate automatically updates dependencies
- Auto-merges minor/patch updates after CI passes
- Requires manual approval for major updates and Kubernetes packages