package config

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OperatorConfig contains centralized configuration for the TGP operator
type OperatorConfig struct {
	// Providers contains configuration for all cloud providers
	Providers ProvidersConfig `yaml:"providers" json:"providers"`

	// Talos contains default Talos configuration
	Talos TalosDefaults `yaml:"talos" json:"talos"`

	// Tailscale contains Tailscale mesh networking configuration
	Tailscale TailscaleDefaults `yaml:"tailscale" json:"tailscale"`
}

// ProvidersConfig contains configuration for all cloud providers
type ProvidersConfig struct {
	// Vultr contains Vultr provider configuration
	Vultr ProviderConfig `yaml:"vultr" json:"vultr"`
	// GCP contains Google Cloud Platform provider configuration
	GCP ProviderConfig `yaml:"gcp" json:"gcp"`
}

// ProviderConfig contains configuration for a single cloud provider
type ProviderConfig struct {
	// Enabled indicates whether this provider is available
	Enabled bool `yaml:"enabled" json:"enabled"`

	// CredentialsRef references the secret containing API credentials
	CredentialsRef SecretReference `yaml:"credentialsRef" json:"credentialsRef"`
}

// SecretReference contains a reference to a secret and key
type SecretReference struct {
	// Name is the name of the secret
	Name string `yaml:"name" json:"name"`

	// Namespace is the namespace of the secret (defaults to operator namespace)
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`

	// Key is the key in the secret containing the value
	Key string `yaml:"key" json:"key"`
}

// TalosDefaults contains default Talos configuration
type TalosDefaults struct {
	// Image is the default Talos image to use
	Image string `yaml:"image" json:"image"`
}

// TailscaleDefaults contains default Tailscale configuration
type TailscaleDefaults struct {
	// Tags are the default tags to apply to devices
	Tags []string `yaml:"tags" json:"tags"`

	// Ephemeral indicates whether devices should be ephemeral by default
	Ephemeral bool `yaml:"ephemeral" json:"ephemeral"`

	// AcceptRoutes indicates whether to accept routes by default
	AcceptRoutes bool `yaml:"acceptRoutes" json:"acceptRoutes"`

	// OAuthCredentialsRef references the secret containing OAuth credentials
	OAuthCredentialsRef OAuthSecretReference `yaml:"oauthCredentialsRef" json:"oauthCredentialsRef"`
}

// OAuthSecretReference contains references to OAuth client ID and secret
type OAuthSecretReference struct {
	// Name is the name of the secret
	Name string `yaml:"name" json:"name"`

	// Namespace is the namespace of the secret (defaults to operator namespace)
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`

	// ClientIDKey is the key containing the OAuth client ID
	ClientIDKey string `yaml:"clientIdKey" json:"clientIdKey"`

	// ClientSecretKey is the key containing the OAuth client secret
	ClientSecretKey string `yaml:"clientSecretKey" json:"clientSecretKey"`
}

// GetProviderCredentials retrieves API credentials for a provider
func (c *OperatorConfig) GetProviderCredentials(ctx context.Context, client client.Client, provider string, operatorNamespace string) (string, error) {
	var providerConfig ProviderConfig

	switch provider {
	case "vultr":
		providerConfig = c.Providers.Vultr
	case "gcp":
		providerConfig = c.Providers.GCP
	default:
		return "", fmt.Errorf("unknown provider: %s", provider)
	}


	if !providerConfig.Enabled {
		return "", fmt.Errorf("provider %s is not enabled", provider)
	}

	secretNamespace := providerConfig.CredentialsRef.Namespace
	if secretNamespace == "" {
		secretNamespace = operatorNamespace
	}

	secret := &corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{
		Name:      providerConfig.CredentialsRef.Name,
		Namespace: secretNamespace,
	}, secret)
	if err != nil {
		return "", fmt.Errorf("failed to get provider secret %s/%s: %w", secretNamespace, providerConfig.CredentialsRef.Name, err)
	}

	apiKey, exists := secret.Data[providerConfig.CredentialsRef.Key]
	if !exists {
		return "", fmt.Errorf("API key %s not found in secret %s/%s", providerConfig.CredentialsRef.Key, secretNamespace, providerConfig.CredentialsRef.Name)
	}

	return string(apiKey), nil
}

