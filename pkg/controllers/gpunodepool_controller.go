// Package controllers implements Kubernetes controllers for the TGP operator
package controllers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/config"
	"github.com/solanyn/tgp-operator/pkg/pricing"
	"github.com/solanyn/tgp-operator/pkg/providers"
	"github.com/solanyn/tgp-operator/pkg/providers/lambdalabs"
	"github.com/solanyn/tgp-operator/pkg/providers/paperspace"
	"github.com/solanyn/tgp-operator/pkg/providers/runpod"
)

const (
	GPUNodePoolFinalizerName = "tgp.io/gpunodepool-finalizer"
)

// GPUNodePoolReconciler reconciles a GPUNodePool object
type GPUNodePoolReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	Config       *config.OperatorConfig
	PricingCache *pricing.Cache
}

// +kubebuilder:rbac:groups=tgp.io,resources=gpunodepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tgp.io,resources=gpunodepools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tgp.io,resources=gpunodepools/finalizers,verbs=update
// +kubebuilder:rbac:groups=tgp.io,resources=gpunodeclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles GPUNodePool reconciliation
func (r *GPUNodePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("gpunodepool", req.NamespacedName)

	// Fetch the GPUNodePool instance
	var nodePool tgpv1.GPUNodePool
	if err := r.Get(ctx, req.NamespacedName, &nodePool); err != nil {
		if errors.IsNotFound(err) {
			log.Info("GPUNodePool resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get GPUNodePool")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if nodePool.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &nodePool, log)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&nodePool, GPUNodePoolFinalizerName) {
		controllerutil.AddFinalizer(&nodePool, GPUNodePoolFinalizerName)
		if err := r.Update(ctx, &nodePool); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Get referenced GPUNodeClass
	nodeClass, err := r.getNodeClass(ctx, &nodePool)
	if err != nil {
		log.Error(err, "Failed to get referenced GPUNodeClass")
		r.updateCondition(&nodePool, "NodeClassReady", metav1.ConditionFalse, "NodeClassNotFound", err.Error())
		if updateErr := r.Status().Update(ctx, &nodePool); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	// Update NodeClass ready condition
	r.updateCondition(&nodePool, "NodeClassReady", metav1.ConditionTrue, "NodeClassFound", "Referenced GPUNodeClass is available")

	// Check for unschedulable pods that need GPU nodes
	if err := r.handlePodDrivenProvisioning(ctx, &nodePool, nodeClass, log); err != nil {
		log.Error(err, "Failed to handle pod-driven provisioning")
		r.updateCondition(&nodePool, "Ready", metav1.ConditionFalse, "ProvisioningFailed", err.Error())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	r.updateCondition(&nodePool, "Ready", metav1.ConditionTrue, "Initialized", "GPUNodePool is ready for provisioning")
	if err := r.Status().Update(ctx, &nodePool); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	log.Info("GPUNodePool reconciled successfully", "nodeClass", nodeClass.Name)
	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

// handleDeletion handles GPUNodePool deletion
func (r *GPUNodePoolReconciler) handleDeletion(ctx context.Context, nodePool *tgpv1.GPUNodePool, log logr.Logger) (ctrl.Result, error) {
	log.Info("Handling GPUNodePool deletion")

	// Clean up all nodes created by this pool
	if err := r.cleanupPoolNodes(ctx, nodePool, log); err != nil {
		log.Error(err, "Failed to clean up pool nodes")
		// Don't fail deletion if cleanup fails, but log the error
		// In production, this might need retry logic or manual intervention
	}

	controllerutil.RemoveFinalizer(nodePool, GPUNodePoolFinalizerName)
	if err := r.Update(ctx, nodePool); err != nil {
		log.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	log.Info("GPUNodePool deleted successfully")
	return ctrl.Result{}, nil
}

// getNodeClass retrieves the GPUNodeClass referenced by the pool
func (r *GPUNodePoolReconciler) getNodeClass(ctx context.Context, nodePool *tgpv1.GPUNodePool) (*tgpv1.GPUNodeClass, error) {
	var nodeClass tgpv1.GPUNodeClass
	namespacedName := types.NamespacedName{
		Name: nodePool.Spec.NodeClassRef.Name,
		// GPUNodeClass is cluster-scoped, so no namespace
	}

	if err := r.Get(ctx, namespacedName, &nodeClass); err != nil {
		return nil, fmt.Errorf("failed to get GPUNodeClass %s: %w", nodePool.Spec.NodeClassRef.Name, err)
	}

	return &nodeClass, nil
}

// updateCondition updates a condition in the GPUNodePool status
func (r *GPUNodePoolReconciler) updateCondition(nodePool *tgpv1.GPUNodePool, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	// Find and update existing condition or append new one
	for i, existingCondition := range nodePool.Status.Conditions {
		if existingCondition.Type == conditionType {
			if existingCondition.Status != status {
				condition.LastTransitionTime = metav1.Now()
			} else {
				condition.LastTransitionTime = existingCondition.LastTransitionTime
			}
			nodePool.Status.Conditions[i] = condition
			return
		}
	}

	nodePool.Status.Conditions = append(nodePool.Status.Conditions, condition)
}

// handlePodDrivenProvisioning checks for unschedulable pods and provisions nodes as needed
func (r *GPUNodePoolReconciler) handlePodDrivenProvisioning(ctx context.Context, nodePool *tgpv1.GPUNodePool, nodeClass *tgpv1.GPUNodeClass, log logr.Logger) error {
	// List all pods and filter by phase
	var pods corev1.PodList
	if err := r.List(ctx, &pods); err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	// Filter for pending pods
	var pendingPods []corev1.Pod
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodPending {
			pendingPods = append(pendingPods, pod)
		}
	}

	// Filter pods that match this node pool's capabilities
	var matchingPods []corev1.Pod
	for _, pod := range pendingPods {
		if r.podMatchesPool(pod, nodePool, log) {
			matchingPods = append(matchingPods, pod)
		}
	}

	if len(matchingPods) == 0 {
		log.V(1).Info("No unschedulable pods found that match this pool")
		return nil
	}

	log.Info("Found pods that need GPU nodes", "count", len(matchingPods))

	// For now, provision one node per unschedulable pod (simple implementation)
	// TODO: Optimize by batching and considering existing capacity
	for _, pod := range matchingPods[:1] { // Start with just one pod to avoid over-provisioning
		if err := r.provisionNodeForPod(ctx, nodePool, nodeClass, &pod, log); err != nil {
			log.Error(err, "Failed to provision node for pod", "pod", pod.Name)
			continue
		}
		break // Only provision one node per reconcile cycle to avoid race conditions
	}

	return nil
}

// podMatchesPool checks if a pod's requirements can be satisfied by this node pool
func (r *GPUNodePoolReconciler) podMatchesPool(pod corev1.Pod, nodePool *tgpv1.GPUNodePool, log logr.Logger) bool {
	// Check if pod has GPU requirements
	hasGPURequirement := false
	for _, container := range pod.Spec.Containers {
		if container.Resources.Requests != nil {
			if _, hasGPU := container.Resources.Requests["nvidia.com/gpu"]; hasGPU {
				hasGPURequirement = true
				break
			}
		}
	}

	if !hasGPURequirement {
		return false
	}

	// Check if pod is unschedulable (no assigned node and has scheduling events)
	if pod.Spec.NodeName != "" {
		return false // Already scheduled
	}

	// Check node selector requirements match
	if pod.Spec.NodeSelector != nil {
		for key, value := range pod.Spec.NodeSelector {
			if !r.poolSupportsRequirement(nodePool, key, value) {
				return false
			}
		}
	}

	// Check if pod tolerates the node pool's taints
	for _, taint := range nodePool.Spec.Template.Spec.Taints {
		if !r.podToleratesTaint(pod, taint) {
			return false
		}
	}

	return true
}

// poolSupportsRequirement checks if the node pool can satisfy a node selector requirement
func (r *GPUNodePoolReconciler) poolSupportsRequirement(nodePool *tgpv1.GPUNodePool, key, value string) bool {
	// Check template labels
	if nodePool.Spec.Template.Metadata != nil && nodePool.Spec.Template.Metadata.Labels != nil {
		if labelValue, exists := nodePool.Spec.Template.Metadata.Labels[key]; exists {
			return labelValue == value
		}
	}

	// Check node requirements
	for _, req := range nodePool.Spec.Template.Spec.Requirements {
		if req.Key == key {
			for _, reqValue := range req.Values {
				if reqValue == value {
					return true
				}
			}
		}
	}

	return false
}

// podToleratesTaint checks if a pod tolerates a specific taint
func (r *GPUNodePoolReconciler) podToleratesTaint(pod corev1.Pod, taint corev1.Taint) bool {
	for _, toleration := range pod.Spec.Tolerations {
		if toleration.ToleratesTaint(&taint) {
			return true
		}
	}
	return false
}

// provisionNodeForPod provisions a new GPU node for the given pod
func (r *GPUNodePoolReconciler) provisionNodeForPod(ctx context.Context, nodePool *tgpv1.GPUNodePool, nodeClass *tgpv1.GPUNodeClass, pod *corev1.Pod, log logr.Logger) error {
	log.Info("Provisioning GPU node for pod", "pod", pod.Name, "namespace", pod.Namespace)

	// Extract GPU requirements from the pod
	gpuRequirement, err := r.extractGPURequirement(pod)
	if err != nil {
		return fmt.Errorf("failed to extract GPU requirement: %w", err)
	}

	// Select the best provider/region for this request
	selectedProvider, providerClient, err := r.selectBestProvider(ctx, nodeClass, gpuRequirement, log)
	if err != nil {
		return fmt.Errorf("failed to select provider: %w", err)
	}

	log.Info("Selected provider for provisioning",
		"provider", selectedProvider.Name,
		"gpuType", gpuRequirement.GPUType)

	// Create launch request
	launchRequest, err := r.createLaunchRequest(nodePool, nodeClass, gpuRequirement)
	if err != nil {
		return fmt.Errorf("failed to create launch request: %w", err)
	}

	// Launch the instance
	instance, err := providerClient.LaunchInstance(ctx, launchRequest)
	if err != nil {
		return fmt.Errorf("failed to launch instance: %w", err)
	}

	log.Info("Instance launched successfully",
		"instanceID", instance.ID,
		"provider", selectedProvider.Name)

	// Create Kubernetes Node object
	if err := r.createKubernetesNode(ctx, nodePool, instance, selectedProvider, log); err != nil {
		// If node creation fails, attempt to clean up the cloud instance
		if cleanupErr := providerClient.TerminateInstance(ctx, instance.ID); cleanupErr != nil {
			log.Error(cleanupErr, "Failed to cleanup instance after node creation failure", "instanceID", instance.ID)
		}
		return fmt.Errorf("failed to create Kubernetes node: %w", err)
	}

	log.Info("GPU node provisioned successfully",
		"pod", pod.Name,
		"instanceID", instance.ID,
		"provider", selectedProvider.Name)

	return nil
}

// GPURequirement represents GPU requirements extracted from a pod
type GPURequirement struct {
	GPUType  string
	GPUCount int
	Region   string // Preferred region from node selector or annotations
}

// extractGPURequirement extracts GPU requirements from a pod specification
func (r *GPUNodePoolReconciler) extractGPURequirement(pod *corev1.Pod) (*GPURequirement, error) {
	requirement := &GPURequirement{
		GPUCount: 1, // Default to 1 GPU
	}

	// Extract GPU count from resource requests
	for _, container := range pod.Spec.Containers {
		if container.Resources.Requests != nil {
			if gpuQuantity, exists := container.Resources.Requests["nvidia.com/gpu"]; exists {
				if count := int(gpuQuantity.Value()); count > 0 {
					requirement.GPUCount = count
					break
				}
			}
		}
	}

	// Extract GPU type from node selector or annotations
	if pod.Spec.NodeSelector != nil {
		if gpuType, exists := pod.Spec.NodeSelector["tgp.io/gpu-type"]; exists {
			requirement.GPUType = gpuType
		}
		if region, exists := pod.Spec.NodeSelector["tgp.io/region"]; exists {
			requirement.Region = region
		}
	}

	// Check annotations as fallback
	if requirement.GPUType == "" && pod.Annotations != nil {
		if gpuType, exists := pod.Annotations["tgp.io/gpu-type"]; exists {
			requirement.GPUType = gpuType
		}
	}

	// Default GPU type if not specified
	if requirement.GPUType == "" {
		requirement.GPUType = "RTX4090" // Default to popular GPU type
	}

	return requirement, nil
}

// selectBestProvider selects the optimal provider based on pricing and availability
func (r *GPUNodePoolReconciler) selectBestProvider(ctx context.Context, nodeClass *tgpv1.GPUNodeClass, requirement *GPURequirement, log logr.Logger) (*tgpv1.ProviderConfig, providers.ProviderClient, error) {
	var bestProvider *tgpv1.ProviderConfig
	var bestClient providers.ProviderClient
	var bestPrice float64 = float64(^uint(0) >> 1) // Max float64

	// Evaluate each enabled provider
	for _, providerConfig := range nodeClass.Spec.Providers {
		if providerConfig.Enabled != nil && !*providerConfig.Enabled {
			continue
		}

		// Get credentials for this provider
		namespace := providerConfig.CredentialsRef.Namespace
		if namespace == "" {
			namespace = "default" // fallback
		}
		credentials, err := r.Config.GetProviderCredentials(ctx, r.Client, providerConfig.Name, namespace)
		if err != nil {
			log.Error(err, "Failed to get credentials for provider", "provider", providerConfig.Name)
			continue
		}

		// Create provider client
		providerClient, err := r.createProviderClient(providerConfig.Name, credentials)
		if err != nil {
			log.Error(err, "Failed to create provider client", "provider", providerConfig.Name)
			continue
		}

		// Get pricing for this GPU type
		pricing, err := providerClient.GetNormalizedPricing(ctx, requirement.GPUType, requirement.Region)
		if err != nil {
			log.V(1).Info("Failed to get pricing", "provider", providerConfig.Name, "error", err)
			continue
		}

		// Apply priority weighting (lower priority number = higher preference)
		weightedPrice := pricing.PricePerHour
		if providerConfig.Priority > 0 {
			weightedPrice = pricing.PricePerHour * (1.0 + float64(providerConfig.Priority)*0.1)
		}

		if weightedPrice < bestPrice {
			bestPrice = weightedPrice
			bestProvider = &providerConfig
			bestClient = providerClient
		}

		log.V(1).Info("Evaluated provider",
			"provider", providerConfig.Name,
			"price", pricing.PricePerHour,
			"weightedPrice", weightedPrice)
	}

	if bestProvider == nil {
		return nil, nil, fmt.Errorf("no suitable provider found for GPU type %s", requirement.GPUType)
	}

	return bestProvider, bestClient, nil
}

// createProviderClient creates a provider client based on provider name
func (r *GPUNodePoolReconciler) createProviderClient(providerName, credentials string) (providers.ProviderClient, error) {
	switch providerName {
	case "runpod":
		return runpod.NewClient(credentials), nil
	case "lambdalabs":
		return lambdalabs.NewClient(credentials), nil
	case "paperspace":
		return paperspace.NewClient(credentials), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
}

// createLaunchRequest creates a launch request for the selected provider
func (r *GPUNodePoolReconciler) createLaunchRequest(nodePool *tgpv1.GPUNodePool, nodeClass *tgpv1.GPUNodeClass, requirement *GPURequirement) (*providers.LaunchRequest, error) {
	// Build user data script for node setup
	userData, err := r.buildUserDataScript(nodePool, nodeClass)
	if err != nil {
		return nil, fmt.Errorf("failed to build user data script: %w", err)
	}

	// Create labels for the instance
	labels := make(map[string]string)
	labels["tgp.io/nodepool"] = nodePool.Name
	labels["tgp.io/nodeclass"] = nodeClass.Name
	labels["tgp.io/gpu-type"] = requirement.GPUType
	if nodePool.Spec.Template.Metadata != nil && nodePool.Spec.Template.Metadata.Labels != nil {
		for k, v := range nodePool.Spec.Template.Metadata.Labels {
			labels[k] = v
		}
	}

	// Determine max price
	maxPrice := 10.0 // Default max price per hour
	if nodePool.Spec.MaxHourlyPrice != nil {
		if price, err := strconv.ParseFloat(*nodePool.Spec.MaxHourlyPrice, 64); err == nil {
			maxPrice = price
		}
	}

	return &providers.LaunchRequest{
		GPUType:      requirement.GPUType,
		Region:       requirement.Region,
		Image:        "talos", // TODO: Make configurable
		UserData:     userData,
		Labels:       labels,
		SpotInstance: false, // TODO: Support spot instances
		MaxPrice:     maxPrice,
		TalosConfig:  nodeClass.Spec.TalosConfig,
	}, nil
}

// buildUserDataScript creates the initialization script for new nodes
func (r *GPUNodePoolReconciler) buildUserDataScript(nodePool *tgpv1.GPUNodePool, nodeClass *tgpv1.GPUNodeClass) (string, error) {
	// TODO: Build proper Talos/cloud-init script
	// This would include:
	// - Talos OS configuration
	// - Tailscale setup if configured
	// - GPU driver installation
	// - Kubernetes node registration

	script := `#!/bin/bash
# TGP Operator Node Initialization Script
echo "Initializing GPU node for pool: ` + nodePool.Name + `"
# TODO: Add actual initialization logic
`
	return script, nil
}

// createKubernetesNode creates a Kubernetes Node object for the provisioned instance
func (r *GPUNodePoolReconciler) createKubernetesNode(ctx context.Context, nodePool *tgpv1.GPUNodePool, instance *providers.GPUInstance, provider *tgpv1.ProviderConfig, log logr.Logger) error {
	// Generate node name
	nodeName := fmt.Sprintf("tgp-%s-%s", nodePool.Name, instance.ID[:8])

	// Create Node object
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				"tgp.io/nodepool":                  nodePool.Name,
				"tgp.io/instance-id":               instance.ID,
				"tgp.io/provider":                  provider.Name,
				"kubernetes.io/arch":               "amd64",
				"kubernetes.io/os":                 "linux",
				"node.kubernetes.io/instance-type": "gpu",
			},
			Annotations: map[string]string{
				"tgp.io/created-at":  instance.CreatedAt.Format(time.RFC3339),
				"tgp.io/instance-id": instance.ID,
				"tgp.io/provider":    provider.Name,
			},
		},
		Spec: corev1.NodeSpec{
			// Node will be initially unschedulable until it's ready
			Unschedulable: true,
		},
		Status: corev1.NodeStatus{
			Phase: corev1.NodePending,
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeExternalIP,
					Address: instance.PublicIP,
				},
				{
					Type:    corev1.NodeInternalIP,
					Address: instance.PrivateIP,
				},
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:               corev1.NodeReady,
					Status:             corev1.ConditionFalse,
					Reason:             "Initializing",
					Message:            "Node is being initialized",
					LastTransitionTime: metav1.Now(),
				},
			},
		},
	}

	// Apply template labels and taints
	if nodePool.Spec.Template.Metadata != nil {
		if nodePool.Spec.Template.Metadata.Labels != nil {
			for k, v := range nodePool.Spec.Template.Metadata.Labels {
				node.Labels[k] = v
			}
		}
		if nodePool.Spec.Template.Metadata.Annotations != nil {
			for k, v := range nodePool.Spec.Template.Metadata.Annotations {
				node.Annotations[k] = v
			}
		}
	}

	// Apply taints from template
	if len(nodePool.Spec.Template.Spec.Taints) > 0 {
		node.Spec.Taints = append(node.Spec.Taints, nodePool.Spec.Template.Spec.Taints...)
	}

	// Set owner reference to enable cleanup
	if err := controllerutil.SetControllerReference(nodePool, node, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Create the node
	if err := r.Create(ctx, node); err != nil {
		return fmt.Errorf("failed to create Kubernetes node: %w", err)
	}

	log.Info("Kubernetes node created", "nodeName", nodeName, "instanceID", instance.ID)
	return nil
}

