# Agent Guidelines

## Development Practices

- **Task-based**: Use `task` for development workflows. Add new tasks if frequent command or action is used. Update documentation if new tasks are added. Assess new tasks in context of all other tasks. Consolidate if necessary.
- **Nix Environment**: Use `nix develop` for reproducible development environment
- **Kubernetes Manifest Validation**: Use `kubeconform` with strict mode and CRD schemas

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

## Quick Reference

### Key Tasks

- `task setup` - Initialize development environment
- `task ci` - Run full CI workflow (build, test, lint)
- `task lint:fix-all` - Auto-fix all formatting issues
- `task release:preview-changelog` - Preview next release

**See [DEVELOPMENT.md](DEVELOPMENT.md) for complete task reference and workflows.**

## Testing Strategy

- **Unit tests** - Test individual components with mocks
- **Integration tests** - Test controller logic (envtest) + operator workflow (Docker Talos + mocked providers)
- **E2E tests** - Test against real cloud providers (cost involved, requires credentials)

**See [DEVELOPMENT.md](DEVELOPMENT.md) for development setup and [README.md](README.md) for installation instructions.**
