#!/usr/bin/env bash
# scripts/generate-provenance.sh
# Generate SLSA-style provenance attestation

set -euo pipefail

VERSION="${1:-}"
COMMIT="${2:-}"
OUTPUT_PATH="${3:-provenance.json}"

if [[ -z "${VERSION}" || -z "${COMMIT}" ]]; then
    echo "Usage: $0 <version> <commit> [output-path]"
    exit 1
fi

echo "Generating provenance for version ${VERSION} (commit ${COMMIT})..."

# Get Go version
GO_VERSION=$(go version | cut -d' ' -f3)

# Get builder identity (GitHub Actions bot)
BUILDER="github-actions[bot]"

# Generate provenance
jq -n \
    --arg version "${VERSION}" \
    --arg commit "${COMMIT}" \
    --arg timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg builder "${BUILDER}" \
    --arg go_version "$(go version | cut -d' ' -f3)" \
    '{
        "buildType": "https://github.com/Debajyoti0-0/aether/.github/workflows/release.yml@refs/heads/main",
        "invocation": {
            "configSource": {
                "uri": "git+https://github.com/Debajyoti0-0/aether",
                "digest": {"sha256": $commit},
                "entryPoint": ".github/workflows/release.yml"
            },
            "parameters": {
                "version": $version,
                "commit": $commit
            },
            "environment": {
                "builder": $builder,
                "go_version": $go_version
            }
        },
        "materials": [
            {"uri": "git+https://github.com/Debajyoti0-0/aether", "digest": {"sha256": $commit}}
        ]
    }' > "${OUTPUT_PATH}"

echo "Provenance generated: ${OUTPUT_PATH}"
cat "${OUTPUT_PATH}"