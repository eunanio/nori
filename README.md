# Nori

**Terraform/OpenTofu Module Management Tool**

Nori is a CLI tool for packaging and deploying Terraform & OpenTofu modules as containers that can be versioned and distributed on any docker registry. 

## Features

-  **Easy module packaging** - package and publish your terraform modules with a single command 
-  **Universal registry support** - Works with GHCR, ECR, ACR, GCR, Docker Hub, Gitea, and more
-  **OpenTofu 1.10+ compatible** - Native OCI module source support
-  **Release Management** - Deploy modules with static values

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/eunanio/nori.git
cd nori

# Build
make build

# Install to GOPATH/bin
make install
```

### Binary Releases

Download pre-built binaries from the [Releases](https://github.com/eunanio/nori/releases) page.

> [!NOTE]
> Nori includes a GitHub Action for packaging and pushing OpenTofu modules to OCI registries.
> See the [Action Documentation](./action/README.md) for usage examples and configuration options.

## Quick Start

### Package a Module

```bash
# Package a zip file
nori package ghcr.io/myorg/s3:v1.0.0 ./module.zip

# Package with description
nori package ghcr.io/myorg/s3:v1.0.0 ./module.tar.gz \
  --description "Creates an S3 bucket with versioning enabled"
```

## OpenTofu Integration

Nori-packaged modules are compatible with OpenTofu 1.10+ native OCI module support:

```hcl
module "s3_bucket" {
  source = "oci://ghcr.io/eunanio/oci-terraform-modules/s3?tag=v1.0.0"

  bucket_name        = "my-bucket"
  versioning_enabled = true
}
```

### Deploy a Module

```yaml
# values.yml
bucket_name: my-application-bucket
versioning_enabled: true
tags:
  Environment: production
  Team: platform
```

```bash
# Deploy the module
nori deploy ghcr.io/eunanio/oci-terraform-modules/s3:v1.0.0 -f values.yaml
```

## Command Reference

### `nori package`

Package a Terraform/OpenTofu module into an OCI artifact and push to registry

```bash
nori package <registry>/<namespace>/<name>:<tag> <module-file>
```

**Options:**
- `--config <file>` - Path to Terraform configuration for metadata extraction
- `--description <text>` - Module description
- `--annotation <key=value>` - Add custom OCI annotations
- `--package-only` - Package without pushing; write artifact to local file
- `-o, --output <file>` - Output file path (used with --package-only)
- `--insecure` - Allow insecure (HTTP) registry connections

### `nori release create`

Create a new release by deploying an OpenTofu module from registry.

```bash
nori release create <release_name> <oci-tag> [flags]
```

**Options:**
- `-f, --values <file>` - Path to values.yaml file
- `--set <key=value>` - Set values on the command line
- `--annotation <key=value>` - Add annotations to the release
- `--auto-approve` - Automatically approve apply
- `--plan-only` - Only create plan, don't apply
- `--parallelism <n>` - Number of parallel operations (default: 10)
- `--var-file <file>` - Additional var files
- `--backend-config <key=value>` - Backend configuration
- `--target <resource>` - Specific resources to target
- `--upgrade` - Upgrade providers during init
- `--description <text>` - Description for this release

Check out [USAGE.md](./docs/USAGE.md) for additional documentation

## Global Flags

- `-v, --verbose` - Enable debug logging
- `--config <path>` - Path to Nori configuration file
- `--registry-config <path>` - Override Docker config location
- `--insecure` - Allow insecure registry connections

## Configuration

Nori uses a configuration file at `~/.nori/config.yaml`:

```yaml
# Default registry to use when not specified
default_registry: ghcr.io

# Registry-specific configuration
registries:
  registry.example.com:
    insecure: true
    skip_tls_verify: false

# Deployment configuration
deploy:
  runtime: tofu        # tofu only, terraform is currently not supported due to lack of oci support.
  parallelism: 10
  auto_approve: false

# Logging configuration
logging:
  level: info          # debug, info, warn, error
  format: text         # text or json
```

## Authentication

Nori uses Docker credentials for registry authentication. Credentials are resolved in this order:

1. **Credential helpers** (`credHelpers` in Docker config)
2. **Default credential store** (`credsStore` in Docker config)
3. **Base64 auth in Docker config** (`auths` section)
4. **Environment variables**

### Environment Variables

Registry-specific variables:
- `GHCR_IO_USERNAME` / `GHCR_IO_PASSWORD` - GitHub Container Registry
- `GITHUB_TOKEN` or `GH_TOKEN` - GitHub token (for GHCR)
- `AZURE_CLIENT_ID` / `AZURE_CLIENT_SECRET` - Azure Container Registry
- `GOOGLE_APPLICATION_CREDENTIALS` - Google Artifact Registry

Generic variables:
- `NORI_REGISTRY_USERNAME` / `NORI_REGISTRY_PASSWORD`

## Development

```bash
# Download dependencies
make deps

# Run tests
make test

# Run linter
make lint

# Build for all platforms
make build-all
```


