#!/usr/bin/env bash
# scripts/generate-checksums.sh
# Generate SHA-256 checksums for all release artifacts

set -euo pipefail

OUTPUT_FILE="${1:-checksums.txt}"

echo "Generating checksums..."

# Generate SHA-256 checksums for all release artifacts
sha256sum aether-*.tar.gz aether-*.zip aether-*.exe 2>/dev/null > "${OUTPUT_FILE}"

if [[ $? -ne 0 ]]; then
    echo "No artifacts found to checksum"
    exit 1
fi

echo "Checksums generated: ${OUTPUT_FILE}"
cat "${OUTPUT_FILE}"