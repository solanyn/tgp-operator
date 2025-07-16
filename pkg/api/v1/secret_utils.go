package v1

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Resolve resolves all secret references in WireGuardConfig
func (w *WireGuardConfig) Resolve(ctx context.Context, client client.Client, namespace string) (*WireGuardConfig, error) {
	resolved := &WireGuardConfig{
		PrivateKey:     w.PrivateKey,
		PublicKey:      w.PublicKey,
		ServerEndpoint: w.ServerEndpoint,
		AllowedIPs:     w.AllowedIPs,
		Address:        w.Address,
	}

	// Resolve string fields
	if err := w.resolveStringField(ctx, client, namespace, "privateKey", w.PrivateKey,
		w.PrivateKeySecretRef, &resolved.PrivateKey); err != nil {
		return nil, err
	}
	if err := w.resolveStringField(ctx, client, namespace, "publicKey", w.PublicKey,
		w.PublicKeySecretRef, &resolved.PublicKey); err != nil {
		return nil, err
	}
	if err := w.resolveStringField(ctx, client, namespace, "serverEndpoint", w.ServerEndpoint,
		w.ServerEndpointSecretRef, &resolved.ServerEndpoint); err != nil {
		return nil, err
	}
	if err := w.resolveStringField(ctx, client, namespace, "address", w.Address,
		w.AddressSecretRef, &resolved.Address); err != nil {
		return nil, err
	}

	// Resolve AllowedIPs (special case for slice)
	if err := w.resolveAllowedIPs(ctx, client, namespace, resolved); err != nil {
		return nil, err
	}

	return resolved, nil
}

// resolveStringField resolves a string field with optional secret reference
func (w *WireGuardConfig) resolveStringField(
	ctx context.Context, client client.Client, namespace, fieldName, directValue string,
	secretRef *SecretKeyRef, target *string,
) error {
	if secretRef != nil {
		if directValue != "" {
			return fmt.Errorf("cannot specify both %s and %sSecretRef", fieldName, fieldName)
		}
		value, err := resolveSecretRef(ctx, client, secretRef, namespace)
		if err != nil {
			return fmt.Errorf("failed to resolve %s secret: %w", fieldName, err)
		}
		*target = value
	}
	return nil
}

// resolveAllowedIPs resolves the AllowedIPs field with special slice handling
func (w *WireGuardConfig) resolveAllowedIPs(
	ctx context.Context, client client.Client, namespace string, resolved *WireGuardConfig,
) error {
	if w.AllowedIPsSecretRef != nil {
		if len(w.AllowedIPs) > 0 {
			return fmt.Errorf("cannot specify both allowedIPs and allowedIPsSecretRef")
		}
		value, err := resolveSecretRef(ctx, client, w.AllowedIPsSecretRef, namespace)
		if err != nil {
			return fmt.Errorf("failed to resolve allowedIPs secret: %w", err)
		}
		// Parse comma-separated IPs
		resolved.AllowedIPs = strings.Split(strings.TrimSpace(value), ",")
		for i, ip := range resolved.AllowedIPs {
			resolved.AllowedIPs[i] = strings.TrimSpace(ip)
		}
	}
	return nil
}

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
