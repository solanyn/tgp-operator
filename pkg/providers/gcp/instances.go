package gcp

import (
	"fmt"
	"strings"
	"time"

	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/solanyn/tgp-operator/pkg/providers"
	"google.golang.org/protobuf/proto"
)

// buildLabels creates labels for the instance
func (c *Client) buildLabels(req *providers.LaunchRequest) map[string]string {
	labels := map[string]string{
		"tgp-operator": "true",
		"gpu-type":     strings.ToLower(strings.ReplaceAll(req.GPUType, "_", "-")),
		"managed-by":   "tgp-operator",
	}

	// Add custom labels from request
	for k, v := range req.Labels {
		// GCP labels must be lowercase
		key := strings.ToLower(strings.ReplaceAll(k, "_", "-"))
		value := strings.ToLower(strings.ReplaceAll(v, "_", "-"))
		labels[key] = value
	}

	return labels
}

// buildMetadata creates metadata for the instance (user data)
func (c *Client) buildMetadata(req *providers.LaunchRequest) *computepb.Metadata {
	items := []*computepb.Items{
		{
			Key:   proto.String("user-data"),
			Value: proto.String(req.UserData),
		},
		{
			Key:   proto.String("tgp-nodepool"),
			Value: proto.String(req.Labels["nodepool"]),
		},
		{
			Key:   proto.String("tgp-gpu-type"),
			Value: proto.String(req.GPUType),
		},
	}

	return &computepb.Metadata{
		Items: items,
	}
}

// buildDiskConfig creates the disk configuration
func (c *Client) buildDiskConfig() []*computepb.AttachedDisk {
	return []*computepb.AttachedDisk{
		{
			Boot:       proto.Bool(true),
			AutoDelete: proto.Bool(true),
			InitializeParams: &computepb.AttachedDiskInitializeParams{
				DiskSizeGb:  proto.Int64(50),        // 50GB boot disk
				DiskType:    proto.String("pd-ssd"), // SSD for better performance
				SourceImage: proto.String(c.getTalosImageURL()),
			},
		},
	}
}

// buildNetworkConfig creates the network configuration
func (c *Client) buildNetworkConfig() []*computepb.NetworkInterface {
	return []*computepb.NetworkInterface{
		{
			Network: proto.String("global/networks/default"),
			AccessConfigs: []*computepb.AccessConfig{
				{
					Type: proto.String("ONE_TO_ONE_NAT"),
					Name: proto.String("External NAT"),
				},
			},
		},
	}
}

// buildServiceAccountConfig creates service account configuration
func (c *Client) buildServiceAccountConfig() []*computepb.ServiceAccount {
	return []*computepb.ServiceAccount{
		{
			Email: proto.String("default"),
			Scopes: []string{
				"https://www.googleapis.com/auth/devstorage.read_only",
				"https://www.googleapis.com/auth/logging.write",
				"https://www.googleapis.com/auth/monitoring.write",
			},
		},
	}
}

// buildGPUConfig creates GPU configuration
func (c *Client) buildGPUConfig(gpuType string, count int32) []*computepb.AcceleratorConfig {
	if gpuType == "" {
		return nil
	}

	// Translate standard GPU type to GCP accelerator type
	gcpGPUType := c.translateGPUTypeToGCP(gpuType)

	return []*computepb.AcceleratorConfig{
		{
			AcceleratorType:  proto.String(gcpGPUType),
			AcceleratorCount: proto.Int32(count),
		},
	}
}

