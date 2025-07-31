# Talos GPU Provisioner

> Still heavily WIP!

Kubernetes operator for ephemeral GPU provisioning across multiple cloud providers with Tailscale mesh networking.

Addresses intermittent GPU compute needs by provisioning instances on-demand from cloud providers and automatically integrating them into existing Talos Kubernetes clusters via Tailscale. Designed for workloads that require GPU resources occasionally rather than continuously.

## Features

- **Multi-cloud support** - RunPod, Lambda Labs and Paperspace
- **Tailscale mesh networking** - Node integration via Tailscale
- **Cost optimization** - Automatic provider selection based on real-time pricing
- **Lifecycle management** - Automated provisioning, configuration and cleanup
- **Production monitoring** - Prometheus metrics for cost tracking and operational visibility
- **Provider validation** - Real API credential verification and connectivity testing

## Installation

### Prerequisites

This operator requires the following:

- **Talos Kubernetes cluster** - See [Talos documentation](https://www.talos.dev/latest/introduction/getting-started/) for cluster setup
- **Tailscale** - For mesh networking and automatic node integration
- **Cloud provider API keys** - From supported providers (RunPod, Lambda Labs, Paperspace)

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

The operator uses resources inspired by Karpenter:

1. **`GPUNodeClass`** - Cluster-scoped infrastructure templates
2. **`GPUNodePool`** - Namespaced provisioning requests

First, create provider credentials and configure infrastructure templates with `GPUNodeClass` resources.

### Usage

#### Step 1: Create Provider Credentials

```bash
kubectl create secret generic provider-credentials \
  --from-literal=RUNPOD_API_KEY=your-runpod-key \
  --from-literal=LAMBDA_LABS_API_KEY=your-lambda-key \
  --from-literal=PAPERSPACE_API_KEY=your-paperspace-key \
  -n tgp-system
```

#### Step 2: Create GPUNodeClass (Infrastructure Template)

```yaml
apiVersion: tgp.io/v1
kind: GPUNodeClass
metadata:
  name: standard-gpu-class
spec:
  providers:
    - name: runpod
      priority: 1
      enabled: true
      credentialsRef:
        name: provider-credentials
        key: RUNPOD_API_KEY
    - name: lambdalabs
      priority: 2
      enabled: true
      credentialsRef:
        name: provider-credentials
        key: LAMBDA_LABS_API_KEY
  talosConfig:
    image: "ghcr.io/siderolabs/talos:v1.10.5"
  tailscaleConfig:
    tags: ["tag:k8s", "tag:gpu"]
    ephemeral: true
    acceptRoutes: true
  instanceRequirements:
    gpuTypes: ["RTX4090", "RTX3090"]
    spotAllowed: true
  limits:
    maxNodes: 10
    maxHourlyCost: "50.0"
```

#### Step 3: Create GPUNodePool (Provisioning Request)

```yaml
apiVersion: tgp.io/v1
kind: GPUNodePool
metadata:
  name: ml-workload-pool
  namespace: default
spec:
  nodeClassRef:
    kind: GPUNodeClass
    name: standard-gpu-class
  template:
    spec:
      requirements:
        - key: "tgp.io/gpu-type"
          operator: In
          values: ["RTX4090"]
        - key: "tgp.io/region"
          operator: In
          values: ["us-west", "us-east"]
      taints:
        - key: "gpu-node"
          value: "true"
          effect: NoSchedule
  maxHourlyPrice: "2.0"
  weight: 10
```

#### Check Status

```bash
# Check node classes and pools
kubectl get gpunodeclass
kubectl get gpunodepool -A

# Check specific resources
kubectl describe gpunodeclass standard-gpu-class
kubectl describe gpunodepool ml-workload-pool -n default

# Monitor operator logs
kubectl logs -n tgp-system deployment/tgp-operator-controller-manager -f
```

## Concepts

The operator enables two Karpenter-inspired resources:

1. Define infrastructure-level configuration `GPUNodeClass` such as:

   - Cloud provider credentials and settings
   - Talos OS and Tailscale networking configuration
   - Instance requirements and cost limits
   - Resource governance and security policies

2. Defines provisioning requests with `GPUNodePool`:
   - Reference a `GPUNodeClass` for infrastructure details
   - Specify node requirements and constraints
   - Handle lifecycle management and disruption policies

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for development setup, testing and contribution guidelines.

## License

MIT License - see [LICENSE](LICENSE) file for details.
