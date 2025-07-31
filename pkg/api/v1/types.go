// Package v1 contains API Schema definitions for the tgp v1beta1 API group
// +kubebuilder:object:generate=true
package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// init registers the TGP types with the scheme builder.
// This is required for Kubernetes controller-runtime functionality.
func init() { //nolint:gochecknoinits // Required for Kubernetes scheme registration
	SchemeBuilder.Register(
		&GPUNodeClass{}, &GPUNodeClassList{},
		&GPUNodePool{}, &GPUNodePoolList{},
	)
}

// GPUNodeClass defines infrastructure templates for GPU node provisioning
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
type GPUNodeClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GPUNodeClassSpec   `json:"spec,omitempty"`
	Status GPUNodeClassStatus `json:"status,omitempty"`
}

// GPUNodeClassList contains a list of GPUNodeClass
// +kubebuilder:object:root=true
type GPUNodeClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GPUNodeClass `json:"items"`
}

// GPUNodeClassSpec defines the desired state of GPUNodeClass
type GPUNodeClassSpec struct {
	// Providers defines the cloud providers and their configuration
	Providers []ProviderConfig `json:"providers"`

	// TalosConfig contains default Talos OS configuration
	TalosConfig *TalosConfig `json:"talosConfig,omitempty"`

	// TailscaleConfig contains default Tailscale networking configuration
	TailscaleConfig *TailscaleConfig `json:"tailscaleConfig,omitempty"`

	// InstanceRequirements defines the instance constraints
	InstanceRequirements *InstanceRequirements `json:"instanceRequirements,omitempty"`

	// Limits defines resource and cost limits for this node class
	Limits *NodeClassLimits `json:"limits,omitempty"`

	// Tags are propagated to all instances created from this node class
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
}

// GPUNodeClassStatus defines the observed state of GPUNodeClass
type GPUNodeClassStatus struct {
	// Conditions represent the latest available observations of the node class's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ActiveNodes is the number of nodes currently active from this class
	// +optional
	ActiveNodes int32 `json:"activeNodes,omitempty"`

	// TotalCost is the current hourly cost of all active nodes
	// +optional
	TotalCost string `json:"totalCost,omitempty"`
}

// GPUNodePool defines provisioning pools that reference GPUNodeClass templates
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type GPUNodePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GPUNodePoolSpec   `json:"spec,omitempty"`
	Status GPUNodePoolStatus `json:"status,omitempty"`
}

// GPUNodePoolList contains a list of GPUNodePool
// +kubebuilder:object:root=true
type GPUNodePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GPUNodePool `json:"items"`
}

// GPUNodePoolSpec defines the desired state of GPUNodePool
type GPUNodePoolSpec struct {
	// NodeClassRef is a reference to the GPUNodeClass to use for nodes
	NodeClassRef NodeClassReference `json:"nodeClassRef"`

	// Template contains the node template specification
	Template NodePoolTemplate `json:"template"`

	// Disruption defines the disruption policy for nodes in this pool
	// +optional
	Disruption *DisruptionSpec `json:"disruption,omitempty"`

	// Limits define resource limits for this node pool
	// +optional
	Limits *NodePoolLimits `json:"limits,omitempty"`

	// MaxHourlyPrice sets the maximum price per hour for instances in this pool
	// +optional
	MaxHourlyPrice *string `json:"maxHourlyPrice,omitempty"`

	// Weight is used for prioritization when multiple pools can satisfy requirements
	// Higher weights are preferred. Defaults to 10.
	// +optional
	Weight *int32 `json:"weight,omitempty"`
}

// GPUNodePoolStatus defines the observed state of GPUNodePool
type GPUNodePoolStatus struct {
	// Conditions represent the latest available observations of the pool's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Resources contains the current resource usage for this pool
	// +optional
	Resources corev1.ResourceList `json:"resources,omitempty"`

	// NodeCount is the current number of nodes in this pool
	// +optional
	NodeCount int32 `json:"nodeCount,omitempty"`
}

