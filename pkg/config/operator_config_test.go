package config

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestOperatorConfig_GetProviderCredentials(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"RUNPOD_API_KEY":      []byte("runpod-key-123"),
			"LAMBDA_LABS_API_KEY": []byte("lambda-key-456"),
			"PAPERSPACE_API_KEY":  []byte("paperspace-key-789"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	config := &OperatorConfig{
		Providers: ProvidersConfig{
			RunPod: ProviderConfig{
				Enabled:         true,
				SecretName:      "test-secret",
				SecretNamespace: "test-namespace",
				APIKeySecretKey: "RUNPOD_API_KEY",
			},
			LambdaLabs: ProviderConfig{
				Enabled:         false, // Disabled for testing
				SecretName:      "test-secret",
				SecretNamespace: "test-namespace",
				APIKeySecretKey: "LAMBDA_LABS_API_KEY",
			},
		},
	}

	ctx := context.Background()

	t.Run("should return API key for enabled provider", func(t *testing.T) {
		apiKey, err := config.GetProviderCredentials(ctx, fakeClient, "runpod", "default")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if apiKey != "runpod-key-123" {
			t.Errorf("Expected 'runpod-key-123', got: %s", apiKey)
		}
	})

	t.Run("should return error for disabled provider", func(t *testing.T) {
		_, err := config.GetProviderCredentials(ctx, fakeClient, "lambdalabs", "default")
		if err == nil {
			t.Error("Expected error for disabled provider")
		}
		expectedMsg := "provider lambdalabs is not enabled"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message '%s', got: %s", expectedMsg, err.Error())
		}
	})

	t.Run("should return error for unknown provider", func(t *testing.T) {
		_, err := config.GetProviderCredentials(ctx, fakeClient, "unknown", "default")
		if err == nil {
			t.Error("Expected error for unknown provider")
		}
		expectedMsg := "unknown provider: unknown"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message '%s', got: %s", expectedMsg, err.Error())
		}
	})
}

