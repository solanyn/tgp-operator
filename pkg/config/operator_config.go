package config

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OperatorConfig contains centralized configuration for the TGP operator
type OperatorConfig struct {
	// Providers contains configuration for all cloud providers
	Providers ProvidersConfig `json:"providers"`

	// Talos contains default Talos configuration
	Talos TalosDefaults `json:"talos"`

	// Tailscale contains Tailscale mesh networking configuration
	Tailscale TailscaleDefaults `json:"tailscale"`
}

// ProvidersConfig contains configuration for all cloud providers
type ProvidersConfig struct {
	// RunPod contains RunPod provider configuration
	RunPod ProviderConfig `json:"runpod"`

	// LambdaLabs contains Lambda Labs provider configuration
	LambdaLabs ProviderConfig `json:"lambdaLabs"`

	// Paperspace contains Paperspace provider configuration
	Paperspace ProviderConfig `json:"paperspace"`
}

// ProviderConfig contains configuration for a single cloud provider
type ProviderConfig struct {
	// Enabled indicates whether this provider is available
	Enabled bool `json:"enabled"`

	// SecretName is the name of the secret containing API credentials
	SecretName string `json:"secretName"`

	// SecretNamespace is the namespace of the secret (defaults to operator namespace)
	SecretNamespace string `json:"secretNamespace,omitempty"`

	// APIKeySecretKey is the key in the secret containing the API key
	APIKeySecretKey string `json:"apiKeySecretKey"`
}

// TalosDefaults contains default Talos configuration
type TalosDefaults struct {
	// Image is the default Talos image to use
	Image string `json:"image"`

	// MachineToken is the token for machines to join the cluster
	MachineToken string `json:"machineToken"`

	// ClusterCA is the cluster CA certificate
	ClusterCA string `json:"clusterCA"`

	// ClusterID is the unique cluster identifier
	ClusterID string `json:"clusterID"`

	// ClusterSecret is the cluster secret for secure communication
	ClusterSecret string `json:"clusterSecret"`

	// ControlPlaneEndpoint is the endpoint for the control plane
	ControlPlaneEndpoint string `json:"controlPlaneEndpoint"`

	// ClusterName is the name of the cluster
	ClusterName string `json:"clusterName"`
}

// TailscaleDefaults contains default Tailscale configuration
type TailscaleDefaults struct {
	// Tags are the default tags to apply to devices
	Tags []string `json:"tags"`

	// Ephemeral indicates whether devices should be ephemeral by default
	Ephemeral bool `json:"ephemeral"`

	// AcceptRoutes indicates whether to accept routes by default
	AcceptRoutes bool `json:"acceptRoutes"`

	// OAuthSecretName is the name of the secret containing OAuth credentials
	OAuthSecretName string `json:"oauthSecretName"`

	// OAuthSecretNamespace is the namespace of the OAuth secret
	OAuthSecretNamespace string `json:"oauthSecretNamespace,omitempty"`

	// ClientIDKey is the key containing the OAuth client ID
	ClientIDKey string `json:"clientIdKey"`

	// ClientSecretKey is the key containing the OAuth client secret
	ClientSecretKey string `json:"clientSecretKey"`
}

// GetProviderCredentials retrieves API credentials for a provider
func (c *OperatorConfig) GetProviderCredentials(ctx context.Context, client client.Client, provider string, operatorNamespace string) (string, error) {
	var providerConfig ProviderConfig

	switch provider {
	case "runpod":
		providerConfig = c.Providers.RunPod
	case "lambdalabs":
		providerConfig = c.Providers.LambdaLabs
	case "paperspace":
		providerConfig = c.Providers.Paperspace
	default:
		return "", fmt.Errorf("unknown provider: %s", provider)
	}

	if !providerConfig.Enabled {
		return "", fmt.Errorf("provider %s is not enabled", provider)
	}

	secretNamespace := providerConfig.SecretNamespace
	if secretNamespace == "" {
		secretNamespace = operatorNamespace
	}

	secret := &corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{
		Name:      providerConfig.SecretName,
		Namespace: secretNamespace,
	}, secret)
	if err != nil {
		return "", fmt.Errorf("failed to get provider secret %s/%s: %w", secretNamespace, providerConfig.SecretName, err)
	}

	apiKey, exists := secret.Data[providerConfig.APIKeySecretKey]
	if !exists {
		return "", fmt.Errorf("API key %s not found in secret %s/%s", providerConfig.APIKeySecretKey, secretNamespace, providerConfig.SecretName)
	}

	return string(apiKey), nil
}

