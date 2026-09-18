# Stage 42 — Phase 5 CI Revival + Signing + Provenance

This phase depends on one operator action. The action has not been taken.

## Auth state

```
gh auth status   (PS, 2026-09-18, clean-room)
→ not logged in (no gh hosts entry)
```

`gh auth login` opens CI: workflow runs become visible, `release.yml` Validate
logs become diagnosable, and the runner-side signing steps can execute. Without
it, every runner-side item is undiagnosable from this host.

## Disposition

| Item | Expected | Disposition | Remedy (exact operator action) |
| --- | --- | --- | --- |
| CI build/vet/unit | green | BLOCKED-WITH-OWNER (local PASS: 29 pkgs + race PASS) | `gh auth login && git push origin master` |
| CI race | green | BLOCKED-WITH-OWNER (local race PASS) | same |
| CI fuzz | green | BLOCKED-WITH-OWNER (local sweep PASS: 21 targets, 0 crashes) | same |
| `release.yml` Validate (run #35253502855) | diagnosable | BLOCKED-WITH-OWNER (carried from Stage 40; local Validate steps green) | `gh auth login && gh run view 35253502855 --log` |
| cosign keyless signing | `.sig` files | BLOCKED-WITH-OWNER (runner-side only) | same auth; then re-run the signing workflow |
| SLSA provenance | attestation | BLOCKED-WITH-OWNER (runner-side only; waiver F-30-3 exists) | same |
| `windows-sign` (F-32-2) | artifact alignment | CARRIED: F-32-2 disposition unchanged from Stage 32 (mismatch documented; requires a CI-visible run to re-verify) | `gh auth login`, then inspect release artifacts |
| Authenticode (SGN-2) | signed binary | BLOCKED-WITH-OWNER (procurement) | purchase an EV certificate; configure the release job |

## What IS locally verified (not runner-dependent)

- `go build ./...` / `go vet ./...` → OK
- `go test -count=1 ./...` → 29 packages, 0 failures
- `go test -race -count=1 ./...` → 29 packages, 0 failures
- `go test -tags=integration -count=1 ./test/integration/...` → ok (15.9s)
- fuzz sweep: 21 targets, 0 crashes
- `golangci-lint run ./...` → 0 issues
- `govulncheck ./...` → 0 called vulnerabilities

The local gate set matches the CI gate set; CI execution is an authorization
gap, not a quality gap. Recorded honestly as BLOCKED-WITH-OWNER — not
IMPLEMENTED, not fabricated PASS.