func TestOperatorConfig_GetTailscaleOAuthCredentials(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tailscale-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"client-id":     []byte("tskey-client-12345"),
			"client-secret": []byte("tskey-secret-67890"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	config := &OperatorConfig{
		Tailscale: TailscaleDefaults{
			OAuthSecretName:      "tailscale-secret",
			OAuthSecretNamespace: "test-namespace",
			ClientIDKey:          "client-id",
			ClientSecretKey:      "client-secret",
		},
	}

	ctx := context.Background()

	t.Run("should return OAuth credentials", func(t *testing.T) {
		clientID, clientSecret, err := config.GetTailscaleOAuthCredentials(ctx, fakeClient, "default")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if clientID != "tskey-client-12345" {
			t.Errorf("Expected 'tskey-client-12345', got: %s", clientID)
		}
		if clientSecret != "tskey-secret-67890" {
			t.Errorf("Expected 'tskey-secret-67890', got: %s", clientSecret)
		}
	})
}

func TestOperatorConfig_GetTalosClusterCredentials(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	tests := []struct {
		name           string
		config         *OperatorConfig
		secret         *corev1.Secret
		expectError    bool
		expectedValues map[string]string
	}{
		{
			name: "should load credentials from secret",
			config: &OperatorConfig{
				Talos: TalosDefaults{
					SecretName:              "talos-secret",
					MachineTokenKey:         "machine-token",
					ClusterCAKey:            "cluster-ca",
					ClusterIDKey:            "cluster-id",
					ClusterSecretKey:        "cluster-secret",
					ControlPlaneEndpointKey: "control-plane-endpoint",
					ClusterNameKey:          "cluster-name",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "talos-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"machine-token":          []byte("test-machine-token"),
					"cluster-ca":             []byte("test-cluster-ca"),
					"cluster-id":             []byte("test-cluster-id"),
					"cluster-secret":         []byte("test-cluster-secret"),
					"control-plane-endpoint": []byte("https://test-endpoint:6443"),
					"cluster-name":           []byte("test-cluster"),
				},
			},
			expectError: false,
			expectedValues: map[string]string{
				"MachineToken":         "test-machine-token",
				"ClusterCA":            "test-cluster-ca",
				"ClusterID":            "test-cluster-id",
				"ClusterSecret":        "test-cluster-secret",
				"ControlPlaneEndpoint": "https://test-endpoint:6443",
				"ClusterName":          "test-cluster",
			},
		},
		{
			name: "should use defaults when no secret configured",
			config: &OperatorConfig{
				Talos: TalosDefaults{
					// No secret configuration
				},
			},
			secret:      nil,
			expectError: false,
			expectedValues: map[string]string{
				"MachineToken":         "placeholder-machine-token",
				"ClusterCA":            "placeholder-cluster-ca",
				"ClusterID":            "default-cluster",
				"ClusterSecret":        "placeholder-cluster-secret",
				"ControlPlaneEndpoint": "https://kubernetes.default.svc:443",
				"ClusterName":          "talos-cluster",
			},
		},
		{
			name: "should use defaults when secret not found",
			config: &OperatorConfig{
				Talos: TalosDefaults{
					SecretName: "missing-secret",
				},
			},
			secret:      nil,
			expectError: false,
			expectedValues: map[string]string{
				"MachineToken":         "placeholder-machine-token",
				"ClusterCA":            "placeholder-cluster-ca",
				"ClusterID":            "default-cluster",
				"ClusterSecret":        "placeholder-cluster-secret",
				"ControlPlaneEndpoint": "https://kubernetes.default.svc:443",
				"ClusterName":          "talos-cluster",
			},
		},
		{
			name: "should not override existing config values",
			config: &OperatorConfig{
				Talos: TalosDefaults{
					MachineToken:    "existing-token",
					SecretName:      "talos-secret",
					MachineTokenKey: "machine-token",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "talos-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"machine-token": []byte("secret-token"),
				},
			},
			expectError: false,
			expectedValues: map[string]string{
				"MachineToken": "existing-token", // Should keep existing value
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.secret != nil {
				clientBuilder = clientBuilder.WithObjects(tt.secret)
			}
			fakeClient := clientBuilder.Build()

			err := tt.config.GetTalosClusterCredentials(context.Background(), fakeClient, "test-namespace")

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check expected values
			for field, expectedValue := range tt.expectedValues {
				var actualValue string
				switch field {
				case "MachineToken":
					actualValue = tt.config.Talos.MachineToken
				case "ClusterCA":
					actualValue = tt.config.Talos.ClusterCA
				case "ClusterID":
					actualValue = tt.config.Talos.ClusterID
				case "ClusterSecret":
					actualValue = tt.config.Talos.ClusterSecret
				case "ControlPlaneEndpoint":
					actualValue = tt.config.Talos.ControlPlaneEndpoint
				case "ClusterName":
					actualValue = tt.config.Talos.ClusterName
				}

				if actualValue != expectedValue {
					t.Errorf("Expected %s to be %q, got %q", field, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	t.Run("should have default provider configurations", func(t *testing.T) {
		if !config.Providers.RunPod.Enabled {
			t.Error("RunPod should be enabled by default")
		}
		if !config.Providers.LambdaLabs.Enabled {
			t.Error("LambdaLabs should be enabled by default")
		}
		if !config.Providers.Paperspace.Enabled {
			t.Error("Paperspace should be enabled by default")
		}

		expectedSecretName := "tgp-operator-secret"
		if config.Providers.RunPod.SecretName != expectedSecretName {
			t.Errorf("Expected secret name '%s', got: %s", expectedSecretName, config.Providers.RunPod.SecretName)
		}
	})

	t.Run("should have default Talos configuration", func(t *testing.T) {
		if config.Talos.Image == "" {
			t.Error("Talos image should not be empty")
		}
	})

	t.Run("should have default Tailscale configuration", func(t *testing.T) {
		if len(config.Tailscale.Tags) == 0 {
			t.Error("Tailscale should have default tags")
		}
		if !config.Tailscale.Ephemeral {
			t.Error("Tailscale should be ephemeral by default")
		}
		if !config.Tailscale.AcceptRoutes {
			t.Error("Tailscale should accept routes by default")
		}
	})
}
