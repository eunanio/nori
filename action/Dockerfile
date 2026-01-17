# Nori GitHub Action Dockerfile
# Multi-stage build: compile nori, then package in minimal runtime image

# =============================================================================
# Stage 1: Build nori binary
# =============================================================================
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go module files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the nori binary with version info
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /nori \
    ./cmd/nori

# =============================================================================
# Stage 2: Runtime image
# =============================================================================
FROM alpine:3.20

# Install runtime dependencies
# - ca-certificates: for HTTPS connections to registries
# - zip: for creating archives from directories
# - jq: for parsing JSON annotations
# - bash: for entrypoint script
RUN apk add --no-cache \
    ca-certificates \
    zip \
    jq \
    bash

# Copy nori binary from builder
COPY --from=builder /nori /usr/local/bin/nori

# Verify nori is executable
RUN nori version

# Copy entrypoint script
COPY action/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Set up non-root user for security (optional, GitHub Actions runs as root)
# RUN adduser -D -u 1000 nori && chown nori:nori /usr/local/bin/nori

ENTRYPOINT ["/entrypoint.sh"]

