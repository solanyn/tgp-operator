# TGP Operator Migration Plan: WireGuard to Tailscale Operator

## Overview

Migrate the TGP (Talos GPU Provisioner) operator from self-managed WireGuard VPN infrastructure to Kubernetes-native Tailscale Operator integration. This migration addresses the core limitation of dynamic GPU node provisioning while significantly reducing operational complexity.

## Current State Analysis

### WireGuard Limitations
- **Manual Infrastructure**: Requires dedicated WireGuard server (VPS/homelab)
- **Static Configuration**: Pre-generated keys and IP assignments prevent dynamic enrollment
- **Operational Overhead**: Manual key management, routing configuration, server maintenance
- **Scaling Challenges**: Each new node requires manual intervention

### Current Architecture
```
GPU Workload → ProxyClass → Generic Device Plugin → WireGuard Tunnel → Remote GPU Node
                                                   (10.0.0.x static)     (cloud provider)
```

## Target Architecture

### Tailscale Operator Integration
- **Kubernetes-Native**: TailscaleDevice CRDs for device lifecycle management
- **OAuth Authentication**: Eliminates manual auth key handling
- **Dynamic Enrollment**: Nodes can self-register automatically
- **Mesh Network**: Direct peer-to-peer connectivity with automatic NAT traversal

### New Architecture
```
GPU Workload → ProxyClass → Generic Device Plugin → Tailscale Mesh → Remote GPU Node
                                                   (100.x.x.x dynamic)  (cloud provider)
```

## Migration Strategy

### Phase 1: Research & Design (1-2 weeks)

**Objectives:**
- Understand Tailscale Operator API surface and capabilities
- Design integration patterns with TGP operator lifecycle
- Plan OAuth credential management strategy

**Tasks:**
- [ ] Study Tailscale Operator CRD specifications (TailscaleDevice, TailscaleService)
- [ ] Research OAuth client setup and permission requirements
- [ ] Design new `TailscaleConfig` API structure
- [ ] Evaluate device lifecycle management patterns
- [ ] Plan feature flag strategy for gradual migration

**Deliverables:**
- Technical design document
- API schema definitions
- Integration architecture diagrams

### Phase 2: API & Schema Changes (1 week)

**Objectives:**
- Implement new Tailscale configuration APIs
- Maintain backward compatibility during transition
- Add configuration switches for networking backends

**Tasks:**
- [ ] Update `pkg/api/v1/types.go` with `TailscaleConfig` struct
- [ ] Implement schema validation and defaults
- [ ] Add backward compatibility for existing `WireGuardConfig`
- [ ] Create feature flags for networking backend selection
- [ ] Update CRD manifests and OpenAPI schemas

**API Design:**
```go
type TailscaleConfig struct {
    // Hostname for the Tailscale device
    Hostname string `json:"hostname,omitempty"`
    
    // Tags to apply to the device for ACL targeting
    Tags []string `json:"tags,omitempty"`
    
    // Whether this device should be ephemeral (cleanup on deletion)
    Ephemeral bool `json:"ephemeral,omitempty"`
    
    // Accept routes from other devices in the tailnet
    AcceptRoutes bool `json:"acceptRoutes,omitempty"`
    
    // Subnet routes to advertise (for gateway nodes)
    AdvertiseRoutes []string `json:"advertiseRoutes,omitempty"`
}

type TalosConfig struct {
    Image string `json:"image"`
    
    // Legacy WireGuard support (deprecated)
    WireGuardConfig *WireGuardConfig `json:"wireGuardConfig,omitempty"`
    
    // New Tailscale support via Tailscale Operator
    TailscaleConfig *TailscaleConfig `json:"tailscaleConfig,omitempty"`
}
```

### Phase 3: Controller Integration (2 weeks)

**Objectives:**
- Integrate TGP controller with Tailscale Operator workflows
- Implement automatic device lifecycle management
- Update Talos configuration generation for Tailscale bootstrap

**Tasks:**
- [ ] Modify `GPURequestReconciler` for TailscaleDevice CRD management
- [ ] Implement OAuth client credential handling
- [ ] Update Talos configuration generation with Tailscale bootstrap
- [ ] Add NVIDIA containerd runtime configuration
- [ ] Implement device cleanup on GPURequest deletion

