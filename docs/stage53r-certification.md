# Stage 53R — Certification

Qualification evidence for the reconciled trunk
`reconciliation/stage53r`.

## 1. Environment

| | |
|---|---|
| OS / arch | Windows, amd64 |
| Go | go1.27.1 windows/amd64 |
| Rust | rustc 1.98.1, cargo 1.98.1, wasm-pack 0.13.1 (off `PATH`, at `~\.cargo\bin`) |
| Node / npm | v24.16.0 / 11.13.0 |
| Docker | 29.7.2, daemon up |
| git | 2.54.0.windows.1 |
| `wasm-opt` | absent — consistent with the known bulk-memory rejection, which is why the verifier builds with `--no-opt` |

Unavailable, and therefore NOT PERFORMED rather than passed:
`golangci-lint`, `staticcheck`, `govulncheck`, stable-toolchain Clippy,
Firefox, Safari (N/A on Windows), `wasm-opt`.

## 2. Build and test evidence

Run on the merged tree at `76685d2`.

| Gate | Command | Result |
|---|---|---|
| Build | `go build ./...` | exit 0 |
| Vet | `go vet ./...` | exit 0 |
| Module integrity | `go mod verify` | all modules verified |
| Unit | `go test -count=1 ./...` | all packages pass |
| Race | `go test -race -count=1 ./...` | all packages pass, incl. `internal/web` |
| Rust — graph | `cargo test` (wasm/graph) | **17/17** |
| Rust — verify | `cargo test` (wasm/verify) | **14/14** |
| WASM cross-impl | `node internal/web/wasm/smoke-test.mjs` | **56/56**, exit 0 |

The WASM smoke count rose from the Stage 52 baseline of 54/54 because the
merge brought in additional graph ABI cases.

### Honest note on one run

The **first** full-module run reported
`--- FAIL: TestBrowserReachesItsOwnVerdict (0.61s)`. It passed in
isolation, in a second full run, and under `-race`. Not counted as a pass
and not dismissed — see R-05. Gate status is **FLAKY**.

## 3. Stage 52 regression

| Check | Result |
|---|---|
| Stage 52 committed with the exact required message | PASS — `8291fb6`, em dash verified as `U+2014` |
| 69 files staged, no build output swept in | PASS — 381 MB of Rust `target/`+`pkg/` correctly excluded by `.gitignore` |
| WASM verifier performs genuine SHA-256 + Ed25519 | PASS — 56/56 smoke |
| Browser reaches its own verdict | **FLAKY** (1 of 3 runs) |
| Templating: no inline handlers/styles, valid UTF-8, no BOM | PASS — byte-verified on the merged CHANGELOG |
| Forged-chain rejection | PASS — covered by the smoke suite |

## 4. Engagement scope enforcement — control cases

Run against the built binary on the reconciled trunk. Capability routing
was verified empirically, not assumed.

### AD commands

| # | Case | Result | Exit |
|---|---|---|---|
| 1 | no `--engagement` | `required flag(s) "engagement" not set` | 1 |
| 2 | out-of-scope domain | `domain "evil.com" is not authorized in engagement example-001` | 1 |
| 3 | out-of-scope DC | `domain controller "dc02.evil.com" is not authorized in engagement example-001` | 1 |
| 4 | capability absent | `capability "ad.kerberos.roast" is not authorized in engagement example-readonly-002` | 1 |
| 5 | expired time window | `operation is after the engagement time window (closed 2020-01-02T00:00:00Z)` | 1 |
| 6 | **in scope** | gate **passed**, proceeded to workspace stage | — |

### LDAP commands

| # | Case | Result | Exit |
|---|---|---|---|
| 1 | no `--engagement` | `required flag(s) "engagement" not set` | 1 |
| 2 | `ldap enum users`, read-only engagement | demands `ad.ldap.read` → refused | 1 |
| 3 | `ldap acl get`, read-only engagement | demands `ad.ldap.acl` → refused | 1 |
| 4 | `ldap path`, read-only engagement | demands `ad.ldap.acl` → refused | 1 |
| 5 | `ldap acl get`, full engagement | gate **passed**, proceeded to connect stage | — |
| 6 | `aether ad ldap enum users` | gated identically to `aether ldap …` | 1 |

Cases 6 in each set are the positive controls: without them, a gate that
refused everything would look identical to a working one.

### Unit tests

17 tests in `internal/engagement`: in-scope acceptance; case-insensitive
domain/DC; refusal of unauthorized domain, DC, capability and
suffix/prefix lookalikes; capability case-sensitivity; window before, after,
empty and reversed; malformed bounds; nil engagement; and `Load` refusal of
empty, whitespace, missing, malformed, identifier-less, incomplete and
bad-timestamp documents.

## 5. Security controls recovered during reconciliation

| Control | Where | Status |
|---|---|---|
| PKI root directory `0700` (protects `teamserver-ca.key`) | `internal/cli/serve_cert.go` | restored; would have been deleted by a side-selection merge |
| Fail-open config guard | `internal/cli/root.go` | restored |
| Strict `--log-level` (F-34-2) | `internal/cli/root.go` | restored |
| `*.keytab` credential exclusion | `.gitignore` | preserved |
| Windows vault handle close before delete | `internal/cli/doctor.go` | workspace version taken |
| Rules-of-engagement scope | 15 commands | **newly enforced** |

## 6. Deliberately not claimed

- **Stage 47 / G3206: `BLOCKED-WITH-OWNER`.** Requires Windows Server 2022
  AD DS + Enterprise AD CS. No Samba, FreeIPA, Certipy, synthetic CA or
  mock KDC was used, and none will be.
- **G3607: NOT PERFORMED.** Docker is up and `aether-ad-lab`
  (realm `AETHER.TEST`) exists but is `exited 255`; it has not been started
  or qualified.
- **50K-node / 60 FPS: not claimed.** Not attempted in this stage.
- **Fuzz, reproducible build, full CLI matrix, dependency audit beyond
  `go mod verify`: NOT PERFORMED.**
- **Firefox/Safari qualification: not possible** in this environment.
