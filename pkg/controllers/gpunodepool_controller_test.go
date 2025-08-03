package controllers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/config"
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
					MachineToken:         "test-token",
					ClusterCA:            "test-ca",
					ClusterID:            "test-cluster-id",
					ControlPlaneEndpoint: "https://control-plane:6443",
					ClusterName:          "test-cluster",
				},
			},
			validate: func(t *testing.T, result string) {
				// Verify critical cluster credentials are substituted
				if !contains(result, "test-token") {
					t.Error("machine token not substituted")
				}
				if !contains(result, "test-ca") {
					t.Error("cluster CA not substituted")
				}
				if !contains(result, "https://control-plane:6443") {
					t.Error("control plane endpoint not substituted")
				}
				// Verify node-specific configuration
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
						MachineConfigTemplate: `version: v1alpha1
machine:
  type: worker
  token: {{.MachineToken}}
  # Custom template for {{.NodePoolName}}`,
					},
				},
			},
			config: &config.OperatorConfig{
				Talos: config.TalosDefaults{MachineToken: "custom-token"},
			},
			validate: func(t *testing.T, result string) {
				if !contains(result, "custom-token") {
					t.Error("custom template token not substituted")
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
						MachineConfigTemplate: `invalid template {{.InvalidField`,
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

			reconciler := &GPUNodePoolReconciler{
				Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
				Log:    logr.Discard(),
				Config: tt.config,
			}

			result, err := reconciler.buildUserDataScript(context.Background(), tt.nodePool, tt.nodeClass)

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
