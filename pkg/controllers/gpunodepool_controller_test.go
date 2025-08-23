package controllers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/config"
	"github.com/solanyn/tgp-operator/pkg/imagefactory"
)

func TestBuildUserDataScript(t *testing.T) {
	tests := []struct {
		name        string
		nodePool    *tgpv1.GPUNodePool
		nodeClass   *tgpv1.GPUNodeClass
		config      *config.OperatorConfig
		expectError bool
		validate    func(t *testing.T, result string)
	}{
		{
			name: "default template with cluster credentials",
			nodePool: &tgpv1.GPUNodePool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pool"},
				Spec: tgpv1.GPUNodePoolSpec{
					Template: tgpv1.NodePoolTemplate{
						Metadata: &tgpv1.NodeMetadata{
							Labels: map[string]string{"gpu-tier": "high-end"},
						},
						Spec: tgpv1.NodeSpec{
							Taints: []corev1.Taint{
								{Key: "gpu-node", Value: "true", Effect: corev1.TaintEffectNoSchedule},
							},
						},
					},
				},
			},
			nodeClass: &tgpv1.GPUNodeClass{
				Spec: tgpv1.GPUNodeClassSpec{
					TalosConfig: &tgpv1.TalosConfig{
						Image: "ghcr.io/siderolabs/talos:v1.10.5",
					},
				},
			},
			config: &config.OperatorConfig{
				Talos: config.TalosDefaults{
					Version: "v1.11.0-beta.1",
					Extensions: []string{
						"siderolabs/nvidia-container-toolkit-production",
						"siderolabs/tailscale",
					},
				},
			},
			validate: func(t *testing.T, result string) {
				// Since template variables are now user-provided, verify basic template generation
				if !contains(result, "{{.MachineToken}}") {
					t.Error("machine token template variable not found")
				}
				if !contains(result, "{{.ClusterCA}}") {
					t.Error("cluster CA template variable not found")
				}
				if !contains(result, "{{.ControlPlaneEndpoint}}") {
					t.Error("control plane endpoint template variable not found")
				}
				// Verify node-specific configuration is still substituted
				if !contains(result, "gpu-tier=high-end") {
					t.Error("node labels not included")
				}
				if !contains(result, "gpu-node") {
					t.Error("node taints not included")
				}
			},
		},
		{
			name: "custom template overrides default",
			nodePool: &tgpv1.GPUNodePool{
				ObjectMeta: metav1.ObjectMeta{Name: "custom-pool"},
			},
			nodeClass: &tgpv1.GPUNodeClass{
				Spec: tgpv1.GPUNodeClassSpec{
					TalosConfig: &tgpv1.TalosConfig{
						MachineConfigSecretRef: &tgpv1.SecretKeyRef{
							Name:      "custom-talos-config",
							Key:       "machine-config",
							Namespace: "default",
						},
					},
				},
			},
			config: &config.OperatorConfig{
				Talos: config.TalosDefaults{
					Version: "v1.11.0-beta.1",
					Extensions: []string{"siderolabs/nvidia-container-toolkit-production"},
				},
			},
			validate: func(t *testing.T, result string) {
				if !contains(result, "{{.MachineToken}}") {
					t.Error("custom template token variable not found")
				}
				if !contains(result, "# Custom template for custom-pool") {
					t.Error("custom template not used")
				}
				// Should NOT contain default template markers
				if contains(result, "TGP node setup complete") {
					t.Error("default template was used instead of custom")
				}
			},
		},
		{
			name: "malformed template returns error",
			nodePool: &tgpv1.GPUNodePool{
				ObjectMeta: metav1.ObjectMeta{Name: "error-pool"},
			},
			nodeClass: &tgpv1.GPUNodeClass{
				Spec: tgpv1.GPUNodeClassSpec{
					TalosConfig: &tgpv1.TalosConfig{
						MachineConfigSecretRef: &tgpv1.SecretKeyRef{
							Name:      "invalid-talos-config",
							Key:       "machine-config",
							Namespace: "default",
						},
					},
				},
			},
			config:      &config.OperatorConfig{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = tgpv1.AddToScheme(scheme)
			_ = corev1.AddToScheme(scheme)

			// Create test secrets
			objects := []client.Object{}
			if tt.nodeClass.Spec.TalosConfig != nil && tt.nodeClass.Spec.TalosConfig.MachineConfigSecretRef != nil {
				secretRef := tt.nodeClass.Spec.TalosConfig.MachineConfigSecretRef
				var templateContent string
				
				switch secretRef.Name {
				case "custom-talos-config":
					templateContent = `version: v1alpha1
machine:
  type: worker
  token: {{.MachineToken}}
  # Custom template for {{.NodePoolName}}`
				case "invalid-talos-config":
					templateContent = `invalid template {{.InvalidField`
				}
				
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretRef.Name,
						Namespace: "default", // test namespace
					},
					Data: map[string][]byte{
						secretRef.Key: []byte(templateContent),
					},
				}
				objects = append(objects, secret)
			}

			reconciler := &GPUNodePoolReconciler{
				Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build(),
				Log:    logr.Discard(),
				Config: tt.config,
				ImageFactory: imagefactory.NewClient(""),
			}

			result, err := reconciler.buildUserDataScript(context.Background(), tt.nodePool, tt.nodeClass, "vultr")

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestApplyTemplate(t *testing.T) {
	reconciler := &GPUNodePoolReconciler{}

	// Test template execution with edge cases
	vars := map[string]interface{}{
		"SimpleString": "test-value",
		"BooleanTrue":  true,
		"BooleanFalse": false,
		"MapData":      map[string]string{"key1": "value1", "key2": "value2"},
		"NilValue":     nil,
	}

	template := `Simple: {{.SimpleString}}
{{- if .BooleanTrue}}
TrueSection: enabled
{{- end}}
{{- if .BooleanFalse}}
FalseSection: should not appear
{{- end}}
{{- range $k, $v := .MapData}}
{{$k}}: {{$v}}
{{- end}}`

	result, err := reconciler.applyTemplate(template, vars)
	if err != nil {
		t.Fatalf("template execution failed: %v", err)
	}

	expected := []string{
		"Simple: test-value",
		"TrueSection: enabled",
		"key1: value1",
		"key2: value2",
	}
	notExpected := []string{"FalseSection: should not appear"}

	for _, exp := range expected {
		if !contains(result, exp) {
			t.Errorf("expected %q in result, got: %s", exp, result)
		}
	}
	for _, notExp := range notExpected {
		if contains(result, notExp) {
			t.Errorf("did not expect %q in result, got: %s", notExp, result)
		}
	}
}