// GetTailscaleOAuthCredentials retrieves Tailscale OAuth credentials
func (c *OperatorConfig) GetTailscaleOAuthCredentials(ctx context.Context, client client.Client, operatorNamespace string) (clientID, clientSecret string, err error) {
	secretNamespace := c.Tailscale.OAuthSecretNamespace
	if secretNamespace == "" {
		secretNamespace = operatorNamespace
	}

	secret := &corev1.Secret{}
	err = client.Get(ctx, types.NamespacedName{
		Name:      c.Tailscale.OAuthSecretName,
		Namespace: secretNamespace,
	}, secret)
	if err != nil {
		return "", "", fmt.Errorf("failed to get Tailscale OAuth secret %s/%s: %w", secretNamespace, c.Tailscale.OAuthSecretName, err)
	}

	clientIDBytes, exists := secret.Data[c.Tailscale.ClientIDKey]
	if !exists {
		return "", "", fmt.Errorf("client ID key %s not found in secret %s/%s", c.Tailscale.ClientIDKey, secretNamespace, c.Tailscale.OAuthSecretName)
	}

	clientSecretBytes, exists := secret.Data[c.Tailscale.ClientSecretKey]
	if !exists {
		return "", "", fmt.Errorf("client secret key %s not found in secret %s/%s", c.Tailscale.ClientSecretKey, secretNamespace, c.Tailscale.OAuthSecretName)
	}

	return string(clientIDBytes), string(clientSecretBytes), nil
}

// DefaultConfig returns a default operator configuration
func DefaultConfig() *OperatorConfig {
	return &OperatorConfig{
		Providers: ProvidersConfig{
			RunPod: ProviderConfig{
				Enabled:         true,
				SecretName:      "tgp-operator-secret",
				APIKeySecretKey: "RUNPOD_API_KEY",
			},
			LambdaLabs: ProviderConfig{
				Enabled:         true,
				SecretName:      "tgp-operator-secret",
				APIKeySecretKey: "LAMBDA_LABS_API_KEY",
			},
			Paperspace: ProviderConfig{
				Enabled:         true,
				SecretName:      "tgp-operator-secret",
				APIKeySecretKey: "PAPERSPACE_API_KEY",
			},
		},
		Talos: TalosDefaults{
			Image: "ghcr.io/siderolabs/talos:v1.10.5",
		},
		Tailscale: TailscaleDefaults{
			Tags:            []string{"tag:k8s", "tag:gpu"},
			Ephemeral:       true,
			AcceptRoutes:    true,
			OAuthSecretName: "tgp-operator-secret",
			ClientIDKey:     "client-id",
			ClientSecretKey: "client-secret",
		},
	}
}

// GetTalosClusterCredentials retrieves Talos cluster credentials from secrets if needed
func (c *OperatorConfig) GetTalosClusterCredentials(ctx context.Context, client client.Client, operatorNamespace string) error {
	// If all required fields are already set in config, no need to fetch from secrets
	if c.Talos.MachineToken != "" && c.Talos.ClusterCA != "" &&
		c.Talos.ClusterID != "" && c.Talos.ClusterSecret != "" &&
		c.Talos.ControlPlaneEndpoint != "" && c.Talos.ClusterName != "" {
		return nil
	}

	// TODO: In a production environment, these sensitive values should be loaded from secrets
	// For now, we'll use placeholder values
	if c.Talos.MachineToken == "" {
		c.Talos.MachineToken = "placeholder-machine-token"
	}
	if c.Talos.ClusterCA == "" {
		c.Talos.ClusterCA = "placeholder-cluster-ca"
	}
	if c.Talos.ClusterID == "" {
		c.Talos.ClusterID = "default-cluster"
	}
	if c.Talos.ClusterSecret == "" {
		c.Talos.ClusterSecret = "placeholder-cluster-secret"
	}
	if c.Talos.ControlPlaneEndpoint == "" {
		c.Talos.ControlPlaneEndpoint = "https://kubernetes.default.svc:443"
	}
	if c.Talos.ClusterName == "" {
		c.Talos.ClusterName = "talos-cluster"
	}

	return nil
}
