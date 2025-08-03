package validation

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"gopkg.in/yaml.v2"
)

// TalosConfigValidator validates Talos machine configuration templates
type TalosConfigValidator struct{}

// NewTalosConfigValidator creates a new validator instance
func NewTalosConfigValidator() *TalosConfigValidator {
	return &TalosConfigValidator{}
}

// ValidateTemplate validates a Talos machine configuration template
func (v *TalosConfigValidator) ValidateTemplate(machineConfigTemplate string) error {
	// 1. Validate template syntax
	tmpl, err := template.New("talos").Parse(machineConfigTemplate)
	if err != nil {
		return fmt.Errorf("invalid template syntax: %w", err)
	}

	// 2. Render template with dummy data to check structure
	dummyData := v.createDummyTemplateData()
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, dummyData); err != nil {
		return fmt.Errorf("template rendering failed: %w", err)
	}

	renderedConfig := buf.String()

	// 3. Validate YAML structure
	var yamlCheck interface{}
	if err := yaml.Unmarshal([]byte(renderedConfig), &yamlCheck); err != nil {
		return fmt.Errorf("rendered config is not valid YAML: %w", err)
	}

	// 4. Validate basic Talos structure
	if err := v.validateBasicTalosStructure(yamlCheck); err != nil {
		return fmt.Errorf("invalid Talos machine config structure: %w", err)
	}

	// 5. Check for required template variables
	if err := v.validateRequiredVariables(machineConfigTemplate); err != nil {
		return fmt.Errorf("missing required template variables: %w", err)
	}

	return nil
}

// validateBasicTalosStructure validates basic Talos configuration structure
func (v *TalosConfigValidator) validateBasicTalosStructure(config interface{}) error {
	configMap, ok := config.(map[interface{}]interface{})
	if !ok {
		return fmt.Errorf("config must be a YAML object")
	}

	// Check for required top-level fields
	requiredFields := []string{"version", "machine", "cluster"}
	for _, field := range requiredFields {
		if _, exists := configMap[field]; !exists {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	// Validate version
	if version, exists := configMap["version"]; exists {
		if versionStr, ok := version.(string); ok {
			if !strings.HasPrefix(versionStr, "v1alpha1") {
				return fmt.Errorf("unsupported version: %s, expected v1alpha1", versionStr)
			}
		}
	}

	// Validate machine section exists and has basic structure
	if machine, exists := configMap["machine"]; exists {
		if machineMap, ok := machine.(map[interface{}]interface{}); ok {
			if _, exists := machineMap["token"]; !exists {
				return fmt.Errorf("machine section missing required 'token' field")
			}
		}
	}

	// Validate cluster section exists and has basic structure
	if cluster, exists := configMap["cluster"]; exists {
		if clusterMap, ok := cluster.(map[interface{}]interface{}); ok {
			requiredClusterFields := []string{"id", "secret", "controlPlane"}
			for _, field := range requiredClusterFields {
				if _, exists := clusterMap[field]; !exists {
					return fmt.Errorf("cluster section missing required '%s' field", field)
				}
			}
		}
	}

	return nil
}

// validateRequiredVariables checks that critical template variables are present
func (v *TalosConfigValidator) validateRequiredVariables(template string) error {
	requiredVars := []string{
		"{{.TailscaleAuthKey}}", // Only dynamic runtime variables are required
		"{{.NodeName}}",         // Node-specific variables must use templates
	}

	var missing []string
	for _, required := range requiredVars {
		found := false
		for _, variant := range v.getVariableVariants(required) {
			if contains(template, variant) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, required)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("template missing required variables: %v", missing)
	}

	return nil
}

// getVariableVariants returns different ways a template variable might be written
func (v *TalosConfigValidator) getVariableVariants(variable string) []string {
	// Handle different template syntaxes like {{.Var}}, {{ .Var }}, etc.
	base := variable[2 : len(variable)-2] // Remove {{ and }}
	return []string{
		variable,                    // {{.Var}}
		"{{ " + base + " }}",       // {{ .Var }}
		"{{" + base + "}}",         // Remove spaces if any
		"{{ " + base + "}}",        // Space before
		"{{" + base + " }}",        // Space after
	}
}

// createDummyTemplateData creates dummy data for template rendering during validation
func (v *TalosConfigValidator) createDummyTemplateData() map[string]interface{} {
	return map[string]interface{}{
		"ControlPlaneEndpoint": "https://192.168.1.120:6443",
		"MachineToken":         "dummy-machine-token",
		"ClusterCA":            "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t", // dummy base64
		"ClusterID":            "dummy-cluster-id",
		"ClusterSecret":        "dummy-cluster-secret",
		"ClusterName":          "test-cluster",
		"TailscaleAuthKey":     "tskey-dummy-auth-key",
		"NodeName":             "gpu-node-1",
		"NodePool":             "test-pool",
		"NodeIndex":            "1",
		"GPUType":              "RTX4090",
		"Provider":             "runpod",
		"Region":               "us-west",
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}