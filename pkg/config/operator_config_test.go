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
