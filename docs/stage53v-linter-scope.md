# Stage 53V — Linter Scope and Static-Analysis Coverage

- Captured (UTC): 2026-09-26
- HEAD at time of measurement: `f5a81c967d0e3be1e09008c470cfd2f8c868f358`
- Branch: `reconciliation/stage53r`
- Tool: `golangci-lint` 2.13.2 (built with go1.27.1)
- Config: `.golangci.yml` (`version: "2"`)

## Summary

The "0 issues" claim recorded in Stage 53U was **narrow**. That config enabled
exactly one linter (`ineffassign`) and had `staticcheck` commented out. Stage 53V
measured what broader analysis would actually cost, adopted the part that is free,
and documented the part that is not.

**The claim this document replaces:** "golangci-lint = 0 issues" is no longer a
substitute for "static analysis is clean."

## Response A was attempted and measured, not assumed

Stage 53V first enabled the full recommended set and ran it:

```yaml
linters:
  default: none
  enable:
    - govet
    - staticcheck
    - errcheck
    - ineffassign
    - unused
    - nilerr
    - errorlint
    - bodyclose
    - copyloopvar
    - unconvert
    - gosec
    - misspell
    - prealloc
```

`golangci-lint config verify` passed. The run produced **668 issues** (exit 1),
measured with `--output.json.path` and tallied by `FromLinter`:

| Linter | Findings | Adjudication |
| --- | ---: | --- |
| errcheck | 300 | Deferred. Largely deliberate deferred-error handling in cleanup and rollback paths; each needs individual judgment. |
| gosec | 240 | Deferred. Dominated by design-inherent rules, not defects. |
| staticcheck | 41 | Deferred. Highest-value deferral; see remediation below. |
| errorlint | 34 | Deferred. Mechanical, but touches error-wrapping sites across the tree. |
| unused | 27 | Deferred. Some may be intentional API surface. |
| prealloc | 13 | Deferred. Style preference, low signal. |
| bodyclose | 7 | Deferred. Real but small; HTTP response bodies. |
| nilerr | 3 | Deferred. Small but security-relevant; should be triaged early. |
| unconvert | 2 | Deferred. Trivially mechanical. |
| copyloopvar | 1 | Deferred. Trivial. |
| **govet** | **0** | **Adopted.** |
| **misspell** | **0** | **Adopted.** |

The brief's premise — that findings on a codebase under active development are
"typically small and mechanical" — **does not hold here.** 668 findings, of which
`errcheck` and `gosec` are 540 (81%), is not a bounded remediation. Suppressing,
`//nolint`-ing, or partially disabling rules to force green is explicitly forbidden
by Stage 54 §51, so the honest outcome is documented scope, not a fabricated pass.

### gosec composition (why 240 findings is not 240 defects)

| Rule | Count | Adjudication |
| --- | ---: | --- |
| G304 (file inclusion via variable) | 74 | Inherent. A security tool that accepts file paths as arguments will always trip this. The correct fix is a documented allowlist, not code change. |
| G115 (integer conversion overflow) | ~90 | Split across several phrasings. Mostly `int`/`uint32` narrowing in protocol and Kerberos code where the bound is enforced by the decoder. Needs per-site proof, not a blanket cast. |
| G101 (hardcoded credentials) | 11 | Expected in a credential-handling tool. Requires a value-by-value review to separate test fixtures from real material. |
| G404 (weak random) | 11 | Must be reviewed individually; some may be genuine defects if used for security-relevant selection. |
| G703 (other) | 13 | Needs triage. |

## Decision: hybrid — adopt what is free, document the rest

Final `.golangci.yml` enables **`govet`, `ineffassign`, `misspell`**. All three
measure zero findings, so the clean result is now a strictly broader claim than
Stage 53U's:

```
golangci-lint run ./...   ->   0 issues.   (exit 0)
```

The deferred linters are **named in the config file itself** with their measured
counts, so the exclusion is visible to anyone who opens the file rather than
hidden in this document.

## What "0 issues" now does and does not mean

**Does mean:** no findings from `govet`, `ineffassign`, or `misspell` at
`f5a81c9`, and config verification passes.

**Does not mean:** comprehensive static analysis is clean. Specifically NOT
covered: `errcheck`, `gosec`, `staticcheck`, `errorlint`, `unused`, `bodyclose`,
`nilerr`, `unconvert`, `copyloopvar`, `prealloc`. This includes all of
`staticcheck`'s SA-series correctness checks, which is the single most valuable
omission in the table above.

**Explicitly not claimed:** that this codebase is free of static-analysis
defects. The honest statement is that 668 known findings are open and enumerated.

## Remediation priority for a successor stage

Ordered by security value per finding, not by count:

1. `gosec` G101 (11) and G404 (11) — potential real credential/randomness defects.
2. `nilerr` (3) — a nil return after a non-nil error check can mask failures.
3. `staticcheck` (41) — correctness class; the largest true-signal group.
4. `gosec` G115 (~90) — prove decoder bounds per site; fix or document.
5. `gosec` G304 (74) — codify a documented path allowlist; mostly configuration.
6. `errorlint` (34), `unused` (27) — mechanical, low risk.
7. `errcheck` (300) — triage in batches; expect a large share to be intentional.
8. `prealloc` (13), `unconvert` (2), `copyloopvar` (1), `bodyclose` (7) — style and
   small correctness.

## Risk accepted

Until the above is remediated, the project carries **known-unremediated static
analysis findings, including 240 security-rule hits.** That is a recorded,
enumerated, owned risk — not a silent one. It must be stated plainly in any
external security review, and it is a contributing factor to keeping the release
status short of production-ready.

**Owner:** toolchain owner. **Status:** `BLOCKED-WITH-OWNER`.

**Evidence:** `artifacts/stage53v/golangci-lint-broad.txt` (both the 668-finding
Response A run and the final 0-issue run), `artifacts/stage53v/linter-config-verify.txt`.
