package vultr

import (
	"testing"

	"github.com/vultr/govultr/v3"
	"github.com/solanyn/tgp-operator/pkg/providers"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
	}{
		{
			name:    "valid api key",
			apiKey:  "test-key",
			wantErr: false,
		},
		{
			name:    "empty api key",
			apiKey:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client without error")
			}
		})
	}
}

func TestClient_GetProviderInfo(t *testing.T) {
	client, _ := NewClient("test-key")
	info := client.GetProviderInfo()
	
	if info.Name != ProviderName {
		t.Errorf("GetProviderInfo().Name = %v, want %v", info.Name, ProviderName)
	}

	expectedGPUs := []string{"H100", "L40S", "A100", "A40", "A16", "MI325X", "MI300X"}
	if len(info.SupportedGPUTypes) != len(expectedGPUs) {
		t.Errorf("GetProviderInfo().SupportedGPUTypes returned %d GPU types, want %d", len(info.SupportedGPUTypes), len(expectedGPUs))
	}
}

func TestClient_GetRateLimits(t *testing.T) {
	client, _ := NewClient("test-key")
	limits := client.GetRateLimits()
	
	if limits.RequestsPerSecond != 30 {
		t.Errorf("GetRateLimits().RequestsPerSecond = %v, want 30", limits.RequestsPerSecond)
	}
}

func TestClient_TranslateGPUType(t *testing.T) {
	client, _ := NewClient("test-key")

	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"H100", "H100", false},
		{"h100", "H100", false},
		{"A100", "A100", false},
		{"a100", "A100", false},
		{"L40S", "L40S", false},
		{"l40s", "L40S", false},
		{"Unknown", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := client.TranslateGPUType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("TranslateGPUType(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("TranslateGPUType(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestClient_TranslateRegion(t *testing.T) {
	client, _ := NewClient("test-key")

	result, err := client.TranslateRegion("us-east")
	if err != nil {
		t.Errorf("TranslateRegion() error = %v", err)
	}
	if result != "us-east" {
		t.Errorf("TranslateRegion() = %s, want us-east", result)
	}
}

func TestClient_extractGPUFromPlan(t *testing.T) {
	client, _ := NewClient("test-key")

	tests := []struct {
		planName     string
		expectedGPU  string
		expectedCount int
	}{
		{"GPU-H100-1", "H100", 1},
		{"GPU-A100-2", "A100", 1},
		{"GPU-L40S-4", "L40S", 1},
		{"Standard-CPU-1", "", 0},
		{"GPU-MI325X-1", "MI325X", 1},
	}

	for _, tt := range tests {
		t.Run(tt.planName, func(t *testing.T) {
			plan := &govultr.Plan{Type: tt.planName}
			gpuType, gpuCount := client.extractGPUFromPlan(plan)
			if gpuType != tt.expectedGPU {
				t.Errorf("extractGPUFromPlan(%s) GPU type = %s, want %s", tt.planName, gpuType, tt.expectedGPU)
			}
			if gpuCount != tt.expectedCount {
				t.Errorf("extractGPUFromPlan(%s) GPU count = %d, want %d", tt.planName, gpuCount, tt.expectedCount)
			}
		})
	}
}

func TestClient_mapInstanceStatus(t *testing.T) {
	client, _ := NewClient("test-key")

	tests := []struct {
		vultrStatus    string
		expectedStatus providers.InstanceState
	}{
		{"active", providers.InstanceStateRunning},
		{"running", providers.InstanceStateRunning},
		{"pending", providers.InstanceStatePending},
		{"installing", providers.InstanceStatePending},
		{"stopped", providers.InstanceStateTerminated},
		{"halted", providers.InstanceStateTerminated},
		{"resizing", providers.InstanceStatePending},
		{"unknown", providers.InstanceStateUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.vultrStatus, func(t *testing.T) {
			result := client.mapInstanceStatus(tt.vultrStatus)
			if result != tt.expectedStatus {
				t.Errorf("mapInstanceStatus(%s) = %s, want %s", tt.vultrStatus, result, tt.expectedStatus)
			}
		})
	}
}

func TestClient_calculateHourlyPrice(t *testing.T) {
	client, _ := NewClient("test-key")

	tests := []struct {
		monthlyCost float32
		expected    float64
	}{
		{730.0, 1.0},
		{365.0, 0.5},
		{146.0, 0.2},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := client.calculateHourlyPrice(tt.monthlyCost)
			if result != tt.expected {
				t.Errorf("calculateHourlyPrice(%f) = %f, want %f", tt.monthlyCost, result, tt.expected)
			}
		})
	}
}