# Talos GPU Provisioner

Kubernetes operator for ephemeral GPU provisioning across multiple cloud providers.

Addresses intermittent GPU compute needs by provisioning instances on-demand from cloud providers and automatically integrating them into existing Talos Kubernetes clusters. Designed for workloads that require GPU resources occasionally rather than continuously.

## Features

- **Multi-cloud support** - RunPod, Lambda Labs, Paperspace with real API integration
- **Cost optimization** - Automatic provider selection based on real-time pricing
- **Secure credentials** - 1Password CLI integration for secret management
- **Lifecycle management** - Automated provisioning, configuration and cleanup
- **Pay-per-use model** - Resources exist only when actively needed
- **Production monitoring** - Prometheus metrics for cost tracking and operational visibility
- **Provider validation** - Real API credential verification and connectivity testing
- **Interactive testing** - CLI tool for testing provider APIs and pricing

## Quick Start

### Prerequisites

- Talos Linux Kubernetes cluster with cluster-admin access
- Cloud provider API credentials (one or more supported providers)
- WireGuard configuration for secure networking

### Installation

#### Option 1: OCI Registry (Recommended)

```bash
# Install directly from OCI registry
helm install tgp-operator oci://ghcr.io/solanyn/charts/tgp-operator \
  --version 0.0.1 \
  --namespace tgp-system \
  --create-namespace
```

#### Option 2: Helm Chart (OCI Registry)

```bash
# Install the operator from OCI registry
helm install tgp-operator oci://ghcr.io/solanyn/charts/tgp-operator \
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
  provider: runpod
  gpuType: RTX4090
  region: us-west
  maxHourlyPrice: "2.0"
  talosConfig:
    image: factory.talos.dev/installer/test:v1.8.2
    wireGuardConfig:
      privateKey: your-private-key
      publicKey: your-public-key
      serverEndpoint: vpn.example.com:51820
      allowedIPs: ["10.0.0.0/24"]
      address: 10.0.0.2/24
```

#### WireGuard with Secrets

For sensitive WireGuard configuration, use Kubernetes secrets:

```yaml
# Create WireGuard secret
apiVersion: v1
kind: Secret
metadata:
  name: wireguard-config
type: Opaque
stringData:
  private-key: your-private-key
  public-key: your-public-key
  server-endpoint: vpn.example.com:51820
  allowed-ips: 10.0.0.0/24
  address: 10.0.0.2/24
---
# GPU request using secret references
apiVersion: tgp.io/v1
kind: GPURequest
metadata:
  name: my-gpu-workload-secure
spec:
  provider: runpod
  gpuType: RTX4090
  region: us-west
  talosConfig:
    image: factory.talos.dev/installer/test:v1.8.2
    wireGuardConfig:
      privateKeySecretRef:
        name: wireguard-config
        key: private-key
      publicKeySecretRef:
        name: wireguard-config
        key: public-key
      serverEndpointSecretRef:
        name: wireguard-config
        key: server-endpoint
      allowedIPsSecretRef:
        name: wireguard-config
        key: allowed-ips
      addressSecretRef:
        name: wireguard-config
        key: address
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

## What it does

The operator provisions ephemeral GPU instances from cloud providers and automatically joins them to your Talos Kubernetes cluster. It handles provider selection, cost optimization, instance lifecycle and cleanup.

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for development setup, testing and contribution guidelines.

## License

MIT License - see [LICENSE](LICENSE) file for details.
