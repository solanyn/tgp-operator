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
			"GOOGLE_APPLICATION_CREDENTIALS_JSON": []byte(`{"type":"service_account","project_id":"test-project"}`),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	config := &OperatorConfig{
		Providers: ProvidersConfig{
			GCP: ProviderConfig{
				Enabled: true,
				CredentialsRef: SecretReference{
					Name:      "test-secret",
					Namespace: "test-namespace",
					Key:       "GOOGLE_APPLICATION_CREDENTIALS_JSON",
				},
			},
		},
	}

	ctx := context.Background()

	t.Run("should return API key for enabled GCP provider", func(t *testing.T) {
		apiKey, err := config.GetProviderCredentials(ctx, fakeClient, "gcp", "default")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		expectedJSON := `{"type":"service_account","project_id":"test-project"}`
		if apiKey != expectedJSON {
			t.Errorf("Expected '%s', got: %s", expectedJSON, apiKey)
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

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	t.Run("should have GCP provider configuration", func(t *testing.T) {
		if config.Providers.GCP.Enabled {
			t.Error("GCP should be disabled by default")
		}

		expectedSecretName := "tgp-operator-secret"
		if config.Providers.GCP.CredentialsRef.Name != expectedSecretName {
			t.Errorf("Expected secret name '%s', got: %s", expectedSecretName, config.Providers.GCP.CredentialsRef.Name)
		}

		expectedAPIKey := "GOOGLE_APPLICATION_CREDENTIALS_JSON"
		if config.Providers.GCP.CredentialsRef.Key != expectedAPIKey {
			t.Errorf("Expected API key '%s', got: %s", expectedAPIKey, config.Providers.GCP.CredentialsRef.Key)
		}
	})

	t.Run("should have default Talos configuration", func(t *testing.T) {
		if config.Talos.Version == "" {
			t.Error("Talos version should not be empty")
		}
		if len(config.Talos.Extensions) == 0 {
			t.Error("Talos extensions should not be empty")
		}
	})

}
