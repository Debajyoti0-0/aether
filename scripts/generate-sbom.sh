#!/usr/bin/env bash
# scripts/generate-sbom.sh
# Generate CycloneDX SBOM using syft

set -euo pipefail

BINARY_PATH="${1:-}"
OUTPUT_PATH="${2:-sbom-cyclonedx.json}"

if [[ -z "${BINARY_PATH}" ]]; then
    echo "Usage: $0 <binary-path> [output-path]"
    exit 1
fi

if [[ ! -f "${BINARY_PATH}" ]]; then
    echo "Binary not found: ${BINARY_PATH}"
    exit 1
fi

echo "Generating SBOM for ${BINARY_PATH}..."

# Generate SBOM using syft
syft "${BINARY_PATH}" -o cyclonedx-json="${OUTPUT_PATH}"

if [[ $? -ne 0 ]]; then
    echo "syft failed"
    exit 1
fi

echo "SBOM generated: ${OUTPUT_PATH}"

# Also generate module-level SBOM using cyclonedx-gomod
echo "Generating module-level SBOM..."
cyclonedx-gomod app -output "${OUTPUT_PATH}.module.json" 2>/dev/null || echo "cyclonedx-gomod not available, skipping module-level SBOM"