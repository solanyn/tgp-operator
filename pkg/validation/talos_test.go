package validation

import (
	"testing"
)

func TestTalosConfigValidator_ValidateTemplate(t *testing.T) {
	validator := NewTalosConfigValidator()

	tests := []struct {
		name     string
		template string
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid minimal template",
			template: `
version: v1alpha1
debug: false
persist: true
machine:
  token: {{.MachineToken}}
  ca:
    crt: {{.ClusterCA}}
  certSANs:
    - 127.0.0.1
  files:
    - path: /etc/tailscale/auth
      content: {{.TailscaleAuthKey}}
cluster:
  id: {{.ClusterID}}
  secret: {{.ClusterSecret}}
  controlPlane:
    endpoint: {{.ControlPlaneEndpoint}}
  clusterName: {{.ClusterName}}
`,
			wantErr: false,
		},
		{
			name:     "invalid template syntax",
			template: `{{.MissingCloseBrace}`,
			wantErr:  true,
			errMsg:   "invalid template syntax",
		},
		{
			name: "missing required variables",
			template: `
version: v1alpha1
machine:
  token: hardcoded-token
cluster:
  id: hardcoded-id
  secret: hardcoded-secret
  controlPlane:
    endpoint: https://hardcoded:6443
  clusterName: hardcoded
`,
			wantErr: true,
			errMsg:  "missing required template variables",
		},
		{
			name: "invalid YAML after rendering",
			template: `
version: v1alpha1
machine:
  token: {{.MachineToken}}
  invalid_yaml: [unclosed array
cluster:
  id: {{.ClusterID}}
  secret: {{.ClusterSecret}}
  controlPlane:
    endpoint: {{.ControlPlaneEndpoint}}
  clusterName: {{.ClusterName}}
`,
			wantErr: true,
			errMsg:  "not valid YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateTemplate(tt.template)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateTemplate() expected error but got none")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateTemplate() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateTemplate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestTalosConfigValidator_ValidateRequiredVariables(t *testing.T) {
	validator := NewTalosConfigValidator()

	tests := []struct {
		name     string
		template string
		wantErr  bool
	}{
		{
			name: "all required variables present",
			template: `
machine:
  token: {{.MachineToken}}
  ca:
    crt: {{.ClusterCA}}
cluster:
  id: {{.ClusterID}}
  secret: {{.ClusterSecret}}
  controlPlane:
    endpoint: {{.ControlPlaneEndpoint}}
  clusterName: {{.ClusterName}}
files:
  - path: /etc/tailscale/auth
    content: {{.TailscaleAuthKey}}
`,
			wantErr: false,
		},
		{
			name: "missing MachineToken",
			template: `
cluster:
  id: {{.ClusterID}}
  secret: {{.ClusterSecret}}
  controlPlane:
    endpoint: {{.ControlPlaneEndpoint}}
  clusterName: {{.ClusterName}}
files:
  - path: /etc/tailscale/auth
    content: {{.TailscaleAuthKey}}
`,
			wantErr: true,
		},
		{
			name: "variables with different spacing",
			template: `
machine:
  token: {{ .MachineToken }}
  ca:
    crt: {{.ClusterCA}}
cluster:
  id: {{ .ClusterID}}
  secret: {{.ClusterSecret }}
  controlPlane:
    endpoint: {{ .ControlPlaneEndpoint }}
  clusterName: {{.ClusterName}}
files:
  - path: /etc/tailscale/auth
    content: {{.TailscaleAuthKey}}
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateRequiredVariables(tt.template)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRequiredVariables() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTalosConfigValidator_CreateDummyTemplateData(t *testing.T) {
	validator := NewTalosConfigValidator()
	data := validator.createDummyTemplateData()

	requiredKeys := []string{
		"ControlPlaneEndpoint",
		"MachineToken", 
		"ClusterCA",
		"ClusterID",
		"ClusterSecret",
		"ClusterName",
		"TailscaleAuthKey",
		"NodeName",
		"NodePool",
		"NodeIndex",
	}

	for _, key := range requiredKeys {
		if _, exists := data[key]; !exists {
			t.Errorf("createDummyTemplateData() missing required key: %s", key)
		}
	}

	// Check that values are non-empty strings
	for key, value := range data {
		if str, ok := value.(string); !ok || str == "" {
			t.Errorf("createDummyTemplateData() key %s has empty or non-string value: %v", key, value)
		}
	}
}