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
