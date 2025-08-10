package imagefactory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	DefaultFactoryURL = "https://factory.talos.dev"
	DefaultTimeout    = 30 * time.Second
)

// Client provides access to Talos Image Factory API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Image Factory API client
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultFactoryURL
	}
	
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

// Extension represents a system extension
type Extension struct {
	Image string `json:"image"`
}

// SchematicRequest represents a request to create a schematic
type SchematicRequest struct {
	Customization struct {
		SystemExtensions struct {
			OfficialExtensions []string `json:"officialExtensions"`
		} `json:"systemExtensions"`
	} `json:"customization"`
}

// SchematicResponse represents the response from creating a schematic
type SchematicResponse struct {
	ID string `json:"id"`
}

// Platform represents a supported platform
type Platform string

const (
	PlatformVultr        Platform = "vultr"
	PlatformGCP          Platform = "gcp" 
	PlatformDigitalOcean Platform = "digital-ocean"
)

var supportedPlatforms = map[Platform]bool{
	PlatformVultr:        true,
	PlatformGCP:          true,
	PlatformDigitalOcean: true,
}

// IsPlatformSupported checks if a platform is supported
func IsPlatformSupported(platform Platform) bool {
	return supportedPlatforms[platform]
}

// CreateSchematic creates a new schematic with the specified extensions
func (c *Client) CreateSchematic(ctx context.Context, extensions []string) (string, error) {
	req := SchematicRequest{}
	req.Customization.SystemExtensions.OfficialExtensions = extensions
	
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/schematics", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var schematicResp SchematicResponse
	if err := json.NewDecoder(resp.Body).Decode(&schematicResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	
	return schematicResp.ID, nil
}

// GetImageURL generates the image URL for a specific platform and version
func (c *Client) GetImageURL(schematicID string, version string, platform Platform) (string, error) {
	if !IsPlatformSupported(platform) {
		return "", fmt.Errorf("unsupported platform: %s", platform)
	}
	
	return fmt.Sprintf("%s/image/%s/%s/%s-amd64.raw.gz", c.baseURL, schematicID, version, platform), nil
}

// GenerateImageForExtensions creates a schematic and returns the image URL
func (c *Client) GenerateImageForExtensions(ctx context.Context, extensions []string, version string, platform Platform) (string, error) {
	schematicID, err := c.CreateSchematic(ctx, extensions)
	if err != nil {
		return "", fmt.Errorf("failed to create schematic: %w", err)
	}
	
	imageURL, err := c.GetImageURL(schematicID, version, platform)
	if err != nil {
		return "", fmt.Errorf("failed to generate image URL: %w", err)
	}
	
	return imageURL, nil
}

// GetCommonExtensions returns extensions commonly needed for GPU nodes
func GetCommonExtensions() []string {
	return []string{
		"siderolabs/nvidia-container-toolkit-production",
		"siderolabs/nvidia-fabricmanager-production",
		"siderolabs/amdgpu",
		"siderolabs/tailscale",
		"siderolabs/amd-ucode",
		"siderolabs/intel-ucode",
		"siderolabs/i915-ucode",
	}
}