// translateGPUTypeToGCP converts standard GPU types to GCP accelerator types
func (c *Client) translateGPUTypeToGCP(standardType string) string {
	translations := map[string]string{
		"K80":              "nvidia-tesla-k80",
		"P4":               "nvidia-tesla-p4",
		"P100":             "nvidia-tesla-p100",
		"V100":             "nvidia-tesla-v100",
		"T4":               "nvidia-tesla-t4",
		"A100":             "nvidia-tesla-a100",
		"A100-80GB":        "nvidia-a100-80gb",
		"H100":             "nvidia-h100-80gb",
		"L4":               "nvidia-l4",
		"NVIDIA_K80":       "nvidia-tesla-k80",
		"NVIDIA_P4":        "nvidia-tesla-p4",
		"NVIDIA_P100":      "nvidia-tesla-p100",
		"NVIDIA_V100":      "nvidia-tesla-v100",
		"NVIDIA_T4":        "nvidia-tesla-t4",
		"NVIDIA_A100":      "nvidia-tesla-a100",
		"NVIDIA_A100_80GB": "nvidia-a100-80gb",
		"NVIDIA_H100_80GB": "nvidia-h100-80gb",
		"NVIDIA_L4":        "nvidia-l4",
		// Add more mappings as needed
	}

	if gcpType, exists := translations[standardType]; exists {
		return gcpType
	}

	// Fallback: convert to lowercase with nvidia- prefix
	return "nvidia-" + strings.ToLower(strings.ReplaceAll(standardType, "_", "-"))
}

// getRecommendedMachineTypeForGPU returns appropriate machine type for GPU
func (c *Client) getRecommendedMachineTypeForGPU(gpuType string) string {
	// Map GPU types to appropriate machine types
	machineTypeMap := map[string]string{
		"K80":              "n1-standard-4",
		"P4":               "n1-standard-4",
		"P100":             "n1-standard-8",
		"V100":             "n1-standard-8",
		"T4":               "n1-standard-4",
		"A100":             "a2-highgpu-1g",
		"A100-80GB":        "a2-ultragpu-1g",
		"H100":             "a3-highgpu-8g",
		"NVIDIA_K80":       "n1-standard-4",
		"NVIDIA_P4":        "n1-standard-4",
		"NVIDIA_P100":      "n1-standard-8",
		"NVIDIA_V100":      "n1-standard-8",
		"NVIDIA_T4":        "n1-standard-4",
		"NVIDIA_A100":      "a2-highgpu-1g",
		"NVIDIA_A100_80GB": "a2-ultragpu-1g",
		"NVIDIA_H100_80GB": "a3-highgpu-8g",
		"L4":               "g2-standard-4",
		"NVIDIA_L4":        "g2-standard-4",
	}

	if machineType, exists := machineTypeMap[gpuType]; exists {
		return machineType
	}

	// Default fallback
	return "n1-standard-4"
}

// getMachineTypeURL builds the full machine type URL
func (c *Client) getMachineTypeURL(machineType, zone string) string {
	return fmt.Sprintf("projects/%s/zones/%s/machineTypes/%s", c.projectID, zone, machineType)
}

// getTalosImageURL returns the URL for the Talos Linux image
func (c *Client) getTalosImageURL() string {
	// Return a standard Talos image name that users should upload to their project
	return fmt.Sprintf("projects/%s/global/images/talos-linux-latest", c.projectID)
}

// instanceToGPUInstance converts a GCP instance to our GPUInstance format
func (c *Client) instanceToGPUInstance(instance *computepb.Instance, zone string) *providers.GPUInstance {
	instanceID := fmt.Sprintf("%s/%s", zone, instance.GetName())

	return &providers.GPUInstance{
		ID:        instanceID,
		PublicIP:  c.extractPublicIP(instance),
		PrivateIP: c.extractPrivateIP(instance),
		Status:    c.translateInstanceState(instance.GetStatus()),
		CreatedAt: c.extractLaunchTime(instance),
	}
}

// translateInstanceState converts GCP instance status to our standard states
func (c *Client) translateInstanceState(status string) providers.InstanceState {
	switch status {
	case "PROVISIONING", "STAGING":
		return providers.InstanceStatePending
	case "RUNNING":
		return providers.InstanceStateRunning
	case "STOPPING":
		return providers.InstanceStateTerminating
	case "STOPPED", "TERMINATED":
		return providers.InstanceStateTerminated
	case "SUSPENDING":
		return providers.InstanceStateTerminating
	case "SUSPENDED":
		return providers.InstanceStateTerminated
	default:
		return providers.InstanceStateUnknown
	}
}

