package providers

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type TGPResourceRequirements struct {
	GPUCount        int64
	MinVRAM         int64
	PreferredVendor string
	WorkloadType    string
}

func ExtractTGPRequirements(pod *corev1.Pod) (*TGPResourceRequirements, bool) {
	requirements := &TGPResourceRequirements{}
	hasTGPResources := false

	// Check for tgp.io/gpu resource
	if gpuQuantity, exists := pod.Spec.Containers[0].Resources.Requests[ResourceTGPGPU]; exists && !gpuQuantity.IsZero() {
		requirements.GPUCount = gpuQuantity.Value()
		hasTGPResources = true
	}

	// Check for tgp.io/memory resource (VRAM)
	if memQuantity, exists := pod.Spec.Containers[0].Resources.Requests[ResourceTGPMemory]; exists && !memQuantity.IsZero() {
		requirements.MinVRAM = memQuantity.Value() / (1024 * 1024 * 1024) // Convert bytes to GB
		hasTGPResources = true
	}

	// Extract vendor preference from annotations
	if vendor := pod.Annotations[AnnotationVendor]; vendor != "" {
		requirements.PreferredVendor = strings.ToLower(vendor)
	}

	// Extract workload type from annotations
	if workload := pod.Annotations[AnnotationWorkload]; workload != "" {
		requirements.WorkloadType = strings.ToLower(workload)
	}

	return requirements, hasTGPResources
}

func HasTGPResources(pod *corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if gpuQuantity, exists := container.Resources.Requests[ResourceTGPGPU]; exists && !gpuQuantity.IsZero() {
			return true
		}
		if memQuantity, exists := container.Resources.Requests[ResourceTGPMemory]; exists && !memQuantity.IsZero() {
			return true
		}
	}
	return false
}

func SelectOptimalGPU(requirements *TGPResourceRequirements, offers []GPUOffer) *GPUOffer {
	var candidates []GPUOffer

	// Filter by VRAM requirement
	for _, offer := range offers {
		if requirements.MinVRAM > 0 && offer.Memory < requirements.MinVRAM {
			continue
		}
		candidates = append(candidates, offer)
	}

	if len(candidates) == 0 {
		return nil
	}

	// Apply vendor preference
	if requirements.PreferredVendor != "" {
		var preferred []GPUOffer
		for _, candidate := range candidates {
			if matchesVendor(candidate.GPUType, requirements.PreferredVendor) {
				preferred = append(preferred, candidate)
			}
		}
		if len(preferred) > 0 {
			candidates = preferred
		}
	}

	// Select cheapest option
	best := &candidates[0]
	for i := range candidates {
		if candidates[i].HourlyPrice < best.HourlyPrice {
			best = &candidates[i]
		}
	}

	return best
}

func matchesVendor(gpuType, preferredVendor string) bool {
	gpuType = strings.ToUpper(gpuType)
	switch strings.ToLower(preferredVendor) {
	case "nvidia":
		return strings.HasPrefix(gpuType, "NVIDIA_")
	case "amd":
		return strings.HasPrefix(gpuType, "AMD_")
	case "intel":
		return strings.HasPrefix(gpuType, "INTEL_")
	}
	return true
}

func ToVendorSpecificResources(requirements *TGPResourceRequirements, gpuType string) map[corev1.ResourceName]resource.Quantity {
	resources := make(map[corev1.ResourceName]resource.Quantity)

	if requirements.GPUCount > 0 {
		vendor := getGPUVendor(gpuType)
		switch vendor {
		case "nvidia":
			resources["nvidia.com/gpu"] = *resource.NewQuantity(requirements.GPUCount, resource.DecimalSI)
		case "amd":
			resources["amd.com/gpu"] = *resource.NewQuantity(requirements.GPUCount, resource.DecimalSI)
		case "intel":
			resources["gpu.intel.com/i915"] = *resource.NewQuantity(requirements.GPUCount, resource.DecimalSI)
		}
	}

	return resources
}

func getGPUVendor(gpuType string) string {
	gpuType = strings.ToUpper(gpuType)
	if strings.HasPrefix(gpuType, "NVIDIA_") {
		return "nvidia"
	}
	if strings.HasPrefix(gpuType, "AMD_") {
		return "amd"
	}
	if strings.HasPrefix(gpuType, "INTEL_") {
		return "intel"
	}
	return "unknown"
}