**Controller Integration Pattern:**
```go
func (r *GPURequestReconciler) reconcileTailscaleDevice(ctx context.Context, gpuRequest *tgpv1.GPURequest) error {
    // Create TailscaleDevice CRD
    device := &tailscalev1alpha1.TailscaleDevice{
        ObjectMeta: metav1.ObjectMeta{
            Name:      fmt.Sprintf("gpu-node-%s", gpuRequest.Name),
            Namespace: gpuRequest.Namespace,
            OwnerReferences: []metav1.OwnerReference{
                *metav1.NewControllerRef(gpuRequest, tgpv1.GroupVersion.WithKind("GPURequest")),
            },
        },
        Spec: tailscalev1alpha1.TailscaleDeviceSpec{
            Hostname:        gpuRequest.Spec.TalosConfig.TailscaleConfig.Hostname,
            Tags:           gpuRequest.Spec.TalosConfig.TailscaleConfig.Tags,
            Ephemeral:      true,
            AcceptRoutes:   true,
        },
    }
    
    return r.Client.Create(ctx, device)
}
```

### Phase 4: Provider Client Updates (1 week)

**Objectives:**
- Update cloud provider clients for Tailscale bootstrap integration
- Implement dynamic auth key retrieval from TailscaleDevice status
- Validate bootstrap timing and connectivity establishment

**Tasks:**
- [ ] Update RunPod client for Tailscale cloud-init generation
- [ ] Update Lambda Labs client for Tailscale user_data integration
- [ ] Update Paperspace client for Tailscale startup scripts
- [ ] Implement auth key polling from TailscaleDevice status
- [ ] Add bootstrap connectivity validation

**Bootstrap Configuration Template:**
```yaml
#cloud-config
write_files:
  - path: /opt/bootstrap-tailscale.sh
    permissions: '0755'
    content: |
      #!/bin/bash
      # Install Tailscale
      curl -fsSL https://tailscale.com/install.sh | sh
      
      # Start daemon and connect
      tailscaled --state=/var/lib/tailscale/tailscaled.state &
      tailscale up --authkey={{.AuthKey}} --hostname={{.Hostname}} --accept-routes
      
      # Wait for mesh connectivity
      until tailscale status --json | jq -r '.BackendState' | grep -q "Running"; do
        sleep 2
      done
      
      # Validate cluster API connectivity
      until curl -k https://k8s-api.tailnet.ts.net:6443/version; do
        echo "Waiting for cluster API via Tailscale..."
        sleep 5
      done

runcmd:
  - /opt/bootstrap-tailscale.sh

# Talos machine config with NVIDIA runtime support
machine:
  files:
    - op: create
      path: /etc/cri/conf.d/20-nvidia.part
      content: |-
        [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.nvidia]
          container_annotations = ["nvidia.cdi.k8s.io/*"]
          privileged_without_host_devices = false
          runtime_type = "io.containerd.runc.v2"
          [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.nvidia.options]
            BinaryName = "/var/nvidia/toolkit/nvidia-container-runtime"

cluster:
  controlPlane:
    endpoint: https://k8s-api.tailnet.ts.net:6443
```

### Phase 5: Documentation & Examples (1 week)

**Objectives:**
- Provide comprehensive migration documentation
- Create user-friendly setup guides
- Document troubleshooting procedures

**Tasks:**
- [ ] Update README.md with Tailscale Operator prerequisites
- [ ] Create migration guide from WireGuard to Tailscale
- [ ] Document OAuth application setup process
- [ ] Provide example configurations and use cases
- [ ] Update Helm chart with Tailscale backend options

**User Setup Documentation:**
```markdown
# Prerequisites

## 1. Tailscale OAuth Application
1. Go to https://login.tailscale.com/admin/settings/oauth
2. Create new OAuth client with device management permissions
3. Note client ID and secret for operator configuration

## 2. Install Tailscale Operator
```bash
helm repo add tailscale https://pkgs.tailscale.com/helmcharts
helm install tailscale-operator tailscale/tailscale-operator \
  --namespace=tailscale \
  --create-namespace \
  --set-string oauth.clientId="your-client-id" \
  --set-string oauth.clientSecret="your-client-secret"
```

## 3. Expose Kubernetes API
```bash
kubectl annotate service kubernetes tailscale.com/expose=true
kubectl annotate service kubernetes tailscale.com/hostname=k8s-api
```

## 4. Install TGP Operator
```bash
helm install tgp-operator oci://ghcr.io/solanyn/charts/tgp-operator \
  --set networking.backend=tailscale \
  --set networking.tailscale.operatorEnabled=true
```
```

### Phase 6: Testing & Validation (1-2 weeks)

**Objectives:**
- Comprehensive end-to-end testing with real cloud providers
- Performance validation against current WireGuard setup
- Validate ProxyClass + Generic Device Plugin integration
- Load testing with multiple concurrent GPU provisioning

