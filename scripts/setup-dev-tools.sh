#!/bin/bash
set -euo pipefail

echo "Setting up TGP Operator development tools..."

# Go development tools
echo "Installing Go development tools..."
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install mvdan.cc/gofumpt@latest
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

# GitHub CLI (if not already installed)
if ! command -v gh &> /dev/null; then
    echo "Please install GitHub CLI: https://cli.github.com/"
    echo "Or via your package manager (brew install gh, apt install gh, etc.)"
fi

# Additional tools that should be installed via package manager
echo ""
echo "Additional tools to install via your package manager:"
echo "  - docker"
echo "  - jq"
echo "  - yq"
echo "  - kubeconform"
echo "  - yamllint"
echo "  - markdownlint-cli"
echo "  - shellcheck"
echo ""

# Setup envtest
echo "Setting up test environment..."
$(go env GOPATH)/bin/setup-envtest use 1.28.0

echo "Development tools setup complete!"
echo ""
echo "Next steps:"
echo "  1. Install mise: https://mise.jdx.dev/getting-started.html"
echo "  2. Run: mise install"
echo "  3. Run: task setup"
echo "  4. Start coding!"