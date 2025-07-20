#!/bin/bash
set -euo pipefail

# Script to verify signed Helm chart from GHCR
# Usage: ./scripts/verify-chart-signature.sh [version]

VERSION=${1:-$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.1.0")}
CHART_REF="ghcr.io/solanyn/charts/tgp-operator:${VERSION}"

echo "Verifying chart signature for: ${CHART_REF}"

# Check if cosign is installed
if ! command -v cosign &> /dev/null; then
    echo "Error: cosign is required but not installed"
    echo "Install from: https://docs.sigstore.dev/cosign/installation/"
    exit 1
fi

# Verify the chart signature
echo "Running cosign verification..."
cosign verify "${CHART_REF}" \
    --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
    --certificate-identity "https://github.com/solanyn/tgp-operator/.github/workflows/helm-chart.yml@refs/heads/main"

echo "✅ Chart signature verification successful!"