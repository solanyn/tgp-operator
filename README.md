# Talos GPU Provisioner

Kubernetes operator for ephemeral GPU provisioning across multiple cloud providers with Tailscale mesh networking.

Addresses intermittent GPU compute needs by provisioning instances on-demand from cloud providers and automatically integrating them into existing Talos Kubernetes clusters via Tailscale. Designed for workloads that require GPU resources occasionally rather than continuously.

## Features

- **Multi-cloud support** - RunPod, Lambda Labs and Paperspace
- **Tailscale mesh networking** - Automatic node integration via Tailscale (no VPN server needed)
- **Cost optimization** - Automatic provider selection based on real-time pricing
- **Secure credentials** - 1Password CLI integration for secret management
- **Lifecycle management** - Automated provisioning, configuration and cleanup
- **Pay-per-use model** - Resources exist only when actively needed
- **Production monitoring** - Prometheus metrics for cost tracking and operational visibility
- **Provider validation** - Real API credential verification and connectivity testing
- **Interactive testing** - CLI tool for testing provider APIs and pricing

## Installation

### Prerequisites

This operator requires the following infrastructure:

- **Talos Kubernetes cluster** - See [Talos documentation](https://www.talos.dev/latest/introduction/getting-started/) for cluster setup
- **Tailscale Operator** - For mesh networking and automatic node integration
- **Cloud provider API keys** - From supported providers (RunPod, Lambda Labs, Paperspace)
- **Tailscale OAuth credentials** - For device management and auth key creation

### Setup Tailscale

First, set up the Tailscale Operator in your cluster:

```bash
# 1. Create Tailscale OAuth client at https://login.tailscale.com/admin/settings/oauth
# 2. Grant device management permissions and tag:k8s-operator tag

# Install Tailscale Operator
helm repo add tailscale https://pkgs.tailscale.com/helmcharts
helm install tailscale-operator tailscale/tailscale-operator \
  --namespace=tailscale \
  --create-namespace \
  --set-string oauth.clientId="your-client-id" \
  --set-string oauth.clientSecret="your-client-secret"

# Expose Kubernetes API via Tailscale
kubectl annotate service kubernetes tailscale.com/expose=true
kubectl annotate service kubernetes tailscale.com/hostname=k8s-api
```

### Install Operator

Once Tailscale is configured:

```bash
# Install the TGP operator
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

> **Note**: For complete setup instructions including Talos cluster deployment, Tailscale configuration, and network setup, see [DEVELOPMENT.md](DEVELOPMENT.md).

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
    tailscaleConfig:
      hostname: my-gpu-node
      tags: ["tag:k8s"]
      ephemeral: true
      acceptRoutes: true
```

#### Tailscale with Auth Keys

For production deployments, use Tailscale auth keys stored in Kubernetes secrets:

```yaml
# Create Tailscale auth key secret
apiVersion: v1
kind: Secret
metadata:
  name: tailscale-auth
type: Opaque
stringData:
  auth-key: tskey-auth-your-key-here
---
# GPU request using auth key secret
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
    tailscaleConfig:
      hostname: secure-gpu-node
      tags: ["tag:k8s", "tag:gpu"]
      ephemeral: true
      acceptRoutes: true
      authKeySecretRef:
        name: tailscale-auth
        key: auth-key
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