// cleanupPoolNodes drains and deletes all nodes created by this GPUNodePool
func (r *GPUNodePoolReconciler) cleanupPoolNodes(ctx context.Context, nodePool *tgpv1.GPUNodePool, log logr.Logger) error {
	// Find all nodes that belong to this pool
	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes, client.MatchingLabels{
		"tgp.io/nodepool": nodePool.Name,
	}); err != nil {
		return fmt.Errorf("failed to list nodes for pool %s: %w", nodePool.Name, err)
	}

	if len(nodes.Items) == 0 {
		log.Info("No nodes found for cleanup")
		return nil
	}

	log.Info("Found nodes to clean up", "count", len(nodes.Items))

	// Process each node for cleanup
	for _, node := range nodes.Items {
		if err := r.cleanupNode(ctx, &node, log); err != nil {
			log.Error(err, "Failed to cleanup node", "node", node.Name)
			// Continue with other nodes even if one fails
		}
	}

	return nil
}

// cleanupNode handles the cleanup of a single node
func (r *GPUNodePoolReconciler) cleanupNode(ctx context.Context, node *corev1.Node, log logr.Logger) error {
	log.Info("Cleaning up node", "node", node.Name)

	// First, cordon the node to prevent new pods from being scheduled
	if !node.Spec.Unschedulable {
		node.Spec.Unschedulable = true
		if err := r.Update(ctx, node); err != nil {
			return fmt.Errorf("failed to cordon node %s: %w", node.Name, err)
		}
		log.Info("Cordoned node", "node", node.Name)
	}

	// Drain the node by deleting pods
	if err := r.drainNode(ctx, node, log); err != nil {
		return fmt.Errorf("failed to drain node %s: %w", node.Name, err)
	}

	// TODO: Terminate the cloud instance
	// This would involve:
	// 1. Finding the provider that created this instance
	// 2. Extracting the instance ID from node labels/annotations
	// 3. Calling provider.TerminateInstance()

	// Delete the node from Kubernetes
	if err := r.Delete(ctx, node); err != nil {
		return fmt.Errorf("failed to delete node %s: %w", node.Name, err)
	}

	log.Info("Successfully cleaned up node", "node", node.Name)
	return nil
}

