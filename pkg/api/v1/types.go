// Package v1 contains API Schema definitions for the tgp v1 API group
// +kubebuilder:object:generate=true
package v1

import (
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// init registers the GPURequest types with the scheme builder.
// This is required for Kubernetes controller-runtime functionality.
func init() { //nolint:gochecknoinits // Required for Kubernetes scheme registration
	SchemeBuilder.Register(&GPURequest{}, &GPURequestList{})
}

// GPURequestSpec defines the desired state of GPURequest
type GPURequestSpec struct {
	// Provider specifies which cloud provider to use
	Provider string `json:"provider"`

	// GPUType specifies the GPU model (e.g., "RTX3090", "A100")
	GPUType string `json:"gpuType"`

	// Region specifies the preferred region for provisioning
	Region string `json:"region,omitempty"`

	// MaxHourlyPrice sets the maximum price per hour willing to pay (as string, e.g., "1.50")
	MaxHourlyPrice *string `json:"maxHourlyPrice,omitempty"`

	// MaxLifetime specifies the maximum time the node can exist before forced termination
	MaxLifetime *metav1.Duration `json:"maxLifetime,omitempty"`

	// IdleTimeout specifies how long the node can be idle before termination
	IdleTimeout *metav1.Duration `json:"idleTimeout,omitempty"`

	// Spot indicates whether to use spot/preemptible instances
	Spot bool `json:"spot,omitempty"`

	// TalosConfig contains Talos-specific configuration
	TalosConfig TalosConfig `json:"talosConfig"`
}

// TalosConfig contains Talos node configuration
type TalosConfig struct {
	// Image specifies the Talos image to use
	Image string `json:"image"`

	// TailscaleConfig contains Tailscale mesh networking configuration
	TailscaleConfig TailscaleConfig `json:"tailscaleConfig"`
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

	// Namespace is the namespace of the secret (optional, defaults to GPURequest namespace)
	Namespace string `json:"namespace,omitempty"`
}

// TailscaleOAuthSecretRef references a secret containing Tailscale OAuth credentials
type TailscaleOAuthSecretRef struct {
	// Name is the name of the secret containing OAuth credentials
	Name string `json:"name"`

	// Namespace is the namespace of the secret (optional, defaults to GPURequest namespace)
	Namespace string `json:"namespace,omitempty"`

	// ClientIDKey is the key containing the OAuth client ID (defaults to "client-id")
	// +optional
	ClientIDKey string `json:"clientIdKey,omitempty"`

	// ClientSecretKey is the key containing the OAuth client secret (defaults to "client-secret")
	// +optional
	ClientSecretKey string `json:"clientSecretKey,omitempty"`
}

// GPURequestStatus defines the observed state of GPURequest
type GPURequestStatus struct {
	// Phase represents the current phase of the GPU request
	Phase GPURequestPhase `json:"phase,omitempty"`

	// InstanceID is the cloud provider instance identifier
	InstanceID string `json:"instanceId,omitempty"`

	// SelectedProvider is the cloud provider that was chosen for this request
	SelectedProvider string `json:"selectedProvider,omitempty"`

	// PublicIP is the public IP address of the provisioned instance
	PublicIP string `json:"publicIp,omitempty"`

	// PrivateIP is the private IP address of the provisioned instance
	PrivateIP string `json:"privateIp,omitempty"`

	// NodeName is the Kubernetes node name after joining the cluster
	NodeName string `json:"nodeName,omitempty"`

	// HourlyPrice is the actual hourly price of the provisioned instance (as string, e.g., "1.50")
	HourlyPrice string `json:"hourlyPrice,omitempty"`

	// ProvisionedAt is the timestamp when the instance was provisioned
	ProvisionedAt *metav1.Time `json:"provisionedAt,omitempty"`

	// TerminationScheduledAt is the calculated termination time based on maxLifetime
	TerminationScheduledAt *metav1.Time `json:"terminationScheduledAt,omitempty"`

	// LastHeartbeat is the last time the node was seen healthy
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Message provides additional information about the current state
	Message string `json:"message,omitempty"`
}

// GPURequestPhase represents the phase of a GPU request
type GPURequestPhase string

const (
	// GPURequestPhasePending indicates the request is waiting to be processed
	GPURequestPhasePending GPURequestPhase = "Pending"

	// GPURequestPhaseProvisioning indicates the instance is being provisioned
	GPURequestPhaseProvisioning GPURequestPhase = "Provisioning"

	// GPURequestPhaseBooting indicates the instance is booting
	GPURequestPhaseBooting GPURequestPhase = "Booting"

	// GPURequestPhaseJoining indicates the node is joining the cluster
	GPURequestPhaseJoining GPURequestPhase = "Joining"

	// GPURequestPhaseReady indicates the node is ready and available
	GPURequestPhaseReady GPURequestPhase = "Ready"

	// GPURequestPhaseTerminating indicates the node is being terminated
	GPURequestPhaseTerminating GPURequestPhase = "Terminating"

	// GPURequestPhaseFailed indicates the request failed
	GPURequestPhaseFailed GPURequestPhase = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Provider",type=string,JSONPath=`.spec.provider`
// +kubebuilder:printcolumn:name="GPU Type",type=string,JSONPath=`.spec.gpuType`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Instance ID",type=string,JSONPath=`.status.instanceId`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GPURequest is the Schema for the gpurequests API
type GPURequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GPURequestSpec   `json:"spec,omitempty"`
	Status GPURequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GPURequestList contains a list of GPURequest
type GPURequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GPURequest `json:"items"`
}

// Helper methods for price conversion

// GetMaxHourlyPriceFloat converts the string MaxHourlyPrice to float64
func (spec *GPURequestSpec) GetMaxHourlyPriceFloat() (float64, error) {
	if spec.MaxHourlyPrice == nil {
		return 0, nil
	}
	return strconv.ParseFloat(*spec.MaxHourlyPrice, 64)
}

// SetMaxHourlyPriceFloat sets the MaxHourlyPrice from a float64 value
func (spec *GPURequestSpec) SetMaxHourlyPriceFloat(price float64) {
	priceStr := fmt.Sprintf("%.2f", price)
	spec.MaxHourlyPrice = &priceStr
}

// GetHourlyPriceFloat converts the string HourlyPrice to float64
func (status *GPURequestStatus) GetHourlyPriceFloat() (float64, error) {
	if status.HourlyPrice == "" {
		return 0, nil
	}
	return strconv.ParseFloat(status.HourlyPrice, 64)
}

// SetHourlyPriceFloat sets the HourlyPrice from a float64 value
func (status *GPURequestStatus) SetHourlyPriceFloat(price float64) {
	status.HourlyPrice = fmt.Sprintf("%.2f", price)
}

// UpdateTerminationTime recalculates the termination time based on current maxLifetime
func (req *GPURequest) UpdateTerminationTime() {
	if req.Spec.MaxLifetime != nil && req.Status.ProvisionedAt != nil {
		duration := req.Spec.MaxLifetime.Duration
		terminationTime := req.Status.ProvisionedAt.Add(duration)
		req.Status.TerminationScheduledAt = &metav1.Time{Time: terminationTime}
	} else {
		req.Status.TerminationScheduledAt = nil
	}
}

// IsTerminationDue checks if the node should be terminated based on maxLifetime
func (req *GPURequest) IsTerminationDue() bool {
	if req.Status.TerminationScheduledAt == nil {
		return false
	}
	return metav1.Now().After(req.Status.TerminationScheduledAt.Time)
}

// TimeUntilTermination returns the duration until scheduled termination
func (req *GPURequest) TimeUntilTermination() *metav1.Duration {
	if req.Status.TerminationScheduledAt == nil {
		return nil
	}
	remaining := req.Status.TerminationScheduledAt.Time.Sub(metav1.Now().Time)
	if remaining <= 0 {
		return &metav1.Duration{Duration: 0}
	}
	return &metav1.Duration{Duration: remaining}
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
