package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
)

type ProviderConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	CredentialsRef SecretReference `yaml:"credentialsRef" json:"credentialsRef"`
}

type SecretReference struct {
	Name string `yaml:"name" json:"name"`
	Key string `yaml:"key" json:"key"`
}

type ProvidersConfig struct {
	Vultr ProviderConfig `yaml:"vultr" json:"vultr"`
	GCP ProviderConfig `yaml:"gcp" json:"gcp"`
}

type OperatorConfig struct {
	Providers ProvidersConfig `yaml:"providers" json:"providers"`
}

func main() {
	yamlData := `providers:
  vultr:
    enabled: true
    credentialsRef:
      name: tgp-operator-secret
      key: VULTR_API_KEY
  gcp:
    enabled: true
    credentialsRef:
      name: tgp-operator-secret
      key: GOOGLE_APPLICATION_CREDENTIALS_JSON
talos:
  image: ghcr.io/siderolabs/talos:v1.10.6
tailscale:
  tags:
    - "tag:k8s"
    - "tag:cloud"
  ephemeral: true
  acceptRoutes: true
  oauthCredentialsRef:
    name: tgp-operator-secret
    clientIdKey: client-id
    clientSecretKey: client-secret`

	config := &OperatorConfig{}
	if err := yaml.Unmarshal([]byte(yamlData), config); err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		return
	}

	fmt.Printf("Vultr enabled: %v\n", config.Providers.Vultr.Enabled)
	fmt.Printf("GCP enabled: %v\n", config.Providers.GCP.Enabled)
	fmt.Printf("Vultr secret: %s\n", config.Providers.Vultr.CredentialsRef.Name)
}