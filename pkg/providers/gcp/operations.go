package gcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/solanyn/tgp-operator/pkg/providers"
	"google.golang.org/api/option"
)

// waitForZoneOperation waits for a GCP zone operation to complete
func (c *Client) waitForZoneOperation(ctx context.Context, opName, zone string) error {
	if opName == "" {
		return fmt.Errorf("operation name is empty")
	}

	// Create zone operations client for monitoring
	opts := []option.ClientOption{
		option.WithCredentialsJSON([]byte(c.credentials)),
	}

	zoneOpsClient, err := compute.NewZoneOperationsRESTClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create zone operations client: %w", err)
	}
	defer zoneOpsClient.Close()

	// Poll operation status
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(10 * time.Minute) // 10 minute timeout

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-timeout:
			return fmt.Errorf("operation timed out after 10 minutes")

		case <-ticker.C:
			currentOp, err := zoneOpsClient.Get(ctx, &computepb.GetZoneOperationRequest{
				Project:   c.projectID,
				Zone:      zone,
				Operation: opName,
			})
			if err != nil {
				return fmt.Errorf("failed to get operation status: %w", err)
			}

			status := currentOp.GetStatus()

			switch status {
			case computepb.Operation_DONE:
				// Check for errors
				if currentOp.GetError() != nil {
					var errorMsgs []string
					for _, e := range currentOp.GetError().GetErrors() {
						errorMsgs = append(errorMsgs, e.GetMessage())
					}
					return fmt.Errorf("operation failed: %s", strings.Join(errorMsgs, "; "))
				}
				return nil

			case computepb.Operation_RUNNING, computepb.Operation_PENDING:
				// Continue polling
				continue

			default:
				return fmt.Errorf("unexpected operation status: %s", status.String())
			}
		}
	}
}

// waitForGlobalOperation waits for a global operation to complete (e.g., image operations)
func (c *Client) waitForGlobalOperation(ctx context.Context, op *computepb.Operation) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}

	opts := []option.ClientOption{
		option.WithCredentialsJSON([]byte(c.credentials)),
	}

	globalOpsClient, err := compute.NewGlobalOperationsRESTClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create global operations client: %w", err)
	}
	defer globalOpsClient.Close()

	opName := op.GetName()
	if opName == "" {
		return fmt.Errorf("operation name is empty")
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(20 * time.Minute) // Longer timeout for global operations

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-timeout:
			return fmt.Errorf("global operation timed out after 20 minutes")

		case <-ticker.C:
			currentOp, err := globalOpsClient.Get(ctx, &computepb.GetGlobalOperationRequest{
				Project:   c.projectID,
				Operation: opName,
			})
			if err != nil {
				return fmt.Errorf("failed to get global operation status: %w", err)
			}

			status := currentOp.GetStatus()

			switch status {
			case computepb.Operation_DONE:
				if currentOp.GetError() != nil {
					var errorMsgs []string
					for _, e := range currentOp.GetError().GetErrors() {
						errorMsgs = append(errorMsgs, e.GetMessage())
					}
					return fmt.Errorf("global operation failed: %s", strings.Join(errorMsgs, "; "))
				}
				return nil

			case computepb.Operation_RUNNING, computepb.Operation_PENDING:
				continue

			default:
				return fmt.Errorf("unexpected global operation status: %s", status.String())
			}
		}
	}
}

// checkQuotas validates that we have sufficient quotas for the requested resources
func (c *Client) checkQuotas(ctx context.Context, region, gpuType string, count int) error {
	// Quota checking would require additional permissions and complexity
	// For now, we'll let GCP return quota errors during instance creation
	// This keeps the implementation simpler and the errors are informative
	return nil
}

// ensureFirewallRules ensures necessary firewall rules exist for Kubernetes nodes
func (c *Client) ensureFirewallRules(ctx context.Context) error {
	// Firewall management is typically handled at the infrastructure level
	// Users should configure their VPC and firewall rules separately
	// The operator focuses on instance provisioning
	return nil
}

// cleanupOrphanedResources cleans up any orphaned resources
func (c *Client) cleanupOrphanedResources(ctx context.Context) error {
	// Resource cleanup is typically handled by Kubernetes garbage collection
	// and the operator's finalizers. This method is not currently used.
	return nil
}

// validateInstanceConfig validates the instance configuration before launch
func (c *Client) validateInstanceConfig(req *providers.LaunchRequest) error {
	// Validate GPU type
	supportedGPUs := c.GetProviderInfo().SupportedGPUTypes
	gpuSupported := false
	for _, supported := range supportedGPUs {
		if strings.EqualFold(supported, req.GPUType) {
			gpuSupported = true
			break
		}
	}

	if !gpuSupported {
		return fmt.Errorf("unsupported GPU type: %s", req.GPUType)
	}

	// Validate region
	supportedRegions := c.GetProviderInfo().SupportedRegions
	regionSupported := false
	for _, supported := range supportedRegions {
		if strings.Contains(strings.ToLower(supported), strings.ToLower(req.Region)) {
			regionSupported = true
			break
		}
	}

	if !regionSupported {
		return fmt.Errorf("unsupported region: %s", req.Region)
	}

	// Validate user data size (GCP metadata limit is 256KB)
	if len(req.UserData) > 256*1024 {
		return fmt.Errorf("user data too large: %d bytes (max 256KB)", len(req.UserData))
	}

	return nil
}

// getOperationProgress returns progress information for long-running operations
func (c *Client) getOperationProgress(ctx context.Context, op *computepb.Operation, zone string) (int32, string, error) {
	if zone == "" {
		// Global operation
		opts := []option.ClientOption{
			option.WithCredentialsJSON([]byte(c.credentials)),
		}

		globalOpsClient, err := compute.NewGlobalOperationsRESTClient(ctx, opts...)
		if err != nil {
			return 0, "", fmt.Errorf("failed to create global operations client: %w", err)
		}
		defer globalOpsClient.Close()

		currentOp, err := globalOpsClient.Get(ctx, &computepb.GetGlobalOperationRequest{
			Project:   c.projectID,
			Operation: op.GetName(),
		})
		if err != nil {
			return 0, "", fmt.Errorf("failed to get global operation: %w", err)
		}

		return currentOp.GetProgress(), currentOp.GetStatusMessage(), nil
	} else {
		// Zone operation
		opts := []option.ClientOption{
			option.WithCredentialsJSON([]byte(c.credentials)),
		}

		zoneOpsClient, err := compute.NewZoneOperationsRESTClient(ctx, opts...)
		if err != nil {
			return 0, "", fmt.Errorf("failed to create zone operations client: %w", err)
		}
		defer zoneOpsClient.Close()

		currentOp, err := zoneOpsClient.Get(ctx, &computepb.GetZoneOperationRequest{
			Project:   c.projectID,
			Zone:      zone,
			Operation: op.GetName(),
		})
		if err != nil {
			return 0, "", fmt.Errorf("failed to get zone operation: %w", err)
		}

		return currentOp.GetProgress(), currentOp.GetStatusMessage(), nil
	}
}
