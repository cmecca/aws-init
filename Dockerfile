# Build stage
FROM golang:1.23-alpine AS builder

# Set build arguments
ARG VERSION=dev

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o aws-init .

# Final stage
FROM alpine:latest

# Install ca-certificates for AWS API calls
RUN apk --no-cache add ca-certificates

# Create non-root user (optional, but good practice)
RUN adduser -D -s /bin/sh awsuser

# Copy binary from builder
COPY --from=builder /app/aws-init /usr/local/bin/aws-init

# Make it executable
RUN chmod +x /usr/local/bin/aws-init

# Use non-root user
USER awsuser

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/aws-init"]