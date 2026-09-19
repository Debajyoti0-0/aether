# Publication Runbook — v3.4.0-ga

**Precondition:** B-1 (independent audit) CLOSED, B-3 (platform gates) CLOSED-or-waived, B-4 (authorization) APPROVED, remote configured. Execute top to bottom; stop on first failure.

## 1. Version flip (single commit)

```bash
cd <repo>
# edit VERSION → 3.4.0-ga; README heading; CHANGELOG GA entry
git add VERSION README.md CHANGELOG.md
git commit -m "release: Aether v3.4.0-ga — General Availability"
git show HEAD:VERSION          # expect: 3.4.0-ga
git status --short             # expect: empty
go test -count=1 ./...         # must be green post-flip
```

## 2. Clean-tree release build (proven reproducible mode)

```bash
V=3.4.0-ga; C=$(git rev-parse --short HEAD)
LDF="-s -w -X github.com/Debajyoti0-0/aether/internal/version.Version=$V -X github.com/Debajyoti0-0/aether/internal/version.Commit=$C"
mkdir -p artifacts/handoff/dist
for t in windows-amd64 windows-arm64 linux-amd64 linux-arm64 darwin-amd64 darwin-arm64; do
  os=${t%-*}; arch=${t#*-}; ext=""; [ "$os" = windows ] && ext=.exe
  GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="$LDF" \
    -o artifacts/handoff/dist/aether-$t$ext ./cmd/aether || exit 1
done
# reproducibility gate: rebuild one target, expect identical hash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="$LDF" -o /tmp/repro-check ./cmd/aether
sha256sum /tmp/repro-check artifacts/handoff/dist/aether-linux-amd64   # MUST match
```

## 3. SBOM, checksums, provenance, signing

```bash
cd artifacts/handoff
cyclonedx-gomod bin -json -output sbom.cdx.json dist/aether-windows-amd64.exe
sha256sum dist/* > checksums.txt
# provenance.json — regenerate for the GA commit (see stage45 provenance.json as template)
KEY=~/.aether-release-keys/release-3.4.0-stage3.key
for f in dist/aether-windows-amd64.exe checksums.txt provenance.json sbom.cdx.json; do
  COSIGN_PASSWORD="" cosign sign-blob --key $KEY --tlog-upload=false --output-signature $f.sig $f
  COSIGN_PASSWORD="" cosign verify-blob --key <pub> --signature $f.sig --insecure-ignore-tlog $f   # Verified OK
done
# negative tests: tamper + wrong key → both must REJECT
```

## 4. Tag & push

```bash
git tag -a v3.4.0-ga -m "Aether 3.4.0 GA" $(git rev-parse HEAD)
git push origin master && git push origin v3.4.0-ga
git ls-remote origin refs/tags/v3.4.0-ga        # verify tag on remote
```

## 5. Publish & post-publish verification

```bash
# Create GitHub release for v3.4.0-ga; upload dist/*, checksums.txt, sbom.cdx.json,
# provenance.json, all .sig files, release-manifest.json, release notes.
# Then, in a CLEAN directory:
curl -sLO <release-url>/aether-linux-amd64 && curl -sLO <release-url>/checksums.txt
sha256sum aether-linux-amd64          # must equal published checksums.txt line
./aether-linux-amd64 --version        # must print: aether version 3.4.0-ga
cosign verify-blob ...                # must verify against published signature
```

No "published" claim without retrieval evidence from a clean environment.
