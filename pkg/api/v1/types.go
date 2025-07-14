// +kubebuilder:object:generate=true
package v1

import (
	"fmt"
	"strconv"
	
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
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

	// TTL specifies how long the node should live before automatic termination
	TTL *metav1.Duration `json:"ttl,omitempty"`

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
	PrivateKey string `json:"privateKey"`

	// PublicKey is the WireGuard public key for the node
	PublicKey string `json:"publicKey"`

	// ServerEndpoint is the WireGuard server endpoint
	ServerEndpoint string `json:"serverEndpoint"`

	// AllowedIPs specifies allowed IP ranges through the VPN
	AllowedIPs []string `json:"allowedIPs"`

	// Address is the VPN IP address for this node
	Address string `json:"address"`
}

// GPURequestStatus defines the observed state of GPURequest
type GPURequestStatus struct {
	// Phase represents the current phase of the GPU request
	Phase GPURequestPhase `json:"phase,omitempty"`

	// InstanceID is the cloud provider instance identifier
	InstanceID string `json:"instanceId,omitempty"`

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
