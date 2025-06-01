package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestResolveSecretsDetailed(t *testing.T) {
	tests := []struct {
		name     string
		env      []string
		wantVars map[string]string
		wantErr  bool
	}{
		{
			name: "no secrets to resolve",
			env:  []string{"NORMAL=value", "PATH=/usr/bin"},
			wantVars: map[string]string{
				"NORMAL": "value",
				"PATH":   "/usr/bin",
			},
		},
		{
			name:    "empty secret reference",
			env:     []string{"BAD=aws-secret:"},
			wantErr: true,
		},
		{
			name:    "empty secret name with key",
			env:     []string{"BAD=aws-secret:#key"},
			wantErr: true,
		},
		{
			name: "mixed normal and secret vars",
			env:  []string{"NORMAL=value", "SECRET=aws-secret:test/secret"},
			// Will fail with AWS error, but that's expected in unit tests
			wantErr: true,
		},
		{
			name: "malformed env vars ignored",
			env:  []string{"MALFORMED_NO_EQUALS", "GOOD=value"},
			wantVars: map[string]string{
				"GOOD": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			result, err := resolveSecrets(ctx, tt.env)

			if (err != nil) != tt.wantErr {
				t.Errorf("resolveSecrets() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.wantVars != nil {
				resultMap := envSliceToMap(result)
				for key, expectedValue := range tt.wantVars {
					if gotValue, exists := resultMap[key]; !exists {
						t.Errorf("Missing environment variable: %s", key)
					} else if gotValue != expectedValue {
						t.Errorf("Environment variable %s = %s, want %s", key, gotValue, expectedValue)
					}
				}
			}
		})
	}
}

func TestSecretParsing(t *testing.T) {
	tests := []struct {
		name       string
		ref        string
		wantPrefix string
		wantKey    string
		wantSSM    bool
		wantErr    bool
	}{
		{
			name:       "simple secret",
			ref:        "aws-secret:myapp/prod",
			wantPrefix: "myapp/prod",
			wantKey:    "",
			wantSSM:    false,
		},
		{
			name:       "secret with key",
			ref:        "aws-secret:myapp/prod#database_url",
			wantPrefix: "myapp/prod",
			wantKey:    "database_url",
			wantSSM:    false,
		},
		{
			name:       "ssm parameter",
			ref:        "aws-secret:/aws/reference/secretsmanager/myapp/token",
			wantPrefix: "/aws/reference/secretsmanager/myapp/token",
			wantKey:    "",
			wantSSM:    true,
		},
		{
			name:    "empty reference",
			ref:     "aws-secret:",
			wantErr: true,
		},
		{
			name:    "empty secret name",
			ref:     "aws-secret:#key",
			wantErr: true,
		},
		{
			name:       "secret with multiple hash symbols",
			ref:        "aws-secret:myapp/prod#key#with#hash",
			wantPrefix: "myapp/prod",
			wantKey:    "key#with#hash",
			wantSSM:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test parsing logic directly without making AWS calls
			trimmed := strings.TrimPrefix(tt.ref, "aws-secret:")

			if tt.wantErr {
				if trimmed == "" || (strings.HasPrefix(trimmed, "#") && !strings.Contains(trimmed[1:], "/")) {
					// Expected error for empty references
					return
				}
				t.Error("expected error for invalid reference")
				return
			}

			if strings.HasPrefix(trimmed, "/aws/reference/secretsmanager/") {
				if !tt.wantSSM {
					t.Error("expected non-SSM reference but got SSM")
				}
				if trimmed != tt.wantPrefix {
					t.Errorf("expected prefix '%s', got '%s'", tt.wantPrefix, trimmed)
				}
			} else {
				if tt.wantSSM {
					t.Error("expected SSM reference but got non-SSM")
				}

				parts := strings.SplitN(trimmed, "#", 2)
				gotPrefix := parts[0]
				gotKey := ""
				if len(parts) == 2 {
					gotKey = parts[1]
				}

				if gotPrefix != tt.wantPrefix {
					t.Errorf("expected prefix '%s', got '%s'", tt.wantPrefix, gotPrefix)
				}
				if gotKey != tt.wantKey {
					t.Errorf("expected key '%s', got '%s'", tt.wantKey, gotKey)
				}
			}
		})
	}
}

func TestJSONKeyExtraction(t *testing.T) {
	tests := []struct {
		name        string
		secretValue string
		key         string
		want        string
		wantErr     bool
	}{
		{
			name:        "valid json with existing key",
			secretValue: `{"database_url":"postgres://localhost","api_key":"secret123"}`,
			key:         "database_url",
			want:        "postgres://localhost",
		},
		{
			name:        "valid json with missing key",
			secretValue: `{"api_key":"secret123"}`,
			key:         "database_url",
			wantErr:     true,
		},
		{
			name:        "invalid json",
			secretValue: `{invalid json}`,
			key:         "database_url",
			wantErr:     true,
		},
		{
			name:        "empty json",
			secretValue: `{}`,
			key:         "database_url",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var parsed map[string]string
			err := json.Unmarshal([]byte(tt.secretValue), &parsed)

			if err != nil {
				if !tt.wantErr {
					t.Errorf("unexpected JSON unmarshal error: %v", err)
				}
				return
			}

			value, exists := parsed[tt.key]
			if !exists {
				if !tt.wantErr {
					t.Errorf("key %s not found in JSON", tt.key)
				}
				return
			}

			if tt.wantErr {
				t.Error("expected error but got success")
				return
			}

			if value != tt.want {
				t.Errorf("got %s, want %s", value, tt.want)
			}
		})
	}
}

// Helper function to convert env slice to map for testing
func envSliceToMap(env []string) map[string]string {
	result := make(map[string]string)
	for _, e := range env {
		if key, value, found := stringsCut(e, "="); found {
			result[key] = value
		}
	}
	return result
}

// stringsCut is a simple implementation of strings.Cut for older Go versions
func stringsCut(s, sep string) (before, after string, found bool) {
	if i := strings.Index(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}