// NodeClassReference is a reference to a GPUNodeClass
type NodeClassReference struct {
	// Group of the referent
	// +optional
	Group string `json:"group,omitempty"`

	// Kind of the referent
	Kind string `json:"kind"`

	// Name of the referent
	Name string `json:"name"`
}

// NodePoolTemplate defines the template for nodes in a pool
type NodePoolTemplate struct {
	// Metadata is applied to nodes created from this template
	// +optional
	Metadata *NodeMetadata `json:"metadata,omitempty"`

	// Spec defines the desired characteristics of nodes
	Spec NodeSpec `json:"spec"`
}

// NodeMetadata contains metadata to apply to nodes
type NodeMetadata struct {
	// Labels to apply to the node
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations to apply to the node
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// NodeSpec defines the desired characteristics of nodes
type NodeSpec struct {
	// Requirements are node requirements that must be met
	Requirements []NodeSelectorRequirement `json:"requirements,omitempty"`

	// Taints are applied to nodes to prevent pods from scheduling onto them
	// +optional
	Taints []corev1.Taint `json:"taints,omitempty"`

	// StartupTaints are applied to nodes during startup and removed once ready
	// +optional
	StartupTaints []corev1.Taint `json:"startupTaints,omitempty"`
}

// NodeSelectorRequirement contains values, a key, and an operator
type NodeSelectorRequirement struct {
	// Key is the label key that the selector applies to
	Key string `json:"key"`

	// Operator represents a key's relationship to a set of values
	Operator NodeSelectorOperator `json:"operator"`

	// Values is an array of string values
	// +optional
	Values []string `json:"values,omitempty"`
}

// NodeSelectorOperator is the set of operators for node selection
type NodeSelectorOperator string

const (
	NodeSelectorOpIn           NodeSelectorOperator = "In"
	NodeSelectorOpNotIn        NodeSelectorOperator = "NotIn"
	NodeSelectorOpExists       NodeSelectorOperator = "Exists"
	NodeSelectorOpDoesNotExist NodeSelectorOperator = "DoesNotExist"
	NodeSelectorOpGt           NodeSelectorOperator = "Gt"
	NodeSelectorOpLt           NodeSelectorOperator = "Lt"
)

// TGP-specific node label keys
const (
	NodeLabelGPUType     = "tgp.io/gpu-type"
	NodeLabelProvider    = "tgp.io/provider"
	NodeLabelRegion      = "tgp.io/region"
	NodeLabelSpot        = "tgp.io/spot"
	NodeLabelProvisioned = "tgp.io/provisioned"
)

// ProviderConfig defines configuration for a cloud provider
type ProviderConfig struct {
	// Name of the provider (runpod, paperspace, lambdalabs)
	Name string `json:"name"`

	// Priority for provider selection (lower numbers = higher priority)
	// +optional
	Priority int32 `json:"priority,omitempty"`

	// CredentialsRef references the secret containing provider credentials
	CredentialsRef SecretKeyRef `json:"credentialsRef"`

	// Enabled indicates whether this provider is available for use
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Regions specifies the allowed regions for this provider
	// +optional
	Regions []string `json:"regions,omitempty"`
}

// InstanceRequirements defines constraints for instance selection
type InstanceRequirements struct {
	// GPUTypes lists the allowed GPU types
	// +optional
	GPUTypes []string `json:"gpuTypes,omitempty"`

	// Regions lists the preferred regions
	// +optional
	Regions []string `json:"regions,omitempty"`

	// SpotAllowed indicates whether spot instances are allowed
	// +optional
	SpotAllowed *bool `json:"spotAllowed,omitempty"`

	// MinVCPU specifies the minimum number of vCPUs
	// +optional
	MinVCPU *int32 `json:"minVCPU,omitempty"`

	// MinMemoryGiB specifies the minimum memory in GiB
	// +optional
	MinMemoryGiB *int32 `json:"minMemoryGiB,omitempty"`

	// MinGPUMemoryGiB specifies the minimum GPU memory in GiB
	// +optional
	MinGPUMemoryGiB *int32 `json:"minGPUMemoryGiB,omitempty"`
}

// NodeClassLimits defines limits for a GPUNodeClass
type NodeClassLimits struct {
	// MaxNodes is the maximum number of nodes that can be created from this class
	// +optional
	MaxNodes *int32 `json:"maxNodes,omitempty"`

	// MaxHourlyCost is the maximum total hourly cost for all nodes from this class
	// +optional
	MaxHourlyCost *string `json:"maxHourlyCost,omitempty"`

	// Resources defines resource limits for this node class
	// +optional
	Resources corev1.ResourceList `json:"resources,omitempty"`
}

// NodePoolLimits defines limits for a GPUNodePool
type NodePoolLimits struct {
	// Resources defines resource limits for this node pool
	// +optional
	Resources corev1.ResourceList `json:"resources,omitempty"`
}

// DisruptionSpec defines the disruption policy for nodes
type DisruptionSpec struct {
	// ConsolidationPolicy describes when nodes should be consolidated
	// +optional
	ConsolidationPolicy ConsolidationPolicy `json:"consolidationPolicy,omitempty"`

	// ConsolidateAfter is the duration after which empty nodes should be consolidated
	// +optional
	ConsolidateAfter *metav1.Duration `json:"consolidateAfter,omitempty"`

	// ExpireAfter is the duration after which nodes should be expired regardless of utilization
	// +optional
	ExpireAfter *metav1.Duration `json:"expireAfter,omitempty"`
}

// ConsolidationPolicy defines when nodes should be consolidated
type ConsolidationPolicy string

const (
	ConsolidationPolicyWhenIdle          ConsolidationPolicy = "WhenIdle"
	ConsolidationPolicyWhenUnderutilized ConsolidationPolicy = "WhenUnderutilized"
	ConsolidationPolicyNever             ConsolidationPolicy = "Never"
)

// TalosConfig contains Talos node configuration
type TalosConfig struct {
	// Image specifies the Talos image to use
	Image string `json:"image"`

	// MachineConfigTemplate contains template for Talos machine configuration
	// +optional
	MachineConfigTemplate string `json:"machineConfigTemplate,omitempty"`

	// KubeletImage specifies the kubelet image to use (defaults to GPU-optimized image)
	// +optional
	KubeletImage string `json:"kubeletImage,omitempty"`
}

// TailscaleConfig contains Tailscale mesh networking configuration
type TailscaleConfig struct {
	// Hostname for the Tailscale device (optional, defaults to generated name)
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// Tags to apply to the device for ACL targeting
	// Default: ["tag:k8s"]
	// +optional
	Tags []string `json:"tags,omitempty"`

	// Ephemeral indicates whether the device should be ephemeral (cleanup on deletion)
	// Default: true
	// +optional
	Ephemeral *bool `json:"ephemeral,omitempty"`

	// AcceptRoutes indicates whether to accept routes from other devices in the tailnet
	// Default: true
	// +optional
	AcceptRoutes *bool `json:"acceptRoutes,omitempty"`

	// AdvertiseRoutes specifies subnet routes to advertise (for gateway nodes)
	// +optional
	AdvertiseRoutes []string `json:"advertiseRoutes,omitempty"`

	// AuthKeySecretRef references a secret containing the Tailscale auth key
	// Deprecated: Use OAuthCredentialsSecretRef for better security and automatic key management
	// +optional
	AuthKeySecretRef *SecretKeyRef `json:"authKeySecretRef,omitempty"`

	// OAuthCredentialsSecretRef references a secret containing Tailscale OAuth credentials
	// The operator will use these to dynamically generate auth keys as needed
	// Secret should contain 'client-id' and 'client-secret' keys
	// +optional
	OAuthCredentialsSecretRef *TailscaleOAuthSecretRef `json:"oauthCredentialsSecretRef,omitempty"`

	// OperatorConfig contains Tailscale Operator integration settings
	// +optional
	OperatorConfig *TailscaleOperatorConfig `json:"operatorConfig,omitempty"`
}

// TailscaleOperatorConfig contains Tailscale Operator specific configuration
type TailscaleOperatorConfig struct {
	// ConnectorEnabled indicates whether to create a Tailscale Connector CRD
	// Default: true
	// +optional
	ConnectorEnabled *bool `json:"connectorEnabled,omitempty"`

	// ConnectorSpec allows customization of the Connector CRD
	// +optional
	ConnectorSpec *TailscaleConnectorSpec `json:"connectorSpec,omitempty"`
}

// TailscaleConnectorSpec contains Tailscale Connector CRD configuration
type TailscaleConnectorSpec struct {
	// SubnetRouter configures the node as a subnet router
	// +optional
	SubnetRouter *TailscaleSubnetRouter `json:"subnetRouter,omitempty"`

	// ExitNode configures the node as an exit node
	// +optional
	ExitNode *bool `json:"exitNode,omitempty"`

	// AppConnector configures the node as an app connector
	// +optional
	AppConnector *bool `json:"appConnector,omitempty"`
}

// TailscaleSubnetRouter contains subnet router configuration
type TailscaleSubnetRouter struct {
	// AdvertiseRoutes specifies the subnet routes to advertise
	AdvertiseRoutes []string `json:"advertiseRoutes"`
}

// SecretKeyRef references a specific key in a Kubernetes secret
type SecretKeyRef struct {
	// Name is the name of the secret
	Name string `json:"name"`

	// Key is the key within the secret
	Key string `json:"key"`

	// Namespace is the namespace of the secret (optional, defaults to current namespace)
	Namespace string `json:"namespace,omitempty"`
}

// TailscaleOAuthSecretRef references a secret containing Tailscale OAuth credentials
type TailscaleOAuthSecretRef struct {
	// Name is the name of the secret containing OAuth credentials
	Name string `json:"name"`

	// Namespace is the namespace of the secret (optional, defaults to current namespace)
	Namespace string `json:"namespace,omitempty"`

	// ClientIDKey is the key containing the OAuth client ID (defaults to "client-id")
	// +optional
	ClientIDKey string `json:"clientIdKey,omitempty"`

	// ClientSecretKey is the key containing the OAuth client secret (defaults to "client-secret")
	// +optional
	ClientSecretKey string `json:"clientSecretKey,omitempty"`
}

// TailscaleConfig helper methods

// GetHostname returns the hostname or a generated default
func (tc *TailscaleConfig) GetHostname(fallback string) string {
	if tc.Hostname != "" {
		return tc.Hostname
	}
	return fallback
}

// GetTags returns the tags or default tags
func (tc *TailscaleConfig) GetTags() []string {
	if len(tc.Tags) > 0 {
		return tc.Tags
	}
	return []string{"tag:k8s"}
}

// GetEphemeral returns the ephemeral setting or default (true)
func (tc *TailscaleConfig) GetEphemeral() bool {
	if tc.Ephemeral != nil {
		return *tc.Ephemeral
	}
	return true
}

// GetAcceptRoutes returns the accept routes setting or default (true)
func (tc *TailscaleConfig) GetAcceptRoutes() bool {
	if tc.AcceptRoutes != nil {
		return *tc.AcceptRoutes
	}
	return true
}

// GetConnectorEnabled returns whether Connector CRD should be created
func (tc *TailscaleConfig) GetConnectorEnabled() bool {
	if tc.OperatorConfig != nil && tc.OperatorConfig.ConnectorEnabled != nil {
		return *tc.OperatorConfig.ConnectorEnabled
	}
	return true
}

// TailscaleOAuthSecretRef helper methods

// GetClientIDKey returns the client ID key or default
func (ref *TailscaleOAuthSecretRef) GetClientIDKey() string {
	if ref.ClientIDKey != "" {
		return ref.ClientIDKey
	}
	return "client-id"
}

// GetClientSecretKey returns the client secret key or default
func (ref *TailscaleOAuthSecretRef) GetClientSecretKey() string {
	if ref.ClientSecretKey != "" {
		return ref.ClientSecretKey
	}
	return "client-secret"
}

// TalosConfig helper methods

// GetNetworkingBackend returns the networking backend being used
func (tc *TalosConfig) GetNetworkingBackend() string {
	return "tailscale"
}
