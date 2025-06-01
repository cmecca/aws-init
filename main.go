// Package main provides aws-init, a lightweight init process that resolves
// AWS secrets at runtime and executes commands with proper signal handling.
//
// aws-init is designed to run as PID 1 in containers, resolving AWS Secrets Manager
// and Parameter Store values at startup before launching the target application.
//
// # Usage
//
//	aws-init command [args...]
//	aws-init -v
//	aws-init -h
//
// # Environment Variables
//
// Environment variables with aws-secret: prefixes are resolved at startup:
//
//	DATABASE_URL=aws-secret:myapp/prod#database_url
//	API_KEY=aws-secret:/aws/reference/secretsmanager/myapp/token
//
// # Secret Reference Formats
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
// # Authentication
//
// Uses standard AWS credential chain including:
//   - IAM Roles for Service Accounts (IRSA) in Kubernetes
//   - Instance profiles on EC2
//   - Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
//   - AWS credential files
//
// # Signal Handling
//
// When running as PID 1, aws-init properly forwards signals to child processes
// and handles graceful shutdown with a 10-second timeout before force-killing.
//
// # Examples
//
// Basic usage:
//
//	export DATABASE_URL="aws-secret:myapp/prod#db_url"
//	aws-init python app.py
//
// Dockerfile integration:
//
//	FROM python:3.11-slim
//	COPY aws-init /usr/local/bin/
//	ENV DATABASE_URL=aws-secret:myapp/prod#database_url
//	ENV API_KEY=aws-secret:myapp/prod#api_key
//	ENTRYPOINT ["/usr/local/bin/aws-init", "python", "app.py"]
//
// Kubernetes deployment:
//
//	apiVersion: apps/v1
//	kind: Deployment
//	spec:
//	  template:
//	    spec:
//	      serviceAccountName: my-app-sa  # with IRSA
//	      containers:
//	      - name: app
//	        image: myapp:latest
//	        command: ["/usr/local/bin/aws-init", "python", "app.py"]
//	        env:
//	        - name: DATABASE_URL
//	          value: "aws-secret:myapp/prod#database_url"
//
// Health check:
//
//	aws-init -h
//
// Version information:
//
//	aws-init -v
//
// # Security
//
// Secrets are resolved once at startup and passed to the child process via
// environment variables. No secrets are logged or persisted to disk.
// Use minimal IAM permissions for production deployments.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

var (
	version = "dev"
)

const (
	healthCheckTimeout = 5 * time.Second
)

func main() {
	versionFlag := flag.Bool("v", false, "show version")
	healthFlag := flag.Bool("h", false, "health check")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("aws-init %s\n", version)
		os.Exit(0)
	}

	if *healthFlag {
		healthCheck()
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		log.Fatal("usage: aws-init command [args...]")
	}

	if os.Getpid() == 1 {
		log.Println("aws-init: running as PID 1")
	}

	// Resolve AWS secrets in environment
	env, err := resolveSecrets(context.Background(), os.Environ())
	if err != nil {
		log.Fatalf("aws-init: %v", err)
	}

	// Execute command with signal handling
	os.Exit(execute(args[0], args[1:], env))
}

// healthCheck verifies AWS credentials and connectivity.
//
// This function attempts to call AWS STS GetCallerIdentity to verify that:
//   - AWS credentials are properly configured
//   - Network connectivity to AWS services works
//   - IAM permissions allow basic AWS API access
//
// The check times out after 5 seconds to prevent hanging in problematic environments.
// This is designed for use in container health checks and debugging authentication issues.
//
// Example Kubernetes usage:
//
//	livenessProbe:
//	  exec:
//	    command: ["/usr/local/bin/aws-init", "-h"]
//	  initialDelaySeconds: 10
//	  periodSeconds: 30
//
// Exits with code 0 on success, or logs fatal error and exits with code 1 on failure.
func healthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("health check failed: %v", err)
	}

	client := sts.NewFromConfig(cfg)
	_, err = client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Fatalf("health check failed: %v", err)
	}

	fmt.Println("health check passed")
}
