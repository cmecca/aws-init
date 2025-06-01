// Package main provides AWS secret resolution functionality.
//
// This file contains functions for resolving AWS Secrets Manager and
// Systems Manager Parameter Store references in environment variables.
//
// # Secret Reference Format
//
// Secrets Manager (string values):
//
//	aws-secret:secret-name
//
// Secrets Manager (JSON key extraction):
//
//	aws-secret:secret-name#key
//
// Parameter Store (via Secrets Manager reference):
//
//	aws-secret:/aws/reference/secretsmanager/secret-name
//
// # Error Handling
//
// Functions implement retry logic with exponential backoff for transient
// AWS API errors. Context cancellation is respected for timeout handling.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

const (
	secretPrefix = "aws-secret:"
	maxRetries   = 3
	retryDelay   = 100 * time.Millisecond
)

// resolveSecrets processes environment variables and resolves AWS secret references.
//
// Environment variables with "aws-secret:" prefixes are resolved by fetching
// the corresponding values from AWS Secrets Manager or Parameter Store.
// Variables without the prefix are passed through unchanged.
//
// Parameters:
//   - ctx: context for request cancellation and timeouts
//   - env: slice of environment variables in "KEY=value" format
//
// Returns a new slice of environment variables with secrets resolved, or an error
// if any secret resolution fails.
//
// Example:
//
//	env := []string{
//	  "DATABASE_URL=aws-secret:myapp/prod#database_url",
//	  "API_KEY=aws-secret:myapp/prod#api_key",
//	  "NORMAL_VAR=regular_value",
//	}
//	resolved, err := resolveSecrets(ctx, env)
//	// resolved contains actual secret values instead of references
//
// Common errors returned by resolveSecrets:
//   - AWS credential errors: check IAM permissions and credential configuration
//   - Network errors: verify connectivity to AWS services
//   - Secret not found: ensure secret exists and name is correct
//   - JSON parsing errors: verify secret format for key extraction
func resolveSecrets(ctx context.Context, env []string) ([]string, error) {
	// Quick scan - do we have any secrets to resolve?
	hasSecrets := false
	for _, e := range env {
		if strings.Contains(e, secretPrefix) {
			hasSecrets = true
			break
		}
	}

	if !hasSecrets {
		return env, nil
	}

	// Initialize AWS clients
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRetryMaxAttempts(maxRetries))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	secretsClient := secretsmanager.NewFromConfig(cfg)
	ssmClient := ssm.NewFromConfig(cfg)

	var result []string
	for _, e := range env {
		name, value, found := strings.Cut(e, "=")
		if !found {
			continue // malformed env var
		}

		if strings.HasPrefix(value, secretPrefix) {
			resolved, err := resolveSecret(ctx, secretsClient, ssmClient, value)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve %s: %w", name, err)
			}
			value = resolved
		}

		result = append(result, name+"="+value)
	}

	return result, nil
}

// resolveSecret resolves a single AWS secret reference to its actual value.
//
// The ref parameter should be in one of these formats:
//   - "aws-secret:secret-name" for simple string secrets
//   - "aws-secret:secret-name#key" for JSON secrets with key extraction
//   - "aws-secret:/aws/reference/secretsmanager/param-name" for Parameter Store
//
// Returns the resolved secret value or an error if resolution fails.
//
// Example:
//
//	value, err := resolveSecret(ctx, sm, ssm, "aws-secret:myapp/prod#db_url")
func resolveSecret(ctx context.Context, secretsClient *secretsmanager.Client, ssmClient *ssm.Client, ref string) (string, error) {
	trimmed := strings.TrimPrefix(ref, secretPrefix)
	if trimmed == "" {
		return "", fmt.Errorf("empty secret reference")
	}

	// SSM Parameter Store reference
	if strings.HasPrefix(trimmed, "/aws/reference/secretsmanager/") {
		return getParameter(ctx, ssmClient, trimmed)
	}

	// Secrets Manager reference
	parts := strings.SplitN(trimmed, "#", 2)
	secretName := parts[0]
	if secretName == "" {
		return "", fmt.Errorf("empty secret name")
	}

	secretValue, err := getSecret(ctx, secretsClient, secretName)
	if err != nil {
		return "", err
	}

	// If no key specified, return the raw secret
	if len(parts) == 1 {
		return secretValue, nil
	}

	// Extract key from JSON secret
	key := parts[1]
	var parsed map[string]string
	if err := json.Unmarshal([]byte(secretValue), &parsed); err != nil {
		return "", fmt.Errorf("secret %s is not valid JSON: %w", secretName, err)
	}

	value, exists := parsed[key]
	if !exists {
		return "", fmt.Errorf("key %s not found in secret %s", key, secretName)
	}

	return value, nil
}

// getSecret retrieves a secret value from AWS Secrets Manager.
//
// The name parameter is the secret name or ARN. This function implements
// retry logic with exponential backoff for handling transient AWS API errors.
//
// Returns the secret string value or an error if retrieval fails after all retries.
func getSecret(ctx context.Context, client *secretsmanager.Client, name string) (string, error) {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(retryDelay * time.Duration(i)):
			}
		}

		resp, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(name),
		})
		if err != nil {
			lastErr = err
			continue
		}

		if resp.SecretString == nil {
			return "", fmt.Errorf("binary secrets not supported")
		}

		return *resp.SecretString, nil
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// getParameter retrieves a parameter value from AWS Systems Manager Parameter Store.
//
// The name parameter should be the full parameter path. Decryption is automatically
// enabled for SecureString parameters. This function implements retry logic with
// exponential backoff for handling transient AWS API errors.
//
// Returns the parameter value or an error if retrieval fails after all retries.
func getParameter(ctx context.Context, client *ssm.Client, name string) (string, error) {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(retryDelay * time.Duration(i)):
			}
		}

		resp, err := client.GetParameter(ctx, &ssm.GetParameterInput{
			Name:           aws.String(name),
			WithDecryption: aws.Bool(true),
		})
		if err != nil {
			lastErr = err
			continue
		}

		if resp.Parameter == nil || resp.Parameter.Value == nil {
			return "", fmt.Errorf("parameter has no value")
		}

		return *resp.Parameter.Value, nil
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}
