#!/usr/bin/env bash
# scripts/generate-manifest.sh
# Generate release manifest with artifact metadata

set -euo pipefail

VERSION="${1:-}"
COMMIT="${2:-}"
OUTPUT_PATH="${3:-release-manifest.json}"

if [[ -z "${VERSION}" || -z "${COMMIT}" ]]; then
    echo "Usage: $0 <version> <commit> [output-path]"
    exit 1
fi

echo "Generating release manifest for version ${VERSION} (commit ${COMMIT})..."

# Build artifact list
ARTIFACTS=()
for f in aether-*.tar.gz aether-*.zip aether-*.exe 2>/dev/null; do
    if [[ -f "$f" ]]; then
        SHA256=$(sha256sum "$f" | cut -d' ' -f1)
        SIZE=$(stat -c%s "$f" 2>/dev/null || stat -f%z "$f" 2>/dev/null)
        
        # Determine signature files
        SIG_FILE=""
        CERT_FILE=""
        ALGO=""
        if [[ "$f" == *.exe ]]; then
            SIG_FILE="embedded-authenticode"
            ALGO="authenticode-sha256-rsa2048"
        else
            SIG_FILE="${f}.sig"
            CERT_FILE="${f}.pem"
            ALGO="cosign-ecdsa-p256"
        fi
        
        # Build JSON for this artifact
        if [[ -n "$CERT_FILE" ]]; then
            ARTIFACTS+=("$(jq -n \
                --arg name "$f" \
                --arg sha256 "$SHA256" \
                --argjson size "$SIZE" \
                --arg signature "$SIG_FILE" \
                --arg certificate "$CERT_FILE" \
                --arg algorithm "$ALGO" \
                '{name: $name, sha256: $sha256, size: $size, signature: $signature, certificate: $certificate, algorithm: $algorithm}')")
        else
            ARTIFACTS+=("$(jq -n \
                --arg name "$f" \
                --arg sha256 "$SHA256" \
                --argjson size "$SIZE" \
                --arg signature "$SIG_FILE" \
                --arg algorithm "$ALGO" \
                '{name: $name, sha256: $sha256, size: $size, signature: $signature, algorithm: $algorithm}')")
        fi
    fi
done

# Build artifacts JSON array
ARTIFACTS_JSON=$(printf '%s\n' "${ARTIFACTS[@]}" | jq -s '.')

# Generate manifest
jq -n \
    --arg version "${VERSION}" \
    --arg commit "${COMMIT}" \
    --arg timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg builder "github-actions[bot]" \
    --arg go_version "$(go version | cut -d' ' -f3)" \
    --argjson artifacts "${ARTIFACTS_JSON}" \
    '{
        version: $version,
        commit: $commit,
        timestamp: $timestamp,
        builder: $builder,
        go_version: $go_version,
        artifacts: $artifacts
    }' > release-manifest.json

echo "Release manifest generated: release-manifest.json"
cat release-manifest.json | jq '.'