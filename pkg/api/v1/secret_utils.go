package v1

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// resolveSecretRef resolves a single secret reference
func resolveSecretRef(
	ctx context.Context, client client.Client, ref *SecretKeyRef, defaultNamespace string,
) (string, error) {
	if ref == nil {
		return "", fmt.Errorf("secret reference is nil")
	}

	namespace := ref.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}

	secret := &corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{
		Name:      ref.Name,
		Namespace: namespace,
	}, secret)
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s/%s: %w", namespace, ref.Name, err)
	}

	value, exists := secret.Data[ref.Key]
	if !exists {
		return "", fmt.Errorf("key %s not found in secret %s/%s", ref.Key, namespace, ref.Name)
	}

	return string(value), nil
}

// Resolve resolves all secret references in TailscaleConfig
func (tc *TailscaleConfig) Resolve(ctx context.Context, client client.Client, namespace string) (*TailscaleConfig, error) {
	if tc == nil {
		return nil, nil
	}

	resolved := &TailscaleConfig{
		Hostname:        tc.Hostname,
		Tags:            tc.Tags,
		Ephemeral:       tc.Ephemeral,
		AcceptRoutes:    tc.AcceptRoutes,
		AdvertiseRoutes: tc.AdvertiseRoutes,
		OperatorConfig:  tc.OperatorConfig,
	}

	// Resolve auth key secret reference
	if tc.AuthKeySecretRef != nil {
		value, err := resolveSecretRef(ctx, client, tc.AuthKeySecretRef, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve authKey secret: %w", err)
		}
		// Create a temporary secret ref for the resolved value
		// Note: In practice, we'd use this value directly in cloud-init
		_ = value // Will be used in cloud-init generation
	}

	return resolved, nil
}