**Testing Strategy:**
- [ ] Unit tests for new Tailscale configuration APIs
- [ ] Integration tests with mock Tailscale Operator
- [ ] E2E tests with real cloud provider APIs
- [ ] Performance benchmarking (network latency, throughput)
- [ ] GPU workload validation with ProxyClass integration
- [ ] Chaos testing (network partitions, operator restarts)

**Success Criteria:**
- GPU nodes can self-enroll without manual intervention
- Network performance within 85% of current WireGuard setup
- Zero-downtime migration for existing deployments
- Reduced provisioning time by >50%
- Elimination of manual WireGuard infrastructure

## Implementation Details

### Prerequisites for Users

**Required Infrastructure:**
1. **Tailscale Operator** installed with OAuth credentials
2. **Kubernetes API exposed via Tailscale** (automatic with annotations)
3. **NVIDIA GPU Operator** for dynamic GPU driver installation

**No Longer Required:**
- Manual WireGuard server setup and maintenance
- Static IP address management
- Manual auth key generation and distribution
- VPN server monitoring and updates

### TGP Operator Enhancements

**New Capabilities:**
- Automatic TailscaleDevice lifecycle management
- OAuth-based device authentication
- Dynamic hostname generation and assignment
- Ephemeral device cleanup on resource deletion
- Integrated NVIDIA runtime configuration

**Backward Compatibility:**
- Existing WireGuard configurations continue to work
- Gradual migration path with feature flags
- Clear deprecation timeline for WireGuard support

### ProxyClass Integration

**Compatibility Requirements:**
- Update ProxyClass configuration to use Tailscale hostnames instead of static IPs
- Configure Tailscale ACLs for GPU communication ports
- Validate proxy performance over Tailscale mesh network

**Configuration Updates:**
```yaml
# ProxyClass configuration changes
apiVersion: device.k8s.io/v1alpha1
kind: DeviceClass
metadata:
  name: gpu-proxy-class
spec:
  config:
    remoteEndpoints:
    # Old: Static WireGuard IPs
    # - address: "10.0.0.100:9999"
    # New: Dynamic Tailscale hostnames
    - address: "gpu-node-1.tailnet.ts.net:9999"
```

## Risk Assessment & Mitigation

### Technical Risks

**Performance Impact:**
- *Risk*: 15-30% network performance reduction vs WireGuard
- *Mitigation*: Benchmark GPU workloads; most are compute-bound, not network-bound

**Vendor Dependency:**
- *Risk*: Increased dependency on Tailscale service availability
- *Mitigation*: Maintain WireGuard fallback during transition period

**OAuth Complexity:**
- *Risk*: More complex authentication setup for users
- *Mitigation*: Comprehensive documentation and setup automation

### Operational Risks

**Migration Complexity:**
- *Risk*: Breaking existing user deployments
- *Mitigation*: Maintain backward compatibility and provide migration tools

**Support Burden:**
- *Risk*: Increased support complexity during transition
- *Mitigation*: Clear documentation and gradual rollout strategy

## Success Metrics

### Primary Objectives
- **Dynamic Provisioning**: Enable GPU nodes to self-enroll without manual intervention
- **Operational Simplicity**: Eliminate WireGuard server maintenance
- **Performance**: Maintain acceptable network performance for GPU workloads
- **User Experience**: Simplify setup to minimal configuration

### Measurable Outcomes
- Reduction in user setup time from hours to minutes
- Elimination of WireGuard infrastructure costs
- Improved reliability through managed service
- Enhanced security through OAuth vs static keys

## Timeline Summary

| Phase | Duration | Key Deliverables |
|-------|----------|------------------|
| 1. Research & Design | 1-2 weeks | Technical design, API specifications |
| 2. API Changes | 1 week | Updated CRDs, backward compatibility |
| 3. Controller Integration | 2 weeks | TailscaleDevice lifecycle management |
| 4. Provider Updates | 1 week | Cloud provider client modifications |
| 5. Documentation | 1 week | User guides, migration documentation |
| 6. Testing & Validation | 1-2 weeks | E2E testing, performance validation |

**Total Estimated Duration: 6-8 weeks**

## Conclusion

This migration from WireGuard to Tailscale Operator integration addresses the fundamental limitation of dynamic GPU node provisioning while significantly reducing operational complexity. The Kubernetes-native approach provides better integration with existing workflows and eliminates the need for manual VPN infrastructure management.

The phased approach ensures backward compatibility and provides a clear migration path for existing users, while the comprehensive testing strategy validates performance and reliability requirements.