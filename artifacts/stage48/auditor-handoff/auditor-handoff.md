# Auditor Handoff Package — Aether 3.4.0-ga Candidate

**Purpose:** enable a genuinely independent auditor to verify or refute every release-critical claim mechanically, without trusting any log produced by the developer.

**Audited commit:** `d0f2d2e` (GA candidate lineage; the version-flip commit will be created only after audit + authorization)
**Prerequisites:** Go ≥ 1.27.1, cosign, python3, docker (optional, for Linux runtime re-verification), ~20 minutes of compute.

## 1. Repository & lineage (5 min)

```bash
git clone <repo> aether-audit && cd aether-audit
git checkout d0f2d2e
git status --short          # expect: empty
git tag --list | wc -l      # expect: 0
git remote -v | wc -l       # expect: 0 (remotes added only after authorization)
git cat-file -t 9a03334     # expect: fatal — foreign-lineage object ABSENT
git cat-file -t bb56cf3     # expect: fatal — foreign-lineage object ABSENT
cat VERSION                 # expect: 3.4.0-stage3
./bin/aether.exe --version  # rebuild first if you don't trust committed binaries (you shouldn't — see §4)
```

## 2. Regression lock (10 min) — run yourself, do not replay logs

```bash
go build ./... && go vet ./...        # expect: silent success
go test -count=1 ./...                # expect: 28/28 packages "ok", 0 FAIL
go test -race -count=1 ./...          # expect: all ok, 0 DATA RACE
go test -tags=integration -count=1 ./test/integration/...   # expect: ok
govulncheck ./...                     # expect: no affecting vulnerabilities
```

## 3. Defect-fix verification (D-001/D-002/D-003)

| Defect | Reproduce the original failure | Verify the fix |
|---|---|---|
| D-001 (doctor roundtrip failed on Windows) | `git stash` the fix? Not possible (committed). Instead: `go test -count=1 ./internal/cli/ -run TestWorkspaceRoundtripCheckClosesBeforeDelete` — then read `internal/cli/doctor.go` (explicit `w.Close()` before `Delete`) and confirm the pre-fix code path would hold an open vault handle (Windows cannot unlink open files) | `go run ./cmd/aether doctor` → all checks OK, exit 0 |
| D-002 (`serve cert revoke` unreachable) | `go run ./cmd/aether serve cert revoke --help` → `--operator` flag now present & required; `go test ./internal/cli/ -run TestServeCertRevokeAcceptsOperatorFlag` | live: issue an operator, revoke, see "revoked (appended to …revoked.txt)" |
| D-003 (revocation was a startup snapshot) | `go test ./internal/api/ -run TestFileRevocationListTakesEffectWithoutRestart` | live: start teamserver, connect OK → `serve cert revoke --operator X` → reconnect X → connection dropped with no protocol; other operators unaffected |

## 4. Supply chain (independent tooling only)

```bash
# Do NOT trust committed signatures' *content* — verify from the artifacts:
cd artifacts/stage45
sha256sum dist/*                       # compare line-by-line to checksums.txt
cosign verify-blob --key supply-chain/release.pub \
  --signature supply-chain/aether-windows-amd64.exe.sig \
  --insecure-ignore-tlog dist/aether-windows-amd64.exe     # expect: Verified OK
# tamper: append 1 byte to a copy → expect: invalid signature
# wrong key: generate a second keypair → expect: invalid signature
python3 - <<'EOF'                      # provenance subjects vs. artifact bytes
import json, hashlib
for s in json.load(open("provenance.json"))["subject"]:
    h = hashlib.sha256(open("dist/"+s["name"],"rb").read()).hexdigest()
    assert h == s["digest"]["sha256"], s["name"]
print("PROVENANCE OK")
EOF
# SBOM: cyclonedx-gomod bin -json - /path/to/rebuilt-binary → spot-check ≥5 components vs sbom/*.cdx.json
```

**Known finding to review:** dist binaries were built from the pre-commit *dirty* tree (VCS stamp `+dirty` in `go version -m`). The GA process requires the final build from the committed clean tree with `-buildvcs=false`; cross-toolchain byte-identity was proven at `321c96fb…` (see `stage47/cross-platform-runtime.md`). Judge whether this is release-blocking for the *final* build (it should not be — final build supersedes dist/).

## 5. Operational sample (≥20% of Stage 44 scenarios) — execute live

```bash
go run ./cmd/aether workspace create audit-ws --passphrase 'Audit-Pw-1!'
go run ./cmd/aether workspace list
go run ./cmd/aether workspace info audit-ws --passphrase 'WRONG'        # expect typed fail-closed error
go run ./cmd/aether workspace rekey audit-ws --old-passphrase 'Audit-Pw-1!' --new-passphrase 'Audit-Pw-2#'
go run ./cmd/aether workspace delete audit-ws --force
# teamserver: serve cert init → issue alice/bob → serve → connect (mTLS) →
#   bob lacks exec.azure → revoke bob → bob reconnect dropped; alice fine
# graph: build 10K synthetic entra dataset → qualify → runbook deterministic
```

Full command sequences with expected outputs: `stage44/ops-qualification.md` (the auditor re-runs them, never replays them).

## 6. Required deliverable

`artifacts/stage48/independent-audit-report-v2.md` containing:

```text
Attestation: "I am independent of the development implementation being reviewed."
Reviewer / organization / role / method / commit reviewed / date
Findings: CRITICAL | HIGH | MEDIUM | LOW | OBSERVATION  (each: evidence + reproduction)
Closure status per finding; explicit GA recommendation
Signature
```

Unsigned or self-signed reports do not close B-1.
