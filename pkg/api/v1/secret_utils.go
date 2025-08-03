package v1

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Resolve resolves all secret references in TailscaleConfig
func (tc *TailscaleConfig) Resolve(ctx context.Context, client client.Client, namespace string) (*TailscaleConfig, error) {
	if tc == nil {
		return nil, nil
	}

	resolved := &TailscaleConfig{
		Hostname:                  tc.Hostname,
		Tags:                      tc.Tags,
		Ephemeral:                 tc.Ephemeral,
		AcceptRoutes:              tc.AcceptRoutes,
		AdvertiseRoutes:           tc.AdvertiseRoutes,
		OAuthCredentialsSecretRef: tc.OAuthCredentialsSecretRef,
		OperatorConfig:            tc.OperatorConfig,
	}

	// Resolve OAuth credentials secret reference
	if tc.OAuthCredentialsSecretRef != nil {
		err := tc.resolveOAuthCredentials(ctx, client, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve OAuth credentials secret: %w", err)
		}
	}

	return resolved, nil
}

// resolveOAuthCredentials validates OAuth credentials in the referenced secret
func (tc *TailscaleConfig) resolveOAuthCredentials(ctx context.Context, client client.Client, defaultNamespace string) error {
	if tc.OAuthCredentialsSecretRef == nil {
		return nil
	}

	ref := tc.OAuthCredentialsSecretRef
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
		return fmt.Errorf("failed to get OAuth credentials secret %s/%s: %w", namespace, ref.Name, err)
	}

	// Validate that both client ID and client secret are present
	clientIDKey := ref.GetClientIDKey()
	clientSecretKey := ref.GetClientSecretKey()

	if _, exists := secret.Data[clientIDKey]; !exists {
		return fmt.Errorf("OAuth client ID key %s not found in secret %s/%s", clientIDKey, namespace, ref.Name)
	}

	if _, exists := secret.Data[clientSecretKey]; !exists {
		return fmt.Errorf("OAuth client secret key %s not found in secret %s/%s", clientSecretKey, namespace, ref.Name)
	}

	return nil
}
