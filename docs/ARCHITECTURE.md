
# Nori Architecture

## Overview

Nori implements a release management workflow for infrastructure modules. The system packages Terraform/OpenTofu modules as OCI artifacts, maintains versioned release state in OCI registries, and orchestrates deployments through the OpenTofu CLI.

```
nori/
├── cmd/              # CLI entry point
│   ├── main.go
│   └── commands/          # Command implementations
├── pkg/                   # Public packages
│   ├── auth/              # Registry authentication
│   ├── codegen/           # Terraform code generation
│   ├── config/            # Configuration management
│   ├── deploy/            # Deployment orchestration
│   ├── oci/               # OCI client and artifacts
│   ├── packaging/         # Module packaging
│   ├── release/           # Local release management
│   ├── runtime/           # OpenTofu runtime
│   └── state/             # OCI-based state storage
├── internal/
│   └── util/              # Internal utilities
└── docs/
```

## Core Components

### OCI Client (`pkg/oci`)

The OCI client wraps `github.com/google/go-containerregistry` to provide registry operations for module and state artifacts. It supports push, pull, inspect, list, and delete operations against any OCI-compliant registry.

The client accepts both digest and tag references. All artifacts follow the ORAS specification to ensure compatibility with OpenTofu's native registry protocol.

### Authentication (`pkg/auth`)

The `CredentialStore` handles registry authentication by resolving credentials through Docker's standard mechanisms. Credentials are resolved in the following order:

1. Registry-specific credential helpers (`credHelpers`)
2. Default credential store (`credsStore`)
3. Base64-encoded auth in Docker config (`auths`)
4. Environment variables (for CI/CD environments)

The module includes specific handling for ECR, GCR, and ACR authentication flows. Supported credential helpers include `osxkeychain`, `wincred`, and `pass`.

### State Management (`pkg/state`)

Release state is persisted as OCI artifacts in a configurable state repository. The `StateStore` handles serialisation and transport of release state to and from the registry.

Each `ReleaseState` contains the release metadata, user-provided values, generated `main.tf`, and the Terraform state file. The `ReleaseMetadata` structure holds the release name, module version, deployment status, and user annotations.

State artifacts conform to the following structure:

```
OCI Manifest
├── Config Layer (application/vnd.oci.image.config.v1+json)
│   └── Release metadata JSON
└── Data Layer (application/vnd.nori.release.state.v1+tar+gzip)
    └── state.tar.gz
        ├── metadata.json
        ├── values.yaml
        ├── main.tf
        └── terraform.tfstate
```

State artifacts are tagged using the format `<release-name>-<version>` (e.g., `my-bucket-v1.0.0`), allowing multiple releases to coexist in a single repository.

### Release Management (`pkg/release`)

The `release` package manages local release state. The `Release` struct represents an in-memory release, while `Store` provides filesystem-backed persistence. This layer tracks release status, applied values, and backend configuration between operations.

### Code Generation (`pkg/codegen`)

The `GenerateMainTF` function produces Terraform configuration from a `ModuleConfig` specification. The generator handles complex value types including maps, lists, and nested objects, producing valid HCL output.

### Packaging (`pkg/packaging`)

The `Packager` converts module archives into OCI artifacts. Input archives may be `.zip` or `.tar.gz` format; tar.gz archives are converted to zip for OpenTofu compatibility. The packager validates archive contents and extracts Terraform metadata during processing.

The packager automatically detects `README.md` files at the root of the module archive. When present, the README is stored as a separate layer and the `io.nori.readme` annotation is added to the manifest.

Module artifacts are structured as follows:

```
OCI Manifest (with io.nori.readme annotation if README present)
├── Config Layer (application/vnd.oci.image.config.v1+json)
│   └── Module metadata (created, version, type)
├── Module Layer (archive/zip)
│   └── Compressed module content
└── README Layer (text/markdown) [optional]
    └── README.md content
```

### Deployment (`pkg/deploy`)

The `Deployer` orchestrates the deployment workflow for releases. A deployment proceeds through the following stages:

1. Pull module artifact from registry
2. Extract module to working directory
3. Write backend configuration (local state)
4. Generate `terraform.tfvars` from release values
5. Execute `tofu init` (with `-reconfigure` for upgrades)
6. Execute `tofu plan`
7. Execute `tofu apply` (unless plan-only mode)
8. Read `terraform.tfstate`
9. Push release state to OCI registry

