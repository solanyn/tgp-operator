package gcp

import (
	"testing"

	"github.com/solanyn/tgp-operator/pkg/providers"
)

func TestNewClient(t *testing.T) {
	credentialsJSON := `{
		"type": "service_account",
		"project_id": "test-project",
		"private_key_id": "test-key-id",
		"private_key": "-----BEGIN PRIVATE KEY-----\ntest-private-key\n-----END PRIVATE KEY-----\n",
		"client_email": "test@test-project.iam.gserviceaccount.com",
		"client_id": "123456789",
		"auth_uri": "https://accounts.google.com/o/oauth2/auth",
		"token_uri": "https://oauth2.googleapis.com/token"
	}`

	client := NewClient(credentialsJSON)
	if client == nil {
		t.Fatal("Expected client to be created")
	}

	if client.credentials != credentialsJSON {
		t.Error("Credentials not set correctly")
	}
}

func TestGetProviderInfo(t *testing.T) {
	client := NewClient("{}")
	info := client.GetProviderInfo()

	if info.Name != "gcp" {
		t.Errorf("Expected provider name 'gcp', got: %s", info.Name)
	}

	if info.APIVersion != "v1" {
		t.Errorf("Expected API version 'v1', got: %s", info.APIVersion)
	}

	if len(info.SupportedRegions) == 0 {
		t.Error("Expected supported regions to be populated")
	}

	if len(info.SupportedGPUTypes) == 0 {
		t.Error("Expected supported GPU types to be populated")
	}

	if !info.SupportsSpotInstances {
		t.Error("Expected GCP to support spot instances")
	}

	if info.BillingGranularity != "per-minute" {
		t.Errorf("Expected billing granularity 'per-minute', got: %s", info.BillingGranularity)
	}
}

func TestGetRateLimits(t *testing.T) {
	client := NewClient("{}")
	rateLimits := client.GetRateLimits()

	if rateLimits.RequestsPerSecond <= 0 {
		t.Error("Expected requests per second to be greater than 0")
	}

	if rateLimits.RequestsPerMinute <= 0 {
		t.Error("Expected requests per minute to be greater than 0")
	}

	if rateLimits.BurstCapacity <= 0 {
		t.Error("Expected burst capacity to be greater than 0")
	}
}

func TestTranslateGPUTypeToGCP(t *testing.T) {
	client := NewClient("{}")

	tests := []struct {
		input    string
		expected string
	}{
		{"K80", "nvidia-tesla-k80"},
		{"P100", "nvidia-tesla-p100"},
		{"V100", "nvidia-tesla-v100"},
		{"T4", "nvidia-tesla-t4"},
		{"A100", "nvidia-tesla-a100"},
		{"A100-80GB", "nvidia-a100-80gb"},
		{"H100", "nvidia-h100-80gb"},
		{"L4", "nvidia-l4"},
		{"NVIDIA_K80", "nvidia-tesla-k80"},
		{"NVIDIA_A100", "nvidia-tesla-a100"},
		{"NVIDIA_A100_80GB", "nvidia-a100-80gb"},
		{"UNKNOWN_GPU", "nvidia-unknown-gpu"}, // fallback case
	}

	for _, test := range tests {
		result := client.translateGPUTypeToGCP(test.input)
		if result != test.expected {
			t.Errorf("translateGPUTypeToGCP(%s): expected %s, got %s", test.input, test.expected, result)
		}
	}
}

func TestTranslateGCPTypeToStandard(t *testing.T) {
	client := NewClient("{}")

	tests := []struct {
		input    string
		expected string
	}{
		{"nvidia-tesla-k80", "NVIDIA_K80"},
		{"nvidia-tesla-p100", "NVIDIA_P100"},
		{"nvidia-tesla-v100", "NVIDIA_V100"},
		{"nvidia-tesla-t4", "NVIDIA_T4"},
		{"nvidia-tesla-a100", "NVIDIA_A100"},
		{"nvidia-a100-80gb", "NVIDIA_A100_80GB"},
		{"nvidia-h100-80gb", "NVIDIA_H100_80GB"},
		{"nvidia-l4", "NVIDIA_L4"},
		{"unknown-gpu", "NVIDIA_UNKNOWN_GPU"}, // fallback case
	}

	for _, test := range tests {
		result := client.translateGCPTypeToStandard(test.input)
		if result != test.expected {
			t.Errorf("translateGCPTypeToStandard(%s): expected %s, got %s", test.input, test.expected, result)
		}
	}
}

func TestGetRecommendedMachineTypeForGPU(t *testing.T) {
	client := NewClient("{}")

	tests := []struct {
		gpuType     string
		machineType string
	}{
		{"K80", "n1-standard-4"},
		{"P100", "n1-standard-8"},
		{"V100", "n1-standard-8"},
		{"T4", "n1-standard-4"},
		{"A100", "a2-highgpu-1g"},
		{"A100-80GB", "a2-ultragpu-1g"},
		{"H100", "a3-highgpu-8g"},
		{"L4", "g2-standard-4"},
		{"UNKNOWN", "n1-standard-4"}, // fallback
	}

	for _, test := range tests {
		result := client.getRecommendedMachineTypeForGPU(test.gpuType)
		if result != test.machineType {
			t.Errorf("getRecommendedMachineTypeForGPU(%s): expected %s, got %s", test.gpuType, test.machineType, result)
		}
	}
}

func TestZoneToRegion(t *testing.T) {
	client := NewClient("{}")

	tests := []struct {
		zone   string
		region string
	}{
		{"us-central1-a", "us-central1"},
		{"us-east1-b", "us-east1"},
		{"europe-west1-c", "europe-west1"},
		{"asia-east1-a", "asia-east1"},
		{"invalid-zone", "invalid-zone"}, // fallback
	}

	for _, test := range tests {
		result := client.zoneToRegion(test.zone)
		if result != test.region {
			t.Errorf("zoneToRegion(%s): expected %s, got %s", test.zone, test.region, result)
		}
	}
}

