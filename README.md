# Talos GPU Provisioner

Kubernetes operator for ephemeral GPU provisioning across multiple cloud providers.

Addresses intermittent GPU compute needs by provisioning instances on-demand from cloud providers and automatically integrating them into existing Talos Kubernetes clusters. Designed for workloads that require GPU resources occasionally rather than continuously.

## Features

- **Multi-cloud support** - RunPod, Lambda Labs, Paperspace with real API integration
- **Cost optimization** - Automatic provider selection based on real-time pricing
- **Secure credentials** - 1Password CLI integration for secret management
- **Lifecycle management** - Automated provisioning, configuration, and cleanup
- **Pay-per-use model** - Resources exist only when actively needed
- **Production monitoring** - Prometheus metrics for cost tracking and operational visibility
- **Provider validation** - Real API credential verification and connectivity testing
- **Interactive testing** - CLI tool for testing provider APIs and pricing

## Quick Start

### Prerequisites

- Kubernetes cluster with cluster-admin access
- Cloud provider API credentials (one or more supported providers)
- WireGuard configuration for secure networking

### Installation

#### Option 1: Helm Chart Repository (Recommended)
```bash
# Add the repository
helm repo add tgp-operator https://solanyn.github.io/tgp-operator/
helm repo update

# Install the operator
helm install tgp-operator tgp-operator/tgp-operator \
  --namespace tgp-system \
  --create-namespace
```

#### Option 2: OCI Registry
```bash
# Install directly from OCI registry
helm install tgp-operator oci://ghcr.io/solanyn/charts/tgp-operator \
  --version 0.0.1 \
  --namespace tgp-system \
  --create-namespace
```

### Configuration

Create provider credentials:

```bash
kubectl create secret generic tgp-secret \
  --from-literal=RUNPOD_API_KEY=your-runpod-key \
  --from-literal=LAMBDA_LABS_API_KEY=your-lambda-key \
  --from-literal=PAPERSPACE_API_KEY=your-paperspace-key \
  -n tgp-system
```

### Usage

#### Basic GPU Request
```yaml
apiVersion: tgp.io/v1
kind: GPURequest
metadata:
  name: my-gpu-workload
spec:
  gpuType: "RTX4090"
  region: "us-west"
  maxPrice: 2.0
```

#### Check Status
```bash
# Check GPU request status
kubectl get gpurequest my-gpu-workload -o yaml

# Monitor operator logs
kubectl logs -n tgp-system deployment/tgp-operator-controller-manager -f

# Check available providers
kubectl get pods -n tgp-system
```

#### Provider Testing
```bash
# Test provider APIs locally (requires 1Password CLI)
task test:provider -- -provider=runpod -action=list
task test:provider -- -provider=lambdalabs -action=pricing -gpu-type=A100
task test:provider -- -provider=paperspace -action=info

# Test all providers
task test:providers
```

## Architecture

The operator consists of:

- **Controller** - Reconciles GPURequest custom resources
- **Provider clients** - Interface with cloud provider APIs
- **Pricing cache** - Tracks real-time pricing for cost optimization
- **Node lifecycle** - Manages instance provisioning and Kubernetes integration
- **Metrics collection** - Prometheus metrics for monitoring and cost tracking

## Monitoring

Prometheus metrics are exposed on `:8080/metrics`:

```bash
# Access metrics locally
kubectl port-forward -n tgp-system deployment/tgp-operator 8080:8080
curl http://localhost:8080/metrics | grep tgp_operator
```

Key metrics include request counts, launch durations, active instances, costs, and provider performance.

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for development setup, testing, and contribution guidelines.

## License

MIT License - see [LICENSE](LICENSE) file for details.