# Talos GPU Provisioner

> Still heavily WIP!

Kubernetes operator for ephemeral GPU provisioning across cloud providers using Tailscale mesh networking.

Provisions GPU instances on-demand and integrates them into Talos Kubernetes clusters.

## Features

- Multi-cloud support: Google Cloud Platform, Vultr
- Pricing optimised GPU instance selection
- Instance lifecycle management

## Installation

### Prerequisites

- Talos Kubernetes cluster
- Mesh networking (e.g., tailscale) or expose control plane publicly or use Omni
- Cloud provider credentials

### Install Operator

```bash
# Install the TGP operator
helm install tgp-operator oci://ghcr.io/solanyn/charts/tgp-operator \
  --namespace tgp-system \
  --create-namespace
```

### Configuration

We provide two resource types:

1. `GPUNodeClass` - Cluster-scoped infrastructure templates
2. `GPUNodePool` - Namespaced provisioning requests

### Usage

#### Step 1: Create Provider Credentials

```bash
# Create provider credentials secret
kubectl create secret generic tgp-operator-secret \
  --from-literal=GOOGLE_APPLICATION_CREDENTIALS_JSON='{"type":"service_account","project_id":"your-project",...}' \
  --from-literal=VULTR_API_KEY=your-vultr-api-key \
  --from-literal=client-id=your-tailscale-oauth-client-id \
  --from-literal=client-secret=your-tailscale-oauth-client-secret \
  -n tgp-system
```

Required credentials:

- Google Cloud service account JSON with IAM permissions
- Vultr API key from account API section
- Tailscale OAuth credentials from admin console

#### Google Cloud Platform Setup

Prepare Talos Linux images in your GCP project.

**Option 1: Manual Upload**

```bash
# Download Talos GCP image
wget https://github.com/siderolabs/talos/releases/download/v1.10.5/gcp-amd64.tar.gz

# Create a temporary GCS bucket (if you don't have one)
gsutil mb gs://YOUR-BUCKET-NAME

# Upload to GCS bucket
gsutil cp gcp-amd64.tar.gz gs://YOUR-BUCKET-NAME/talos-v1.10.5.tar.gz

# Create compute image from bucket
gcloud compute images create talos-linux-latest \
  --source-uri gs://YOUR-BUCKET-NAME/talos-v1.10.5.tar.gz \
  --family talos-linux \
  --description "Talos Linux v1.10.5 for GPU workloads"

# Clean up temporary files
rm gcp-amd64.tar.gz
gsutil rm gs://YOUR-BUCKET-NAME/talos-v1.10.5.tar.gz
```

**Option 2: Custom Image Reference**
Specify image URL in `GPUNodeClass`:

```yaml
spec:
  talosConfig:
    image: "projects/MY-PROJECT/global/images/my-custom-talos-image"
```

**Required GCP IAM roles:**

- `Compute Instance Admin (v1)`
- `Compute Image User`
- `Service Account User`

#### Vultr Setup

- Get API key from Vultr Control Panel → Account → API
- Talos Linux available via marketplace (OS ID: 2284)
- GPU types: H100, L40S, A100, A40, A16, MI325X, MI300X

**Required permissions:**

- Instance management
- Plan access
- Region access

#### Step 2: Create GPUNodeClass (Infrastructure Template)

`GPUNodeClass` requires Talos machine configuration template with variables:

- `{{.MachineToken}}`
- `{{.ClusterCA}}`
- `{{.ClusterID}}`
- `{{.ClusterSecret}}`
- `{{.ControlPlaneEndpoint}}`
- `{{.ClusterName}}`
- `{{.TailscaleAuthKey}}`
- `{{.NodeName}}` - Generated node name
- `{{.NodePool}}` - NodePool name
- `{{.NodeIndex}}` - Node index in pool

```yaml
apiVersion: tgp.io/v1
kind: GPUNodeClass
metadata:
  name: standard-gpu-class
spec:
  providers:
    - name: gcp
      priority: 1
      enabled: true
      credentialsRef:
        name: tgp-operator-secret
        key: GOOGLE_APPLICATION_CREDENTIALS_JSON
  talosConfig:
    image: "ghcr.io/siderolabs/talos:v1.10.5"
    machineConfigTemplate: |
      version: v1alpha1
      debug: false
      persist: true
      machine:
        token: {{.MachineToken}}
        ca:
          crt: {{.ClusterCA}}
        certSANs:
          - 127.0.0.1
        kubelet:
          extraMounts:
            - destination: /var/mnt/extra
              type: bind
              source: /var/mnt/extra
              options: [bind, rshared, rw]
        files:
          - path: /etc/tailscale/authkey
            permissions: 0o600
            op: create
            content: {{.TailscaleAuthKey}}
          - path: /etc/systemd/system/tailscaled.service
            op: create
            content: |
              [Unit]
              Description=Tailscale VPN
              After=network.target
              [Service]
              Type=notify
              ExecStart=/usr/bin/tailscaled --state=/var/lib/tailscale/tailscaled.state
              ExecStartPost=/usr/bin/tailscale up --authkey-file=/etc/tailscale/authkey --hostname={{.NodeName}}
              Restart=always
              [Install]
              WantedBy=multi-user.target
        systemd:
          services:
            - name: tailscaled.service
              enabled: true
        nodeLabels:
          tgp.io/nodepool: {{.NodePool}}
          tgp.io/provisioned: "true"
          node.kubernetes.io/instance-type: "gpu"
      cluster:
        id: {{.ClusterID}}
        secret: {{.ClusterSecret}}
        controlPlane:
          endpoint: {{.ControlPlaneEndpoint}}
        clusterName: {{.ClusterName}}
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

This operator exposes models CRDs inspired by [Karpenter](https://karpenter.sh):

1. `GPUNodeClass` - Infrastructure configuration:

   - Provider credentials and settings
   - Talos OS and Tailscale configuration
   - Instance requirements and cost limits
   - Security policies

2. `GPUNodePool` - Provisioning requests:
   - References `GPUNodeClass`
   - Node requirements and constraints
   - Lifecycle and disruption policies

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for development setup, testing and contribution guidelines.

## License

MIT License - see [LICENSE](LICENSE) file for details.
