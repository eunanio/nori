#!/bin/bash
# Nori GitHub Action Entrypoint
# Packages and pushes Terraform/OpenTofu modules as OCI artifacts

set -euo pipefail

# =============================================================================
# Input Arguments
# =============================================================================
REGISTRY="${1}"
REPOSITORY="${2}"
TAG="${3}"
MODULE_PATH="${4}"
DESCRIPTION="${5:-}"
ANNOTATIONS="${6:-{}}"
INSECURE="${7:-false}"
SIGN="${8:-false}"

# =============================================================================
# Validation
# =============================================================================
echo "::group::Validating inputs"

if [[ -z "${REGISTRY}" ]]; then
    echo "::error::Input 'registry' is required"
    exit 1
fi

if [[ -z "${REPOSITORY}" ]]; then
    echo "::error::Input 'repository' is required"
    exit 1
fi

if [[ -z "${TAG}" ]]; then
    echo "::error::Input 'tag' is required"
    exit 1
fi

if [[ -z "${MODULE_PATH}" ]]; then
    echo "::error::Input 'module-path' is required"
    exit 1
fi

if [[ ! -e "${MODULE_PATH}" ]]; then
    echo "::error::Module path '${MODULE_PATH}' does not exist"
    exit 1
fi

echo "Registry: ${REGISTRY}"
echo "Repository: ${REPOSITORY}"
echo "Tag: ${TAG}"
echo "Module Path: ${MODULE_PATH}"
echo "Description: ${DESCRIPTION:-<not set>}"
echo "Insecure: ${INSECURE}"
echo "Sign: ${SIGN}"

# Log authentication method (without exposing secrets)
if [[ -n "${NORI_REGISTRY_USERNAME:-}" && -n "${NORI_REGISTRY_PASSWORD:-}" ]]; then
    echo "Authentication: using explicit username/token inputs"
elif [[ -n "${GITHUB_TOKEN:-}" ]]; then
    echo "Authentication: using GITHUB_TOKEN"
else
    echo "Authentication: no credentials detected (anonymous)"
fi
echo "::endgroup::"

# =============================================================================
# Prepare Module Archive
# =============================================================================
echo "::group::Preparing module archive"

ARCHIVE_PATH="${MODULE_PATH}"

# If module-path is a directory, create a zip archive
if [[ -d "${MODULE_PATH}" ]]; then
    echo "Module path is a directory, creating zip archive..."
    
    ARCHIVE_PATH="/tmp/module.zip"
    
    # Create zip from directory contents
    pushd "${MODULE_PATH}" > /dev/null
    zip -r "${ARCHIVE_PATH}" . -x "*.git*" -x "*.terraform*" -x "*.tfstate*"
    popd > /dev/null
    
    echo "Created archive: ${ARCHIVE_PATH}"
    echo "Archive size: $(du -h "${ARCHIVE_PATH}" | cut -f1)"
fi

# Validate archive format
if [[ ! "${ARCHIVE_PATH}" =~ \.(zip|tar\.gz|tgz)$ ]]; then
    echo "::error::Module must be a .zip, .tar.gz, or .tgz archive (or a directory)"
    exit 1
fi

echo "::endgroup::"

# =============================================================================
# Build Reference
# =============================================================================
REFERENCE="${REGISTRY}/${REPOSITORY}:${TAG}"
echo "Full reference: ${REFERENCE}"

# =============================================================================
# Build nori package command
# =============================================================================
echo "::group::Packaging module"

NORI_ARGS=("package" "${REFERENCE}" "${ARCHIVE_PATH}")

# Add description if provided
if [[ -n "${DESCRIPTION}" ]]; then
    NORI_ARGS+=("--description" "${DESCRIPTION}")
fi

# Add insecure flag if enabled
if [[ "${INSECURE}" == "true" ]]; then
    NORI_ARGS+=("--insecure")
fi

# Add signing flags if enabled (uses keyless OIDC in GitHub Actions)
if [[ "${SIGN}" == "true" ]]; then
    NORI_ARGS+=("--sign" "--keyless")
    echo "Signing enabled (keyless OIDC)"
fi

# Parse and add annotations from JSON
if [[ "${ANNOTATIONS}" != "{}" && -n "${ANNOTATIONS}" ]]; then
    echo "Parsing annotations..."
    
    # Parse JSON annotations and add as --annotation flags
    while IFS='=' read -r key value; do
        if [[ -n "${key}" ]]; then
            NORI_ARGS+=("--annotation" "${key}=${value}")
            echo "  ${key}=${value}"
        fi
    done < <(echo "${ANNOTATIONS}" | jq -r 'to_entries | .[] | "\(.key)=\(.value)"' 2>/dev/null || true)
fi

echo "Running: nori ${NORI_ARGS[*]}"
echo ""

# =============================================================================
# Execute nori package
# =============================================================================
OUTPUT=$(nori "${NORI_ARGS[@]}" 2>&1) || {
    echo "::error::Failed to package module"
    echo "${OUTPUT}"
    exit 1
}

echo "${OUTPUT}"
echo "::endgroup::"

# =============================================================================
# Parse Output and Set GitHub Action Outputs
# =============================================================================
echo "::group::Setting outputs"

# Extract digest from output (format: "Digest:    sha256:...")
DIGEST=$(echo "${OUTPUT}" | grep -oP 'Digest:\s+\K\S+' || echo "")

if [[ -z "${DIGEST}" ]]; then
    echo "::warning::Could not extract digest from nori output"
fi

# Set outputs using GitHub Actions output syntax
echo "reference=${REFERENCE}" >> "${GITHUB_OUTPUT}"
echo "digest=${DIGEST}" >> "${GITHUB_OUTPUT}"
echo "signed=${SIGN}" >> "${GITHUB_OUTPUT}"

echo "Reference: ${REFERENCE}"
echo "Digest: ${DIGEST}"
echo "Signed: ${SIGN}"
echo "::endgroup::"

echo ""
echo "✓ Module packaged and pushed successfully!"
echo "  Reference: ${REFERENCE}"
echo "  Digest: ${DIGEST}"
if [[ "${SIGN}" == "true" ]]; then
    echo "  Signed: Yes (keyless OIDC)"
fi

