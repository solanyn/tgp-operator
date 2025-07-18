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

	// WireGuardConfig contains VPN configuration for cluster connectivity
	WireGuardConfig WireGuardConfig `json:"wireGuardConfig"`
}

// WireGuardConfig contains WireGuard VPN configuration
type WireGuardConfig struct {
	// PrivateKey is the WireGuard private key for the node
	// Can be specified directly or via PrivateKeySecretRef
	PrivateKey string `json:"privateKey,omitempty"`

	// PrivateKeySecretRef references a secret containing the private key
	PrivateKeySecretRef *SecretKeyRef `json:"privateKeySecretRef,omitempty"`

	// PublicKey is the WireGuard public key for the node
	// Can be specified directly or via PublicKeySecretRef
	PublicKey string `json:"publicKey,omitempty"`

	// PublicKeySecretRef references a secret containing the public key
	PublicKeySecretRef *SecretKeyRef `json:"publicKeySecretRef,omitempty"`

	// ServerEndpoint is the WireGuard server endpoint
	// Can be specified directly or via ServerEndpointSecretRef
	ServerEndpoint string `json:"serverEndpoint,omitempty"`

	// ServerEndpointSecretRef references a secret containing the server endpoint
	ServerEndpointSecretRef *SecretKeyRef `json:"serverEndpointSecretRef,omitempty"`

	// AllowedIPs specifies allowed IP ranges through the VPN
	// Can be specified directly or via AllowedIPsSecretRef
	AllowedIPs []string `json:"allowedIPs,omitempty"`

	// AllowedIPsSecretRef references a secret containing allowed IPs (comma-separated)
	AllowedIPsSecretRef *SecretKeyRef `json:"allowedIPsSecretRef,omitempty"`

	// Address is the VPN IP address for this node
	// Can be specified directly or via AddressSecretRef
	Address string `json:"address,omitempty"`

	// AddressSecretRef references a secret containing the VPN address
	AddressSecretRef *SecretKeyRef `json:"addressSecretRef,omitempty"`
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
