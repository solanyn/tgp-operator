package v1

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolveWireGuardConfig resolves all secret references in WireGuardConfig
func (w *WireGuardConfig) Resolve(ctx context.Context, client client.Client, namespace string) (*WireGuardConfig, error) {
	resolved := &WireGuardConfig{
		PrivateKey:     w.PrivateKey,
		PublicKey:      w.PublicKey,
		ServerEndpoint: w.ServerEndpoint,
		AllowedIPs:     w.AllowedIPs,
		Address:        w.Address,
	}

	// Resolve PrivateKey
	if w.PrivateKeySecretRef != nil {
		if w.PrivateKey != "" {
			return nil, fmt.Errorf("cannot specify both privateKey and privateKeySecretRef")
		}
		value, err := resolveSecretRef(ctx, client, w.PrivateKeySecretRef, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve privateKey secret: %w", err)
		}
		resolved.PrivateKey = value
	}

	// Resolve PublicKey
	if w.PublicKeySecretRef != nil {
		if w.PublicKey != "" {
			return nil, fmt.Errorf("cannot specify both publicKey and publicKeySecretRef")
		}
		value, err := resolveSecretRef(ctx, client, w.PublicKeySecretRef, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve publicKey secret: %w", err)
		}
		resolved.PublicKey = value
	}

	// Resolve ServerEndpoint
	if w.ServerEndpointSecretRef != nil {
		if w.ServerEndpoint != "" {
			return nil, fmt.Errorf("cannot specify both serverEndpoint and serverEndpointSecretRef")
		}
		value, err := resolveSecretRef(ctx, client, w.ServerEndpointSecretRef, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve serverEndpoint secret: %w", err)
		}
		resolved.ServerEndpoint = value
	}

	// Resolve AllowedIPs
	if w.AllowedIPsSecretRef != nil {
		if len(w.AllowedIPs) > 0 {
			return nil, fmt.Errorf("cannot specify both allowedIPs and allowedIPsSecretRef")
		}
		value, err := resolveSecretRef(ctx, client, w.AllowedIPsSecretRef, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve allowedIPs secret: %w", err)
		}
		// Parse comma-separated IPs
		resolved.AllowedIPs = strings.Split(strings.TrimSpace(value), ",")
		for i, ip := range resolved.AllowedIPs {
			resolved.AllowedIPs[i] = strings.TrimSpace(ip)
		}
	}

	// Resolve Address
	if w.AddressSecretRef != nil {
		if w.Address != "" {
			return nil, fmt.Errorf("cannot specify both address and addressSecretRef")
		}
		value, err := resolveSecretRef(ctx, client, w.AddressSecretRef, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve address secret: %w", err)
		}
		resolved.Address = value
	}

	return resolved, nil
}

// resolveSecretRef resolves a single secret reference
func resolveSecretRef(ctx context.Context, client client.Client, ref *SecretKeyRef, defaultNamespace string) (string, error) {
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