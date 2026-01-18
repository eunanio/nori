# Nori GitHub Action Dockerfile
# Downloads pre-built nori binary from GitHub releases

FROM debian:bookworm-slim

# Install runtime dependencies
# - ca-certificates: for HTTPS connections to registries
# - zip: for creating archives from directories
# - jq: for parsing JSON annotations and GitHub API responses
# - curl: for downloading nori binary
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    zip \
    jq \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Download pre-built nori binary from GitHub releases
# Using GitHub API to get the latest release asset URL
ARG NORI_VERSION=latest
RUN set -ex && \
    if [ "$NORI_VERSION" = "latest" ]; then \
        DOWNLOAD_URL=$(curl -fsSL https://api.github.com/repos/eunanio/nori/releases/latest | \
            jq -r '.assets[] | select(.name | test("linux_amd64.*\\.tar\\.gz$")) | .browser_download_url'); \
    else \
        DOWNLOAD_URL="https://github.com/eunanio/nori/releases/download/${NORI_VERSION}/nori_${NORI_VERSION#v}_linux_amd64.tar.gz"; \
    fi && \
    echo "Downloading nori from: ${DOWNLOAD_URL}" && \
    mkdir -p /tmp/nori-download && \
    curl -fsSL "${DOWNLOAD_URL}" | tar -xz -C /tmp/nori-download && \
    find /tmp/nori-download -name "nori" -type f -exec mv {} /usr/local/bin/nori \; && \
    chmod +x /usr/local/bin/nori && \
    rm -rf /tmp/nori-download

# Verify nori is executable
RUN nori version

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
