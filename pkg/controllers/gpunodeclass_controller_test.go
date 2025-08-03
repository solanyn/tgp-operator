package controllers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/config"
)

func TestGPUNodeClassReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = tgpv1.AddToScheme(scheme)

	nodeClass := &tgpv1.GPUNodeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node-class",
		},
		Spec: tgpv1.GPUNodeClassSpec{
			Providers: []tgpv1.ProviderConfig{
				{
					Name:     "runpod",
					Priority: 1,
					Enabled:  &[]bool{true}[0],
					CredentialsRef: tgpv1.SecretKeyRef{
						Name: "test-secret",
						Key:  "api-key",
					},
				},
			},
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(nodeClass).
		Build()

	reconciler := &GPUNodeClassReconciler{
		Client: client,
		Log:    logr.Discard(),
		Scheme: scheme,
		Config: &config.OperatorConfig{},
	}

	ctx := context.Background()
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "test-node-class",
		},
	}

	// Test reconciliation
	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if result.RequeueAfter == 0 {
		t.Error("Expected requeue after some time")
	}

	// Verify finalizer was added
	var updatedNodeClass tgpv1.GPUNodeClass
	if err := client.Get(ctx, req.NamespacedName, &updatedNodeClass); err != nil {
		t.Fatalf("Failed to get updated GPUNodeClass: %v", err)
	}

	found := false
	for _, finalizer := range updatedNodeClass.Finalizers {
		if finalizer == GPUNodeClassFinalizerName {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected finalizer to be added")
	}
}

func TestGPUNodeClassReconciler_handleDeletion(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = tgpv1.AddToScheme(scheme)

	now := metav1.Now()
	nodeClass := &tgpv1.GPUNodeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-node-class",
			DeletionTimestamp: &now,
			Finalizers:        []string{GPUNodeClassFinalizerName},
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(nodeClass).
		Build()

	reconciler := &GPUNodeClassReconciler{
		Client: client,
		Log:    logr.Discard(),
		Scheme: scheme,
		Config: &config.OperatorConfig{},
	}

	ctx := context.Background()
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "test-node-class",
		},
	}

	// Test deletion handling
	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if result.RequeueAfter > 0 {
		t.Error("Expected no requeue for successful deletion")
	}

	// Object should no longer exist after successful deletion
	var updatedNodeClass tgpv1.GPUNodeClass
	err = client.Get(ctx, req.NamespacedName, &updatedNodeClass)
	if err == nil {
		// If object still exists, check that finalizer was removed
		for _, finalizer := range updatedNodeClass.Finalizers {
			if finalizer == GPUNodeClassFinalizerName {
				t.Error("Expected finalizer to be removed")
			}
		}
	}
	// It's also acceptable if the object is deleted entirely
}
