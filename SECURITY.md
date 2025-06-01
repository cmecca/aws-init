# Security Policy

## Supported Versions

| Version | Supported |
| ------- |-----------|
| 1.x.x   | Yes       |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report security issues by email to: [mec@moonlab.org]

You should receive a response within 48 hours. If the issue is confirmed, we will:

1. Acknowledge the report within 48 hours
2. Provide an estimated timeline for a fix
3. Release a security update as soon as possible
4. Credit you in the security advisory (if desired)

## Security Considerations

aws-init handles sensitive data (AWS secrets) and runs as PID 1. Key security practices:

### Secrets Handling
- Secrets are only held in memory temporarily
- No secrets are logged or written to disk
- Process memory is not swapped to disk
- Child processes inherit resolved secrets via environment

### Process Security
- Runs with minimal privileges required for AWS access
- Child processes are isolated in process groups
- Signal handling prevents resource leaks
- Graceful shutdown prevents data corruption

### AWS Permissions
Use minimal IAM permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue",
        "ssm:GetParameter"
      ],
      "Resource": [
        "arn:aws:secretsmanager:region:account:secret:your-secrets/*",
        "arn:aws:ssm:region:account:parameter/your/parameters/*"
      ]
    }
  ]
}
```

### Container Security
- Use distroless or minimal base images
- Run as non-root user when possible
- Set read-only filesystem where appropriate
- Use security contexts in Kubernetes

## Known Security Considerations

1. **Secrets in Process Environment**: Child processes can access all resolved secrets via their environment
2. **Memory Dumps**: Secrets exist in process memory and could be exposed in core dumps
3. **Process Monitoring**: Tools with process monitoring capabilities may observe environment variables

These are inherent limitations of any init process that resolves secrets.
