# Testing TGP Operator

This document provides instructions for testing the TGP Operator, particularly in the home-ops cluster environment.

## Prerequisites

- Access to the home-ops Talos cluster
- `kubectl` configured with cluster access
- `helm` installed
- Docker installed (for building images)

## Testing in Home-Ops Cluster

### 1. Deploy Latest Changes

To deploy the latest operator code to the home-ops cluster:

```bash
# Build and deploy the operator with latest changes
KUBECONFIG=../home-ops/kubeconfig task deploy:talos

# This will:
# - Build the latest Docker image with your changes
# - Deploy using Helm to the tgp-system namespace
# - Use the centralized configuration system
```

### 2. Verify Deployment

```bash
# Check operator pod status
KUBECONFIG=../home-ops/kubeconfig kubectl get pods -n tgp-system

# Check operator logs
KUBECONFIG=../home-ops/kubeconfig kubectl logs -n tgp-system deployment/tgp-operator-controller-manager --tail=20

# Verify ConfigMap with centralized config
KUBECONFIG=../home-ops/kubeconfig kubectl get configmap tgp-operator-config -n tgp-system -o yaml
```

### 3. Test Simplified GPURequests

The operator now supports simplified GPURequests without requiring TalosConfig:

```yaml
# test-simple-gpu.yaml
apiVersion: tgp.io/v1
kind: GPURequest
metadata:
  name: test-simple-gpu
spec:
  provider: runpod
  gpuType: RTX3090
  region: US
  maxHourlyPrice: "0.50"
  spot: true
  # No TalosConfig needed - operator uses centralized defaults
```

Apply and test:

```bash
# Apply the test GPURequest
KUBECONFIG=../home-ops/kubeconfig kubectl apply -f test-simple-gpu.yaml

# Check status
KUBECONFIG=../home-ops/kubeconfig kubectl get gpurequests

# View detailed status
KUBECONFIG=../home-ops/kubeconfig kubectl get gpurequest test-simple-gpu -o yaml

# Clean up
KUBECONFIG=../home-ops/kubeconfig kubectl delete gpurequest test-simple-gpu
```

### 4. Test Provider Credentials

Verify that provider credentials are working by testing API connectivity:

```bash
# Get the API key from the cluster secret
RUNPOD_API_KEY=$(KUBECONFIG=../home-ops/kubeconfig kubectl get secret tgp-operator-secret -n tgp-system -o jsonpath='{.data.RUNPOD_API_KEY}' | base64 -d)

# Test API connectivity locally
RUNPOD_API_KEY="$RUNPOD_API_KEY" go run ./cmd/test-providers --provider=runpod

# This should show available GPU offers if credentials are valid
```

### 5. Verify Centralized Configuration

Check that the operator is using centralized configuration:

```bash
# View the generated configuration
KUBECONFIG=../home-ops/kubeconfig kubectl get configmap tgp-operator-config -n tgp-system -o jsonpath='{.data.config\.yaml}'

# Check that secrets are properly referenced
KUBECONFIG=../home-ops/kubeconfig kubectl get secret tgp-operator-secret -n tgp-system -o yaml
```

## Configuration Testing

### Default Configuration

The operator uses these defaults when TalosConfig is not provided:

- **Talos Image**: `ghcr.io/siderolabs/talos:v1.10.5`
- **Tailscale Tags**: `["tag:k8s", "tag:gpu"]`
- **Tailscale Settings**: ephemeral=true, acceptRoutes=true
- **Provider Credentials**: Retrieved from `tgp-operator-secret`

### Custom Configuration

You can customize the operator configuration via Helm values:

```yaml
# custom-values.yaml
config:
  providers:
    runpod:
      enabled: true
      secretName: my-custom-secret
      apiKeySecretKey: MY_RUNPOD_KEY
  talos:
    image: "ghcr.io/siderolabs/talos:v1.11.0"
  tailscale:
    tags:
      - "tag:custom"
      - "tag:test"
```

Deploy with custom values:

