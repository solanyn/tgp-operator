<a name="v0.1.0"></a>

## [v0.1.0](https://github.com/solanyn/tgp-operator/compare/v0.0.1...v0.1.0)

> 2025-07-19

### Bug Fixes

- remove GitHub Pages deployment for OCI charts
- chart maintainer format for chart-testing
- container image tagging for releases
- resolve CI failures

### Code Refactoring

- refactor tests
- replace mock E2E tests with real provider testing

### Features

- optimize multi-arch builds with buildx
- automate helm chart versioning with svu
- load wireguard config from secret
- integrate real API clients for all providers

<a name="v0.0.1"></a>

## v0.0.1

> 2025-07-15

### Bug Fixes

- resolve CI pipeline issues for deployment readiness
- resolve linting issues and improve code quality
- handle concurrent status updates with retry logic
- declare prices as strings
- remove colon from echo command to fix YAML parsing
- use full path for Go tools installed via go install
- install Go tools automatically in CI
- resolve Nix flake compatibility issues
- update license copyright to 2025 solanyn

### Code Refactoring

- improve provider clients and test reliability
- align task runner with CI workflows
- redesign provider interface with rate limiting and normalization
- rename TTL to maxLifetime for clarity

### Features

- integrate real API clients for all providers
- set up API client code generation toolchain
- add comprehensive Prometheus metrics system
- add retry logic with exponential backoff
- add health monitoring and idle timeout
- restructure CI workflows for modularity
- migrate from nix to mise for lighter development setup
- update reconciliation logic for new provider interface
- add dynamic termination scheduling for GitOps
- make GPURequest cluster-scoped resource
- enhance CI/CD with multi-platform builds and resource optimization
- add proper CRD generation and clean up task preconditions
- add comprehensive documentation and test framework
- add automated changelog generation
- add comprehensive linting and formatting
- add comprehensive CI/CD pipeline
- add container build and Helm chart
- implement core controller and provider system
- add Nix development environment and task automation
- add GPURequest CRD and controller foundation
- initialize project with Go module and git configuration
