package tailscale

import (
	"context"
	"testing"
)

func TestGenerateAuthKey_validateCredentials(t *testing.T) {
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		wantErr      bool
	}{
		{
			name:         "empty client ID",
			clientID:     "",
			clientSecret: "test-client-secret",
			wantErr:      true,
		},
		{
			name:         "empty client secret",
			clientID:     "test-client-id",
			clientSecret: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.clientID, tt.clientSecret)

			ctx := context.Background()
			_, err := client.GenerateAuthKey(ctx, AuthKeyOptions{
				Tags:      []string{"tag:k8s"},
				Ephemeral: true,
			})

			if tt.wantErr {
				if err == nil {
					t.Errorf("GenerateAuthKey() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("GenerateAuthKey() unexpected error: %v", err)
				return
			}
		})
	}
}

func TestClient_validateCredentials(t *testing.T) {
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		wantErr      bool
	}{
		{
			name:         "valid credentials",
			clientID:     "test-client-id",
			clientSecret: "test-client-secret",
			wantErr:      false,
		},
		{
			name:         "empty client ID",
			clientID:     "",
			clientSecret: "test-client-secret",
			wantErr:      true,
		},
		{
			name:         "empty client secret",
			clientID:     "test-client-id",
			clientSecret: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				clientID:     tt.clientID,
				clientSecret: tt.clientSecret,
			}

			err := client.validateCredentials()
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