```bash
KUBECONFIG=../home-ops/kubeconfig helm upgrade tgp-operator ./chart -n tgp-system --values custom-values.yaml
```

## Troubleshooting

### Common Issues

1. **401 Authentication Errors**: 
   - Verify API keys are valid and not expired
   - Check that secrets exist and have correct keys
   - Test API connectivity with `cmd/test-providers`

2. **Config Not Loading**:
   - Check ConfigMap exists: `kubectl get configmap tgp-operator-config -n tgp-system`
   - Verify volume mount: `kubectl describe pod -n tgp-system -l app.kubernetes.io/name=tgp-operator`
   - Check operator logs for "Loaded operator configuration" messages

3. **CRD Issues**:
   - Ensure CRD is updated: `kubectl get crd gpurequests.tgp.io -o yaml | grep required`
   - TalosConfig should not be in required fields for simplified GPURequests

### Debug Commands

```bash
# Check operator startup logs
KUBECONFIG=../home-ops/kubeconfig kubectl logs -n tgp-system deployment/tgp-operator-controller-manager --since=5m

# Restart operator to see fresh logs
KUBECONFIG=../home-ops/kubeconfig kubectl rollout restart deployment/tgp-operator-controller-manager -n tgp-system

# Check all resources in tgp-system namespace
KUBECONFIG=../home-ops/kubeconfig kubectl get all,configmap,secret -n tgp-system

# Verify Helm deployment status
KUBECONFIG=../home-ops/kubeconfig helm list -n tgp-system
KUBECONFIG=../home-ops/kubeconfig helm get values tgp-operator -n tgp-system
```

## Testing Scenarios

### 1. Basic Functionality Test

```bash
# Create a simple GPURequest
cat <<EOF | KUBECONFIG=../home-ops/kubeconfig kubectl apply -f -
apiVersion: tgp.io/v1
kind: GPURequest
metadata:
  name: basic-test
spec:
  provider: runpod
  gpuType: RTX3090
EOF

# Should be accepted and use operator defaults
```

### 2. Complete Configuration Test

```bash
# Create a GPURequest with full configuration
cat <<EOF | KUBECONFIG=../home-ops/kubeconfig kubectl apply -f -
apiVersion: tgp.io/v1
kind: GPURequest
metadata:
  name: full-test
spec:
  provider: runpod
  gpuType: RTX3090
  region: US
  maxHourlyPrice: "1.00"
  spot: true
  talosConfig:
    image: "ghcr.io/siderolabs/talos:v1.10.5"
    tailscaleConfig:
      hostname: "custom-gpu-node"
      tags: ["tag:test"]
      ephemeral: true
EOF

# Should use user-provided configuration
```

### 3. Credential Validation Test

```bash
# Test credential validation during startup
KUBECONFIG=../home-ops/kubeconfig kubectl rollout restart deployment/tgp-operator-controller-manager -n tgp-system

# Watch logs for credential validation
KUBECONFIG=../home-ops/kubeconfig kubectl logs -n tgp-system deployment/tgp-operator-controller-manager -f
```

## Expected Behavior

### Successful Deployment

- Operator pod running in `tgp-system` namespace
- ConfigMap `tgp-operator-config` contains YAML configuration
- Secret `tgp-operator-secret` contains provider API keys
- GPURequests can be created without TalosConfig
- Operator logs show successful configuration loading

### Centralized Configuration Features

- ✅ Optional TalosConfig in GPURequest CRD
- ✅ Operator-level default configuration via ConfigMap
- ✅ Provider credentials from centralized secret
- ✅ Backwards compatibility with environment variables
- ✅ Helm chart drives configuration generation
- ✅ Credential validation on startup (if implemented)

## Development Workflow

1. Make changes to operator code
2. Test locally with `task test:unit`
3. Deploy to cluster with `KUBECONFIG=../home-ops/kubeconfig task deploy:talos`
4. Test with simplified GPURequests
5. Verify logs and behavior
6. Clean up test resources