// extractPublicIP gets the public IP from instance network interfaces
func (c *Client) extractPublicIP(instance *computepb.Instance) string {
	for _, nic := range instance.GetNetworkInterfaces() {
		for _, accessConfig := range nic.GetAccessConfigs() {
			if accessConfig.GetNatIP() != "" {
				return accessConfig.GetNatIP()
			}
		}
	}
	return ""
}

// extractPrivateIP gets the private IP from instance network interfaces
func (c *Client) extractPrivateIP(instance *computepb.Instance) string {
	if len(instance.GetNetworkInterfaces()) > 0 {
		return instance.GetNetworkInterfaces()[0].GetNetworkIP()
	}
	return ""
}

// extractGPUTypeFromInstance gets GPU type from instance configuration
func (c *Client) extractGPUTypeFromInstance(instance *computepb.Instance) string {
	for _, accelerator := range instance.GetGuestAccelerators() {
		gcpType := accelerator.GetAcceleratorType()
		// Extract just the GPU type from the full URL
		parts := strings.Split(gcpType, "/")
		if len(parts) > 0 {
			return c.translateGCPTypeToStandard(parts[len(parts)-1])
		}
	}
	return ""
}

// translateGCPTypeToStandard converts GCP accelerator types back to standard types
func (c *Client) translateGCPTypeToStandard(gcpType string) string {
	translations := map[string]string{
		"nvidia-tesla-k80":  "NVIDIA_K80",
		"nvidia-tesla-p4":   "NVIDIA_P4",
		"nvidia-tesla-p100": "NVIDIA_P100",
		"nvidia-tesla-v100": "NVIDIA_V100",
		"nvidia-tesla-t4":   "NVIDIA_T4",
		"nvidia-tesla-a100": "NVIDIA_A100",
		"nvidia-a100-80gb":  "NVIDIA_A100_80GB",
		"nvidia-h100-80gb":  "NVIDIA_H100_80GB",
		"nvidia-l4":         "NVIDIA_L4",
	}

	if standardType, exists := translations[gcpType]; exists {
		return standardType
	}

	// Fallback: remove nvidia- prefix and add NVIDIA_ prefix
	if strings.HasPrefix(gcpType, "nvidia-") {
		cleaned := strings.TrimPrefix(gcpType, "nvidia-")
		cleaned = strings.ToUpper(strings.ReplaceAll(cleaned, "-", "_"))
		return "NVIDIA_" + cleaned
	}

	return "NVIDIA_" + strings.ToUpper(strings.ReplaceAll(gcpType, "-", "_"))
}

// extractLaunchTime gets the instance creation time
func (c *Client) extractLaunchTime(instance *computepb.Instance) time.Time {
	creationTimestamp := instance.GetCreationTimestamp()
	if creationTimestamp == "" {
		return time.Now()
	}

	// Parse RFC3339 timestamp
	if t, err := time.Parse(time.RFC3339, creationTimestamp); err == nil {
		return t
	}

	return time.Now()
}

// isSpotInstance checks if instance is preemptible (spot)
func (c *Client) isSpotInstance(instance *computepb.Instance) bool {
	if instance.GetScheduling() != nil {
		return instance.GetScheduling().GetPreemptible()
	}
	return false
}

// parseInstanceID splits instanceID into zone and instance name
func (c *Client) parseInstanceID(instanceID string) (zone, instanceName string) {
	parts := strings.Split(instanceID, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	// Fallback - assume it's just the instance name in us-central1-a
	return "us-central1-a", instanceID
}

// zoneToRegion converts zone name to region name
func (c *Client) zoneToRegion(zone string) string {
	// Remove the last part (e.g., us-central1-a -> us-central1)
	parts := strings.Split(zone, "-")
	if len(parts) >= 3 {
		return strings.Join(parts[:len(parts)-1], "-")
	}
	return zone
}