### Runtime Management (`pkg/runtime`)

The `Manager` locates or provisions an OpenTofu binary. If OpenTofu is not found in `PATH`, the manager downloads the appropriate release from GitHub and caches it in `~/.nori/bin/`.

### Configuration (`pkg/config`)

Nori configuration is stored in YAML format at `~/.nori/config.yaml`. Configuration includes the state repository location and registry-specific settings.

## CLI Design

The CLI uses Cobra and follows standard conventions for help text, examples, and flag naming. Global flags are available for common options such as verbosity and registry selection.

```
nori
├── config         Manage configuration
│   ├── get        Retrieve configuration values
│   └── set        Update configuration values
├── release        Release management
│   ├── create     Create a new release
│   ├── upgrade    Upgrade an existing release
│   ├── destroy    Remove a release
│   ├── list       List all releases
│   ├── history    Display release history
│   ├── inspect    Display release details
│   └── status     Display release status
├── package        Package module as OCI artifact
├── pull           Download artifact to local filesystem
├── push           Upload artifact to registry
├── deploy         Deploy module without release tracking
├── list           List artifact versions
├── inspect        Display artifact metadata (--readme to view README)
├── login          Authenticate with registry
├── logout         Remove stored credentials
└── version        Display version information
```

## Data Flow

### Release Creation

```
User Request (name, module-ref, values.yaml)
    ↓
Resolve Credentials
    ↓
Pull Module Artifact
    ↓
Extract to Working Directory
    ↓
Generate main.tf
    ↓
Write terraform.tfvars
    ↓
Write backend_override.tf
    ↓
tofu init
    ↓
tofu plan
    ↓
tofu apply
    ↓
Read terraform.tfstate
    ↓
Create ReleaseState
    ↓
Push State to OCI (tagged: <name>-v1.0.0)
```

### Release Upgrade

```
User Request (name, new values)
    ↓
Pull Existing State from OCI
    ↓
Merge Values
    ↓
Calculate New Version
    ↓
Restore terraform.tfstate
    ↓
Pull Module Artifact
    ↓
Generate main.tf
    ↓
Write terraform.tfvars
    ↓
tofu init -reconfigure
    ↓
tofu plan
    ↓
tofu apply
    ↓
Read Updated terraform.tfstate
    ↓
Push New State to OCI (tagged: <name>-v1.1.0)
```

### Module Packaging

```
User Archive (.zip/.tar.gz)
    ↓
Validate Archive
    ↓
Convert to zip (if tar.gz)
    ↓
Extract README.md (if present)
    ↓
Create OCI Manifest
    ↓
Attach Config Layer
    ↓
Attach Module Layer
    ↓
Attach README Layer (if README found)
    ↓
Set io.nori.readme annotation (if README found)
    ↓
Push to Registry
```

## Versioning

Releases follow semantic versioning. By default, minor versions increment for value-only changes, while major versions increment when the module version changes. The `--version` flag allows explicit version specification.

State artifacts are tagged as `<release-name>-<version>`, enabling multiple independent releases within a single state repository.

## Error Handling

Errors are wrapped with context using `fmt.Errorf` and propagate to the CLI layer for consistent formatting. Debug output is available via the `--verbose` flag.

## Logging

Logging uses Go 1.21's `log/slog` package. Output is written to stderr with configurable log levels (debug, info, warn, error). JSON-formatted output is available through configuration.

## Testing

The test suite includes unit tests for core packages, table-driven tests for parsing logic, mock registries for OCI operations, and integration tests for CLI commands.

## Dependencies

The project maintains a minimal dependency footprint:

- `github.com/google/go-containerregistry` — OCI registry client
- `github.com/spf13/cobra` — CLI framework
- `gopkg.in/yaml.v3` — YAML parsing
- `golang.org/x/term` — Terminal input handling

Transitive dependencies are pinned for reproducible builds.

## Security

Credentials are stored through Docker credential helpers; plaintext password storage is not supported. TLS verification is enabled by default, with insecure mode requiring an explicit flag. Archive extraction includes path traversal protection.

Terraform state files may contain sensitive data. These are stored in OCI registries and protected by registry authentication.

## Future Work

Potential enhancements under consideration:

- Module signing and verification
- Module caching
- Parallel deployment execution
- Rollback to previous release versions