// drainNode removes all pods from a node
func (r *GPUNodePoolReconciler) drainNode(ctx context.Context, node *corev1.Node, log logr.Logger) error {
	// List all pods and filter by node name
	var pods corev1.PodList
	if err := r.List(ctx, &pods); err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	// Filter pods running on this node
	var nodePods []corev1.Pod
	for _, pod := range pods.Items {
		if pod.Spec.NodeName == node.Name {
			nodePods = append(nodePods, pod)
		}
	}

	if len(nodePods) == 0 {
		log.Info("No pods to drain from node", "node", node.Name)
		return nil
	}

	log.Info("Draining pods from node", "node", node.Name, "podCount", len(nodePods))

	// Delete non-DaemonSet pods
	for _, pod := range nodePods {
		// Skip pods that are already terminating
		if pod.DeletionTimestamp != nil {
			continue
		}

		// Skip DaemonSet pods (they will be handled by the DaemonSet controller)
		if r.isDaemonSetPod(&pod) {
			continue
		}

		// Skip static pods (controlled by kubelet)
		if r.isStaticPod(&pod) {
			continue
		}

		log.Info("Deleting pod from node", "pod", pod.Name, "namespace", pod.Namespace, "node", node.Name)
		if err := r.Delete(ctx, &pod); err != nil {
			log.Error(err, "Failed to delete pod", "pod", pod.Name, "namespace", pod.Namespace)
			// Continue with other pods
		}
	}

	return nil
}

// isDaemonSetPod checks if a pod is controlled by a DaemonSet
func (r *GPUNodePoolReconciler) isDaemonSetPod(pod *corev1.Pod) bool {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

// isStaticPod checks if a pod is a static pod (controlled by kubelet)
func (r *GPUNodePoolReconciler) isStaticPod(pod *corev1.Pod) bool {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "Node" {
			return true
		}
	}
	// Static pods also often have specific annotations
	if pod.Annotations != nil {
		if _, exists := pod.Annotations["kubernetes.io/config.source"]; exists {
			return true
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager
func (r *GPUNodePoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tgpv1.GPUNodePool{}).
		Owns(&corev1.Node{}). // Watch nodes created by this controller
		Complete(r)
}
