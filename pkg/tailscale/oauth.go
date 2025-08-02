package tailscale

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	// TailscaleAPI is the base URL for Tailscale API
	TailscaleAPI = "https://api.tailscale.com"

	// OAuth endpoints
	tokenURL = TailscaleAPI + "/api/v2/oauth/token"

	// API endpoints
	authKeysEndpoint = TailscaleAPI + "/api/v2/tailnet/%s/keys"
)

// Client represents a Tailscale OAuth client
type Client struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
	oauthConfig  *clientcredentials.Config
}

// AuthKeyOptions contains options for generating auth keys
type AuthKeyOptions struct {
	// Tags to apply to devices that use this key
	Tags []string `json:"tags,omitempty"`

	// Ephemeral indicates if devices should be removed when they go offline
	Ephemeral bool `json:"ephemeral,omitempty"`

	// Preauthorized indicates if devices should be pre-authorized
	Preauthorized bool `json:"preauthorized,omitempty"`

	// ExpirySeconds specifies when the key expires (max 90 days)
	ExpirySeconds int `json:"expirySeconds,omitempty"`
}

// AuthKeyResponse represents the response from creating an auth key
type AuthKeyResponse struct {
	Key           string    `json:"key"`
	ID            string    `json:"id"`
	Created       time.Time `json:"created"`
	Expires       time.Time `json:"expires"`
	Revoked       bool      `json:"revoked"`
	Tags          []string  `json:"tags"`
	Ephemeral     bool      `json:"ephemeral"`
	Preauthorized bool      `json:"preauthorized"`
}

// TailnetResponse represents a tailnet in the API
type TailnetResponse struct {
	Name string `json:"name"`
}

// NewClient creates a new Tailscale OAuth client
func NewClient(clientID, clientSecret string) *Client {
	config := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
		Scopes:       []string{"devices"},
	}

	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		oauthConfig:  config,
	}
}

// GenerateAuthKey generates a new Tailscale auth key using OAuth credentials
func (c *Client) GenerateAuthKey(ctx context.Context, options AuthKeyOptions) (string, error) {
	if err := c.validateCredentials(); err != nil {
		return "", fmt.Errorf("invalid credentials: %w", err)
	}

	// Get OAuth token
	token, err := c.oauthConfig.Token(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get OAuth token: %w", err)
	}

	// Get tailnet name (organization)
	tailnet, err := c.getTailnet(ctx, token)
	if err != nil {
		return "", fmt.Errorf("failed to get tailnet: %w", err)
	}

	// Set defaults for options
	if len(options.Tags) == 0 {
		options.Tags = []string{"tag:k8s"}
	}
	if options.ExpirySeconds == 0 {
		options.ExpirySeconds = 86400 // 24 hours default
	}
	// Default to ephemeral and preauthorized for k8s nodes
	options.Ephemeral = true
	options.Preauthorized = true

	// Create auth key
	authKey, err := c.createAuthKey(ctx, token, tailnet, options)
	if err != nil {
		return "", fmt.Errorf("failed to create auth key: %w", err)
	}

	return authKey, nil
}

// validateCredentials validates that the client has required credentials
func (c *Client) validateCredentials() error {
	if c.clientID == "" {
		return fmt.Errorf("client ID is required")
	}
	if c.clientSecret == "" {
		return fmt.Errorf("client secret is required")
	}
	return nil
}

// getTailnet gets the tailnet name for the authenticated user
func (c *Client) getTailnet(ctx context.Context, token *oauth2.Token) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", TailscaleAPI+"/api/v2/tailnet", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var tailnets map[string][]TailnetResponse
	if err := json.NewDecoder(resp.Body).Decode(&tailnets); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Get the first tailnet (organization)
	if tailnetList, ok := tailnets["tailnets"]; ok && len(tailnetList) > 0 {
		return tailnetList[0].Name, nil
	}

	return "", fmt.Errorf("no tailnets found")
}

// createAuthKey creates a new auth key in the specified tailnet
func (c *Client) createAuthKey(ctx context.Context, token *oauth2.Token, tailnet string, options AuthKeyOptions) (string, error) {
	url := fmt.Sprintf(authKeysEndpoint, tailnet)

	// Create request body
	requestBody, err := json.Marshal(map[string]interface{}{
		"capabilities": map[string]interface{}{
			"devices": map[string]interface{}{
				"create": map[string]interface{}{
					"reusable":      false,
					"ephemeral":     options.Ephemeral,
					"preauthorized": options.Preauthorized,
					"tags":          options.Tags,
				},
			},
		},
		"expirySeconds": options.ExpirySeconds,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var authKeyResp AuthKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&authKeyResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return authKeyResp.Key, nil
}

