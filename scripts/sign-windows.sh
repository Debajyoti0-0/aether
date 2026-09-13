#!/usr/bin/env bash
# scripts/sign-windows.sh
# Windows Authenticode signing script for goreleaser post-hook
# Requires: AUTHENTICODE_CERT (base64-encoded PFX), AUTHENTICODE_PASSWORD env vars
# This script is called from goreleaser post-hook on Windows runners

set -euo pipefail

BINARY_PATH="${1:-}"

if [[ -z "${BINARY_PATH}" ]]; then
    echo "Usage: $0 <binary-path>"
    exit 1
fi

if [[ -z "${AUTHENTICODE_CERT:-}" ]]; then
    echo "AUTHENTICODE_CERT not set, skipping Windows signing"
    exit 0
fi

if [[ -z "${AUTHENTICODE_PASSWORD:-}" ]]; then
    echo "AUTHENTICODE_PASSWORD not set"
    exit 1
fi

if [[ ! -f "${BINARY_PATH}" ]]; then
    echo "Binary not found: ${BINARY_PATH}"
    exit 1
fi

# Write cert to temp file
CERT_PATH=$(mktemp)
echo "${AUTHENTICODE_CERT}" | base64 -d > "${CERT_PATH}"

cleanup() {
    rm -f "${CERT_PATH}"
}
trap cleanup EXIT

echo "Signing ${BINARY_PATH} with Authenticode..."

# Sign with timestamp (RFC 3161)
signtool sign \
    /fd SHA256 \
    /tr http://timestamp.digicert.com \
    /td SHA256 \
    /f "${CERT_PATH}" \
    /p "${AUTHENTICODE_PASSWORD}" \
    "${BINARY_PATH}"

if [[ $? -ne 0 ]]; then
    echo "signtool sign failed"
    exit 1
fi

echo "Signing successful"

# Verify the signature
signtool verify /pa /v "${BINARY_PATH}"
if [[ $? -ne 0 ]]; then
    echo "signtool verify failed"
    exit 1
fi

echo "Verification successful"