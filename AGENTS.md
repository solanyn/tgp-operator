# Agent Guidelines

Use British English spelling and grammar except in code. Code should use American English spelling.

## Development Practices

- **TDD**: Write tests first, then implement functionality
- **Build & Test**: Run formatter, linter and tests after each change
- **Add Tests**: Create tests when adding new functions
- **No Comments**: Don't add unnecessary comments or emojis
- **Edit First**: Always prefer editing existing files over creating new ones
- **Task-based**: Use `task` for all development workflows
- **Minimal Dependencies**: Favor well-maintained Go packages or implement our own. Keep dependencies minimal
- **Nix Environment**: Use `nix develop` for reproducible development environment
- **Conventional Commits**: Enforce via git hooks for semantic versioning
- **Comprehensive Linting**: Go, YAML, Markdown, Shell, Nix, GitHub Actions
- **Kubernetes Manifest Validation**: Use `kubeconform` with strict mode and CRD schemas

## Commits

- **Use imperative tone**  
  Example: `Add provisioning flowchart`

- **Keep messages concise and specific**  
  Limit the summary line to 50 characters or fewer.

- **Avoid agent identifiers in the message**  
  Attribution is handled by Git metadata.

- **Use conventional commits**

  - `docs:` for documentation
  - `infra:` for infrastructure logic
  - `agent:` for agent behavior
  - feat: for new feature
  - fix: for bug fixes

  Example: `agent: Improve GPU selection logic`

- **Include context when needed**  
  Use a short message body to explain non-obvious changes.

  ```
  agent: Adjust GPU selection to prefer 3090

  Updated logic to prioritize 3090s based on $/hr and availability.
  ```

- **Avoid noise from minor formatting-only changes**  
  Group small edits under a general message like:  
  `docs: Reformat AGENTS.md`

## Project Structure

- `/proposals/` - Design documents and RFCs
- `/.taskfiles/` - Modular task definitions
- `/chart/` - Helm chart generation
- `/pkg/api/v1/` - Kubernetes API types
- `/pkg/controllers/` - Controller implementations
- `/pkg/providers/` - Cloud provider clients
- `/pkg/pricing/` - Pricing cache system
- `/test/e2e/` - End-to-end tests
- `/test/integration/` - Integration tests

## Why This Exists

Built for ML engineers with budget homelabs who need occasional GPU access without the cost of dedicated hardware. Enables cost-effective GPU provisioning across multiple cloud providers for intermittent workloads.

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

## Installation

### Via Helm Chart (Recommended)
```bash
helm install tgp-operator oci://ghcr.io/solanyn/charts/tgp-operator \
  --version 0.1.0 \
  --namespace tgp-system \
  --create-namespace
```

### Via Generated Manifests
```bash
task chart:template
kubectl apply -f dist/chart/tgp-operator/templates/
```

### Configure Provider Secrets
```bash
kubectl create secret generic tgp-provider-secrets \
  --from-literal=VAST_API_KEY=your-key \
  --from-literal=RUNPOD_API_KEY=your-key \
  --from-literal=LAMBDA_LABS_API_KEY=your-key \
  --from-literal=PAPERSPACE_API_KEY=your-key \
  -n tgp-system
```

## Development Setup

### Prerequisites
- [Nix](https://nixos.org/download.html) with flakes enabled
- [direnv](https://direnv.net/) (optional but recommended)

### Environment Setup
```bash
git clone https://github.com/solanyn/tgp-operator
cd tgp-operator

# Option 1: With direnv (automatic)
direnv allow

# Option 2: Manual nix shell
nix develop

# Initialize development environment
task setup
```
