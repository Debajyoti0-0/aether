# Stage 53U Dependency and Static-Analysis Audit

- Captured (UTC): 2026-09-26T07:37:43Z
- HEAD: `5bb26bd4e3f1543871e80160a66a6cd8f31e645d` (branch `reconciliation/stage53r`)
- Workdir: `C:\dev\aether`
- Toolchain: Go 1.27.1, `golangci-lint` 2.13.2, `govulncheck` 1.8.0
- Vuln DB snapshot: `2026-09-24 20:07:49 +0000 UTC`

Raw tool output: `artifacts/stage53u/quality-gates.txt`.
Baseline (pre-correction) output: `artifacts/stage53u/golangci-lint.txt`.

## Gate results

| Gate | Baseline | Post-correction |
| --- | --- | --- |
| `go build ./...` | pass | pass |
| `go vet ./...` | fail (see below) | pass |
| `go test -count=1 ./...` | pass | pass |
| `go test -race -count=1 ./...` | pass | pass |
| `golangci-lint run ./...` | 2 issues, exit 1 | **0 issues, exit 0** |
| `govulncheck ./...` | not recorded | **0 code-affected, exit 0** |

## Static-analysis corrections (G53R-27)

The baseline lint run found exactly two `ineffassign` defects. Both are now fixed.

1. `internal/engine/ad/acl/engine.go:406` — ineffectual assignment to `maxDepth`.
   Root cause was not merely a dead store: `FindACLPaths` accepted an arbitrary
   `maxDepth`, silently ignored it, and advertised depth control while performing
   only target-DACL single-hop analysis. The correction exports
   `acl.PathSearchDepth = 1` and rejects every other depth with an explicit
   `single-hop` error, so an unsupported depth can no longer be silently accepted.
   CLI default and help text were aligned to the same value.

2. `internal/protocol/ldap/bind.go:145` — ineffectual assignment to `rest`.
   The SASL remainder decode now discards the returned remainder
   (`_, err = asn1.Unmarshal(rest, &saslRaw)`). Parsing behavior is unchanged; only
   the dead assignment was removed.

Note that `.golangci.yml` declares version `2`, enables only `ineffassign`, and
disables `staticcheck`. A clean result here is therefore a narrow claim and must
not be read as broad static-analysis coverage.

## Build-graph defect found during the audit

`go vet ./...` and `go build ./...` failed with:

```
vet.exe: artifacts\stage54\generate_matrices.go:51:6: main redeclared in this block
```

Two standalone `package main` generators (`generate_matrices.go`,
`generate_cli_inventory.go`) shared one directory, so the package was
unbuildable. Neither file was referenced anywhere in the repository. Each was
moved into its own package directory, which is behavior-preserving because all
of their output paths are repository-root relative:

- `artifacts/stage54/generate-matrices/main.go`
- `artifacts/stage54/generate-cli-inventory/main.go`

Verified after the move: both `go run ./artifacts/stage54/generate-matrices` and
`go run ./artifacts/stage54/generate-cli-inventory` exit 0 and regenerate all six
JSON matrices byte-identically (SHA-256 drift 0 of 6).

## Dependency audit (G53R-34)

`govulncheck ./...` reports **0 vulnerabilities affecting this code** and
**0 vulnerabilities in imported packages**. One finding is raised at module level
and is recorded here as accepted, no-action:

- **GO-2026-5932** — `golang.org/x/crypto/openpgp` is unmaintained, unsafe by
  design, and has known security issues.
  - Module: `golang.org/x/crypto@v0.57.0`
  - `Fixed in: N/A` (no fixed release exists)
  - Reachability: **not imported**. Confirmed by repository-wide search for
    `x/crypto/openpgp`, which returns no matches outside `go.sum`.
  - Rationale: the advisory concerns an unmaintained package the module does not
    depend on at package level, and no fix is published. Tracked as accepted
    risk rather than an open defect. If `openpgp` is ever introduced, this
    finding becomes blocking.

## Not closed by this audit

This audit does not close the following Stage 53U items:

- **G53R-26** security differential — still requires the security reviewer.
- **G53R-25** configuration-consumer and precedence evidence — the Stage 54
  `config-consumer-map.json` is generated, but precedence behavior is not yet
  demonstrated.
- **G53R-20 / G53R-23 / G53R-24** — remain partial pending the exhaustive
  recursive CLI tree matrix.
- Browser cross-platform qualification — blocked; Firefox is not installed and
  Safari cannot run on Windows.

## Promotion status

`reconciliation/stage53r` is **not** promoted and `master` is **not** moved.
Stage 54 has not executed, and per the Stage 54 §38 rule promotion stays
deferred until the Stage 54 matrix passes.
