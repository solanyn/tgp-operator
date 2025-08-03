package webhooks

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/validation"
)

// GPUNodeClassValidator validates GPUNodeClass resources
type GPUNodeClassValidator struct {
	talosValidator *validation.TalosConfigValidator
}

// NewGPUNodeClassValidator creates a new GPUNodeClass validator
func NewGPUNodeClassValidator() *GPUNodeClassValidator {
	return &GPUNodeClassValidator{
		talosValidator: validation.NewTalosConfigValidator(),
	}
}

// SetupWithManager registers the webhook with the manager
func (v *GPUNodeClassValidator) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&tgpv1.GPUNodeClass{}).
		WithValidator(v).
		Complete()
}

// ValidateCreate validates GPUNodeClass creation
func (v *GPUNodeClassValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	nodeClass, ok := obj.(*tgpv1.GPUNodeClass)
	if !ok {
		return nil, fmt.Errorf("expected GPUNodeClass, got %T", obj)
	}

	return v.validateGPUNodeClass(nodeClass)
}

// ValidateUpdate validates GPUNodeClass updates
func (v *GPUNodeClassValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newNodeClass, ok := newObj.(*tgpv1.GPUNodeClass)
	if !ok {
		return nil, fmt.Errorf("expected GPUNodeClass, got %T", newObj)
	}

	return v.validateGPUNodeClass(newNodeClass)
}

// ValidateDelete validates GPUNodeClass deletion (no validation needed)
func (v *GPUNodeClassValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

// validateGPUNodeClass performs comprehensive validation of a GPUNodeClass
func (v *GPUNodeClassValidator) validateGPUNodeClass(nodeClass *tgpv1.GPUNodeClass) (admission.Warnings, error) {
	var warnings admission.Warnings

	// Validate TalosConfig if present
	if nodeClass.Spec.TalosConfig != nil {
		if err := v.validateTalosConfig(nodeClass.Spec.TalosConfig); err != nil {
			return warnings, fmt.Errorf("invalid TalosConfig: %w", err)
		}
	}

	// Add other validations as needed
	if err := v.validateProviders(nodeClass.Spec.Providers); err != nil {
		return warnings, fmt.Errorf("invalid provider configuration: %w", err)
	}

	if err := v.validateLimits(nodeClass.Spec.Limits); err != nil {
		return warnings, fmt.Errorf("invalid limits: %w", err)
	}

	return warnings, nil
}

// validateTalosConfig validates the TalosConfig
func (v *GPUNodeClassValidator) validateTalosConfig(talosConfig *tgpv1.TalosConfig) error {
	// Check that MachineConfigSecretRef is provided
	if talosConfig.MachineConfigSecretRef == nil {
		return fmt.Errorf("machineConfigSecretRef is required")
	}

	// Validate the secret reference
	if err := v.validateSecretRef(talosConfig.MachineConfigSecretRef); err != nil {
		return fmt.Errorf("invalid machine config secret reference: %w", err)
	}

	return nil
}

// validateSecretRef validates a secret reference
func (v *GPUNodeClassValidator) validateSecretRef(secretRef *tgpv1.SecretKeyRef) error {
	if secretRef.Name == "" {
		return fmt.Errorf("secret name cannot be empty")
	}
	if secretRef.Key == "" {
		return fmt.Errorf("secret key cannot be empty")
	}
	return nil
}

// validateProviders validates provider configurations
func (v *GPUNodeClassValidator) validateProviders(providers []tgpv1.ProviderConfig) error {
	if len(providers) == 0 {
		return fmt.Errorf("at least one provider must be configured")
	}

	enabledCount := 0
	for _, provider := range providers {
		if provider.Enabled != nil && *provider.Enabled {
			enabledCount++
		}

		// Validate provider name
		validProviders := map[string]bool{
			"runpod":     true,
			"lambdalabs": true,
			"paperspace": true,
		}
		if !validProviders[provider.Name] {
			return fmt.Errorf("invalid provider name: %s", provider.Name)
		}

		// Validate credentials reference
		if provider.CredentialsRef.Name == "" {
			return fmt.Errorf("provider %s missing credentials reference", provider.Name)
		}
		if provider.CredentialsRef.Key == "" {
			return fmt.Errorf("provider %s missing credentials key", provider.Name)
		}
	}

	if enabledCount == 0 {
		return fmt.Errorf("at least one provider must be enabled")
	}

	return nil
}

// validateLimits validates resource limits
func (v *GPUNodeClassValidator) validateLimits(limits *tgpv1.NodeClassLimits) error {
	if limits == nil {
		return nil // Limits are optional
	}

	if limits.MaxNodes != nil && *limits.MaxNodes <= 0 {
		return fmt.Errorf("maxNodes must be greater than 0")
	}

	// Validate maxHourlyCost format if present
	if limits.MaxHourlyCost != nil && *limits.MaxHourlyCost != "" {
		// Could add more sophisticated cost validation here
		if len(*limits.MaxHourlyCost) == 0 {
			return fmt.Errorf("maxHourlyCost cannot be empty string")
		}
	}

	return nil
}

// Ensure GPUNodeClassValidator implements the webhook.CustomValidator interface
var _ webhook.CustomValidator = &GPUNodeClassValidator{}