// GetTailscaleOAuthCredentials retrieves Tailscale OAuth credentials
func (c *OperatorConfig) GetTailscaleOAuthCredentials(ctx context.Context, client client.Client, operatorNamespace string) (clientID, clientSecret string, err error) {
	secretNamespace := c.Tailscale.OAuthCredentialsRef.Namespace
	if secretNamespace == "" {
		secretNamespace = operatorNamespace
	}

	secret := &corev1.Secret{}
	err = client.Get(ctx, types.NamespacedName{
		Name:      c.Tailscale.OAuthCredentialsRef.Name,
		Namespace: secretNamespace,
	}, secret)
	if err != nil {
		return "", "", fmt.Errorf("failed to get Tailscale OAuth secret %s/%s: %w", secretNamespace, c.Tailscale.OAuthCredentialsRef.Name, err)
	}

	clientIDBytes, exists := secret.Data[c.Tailscale.OAuthCredentialsRef.ClientIDKey]
	if !exists {
		return "", "", fmt.Errorf("client ID key %s not found in secret %s/%s", c.Tailscale.OAuthCredentialsRef.ClientIDKey, secretNamespace, c.Tailscale.OAuthCredentialsRef.Name)
	}

	clientSecretBytes, exists := secret.Data[c.Tailscale.OAuthCredentialsRef.ClientSecretKey]
	if !exists {
		return "", "", fmt.Errorf("client secret key %s not found in secret %s/%s", c.Tailscale.OAuthCredentialsRef.ClientSecretKey, secretNamespace, c.Tailscale.OAuthCredentialsRef.Name)
	}

	return string(clientIDBytes), string(clientSecretBytes), nil
}

// LoadConfig loads operator configuration from a ConfigMap or returns default config
func LoadConfig(ctx context.Context, client client.Client, configMapName, namespace string) (*OperatorConfig, error) {
	// Try to load from ConfigMap first
	configMap := &corev1.ConfigMap{}
	err := client.Get(ctx, types.NamespacedName{
		Name:      configMapName,
		Namespace: namespace,
	}, configMap)
	
	if err != nil {
		// Return error instead of silently falling back to defaults
		return nil, fmt.Errorf("failed to load ConfigMap %s/%s: %w", namespace, configMapName, err)
	}

	configYAML, exists := configMap.Data["config.yaml"]
	if !exists {
		return nil, fmt.Errorf("config.yaml key not found in ConfigMap %s/%s", namespace, configMapName)
	}

	config := &OperatorConfig{}
	if err := yaml.Unmarshal([]byte(configYAML), config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// validateConfig validates that the configuration has reasonable values
func validateConfig(config *OperatorConfig) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	// Check that at least one provider is configured
	hasEnabledProvider := false
	
	if config.Providers.Vultr.Enabled {
		hasEnabledProvider = true
		if config.Providers.Vultr.CredentialsRef.Name == "" {
			return fmt.Errorf("vultr provider is enabled but credentialsRef.name is empty")
		}
		if config.Providers.Vultr.CredentialsRef.Key == "" {
			return fmt.Errorf("vultr provider is enabled but credentialsRef.key is empty")
		}
	}
	
	if config.Providers.GCP.Enabled {
		hasEnabledProvider = true
		if config.Providers.GCP.CredentialsRef.Name == "" {
			return fmt.Errorf("gcp provider is enabled but credentialsRef.name is empty")
		}
		if config.Providers.GCP.CredentialsRef.Key == "" {
			return fmt.Errorf("gcp provider is enabled but credentialsRef.key is empty")
		}
	}
	
	if !hasEnabledProvider {
		return fmt.Errorf("no providers are enabled - at least one provider must be enabled")
	}

	return nil
}

// DefaultConfig returns a default operator configuration
func DefaultConfig() *OperatorConfig {
	return &OperatorConfig{
		Providers: ProvidersConfig{
			Vultr: ProviderConfig{
				Enabled: false,
				CredentialsRef: SecretReference{
					Name: "tgp-operator-secret",
					Key:  "VULTR_API_KEY",
				},
			},
			GCP: ProviderConfig{
				Enabled: false,
				CredentialsRef: SecretReference{
					Name: "tgp-operator-secret",
					Key:  "GOOGLE_APPLICATION_CREDENTIALS_JSON",
				},
			},
		},
		Talos: TalosDefaults{
			Image: "ghcr.io/siderolabs/talos:v1.10.5",
		},
		Tailscale: TailscaleDefaults{
			Tags:         []string{"tag:k8s", "tag:gpu"},
			Ephemeral:    true,
			AcceptRoutes: true,
			OAuthCredentialsRef: OAuthSecretReference{
				Name:            "tgp-operator-secret",
				ClientIDKey:     "client-id",
				ClientSecretKey: "client-secret",
			},
		},
	}
}
