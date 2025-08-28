# TGP Operator Makefile
.DEFAULT_GOAL := help

# Variables
GO_VERSION = 1.25
DOCKER_REGISTRY = ghcr.io
DOCKER_IMAGE = solanyn/tgp-operator
KUBEBUILDER_ASSETS ?= $(shell setup-envtest use 1.28.0 -p path 2>/dev/null || echo "")

# Build info
GIT_COMMIT = $(shell git rev-parse HEAD)
GIT_TAG = $(shell git describe --tags --exact-match 2>/dev/null || echo "dev")
BUILD_DATE = $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS = -X main.version=$(GIT_TAG) -X main.commit=$(GIT_COMMIT) -X main.date=$(BUILD_DATE)

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: deps
deps: ## Download dependencies
	go mod download
	go mod tidy

.PHONY: generate
generate: deps ## Generate code (CRDs, deepcopy, etc.)
	go run sigs.k8s.io/controller-tools/cmd/controller-gen object paths="./pkg/api/..."
	go run sigs.k8s.io/controller-tools/cmd/controller-gen crd paths="./pkg/api/..." output:crd:artifacts:config=config/crd/bases

.PHONY: fmt
fmt: ## Format Go code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Run linter
	golangci-lint run

.PHONY: lint-fix
lint-fix: ## Run linter with autofix
	golangci-lint run --fix

##@ Testing

.PHONY: test
test: generate fmt vet ## Run tests
	go test ./pkg/... -v

.PHONY: test-unit
test-unit: ## Run unit tests only
	go test ./pkg/... -v

.PHONY: test-integration
test-integration: ## Run integration tests
	@if [ -n "$(KUBEBUILDER_ASSETS)" ]; then \
		KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" go test ./test/integration/... -v; \
	else \
		echo "Kubebuilder assets not found. Run 'setup-envtest use 1.28.0' first."; \
		exit 1; \
	fi

.PHONY: coverage
coverage: ## Generate test coverage report
	go test ./pkg/... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

##@ Build

.PHONY: build
build: generate ## Build manager binary
	go build -ldflags "$(LDFLAGS)" -o bin/manager ./cmd/manager

.PHONY: run
run: generate fmt vet ## Run locally
	go run ./cmd/manager

##@ Docker

.PHONY: docker-build
docker-build: ## Build docker image
	docker build -t $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(GIT_TAG) .
	docker build -t $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):latest .

.PHONY: docker-push
docker-push: ## Push docker image
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(GIT_TAG)
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):latest

##@ Helm

.PHONY: chart-crd
chart-crd: generate ## Sync generated CRDs to Helm chart
	@echo "---" > chart/templates/crd.yaml
	@cat config/crd/bases/tgp.io_gpunodeclasses.yaml >> chart/templates/crd.yaml
	@echo "---" >> chart/templates/crd.yaml
	@cat config/crd/bases/tgp.io_gpunodepools.yaml >> chart/templates/crd.yaml

.PHONY: chart-lint
chart-lint: ## Lint Helm chart
	helm lint chart/

.PHONY: chart-package
chart-package: chart-crd ## Package Helm chart
	helm package chart/

##@ Security

.PHONY: security
security: ## Run security scans
	gosec ./...
	trivy fs .

##@ Utilities

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f *.tgz
	go clean -cache

.PHONY: check
check: fmt vet lint test ## Run all checks (format, vet, lint, test)

.PHONY: dev
dev: generate fmt vet test build ## Full development cycle

##@ CI/CD

.PHONY: ci-test
ci-test: deps generate fmt vet test ## CI test pipeline

.PHONY: ci-build
ci-build: deps generate build ## CI build pipeline

.PHONY: ci-docker
ci-docker: docker-build ## CI docker build pipeline