func TestParseInstanceID(t *testing.T) {
	client := NewClient("{}")

	tests := []struct {
		instanceID   string
		expectedZone string
		expectedName string
	}{
		{"us-central1-a/test-instance", "us-central1-a", "test-instance"},
		{"europe-west1-b/my-gpu-node", "europe-west1-b", "my-gpu-node"},
		{"just-instance-name", "us-central1-a", "just-instance-name"}, // fallback
	}

	for _, test := range tests {
		zone, name := client.parseInstanceID(test.instanceID)
		if zone != test.expectedZone || name != test.expectedName {
			t.Errorf("parseInstanceID(%s): expected (%s, %s), got (%s, %s)",
				test.instanceID, test.expectedZone, test.expectedName, zone, name)
		}
	}
}

func TestGetGPUMemory(t *testing.T) {
	client := NewClient("{}")

	tests := []struct {
		gpuType string
		memory  int64
	}{
		{"K80", 12},
		{"P100", 16},
		{"V100", 16},
		{"T4", 16},
		{"A100", 40},
		{"A100-80GB", 80},
		{"H100", 80},
		{"L4", 24},
		{"UNKNOWN", 16}, // fallback
	}

	for _, test := range tests {
		result := client.getGPUMemory(test.gpuType)
		if result != test.memory {
			t.Errorf("getGPUMemory(%s): expected %d, got %d", test.gpuType, test.memory, result)
		}
	}
}

func TestTranslateInstanceState(t *testing.T) {
	client := NewClient("{}")

	tests := []struct {
		gcpState      string
		expectedState providers.InstanceState
	}{
		{"PROVISIONING", providers.InstanceStatePending},
		{"STAGING", providers.InstanceStatePending},
		{"RUNNING", providers.InstanceStateRunning},
		{"STOPPING", providers.InstanceStateTerminating},
		{"STOPPED", providers.InstanceStateTerminated},
		{"TERMINATED", providers.InstanceStateTerminated},
		{"SUSPENDING", providers.InstanceStateTerminating},
		{"SUSPENDED", providers.InstanceStateTerminated},
		{"UNKNOWN_STATE", providers.InstanceStateUnknown},
	}

	for _, test := range tests {
		result := client.translateInstanceState(test.gcpState)
		if result != test.expectedState {
			t.Errorf("translateInstanceState(%s): expected %v, got %v", test.gcpState, test.expectedState, result)
		}
	}
}

func TestGetRegionsToSearch(t *testing.T) {
	client := NewClient("{}")

	tests := []struct {
		filter   string
		minCount int // minimum expected regions
	}{
		{"", 10},           // all regions
		{"us", 7},          // US regions
		{"europe", 5},      // Europe regions
		{"asia", 2},        // Asia regions
		{"us-central1", 1}, // specific region
		{"nonexistent", 1}, // fallback
	}

	for _, test := range tests {
		result := client.getRegionsToSearch(test.filter)
		if len(result) < test.minCount {
			t.Errorf("getRegionsToSearch(%s): expected at least %d regions, got %d", test.filter, test.minCount, len(result))
		}
	}
}

func TestGetZonesForRegion(t *testing.T) {
	client := NewClient("{}")

	tests := []struct {
		region   string
		minZones int
	}{
		{"us-central1", 3},
		{"us-east1", 3},
		{"europe-west1", 3},
		{"unknown-region", 3}, // fallback should generate zones
	}

	for _, test := range tests {
		result := client.getZonesForRegion(test.region)
		if len(result) < test.minZones {
			t.Errorf("getZonesForRegion(%s): expected at least %d zones, got %d", test.region, test.minZones, len(result))
		}
	}
}

func TestGetMachinePricing(t *testing.T) {
	client := NewClient("{}")

	// Test that pricing returns positive values
	machineTypes := []string{
		"n1-standard-4",
		"n1-standard-8",
		"a2-highgpu-1g",
		"g2-standard-4",
		"unknown-machine-type", // fallback
	}

	for _, machineType := range machineTypes {
		price := client.getMachinePricing(machineType, "us-central1")
		if price <= 0 {
			t.Errorf("getMachinePricing(%s): expected positive price, got %f", machineType, price)
		}
	}
}

func TestGetGPUPricing(t *testing.T) {
	client := NewClient("{}")

	// Test that GPU pricing returns positive values
	gpuTypes := []string{
		"K80",
		"P100",
		"V100",
		"T4",
		"A100",
		"H100",
		"L4",
		"NVIDIA_K80",
		"NVIDIA_A100",
		"NVIDIA_H100_80GB",
		"UNKNOWN", // fallback
	}

	for _, gpuType := range gpuTypes {
		price := client.getGPUPricing(gpuType, "us-central1")
		if price <= 0 {
			t.Errorf("getGPUPricing(%s): expected positive price, got %f", gpuType, price)
		}
	}
}

// Mock tests that don't require actual GCP API calls
func TestGenerateInstanceName(t *testing.T) {
	client := NewClient("{}")

	req := &providers.LaunchRequest{
		Labels: map[string]string{
			"nodepool": "test-pool",
		},
	}

	name := client.generateInstanceName(req)

	if len(name) == 0 {
		t.Error("Expected non-empty instance name")
	}

	if len(name) > 63 {
		t.Errorf("Instance name too long: %d characters (max 63)", len(name))
	}

	// Should start with tgp- and contain nodepool name
	if name[:4] != "tgp-" {
		t.Errorf("Expected instance name to start with 'tgp-', got: %s", name)
	}
}
