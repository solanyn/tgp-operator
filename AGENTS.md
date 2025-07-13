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
- **YAML Validation**: Use `yq` for YAML syntax validation and structure checks
- **YAML Formatting**: Run `yamlfmt` after changing YAML files
- **Kubernetes Manifest Validation**: Use `kubeconform` to validate Kubernetes manifests

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

- `task setup` - Complete development environment setup
- `task dev:build` - Build manager binary
- `task dev:lint` - Run linters
- `task test:unit` - Run unit tests
- `task test:integration` - Run all integration tests (envtest + Talos Docker)
- `task test:e2e` - Run true e2e tests against real cloud providers
- `task test:all` - Run all safe tests (unit + integration)
- `task test:provision-talos-cloud` - Provision cloud-based Talos cluster
- `task docker:build` - Build rootless container image
- `task docker:push` - Push container to registry
- `task chart:template` - Generate Kubernetes manifests
- `task chart:validate` - Validate CUE and YAML files
- `task deploy` - Build container and generate chart
- `task ci` - Full CI workflow

## Testing Strategy

- **Unit tests** - Test individual components with mocks
- **Integration tests** - Test controller logic (envtest) + operator workflow (Docker Talos + mocked providers)
- **E2E tests** - Test against real cloud providers (cost involved, requires credentials)

## Installation

Deploy via CUE-generated Kubernetes manifests:

```bash
task chart:generate
kubectl apply -f dist/chart/
```

Create provider secrets:

```bash
kubectl create secret generic tgp-provider-secrets \
  --from-literal=VAST_API_KEY=your-key \
  --from-literal=RUNPOD_API_KEY=your-key \
  --from-literal=LAMBDA_LABS_API_KEY=your-key \
  --from-literal=PAPERSPACE_API_KEY=your-key \
  -n tgp-system
```
