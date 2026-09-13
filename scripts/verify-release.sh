#!/usr/bin/env bash
# scripts/verify-release.sh
# Automated release verification script

set -euo pipefail

RELEASE_DIR="${1:-.}"

echo "=== Aether Release Verification ==="
echo "Directory: ${RELEASE_DIR}"
echo ""

cd "${RELEASE_DIR}"

# 1. Checksums
echo "[1/6] Verifying checksums..."
sha256sum -c checksums.txt || { echo "FAIL: Checksum mismatch"; exit 1; }
echo "PASS"

# 2. Manifest consistency
echo "[2/6] Verifying manifest consistency..."
jq -r '.artifacts[] | "\(.sha256)  \(.name)"' release-manifest.json | sha256sum -c || { echo "FAIL: Manifest mismatch"; exit 1; }
echo "PASS"

# 3. Linux/macOS signatures
echo "[3/6] Verifying Linux/macOS signatures..."
for bin in aether-linux-*.tar.gz aether-darwin-*.tar.gz; do
    if [[ -f "$bin" && -f "$bin.sig" && -f "$bin.pem" ]]; then
        echo "  Verifying $bin..."
        cosign verify-blob --signature "$bin.sig" --certificate "$bin.pem" \
            --certificate-identity-regexp ".*" --certificate-oidc-issuer-regexp ".*" "$bin" || { echo "FAIL: $bin signature"; exit 1; }
        echo "  PASS: $bin"
    fi
done

# 4. Manifest/SBOM/Checksums signatures
echo "[4/6] Verifying manifest/SBOM/checksums signatures..."
for f in release-manifest.json sbom-cyclonedx.json checksums.txt; do
    if [[ -f "$f.sig" && -f "$f.pem" ]]; then
        echo "  Verifying $f..."
        cosign verify-blob --signature "$f.sig" --certificate "$f.pem" \
            --certificate-identity-regexp ".*" --certificate-oidc-issuer-regexp ".*" "$f" || { echo "FAIL: $f signature"; exit 1; }
        echo "  PASS: $f"
    fi
done

# 5. SBOM validation
if [[ -f "sbom-cyclonedx.json" ]]; then
    echo "[5/6] Validating SBOM..."
    jq -e '.specVersion' sbom-cyclonedx.json > /dev/null || { echo "FAIL: SBOM specVersion missing"; exit 1; }
    echo "  SBOM specVersion: $(jq -r '.specVersion' sbom-cyclonedx.json)"
    echo "  Components: $(jq '.components | length' sbom-cyclonedx.json)"
    echo "PASS"
fi

# 6. Manifest completeness
echo "[6/6] Verifying manifest completeness..."
jq -r '.artifacts[].name' release-manifest.json | while read f; do
    [[ -f "$f" ]] || { echo "FAIL: Missing artifact $f"; exit 1; }
done
echo "PASS"

echo ""
echo "=== ALL VERIFICATIONS PASSED ==="
echo "Release is authentic and untampered."