# Nori Usage Guide

This guide provides detailed instructions for using Nori to manage OpenTofu modules as OCI artifacts with release management.

## Table of Contents

1. [Getting Started](#getting-started)
2. [Configuration](#configuration)
3. [Packaging Modules](#packaging-modules)
4. [Artifact Signing](#artifact-signing)
5. [Registry Authentication](#registry-authentication)
6. [Release Management](#release-management)
7. [Working with Registries](#working-with-registries)
8. [Advanced Usage](#advanced-usage)

## Getting Started

### Installation

```bash
# Build from source
git clone https://github.com/eunanio/nori.git
cd nori
make build

# Add to PATH
export PATH=$PATH:$(pwd)/bin
```

### Basic Workflow

Nori provides release management for OpenTofu modules stored in OCI registries:

1. **Package** your OpenTofu module as an OCI artifact
2. **Create a release** to deploy the module with values
3. **Upgrade releases** with new values or module versions
4. **Track release history** stored in OCI

```bash
# 1. Package and push module to registry
nori package ghcr.io/myorg/s3-bucket:v1.0.0 ./terraform-module

# 2. Configure state repository
nori config set state_repository ghcr.io/myorg/nori-state

# 3. Create a release
nori release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml

# 4. Upgrade with new values
nori release upgrade my-bucket -f new-values.yaml

# 5. List all releases
nori release list
```

## Configuration

### State Repository

Nori stores release state (metadata, values, Terraform state) as OCI artifacts. Configure your state repository:

```bash
# Set state repository
nori config set state_repository ghcr.io/myorg/nori-releases

# View current configuration
nori config get state_repository

# List all config settings
nori config get
```

### Configuration File

Configuration is stored in `~/.nori/config.yaml`:

```yaml
state_repository: ghcr.io/myorg/nori-releases
registries:
  registry.local:5000:
    insecure: true
```

## Packaging Modules

### Module Archive Requirements

Nori accepts `.zip` or `.tar.gz` archives containing OpenTofu configuration files. The archive should contain the module files at the root level or in a single subdirectory.

**Recommended structure:**
```
module.zip
├── main.tf
├── variables.tf
├── outputs.tf
├── versions.tf
└── README.md
```

### Creating Archives

**Using zip:**
```bash
cd terraform-module/
zip -r ../module.zip .
```

**Using tar.gz:**
```bash
cd terraform-module/
tar -czvf ../module.tar.gz .
```

### Packaging with Metadata

Extract metadata from OpenTofu configuration:
```bash
nori package ghcr.io/org/module:v1.0.0 module.zip \
  --config ./terraform-module \
  --description "My awesome Terraform module"
```

Add custom annotations:
```bash
nori package ghcr.io/org/module:v1.0.0 module.zip \
  --annotation org.opencontainers.image.authors="team@example.com" \
  --annotation org.opencontainers.image.documentation="https://docs.example.com"
```

### Package Only (Offline Packaging)

Package a module without pushing to a registry. This is useful for CI/CD pipelines where you want to separate the packaging and pushing steps, or for offline environments.

```bash
# Package to default file (module-v1.0.0.zip)
nori package ghcr.io/org/module:v1.0.0 module.zip --package-only

# Package to a custom output file
nori package ghcr.io/org/module:v1.0.0 module.tar.gz --package-only --output ./dist/module.zip

# Later, push the packaged file
nori push ghcr.io/org/module:v1.0.0 ./dist/module.zip
```

The `--package-only` flag:
- Validates and processes the archive (converts tar.gz to zip if needed)
- Builds OCI annotations from metadata
- Writes the processed artifact to a local file
- Outputs instructions for pushing later with `nori push`

### README Auto-Detection

Nori automatically detects `README.md` files at the root of your module archive. When a README is found:

- The README content is stored as a separate layer in the OCI artifact
- The `io.nori.readme` annotation is added to the manifest
- Users can view the README using `nori inspect --readme`

This allows module consumers to view documentation without downloading the entire module.

## Artifact Signing

Nori supports cryptographic signing of OCI artifacts using cosign-compatible keys. This allows you to verify the authenticity and integrity of modules before deployment.

### Generating Signing Keys

Generate a new key pair for signing artifacts:

```bash
# Generate keys in current directory (creates nori.key and nori.pub)
nori config generate-key-pair

# Generate keys in a specific directory
nori config generate-key-pair --output ~/.nori/keys
```

You will be prompted to enter a password to protect the private key.

### Signing Artifacts

Sign an artifact when packaging:

```bash
# Package and sign with a key file
nori package ghcr.io/myorg/s3-bucket:v1.0.0 module.zip --sign --key nori.key

# Package and sign using keyless OIDC (for CI/CD environments)
nori package ghcr.io/myorg/s3-bucket:v1.0.0 module.zip --sign --keyless
```

When using `--sign --key`, you will be prompted for the private key password.

### Verifying Signatures

Check if an artifact is signed and verify its signature:

```bash
# Inspect an artifact (shows signature status)
nori inspect ghcr.io/myorg/s3-bucket:v1.0.0

# Verify signature with a public key
nori inspect ghcr.io/myorg/s3-bucket:v1.0.0 --key nori.pub

# Simple verification check (returns true/false)
nori inspect ghcr.io/myorg/s3-bucket:v1.0.0 --verify

# Verify with specific public key
nori inspect ghcr.io/myorg/s3-bucket:v1.0.0 --verify --key nori.pub
```

The `--verify` flag outputs `true` or `false` and can be used in scripts:

```bash
if nori inspect ghcr.io/myorg/s3-bucket:v1.0.0 --verify --key nori.pub; then
    echo "Signature verified!"
    nori release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml
else
    echo "Signature verification failed!"
    exit 1
fi
```

### Signing in GitHub Actions

Use keyless OIDC signing in GitHub Actions workflows:

```yaml
name: Publish Module

on:
  push:
    tags:
      - 'v*'

jobs:
  publish:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
      id-token: write  # Required for keyless signing
    steps:
      - uses: actions/checkout@v4

      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Package and Sign Module
        uses: eunanio/nori/action@v1
        with:
          registry: ghcr.io
          repository: ${{ github.repository_owner }}/my-module
          tag: ${{ github.ref_name }}
          module-path: ./terraform
          sign: 'true'  # Enable keyless OIDC signing
```

Keyless signing uses GitHub's OIDC provider to generate short-lived certificates, eliminating the need to manage signing keys.

## Registry Authentication

### Using Docker Credentials

Nori uses your existing Docker credentials. Log in using Docker CLI:
```bash
docker login ghcr.io
docker login registry.example.com
```

Or use Nori's login command:
```bash
nori login ghcr.io
nori login registry.example.com
```

### Supported Registries

| Registry | Login Example |
|----------|---------------|
| GitHub Container Registry | `nori login ghcr.io` |
| Docker Hub | `nori login docker.io` |
| AWS ECR | `aws ecr get-login-password \| nori login --password-stdin <account>.dkr.ecr.<region>.amazonaws.com` |
| Google Artifact Registry | `gcloud auth print-access-token \| nori login --password-stdin <region>-docker.pkg.dev` |
| Azure Container Registry | `az acr login --name myregistry` |
| Gitea | `nori login gitea.example.com` |

### Environment Variables

For CI/CD pipelines, use environment variables:

```bash
# GitHub Actions
export GITHUB_TOKEN=${{ secrets.GITHUB_TOKEN }}

# Generic credentials
export NORI_REGISTRY_USERNAME=myuser
export NORI_REGISTRY_PASSWORD=mytoken
```

### Credential Helpers

Nori supports Docker credential helpers:

- **macOS:** `osxkeychain`
- **Windows:** `wincred`
- **Linux:** `pass`, `secretservice`

Configure in `~/.docker/config.json`:
```json
{
  "credsStore": "osxkeychain",
  "credHelpers": {
    "gcr.io": "gcloud",
    "public.ecr.aws": "ecr-login"
  }
}
```

### OpenTofu Authentication

When Nori runs OpenTofu commands (`tofu init`, `tofu plan`, `tofu apply`, `tofu destroy`), it automatically configures OpenTofu to use the appropriate Docker credential helper for OCI registry authentication.

**How it works:**

1. Nori detects the credential helper based on your platform:
   - **macOS:** Uses `osxkeychain`
   - **Windows:** Uses `wincred`
   - **Linux:** Uses `pass` or `secretservice` if available

2. If no platform default is available (common on Linux/WSL), Nori reads the `credsStore` setting from your Docker config (`~/.docker/config.json`)

3. Nori generates a temporary OpenTofu CLI configuration file with the credential helper and sets `TF_CLI_CONFIG_FILE` before running OpenTofu commands

**WSL (Windows Subsystem for Linux):**

In WSL environments, Docker Desktop typically configures `~/.docker/config.json` with:
```json
{
  "credsStore": "wincred"
}
```

Nori automatically detects this and configures OpenTofu to use `wincred` for authentication, allowing seamless access to private registries like `ghcr.io`.

**Troubleshooting:**

If you encounter authentication errors with OpenTofu:

1. Verify you're logged in: `docker login ghcr.io` or `nori login ghcr.io`
2. Check your Docker config has credentials: `cat ~/.docker/config.json`
3. Ensure the credential helper is accessible: `docker-credential-wincred list` (or appropriate helper)
4. Use verbose mode to see what credential helper Nori detects: `nori --verbose release create ...`

## Release Management

Nori provides release management for OpenTofu modules. Release state is stored as OCI artifacts for versioning and portability.

### Values Files

Create a `values.yaml` file with variable values:

```yaml
# values.yaml
bucket_name: my-application-bucket
region: us-east-1
versioning_enabled: true

tags:
  Environment: production
  Team: platform
  CostCenter: "12345"

lifecycle_rules:
  - id: archive
    enabled: true
    transition:
      days: 90
      storage_class: GLACIER
```

### Creating Releases

Create a new release from a module:

```bash
# Basic create
nori release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml


# With inline values
nori release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 \
  --set bucket_name=my-bucket \
  --set versioning_enabled=true

# With annotations
nori release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml \
  --annotation team=platform \
  --annotation environment=production

# Plan only (preview changes)
nori release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml --plan-only
```

### Upgrading Releases

Upgrade an existing release with new values or module version:

```bash
# Upgrade with new values (same module version)
nori release upgrade my-bucket -f values.yaml

# Upgrade to a new module version using -t flag
nori release upgrade my-bucket -t v2.0.0 -f values.yaml

# Upgrade to a new module version (full reference)
nori release upgrade my-bucket ghcr.io/myorg/s3-bucket:v2.0.0

# Upgrade with inline values
nori release upgrade my-bucket --set bucket_name=new-bucket

# Reuse previous values and merge with new ones
nori release upgrade my-bucket -f values.yaml --reuse-values

# Reset to default values
nori release upgrade my-bucket -f values.yaml --reset-values

# Add annotations to upgrade
nori release upgrade my-bucket -f values.yaml --annotation release-notes="Fixed bug"
```

### Listing Releases

View all releases stored in the state repository:

```bash
# List all releases
nori release list

# Output as JSON
nori release list -o json

# Show all releases including failed
nori release list -a
```

### Release History

View version history for a release:

```bash
# Show history
nori release history my-bucket

# Limit to last 5 versions
nori release history my-bucket --limit 5

# Output as JSON
nori release history my-bucket -o json
```

### Inspecting Releases

View detailed information about a release:

```bash
# Inspect latest version
nori release inspect my-bucket

# Inspect specific version
nori release inspect my-bucket --version v1.2.0

# Output as JSON
nori release inspect my-bucket -o json
```

### destroying Releases

Remove a release and destroy its infrastructure:

```bash
# destroy with confirmation
nori release destroy my-bucket

# Auto-approve destruction
nori release destroy my-bucket --auto-approve
```

## Working with Registries

### Listing Module Versions

```bash
nori list ghcr.io/myorg/s3-bucket
```

Output:
```
Tags for ghcr.io/myorg/s3-bucket:
  v1.0.0
  v1.1.0
  v1.2.0
  latest
```

### Inspecting Artifacts

View artifact metadata without downloading:
```bash
nori inspect ghcr.io/myorg/s3-bucket:v1.0.0
```

JSON output for scripting:
```bash
nori inspect ghcr.io/myorg/s3-bucket:v1.0.0 --format json
```

View the module's README (if available):
```bash
nori inspect ghcr.io/myorg/s3-bucket:v1.0.0 --readme
```

### Pulling Artifacts

Download artifacts locally:
```bash
nori pull ghcr.io/myorg/s3-bucket:v1.0.0
nori pull ghcr.io/myorg/s3-bucket:v1.0.0 -o my-module.tar.gz
```

### Pushing Pre-built Artifacts

Push an existing archive:
```bash
nori push ghcr.io/myorg/s3-bucket:v1.1.0 module.tar.gz
```

## Advanced Usage

### OpenTofu Integration

Use Nori-packaged modules directly in OpenTofu 1.10+:

```hcl
terraform {
  required_version = ">= 1.10"
}

module "s3_bucket" {
  source = "oci://ghcr.io/myorg/s3-bucket?tag=v1.0.0"

  bucket_name        = "my-bucket"
  versioning_enabled = true
}
```


### Backend Configuration

Pass backend configuration at deploy time:
```bash
nori release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml \
  --backend-config bucket=my-tf-state \
  --backend-config key=s3-bucket/terraform.tfstate \
  --backend-config region=us-east-1
```

### Targeting Resources

Apply to specific resources:
```bash
nori release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml \
  --target aws_s3_bucket.main \
  --target aws_s3_bucket_versioning.main
```

### Parallelism

Control resource operation parallelism:
```bash
nori release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml --parallelism 20
```

### Verbose Logging

Enable debug output:
```bash
nori --verbose release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml
```

### Insecure Registries

Allow HTTP connections (not recommended for production):
```bash
nori --insecure package registry.local:5000/module:v1.0.0 module.zip
```

Or configure in `~/.nori/config.yaml`:
```yaml
registries:
  registry.local:5000:
    insecure: true
```

### Legacy Deploy Command

For one-off deployments without release tracking, use the deploy command:
```bash
nori deploy ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml
```

This deploys without storing state in OCI and is useful for testing or ephemeral environments.
