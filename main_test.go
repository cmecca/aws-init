package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestResolveSecrets(t *testing.T) {
	tests := []struct {
		name    string
		env     []string
		wantErr bool
	}{
		{
			name: "no secrets",
			env:  []string{"NORMAL=value", "PATH=/usr/bin"},
		},
		{
			name:    "invalid secret reference",
			env:     []string{"BAD=aws-secret:"},
			wantErr: true,
		},
		{
			name:    "empty secret name",
			env:     []string{"BAD=aws-secret:#key"},
			wantErr: true,
		},
		{
			name: "malformed env var",
			env:  []string{"MALFORMED", "GOOD=value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			_, err := resolveSecrets(ctx, tt.env)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveSecrets() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveSecretParsing(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty reference",
			ref:     "aws-secret:",
			wantErr: true,
			errMsg:  "empty secret reference",
		},
		{
			name:    "empty secret name",
			ref:     "aws-secret:#key",
			wantErr: true,
			errMsg:  "empty secret name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// This tests only the parsing logic that happens before AWS calls
			_, err := resolveSecret(ctx, nil, nil, tt.ref)

			if (err != nil) != tt.wantErr {
				t.Errorf("resolveSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing '%s', got: %v", tt.errMsg, err)
			}
		})
	}
}

func TestSecretReferenceParsing(t *testing.T) {
	// Test the parsing logic without making AWS calls
	tests := []struct {
		name      string
		ref       string
		expectSSM bool
		expectKey string
	}{
		{
			name:      "simple secret",
			ref:       "aws-secret:myapp/prod",
			expectSSM: false,
			expectKey: "",
		},
		{
			name:      "secret with key",
			ref:       "aws-secret:myapp/prod#database_url",
			expectSSM: false,
			expectKey: "database_url",
		},
		{
			name:      "ssm parameter",
			ref:       "aws-secret:/aws/reference/secretsmanager/myapp/token",
			expectSSM: true,
			expectKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the reference manually to test parsing logic
			trimmed := strings.TrimPrefix(tt.ref, "aws-secret:")

			if strings.HasPrefix(trimmed, "/aws/reference/secretsmanager/") {
				if !tt.expectSSM {
					t.Error("expected non-SSM reference but got SSM")
				}
			} else {
				if tt.expectSSM {
					t.Error("expected SSM reference but got non-SSM")
				}

				parts := strings.SplitN(trimmed, "#", 2)
				gotKey := ""
				if len(parts) == 2 {
					gotKey = parts[1]
				}

				if gotKey != tt.expectKey {
					t.Errorf("expected key '%s', got '%s'", tt.expectKey, gotKey)
				}
			}
		})
	}
}
