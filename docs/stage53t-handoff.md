# Stage 53T — Handoff to Stage 52b and Stage 47

**State at exit.** `reconciliation/stage53r` @ `d14e847`, pushed to
`origin/reconciliation/stage53r`. `master` retained at `c75732a` — see
`docs/stage53t-promotion-policy.md` for why. `VERSION` = `5.0.0-alpha2`. No
`v5.*` tag exists; all 10 historical tags (6 of them `v4.*`) are intact and
unmoved.

**Neither stage needs master to have been promoted.** Both should branch from
`reconciliation/stage53r` @ `d14e847`. Nothing in either track depends on the
promotion decision, and nothing is lost by the delay.

---

## Stage 52b — Dashboard / graph / audit

**Ready to start immediately. No blocker.**

### Inherited working

- Stage 52 dashboard foundation: WASM verifier performing genuine SHA-256 +
  Ed25519, 56/56 smoke checks green.
- Barnes-Hut graph layout, 17/17 Rust tests, with deterministic coincident-node
  handling and finite-force clamps.
- **The browser harness is no longer flaky.** Stage 53S proved the mechanism
  (unguarded `JSON.parse` on an empty post-navigation evaluate, plus cleanup
  escaping its `finally`) and fixed all three contributing defects. 80 runs
  across the stage, 0 failures; profile leaks 5/5 → 0. The harness is now safe
  to build Stage 52b's E2E evidence on.

### Remaining, unchanged by Stage 53T

| Item | Status |
| --- | --- |
| WebGL renderer (currently Canvas 2D) | not started |
| WASM layout on a Web Worker (currently main thread) | not started |
| 50,000 nodes at ≥55 FPS for 10s qualification | not run — **the gate is unmeasured** |
| Vendored HTMX (currently custom `hx-*`) | not started |
| Embedded Inter / JetBrains Mono (currently system fonts) | not started |
| Firefox / Safari qualification | **environment-blocked** — Firefox not installed, Safari cannot run on Windows |
| Signed graph-op chain producer (`addNode`/`addEdge`/`removeNode`/`removeEdge`/`checkpoint`) | not started |
| Checkpoint/replay via `/api/graph/at/{seq}` | not started |
| Release artifacts still labelled `3.4.0-stage3` while `VERSION` is `5.0.0-alpha2` | **unreconciled** — carried from Stage 53R as B-7, needs a release decision |

### Notes for whoever picks this up

- Do not trust `gofmt -l` on `internal/cli/ad/*.go`; those files are CRLF and
  report as wholly unformatted. Normalise line endings in a scratch copy before
  concluding anything about formatting.
- The 50K-node performance claim must be measured, not asserted. Stage 53S
  declined to certify an unmeasured target, and the same standard applies.

---

## Stage 47 — AD CS / MS-WCCE / PKINIT

**BLOCKED-WITH-OWNER on G3206. No work can proceed until the operator
provisions a genuine Windows Server 2022 AD DS + KDC + AD CS.**

### What must not be used to close G3206

Samba, FreeIPA, Certipy fixtures, synthetic CAs, mock KDC responses. Stage 53S
and 53T both declined to substitute any of these, and the prohibition is not
advisory — a substitution would produce a false certification, which is the
specific failure mode this project has spent Stages 20–26 and 53S correcting.

### The remedy, ready to execute

`scripts/ad-lab/adcs-windows-setup.ps1` is present in the trunk. On a genuine
Windows Server 2022 host, the operator runs it, provisions an Enterprise CA,
and Stage 47 resumes against real `CertRequest` / CMS / PKINIT traffic.

### ms-wcce — stranded, with a precise starting point

`internal/protocol/ms-wcce/` is **not in the trunk**. Three files survive on
local branch `stranded-stage-45-46`:

```
internal/protocol/ms-wcce/constants_ldap.go
internal/protocol/ms-wcce/request.go
internal/protocol/ms-wcce/types.go
```

Port these by hand. A wholesale merge is wrong: the workspace lineage's AD,
Kerberos and LDAP files supersede the stranded flat files, and the reconciliation
already took the engagement-scope control from that lineage. Copy the protocol
package only, and reconcile it against current interfaces.

The five recorded defects, with what Stage 53T could verify without porting:

| # | Defect | Verified in stranded code? |
| --- | --- | --- |
| 1 | Duplicate `CertRequest` semantics | not checked — needs porting |
| 2 | Nonexistent `ExtKeyUsage` | not checked — needs porting |
| 3 | Missing `parseURI` | **confirmed absent** — zero occurrences in all three files |
| 4 | SAN URI values silently dropped | not checked — needs porting |
| 5 | Docs reference nonexistent `Transport` / `ErrTransportUnavailable` | **confirmed absent** — neither identifier exists in the package |

Defects 3 and 5 are the same class: the documentation describes an API the code
does not have. That is worth re-testing at the call sites after porting, because
a caller trusting the docs would compile against something that is not there.

Stage 54 §22 is explicit that **a defect is not closed merely because the package
is unused.** All five remain open.

### G3607 is separate

The Samba lab (`scripts/ad-lab/docker-compose.yml` and companions) exists but
has not been started. It qualifies real DNS / Kerberos / LDAP / LDAPS / SMB, and
is **not** a substitute for G3206. The two must not be conflated in any report.

---

## Preserved pending work

No roadmap stage has been removed, merged away or silently dropped:

| Stage | Scope | Blocker |
| --- | --- | --- |
| 47 / 47B | AD CS, MS-WCCE, PKINIT | G3206 — operator provisions CA |
| 48 | Coercion + NTLM relay | Stage 47 |
| 49 | DCSync + DCShadow | Stage 47 |
| 50 | Credential capture | Stage 47 |
| 51 | v5.0.0 GA, full production qualification | Stage 54 matrix |
| 52b | Dashboard, graph, audit, replay | none — start now |

**Recommended order:** Stage 52b first, since it is unblocked. Stage 47 when the
CA exists. Stage 48 onward follow Stage 47. Stage 51 cannot precede Stage 54's
production matrix reaching a decision.
