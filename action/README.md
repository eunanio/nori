# Nori Package Action

A GitHub Action to package and push Terraform/OpenTofu modules as OCI artifacts using [Nori](https://github.com/eunanio/nori).

## Features

- Package Terraform/OpenTofu modules as OCI-compliant artifacts
- Push to any OCI registry (GHCR, ECR, GCR, ACR, Docker Hub, etc.)
- Automatic archive creation from directories
- Custom annotations and metadata support
- Compatible with `docker/login-action` for authentication

## Prerequisites

This action requires authentication to be configured **before** it runs. Use [`docker/login-action`](https://github.com/docker/login-action) to authenticate to your registry:

```yaml
- name: Login to Registry
  uses: docker/login-action@v3
  with:
    registry: ghcr.io
    username: ${{ github.actor }}
    password: ${{ secrets.GITHUB_TOKEN }}
```

## Usage

### Basic Example

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
    steps:
      - uses: actions/checkout@v4

      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Package and Push Module
        uses: eunanio/nori/action@v1
        with:
          registry: ghcr.io
          repository: ${{ github.repository_owner }}/my-terraform-module
          tag: ${{ github.ref_name }}
          module-path: ./terraform
```

### From a Directory

Package a directory containing Terraform files:

```yaml
- name: Package Module
  uses: eunanio/nori/action@v1
  with:
    registry: ghcr.io
    repository: myorg/s3-bucket
    tag: v1.0.0
    module-path: ./modules/s3-bucket
    description: "S3 bucket module with versioning and encryption"
```

### From an Archive

Package a pre-built archive:

```yaml
- name: Package Module
  uses: eunanio/nori/action@v1
  with:
    registry: ghcr.io
    repository: myorg/vpc
    tag: v2.1.0
    module-path: ./dist/vpc-module.zip
```

### With Custom Annotations

Add OCI annotations for better discoverability:

```yaml
- name: Package Module
  uses: eunanio/nori/action@v1
  with:
    registry: ghcr.io
    repository: myorg/eks-cluster
    tag: v1.0.0
    module-path: ./modules/eks
    description: "Production-ready EKS cluster module"
    annotations: |
      {
        "org.opencontainers.image.authors": "platform-team@example.com",
        "org.opencontainers.image.documentation": "https://docs.example.com/modules/eks",
        "org.opencontainers.image.source": "${{ github.server_url }}/${{ github.repository }}"
      }
```

### Using Outputs

Access the reference and digest in subsequent steps:

```yaml
- name: Package Module
  id: package
  uses: eunanio/nori/action@v1
  with:
    registry: ghcr.io
    repository: myorg/my-module
    tag: v1.0.0
    module-path: ./terraform

- name: Display Results
  run: |
    echo "Published: ${{ steps.package.outputs.reference }}"
    echo "Digest: ${{ steps.package.outputs.digest }}"
```

### Multi-Module Repository

For repositories containing multiple Terraform modules, you can configure workflows to publish only the modules that have changed.

**Repository structure:**

```
terraform-modules/
├── .github/workflows/
│   └── publish-modules.yml
├── modules/
│   ├── vpc/
│   │   ├── main.tf
│   │   ├── variables.tf
│   │   └── outputs.tf
│   ├── eks/
│   │   ├── main.tf
│   │   ├── variables.tf
│   │   └── outputs.tf
│   └── rds/
│       ├── main.tf
│       ├── variables.tf
│       └── outputs.tf
```

#### Publish Changed Modules on Merged PR

This workflow detects which modules changed in a PR and publishes only those modules when the PR is merged:

```yaml
name: Publish Changed Modules

on:
  pull_request:
    types: [closed]
    paths:
      - 'modules/**'

jobs:
  detect-changes:
    if: github.event.pull_request.merged == true
    runs-on: ubuntu-latest
    outputs:
      modules: ${{ steps.changes.outputs.modules }}
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Detect changed modules
        id: changes
        run: |
          CHANGED_MODULES=$(git diff --name-only ${{ github.event.pull_request.base.sha }} ${{ github.event.pull_request.head.sha }} \
            | grep '^modules/' \
            | cut -d'/' -f2 \
            | sort -u \
            | jq -R -s -c 'split("\n") | map(select(length > 0))')
          echo "modules=${CHANGED_MODULES}" >> $GITHUB_OUTPUT
          echo "Changed modules: ${CHANGED_MODULES}"

  publish:
    needs: detect-changes
    if: needs.detect-changes.outputs.modules != '[]'
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    strategy:
      matrix:
        module: ${{ fromJson(needs.detect-changes.outputs.modules) }}
    steps:
      - uses: actions/checkout@v4

      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Package and Push ${{ matrix.module }}
        uses: eunanio/nori/action@v1
        with:
          registry: ghcr.io
          repository: ${{ github.repository }}/${{ matrix.module }}
          tag: 0.0.0-dev.${{ github.sha }}
          module-path: ./modules/${{ matrix.module }}
          description: "Terraform ${{ matrix.module }} module"
          annotations: |
            {
              "org.opencontainers.image.source": "${{ github.server_url }}/${{ github.repository }}",
              "org.opencontainers.image.revision": "${{ github.sha }}"
            }
```

#### Tag-Based Module Release

Use module-prefixed tags to release specific modules. A tag like `eks-v1.0.0` publishes to `ghcr.io/{owner}/{repo}/eks:v1.0.0`:

```yaml
name: Release Module

on:
  push:
    tags:
      - '*-v*'

jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v4

      - name: Parse tag
        id: parse
        run: |
          TAG="${GITHUB_REF#refs/tags/}"
          # Extract module name (everything before -v)
          MODULE="${TAG%-v*}"
          # Extract version (everything after {module}-)
          VERSION="${TAG#${MODULE}-}"
          echo "module=${MODULE}" >> $GITHUB_OUTPUT
          echo "version=${VERSION}" >> $GITHUB_OUTPUT
          echo "Releasing ${MODULE} at ${VERSION}"

      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Package and Push
        uses: eunanio/nori/action@v1
        with:
          registry: ghcr.io
          repository: ${{ github.repository }}/${{ steps.parse.outputs.module }}
          tag: ${{ steps.parse.outputs.version }}
          module-path: ./modules/${{ steps.parse.outputs.module }}
          description: "Terraform ${{ steps.parse.outputs.module }} module"
          annotations: |
            {
              "org.opencontainers.image.source": "${{ github.server_url }}/${{ github.repository }}",
              "org.opencontainers.image.revision": "${{ github.sha }}"
            }
```

**Creating releases:**

```bash
# Release EKS module v1.0.0 → ghcr.io/{owner}/{repo}/eks:v1.0.0
git tag eks-v1.0.0
git push origin eks-v1.0.0

# Release VPC module v2.1.0 → ghcr.io/{owner}/{repo}/vpc:v2.1.0
git tag vpc-v2.1.0
git push origin vpc-v2.1.0
```

## Inputs

| Input | Description | Required | Default |
|-------|-------------|----------|---------|
| `registry` | OCI registry hostname (e.g., `ghcr.io`, `docker.io`) | Yes | - |
| `repository` | Repository path within the registry (e.g., `myorg/my-module`) | Yes | - |
| `tag` | Version tag for the module (e.g., `v1.0.0`, `latest`) | Yes | - |
| `module-path` | Path to module archive (`.zip`, `.tar.gz`) or directory | Yes | - |
| `description` | Module description for OCI annotations | No | `""` |
| `annotations` | Custom OCI annotations as JSON object | No | `{}` |
| `insecure` | Allow insecure (HTTP) registry connections | No | `false` |

## Outputs

| Output | Description |
|--------|-------------|
| `reference` | Full OCI reference of the pushed artifact (`registry/repository:tag`) |
| `digest` | SHA256 digest of the pushed artifact |

## Registry Examples

### GitHub Container Registry (GHCR)

```yaml
- uses: docker/login-action@v3
  with:
    registry: ghcr.io
    username: ${{ github.actor }}
    password: ${{ secrets.GITHUB_TOKEN }}

- uses: eunanio/nori/action@v1
  with:
    registry: ghcr.io
    repository: ${{ github.repository_owner }}/my-module
    tag: v1.0.0
    module-path: ./terraform
```

### Amazon ECR

```yaml
- name: Configure AWS Credentials
  uses: aws-actions/configure-aws-credentials@v4
  with:
    aws-region: us-east-1
    role-to-assume: arn:aws:iam::123456789012:role/github-actions

- name: Login to ECR
  uses: docker/login-action@v3
  with:
    registry: 123456789012.dkr.ecr.us-east-1.amazonaws.com

- uses: eunanio/nori/action@v1
  with:
    registry: 123456789012.dkr.ecr.us-east-1.amazonaws.com
    repository: terraform-modules/my-module
    tag: v1.0.0
    module-path: ./terraform
```

### Google Artifact Registry

```yaml
- name: Authenticate to Google Cloud
  uses: google-github-actions/auth@v2
  with:
    workload_identity_provider: projects/123456789/locations/global/workloadIdentityPools/github/providers/github
    service_account: github-actions@my-project.iam.gserviceaccount.com

- name: Login to GAR
  uses: docker/login-action@v3
  with:
    registry: us-docker.pkg.dev

- uses: eunanio/nori/action@v1
  with:
    registry: us-docker.pkg.dev
    repository: my-project/terraform-modules/my-module
    tag: v1.0.0
    module-path: ./terraform
```

### Azure Container Registry

```yaml
- name: Login to ACR
  uses: docker/login-action@v3
  with:
    registry: myregistry.azurecr.io
    username: ${{ secrets.ACR_USERNAME }}
    password: ${{ secrets.ACR_PASSWORD }}

- uses: eunanio/nori/action@v1
  with:
    registry: myregistry.azurecr.io
    repository: terraform-modules/my-module
    tag: v1.0.0
    module-path: ./terraform
```

### Docker Hub

```yaml
- uses: docker/login-action@v3
  with:
    username: ${{ secrets.DOCKERHUB_USERNAME }}
    password: ${{ secrets.DOCKERHUB_TOKEN }}

- uses: eunanio/nori/action@v1
  with:
    registry: docker.io
    repository: myuser/my-module
    tag: v1.0.0
    module-path: ./terraform
```

## Module Path Handling

The `module-path` input accepts either:

### Directories

When pointing to a directory, the action automatically creates a `.zip` archive:

```yaml
module-path: ./modules/s3-bucket
```

The following files/directories are excluded from the archive:
- `.git/` and `.git*` files
- `.terraform/` directories
- `.tfstate` files

### Archives

Pre-built archives are used directly:

```yaml
module-path: ./dist/module.zip      # ZIP format
module-path: ./dist/module.tar.gz   # Gzipped tarball
module-path: ./dist/module.tgz      # Gzipped tarball (alternative extension)
```

## Compatibility

Modules packaged with this action are compatible with:

- **OpenTofu 1.10+** native OCI module source support
- **Nori CLI** for deployment and release management
- **ORAS CLI** for low-level artifact operations

Example usage in OpenTofu:

```hcl
module "s3_bucket" {
  source = "oci://ghcr.io/myorg/s3-bucket?tag=v1.0.0"

  bucket_name        = "my-bucket"
  versioning_enabled = true
}
```

## Troubleshooting

### Authentication Errors

Ensure `docker/login-action` runs **before** this action:

```yaml
# ❌ Wrong order
- uses: eunanio/nori/action@v1
  with: ...
- uses: docker/login-action@v3
  with: ...

# ✓ Correct order
- uses: docker/login-action@v3
  with: ...
- uses: eunanio/nori/action@v1
  with: ...
```

### Permission Denied on GHCR

Ensure your workflow has the required permissions:

```yaml
permissions:
  contents: read
  packages: write
```

### Module Path Not Found

The path is relative to the repository root. Use `actions/checkout` first:

```yaml
- uses: actions/checkout@v4
- uses: eunanio/nori/action@v1
  with:
    module-path: ./terraform  # Relative to repo root
```

### Insecure Registry

For development registries using HTTP:

```yaml
- uses: eunanio/nori/action@v1
  with:
    registry: localhost:5000
    insecure: 'true'
    ...
```
