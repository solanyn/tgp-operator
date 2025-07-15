# Talos GPU Provisioner

Kubernetes operator for ephemeral GPU provisioning across multiple cloud providers.

Addresses intermittent GPU compute needs by provisioning instances on-demand from cloud providers and automatically integrating them into existing Talos Kubernetes clusters. Designed for workloads that require GPU resources occasionally rather than continuously.

## Features

- **Multi-cloud support** - Vast.ai, RunPod, Lambda Labs, Paperspace
- **Cost optimization** - Automatic provider selection based on real-time pricing
- **Talos integration** - Immutable node provisioning with secure networking
- **WireGuard connectivity** - Encrypted networking between cloud instances and cluster
- **Lifecycle management** - Automated provisioning, configuration, and cleanup
- **Pay-per-use model** - Resources exist only when actively needed
- **Production monitoring** - Prometheus metrics for cost tracking and operational visibility
- **Provider validation** - Credential verification and API connectivity testing

## Quick Start

### Prerequisites

- Kubernetes cluster with cluster-admin access
- Cloud provider API credentials (one or more supported providers)
- WireGuard configuration for secure networking

### Installation

```bash
# Install via Helm chart
helm install tgp-operator oci://ghcr.io/solanyn/charts/tgp-operator \
  --version 0.1.0 \
  --namespace tgp-system \
  --create-namespace
```

### Configuration

Create provider credentials:

```bash
kubectl create secret generic tgp-provider-secrets \
  --from-literal=VAST_API_KEY=your-vast-key \
  --from-literal=RUNPOD_API_KEY=your-runpod-key \
  --from-literal=LAMBDA_LABS_API_KEY=your-lambda-key \
  --from-literal=PAPERSPACE_API_KEY=your-paperspace-key \
  -n tgp-system
```

### Usage

```yaml
apiVersion: tgp.io/v1
kind: GPURequest
metadata:
  name: my-gpu-workload
spec:
  provider: "vast.ai"
  gpuType: "RTX4090"
  region: "us-east"
  maxHourlyPrice: "2.00"
  talosConfig:
    image: "ghcr.io/siderolabs/talos:v1.8.0"
    wireGuardConfig:
      privateKey: "your-private-key"
      publicKey: "your-public-key"
      serverEndpoint: "your-cluster-endpoint:51820"
      allowedIPs: ["10.244.0.0/16"]
      address: "10.5.0.10/24"
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