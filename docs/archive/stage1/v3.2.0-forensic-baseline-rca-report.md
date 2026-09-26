# AETHER v3.2.0 — FORENSIC BASELINE / RCA / GAP ANALYSIS REPORT

**Audit date:** 2026-09-10
**Scope:** Full repository forensic reconstruction, feature reality audit, architecture audit, RCA, maturity model, and dependency-aware roadmap.
**Method:** Read-only source-level audit. Every claim below cites `file:line`. No code was modified.
**Repo state at audit:** 184 Go files, 57 test files, ~23,570 LOC (incl. ~6,122 test LOC), single module `github.com/Debajyoti0-0/aether`, `go 1.26.0`.

---

# 1 Executive Summary

Aether is **not a mock codebase**. The low-level protocol and execution machinery — OAuth2/OIDC grant flows, WS-Trust RST/RSTR, MS-OAPX PRT exchange, AWS SigV4 (spec-correct, `internal/engine/exec/sigv4.go`), Azure ARM RunCommand, SSM SendCommand, GitHub workflow dispatch, Microsoft Graph CAP policy parsing — is genuinely implemented and live-capable. That is the project's real asset.

However, the platform's **governance layer is architecturally detached from the machinery it claims to govern**:

- The **only risk gate in the system** (`internal/engine/orchestrate/run.go:160-164`) lives in an orchestration path that performs **no external mutations**, is **disabled by `--auto`** (`run.go:94-97`), and is trivially bypassed by not using `aether run` — because the mutating executors (`exec azure/aws/github/parallel`) sit in completely separate CLI paths with **no risk gate, no policy check, no approval, no audit, and no rollback recording** (`internal/cli/exec.go:112-156`).
- **No execution path that mutates cloud state records a rollback entry.** The rollback engine (`internal/engine/rollback/stack.go`) is complete and tested, but is populated only manually via `aether rollback push` (`internal/cli/v3b.go:105-131`).
- The **teamserver enforces zero authorization**, has **no request/response correlation IDs** (`internal/api/protocol.go:28-40`), can **panic and crash** on a publish/unsubscribe race (`internal/api/server.go:184-188` vs `:216`), and its mTLS configuration **cannot succeed out of the box** (self-signed client certs vs `RequireAndVerifyClientCert` with no `ClientCAs` pool — `server.go:37`, `protocol.go:138`).
- **Evidence and audit are decoupled from execution.** The Ed25519-signed audit chain (`internal/store/audit.go`) is written only by the explicit `audit record` command. The workspace journal is a read-modify-rewrite blob that **silently destroys all prior events on a corrupt read** (`internal/workspace/workspace.go:288`) and is O(N²), race-prone, and non-atomic.
- **Workspace "encryption at rest" is void in convenience mode**: `workspace.Open(name, "")` derives the key deterministically from the workspace name (`internal/workspace/workspace.go:163-169`), and the teamserver uses exactly that (`internal/cli/connect.go:114` — verified).
- Several headline capabilities are **misrepresentations**: the PQC module performs zero post-quantum cryptography (`internal/protocol/oauth2/pqc.go` — substring matching only), the SAML forger produces XML-DSig that cannot verify against any real verifier (`internal/protocol/saml/signature.go:42-51`, `internal/engine/token/saml.go:63-82`), the Kerberos cloud pivot emits a **structurally invalid AS-REQ** and a **parser off-by-one against real AS-REPs** (`internal/engine/pivot/cloudkerberos.go:186-272, 320-370`) plus a ccache with a **zeroed session key** (`ccache.go:65` — verified), and `ztna exec` transmits no command at all (`internal/engine/exec/ztna.go:171-191`).
- **Version reality is incoherent**: CHANGELOG claims v3.2.0; every binary reports 2.0.0 (`cmd/aether/main.go:13`, `Makefile:5`); the deb package says 1.0.0 (`deploy/debian/changelog:1`); SARIF output hardcodes 2.0.0 (`internal/cli/capabilities.go:344`).
- **No CI, no release automation, no LICENSE, no SECURITY.md** exist (`github/` directory absent; `bin/aether.exe` — 15 MB — is committed).
- **57 test files, zero integration tests** (`test/integration/` contains only `.gitkeep`), **zero `t.Parallel()`, zero `-race`** anywhere, and several tests validate self-fabricated data (the RL "convergence" test, the synthetic AS-REP round-trip, mock token strings).

**Verdict:** Aether v3.2.0 is a collection of **live-capable protocol primitives and honestly-built infrastructure islands** surrounded by a **fictional governance and coordination narrative**. It is TEST PASSING, not RELEASE READY. The correct next phase is not features — it is wiring execution to authorization, evidence, audit, and rollback through a single spine (Section 35–37).

---

# 2 What Aether Is Today

- A single-binary Go CLI (`cmd/aether/main.go`) with ~45 commands across 24 top-level verbs, built on cobra/viper (`internal/cli/root.go`).
- Three parallel and mutually independent execution engines:
  1. `internal/engine/orchestrate/run.go` — sequential 5-phase "kill chain" (bypass → convert → validate → predict → report), the only place a risk gate exists.
  2. `internal/engine/orchestrator/dag.go` — generic DAG executor, driven by `aether run plan`, which re-enters the CLI root to execute arbitrary commands (`internal/cli/v3.go:104-112`).
  3. `internal/engine/graph/executor.go` — path qualification and runbook generation (output-only).
- A workspace layer that is **filesystem directories + per-file AES-256-GCM blobs**, not BoltDB (see §11).
- A teamserver (`internal/api`) with TLS 1.3, broken mTLS pairing, no correlation IDs, and a **stub command handler** (`internal/cli/connect.go:112-119` returns fabricated `"queued: ..."` success without executing).
- A plugin system that is **compile-in Go packages**, plus a remote registry that installs metadata JSON files that **nothing ever loads** (`pkg/plugins/remote.go:172-173`).
- Persistence spread across **four uncoordinated systems**: workspace record files, workspace journal, signed audit JSONL, planner/rollback JSONL stores.

---

# 3 What Has Actually Been Implemented

Verified real implementations (file:line):

| Capability | Location | Status |
|---|---|---|
| OAuth2 grants (client_credentials, refresh, ROPC, device code) | `internal/protocol/oauth2/client.go:89-187` | Real, live-capable |
| CAE claims-challenge handling + one retry | `internal/protocol/oauth2/cae_handler.go:30-172` | Real |
| WS-Trust 1.3 RST construction + RSTR parse | `internal/protocol/wstrust/request.go:18-59`, `client.go:62-94` | Real envelope; unsigned |
| WS-Trust→SAML1.1-bearer→OAuth relay | `internal/engine/relay/wstrust.go:42-109` | Real, live-capable |
| Device-code relay poll loop | `internal/engine/relay/devicecode.go:21-62` | Real, correct |
| Session stretching (refresh rotation) | `internal/engine/relay/session.go:39-90` | Real (not thread-safe) |
| AWS SigV4 signing | `internal/engine/exec/sigv4.go:18-156` | **Spec-correct** (canonical request, key derivation, RFC 3986 escaping) |
| SSM SendCommand executor | `internal/engine/exec/aws.go:55-115` | Real, mutating |
| Azure RunCommand (sync + async poll) | `internal/engine/exec/azure.go:51-126` | Real, mutating |
| GitHub workflow dispatch | `internal/engine/exec/github.go:40-86` | Real, mutating |
| Parallel fan-out worker pool | `internal/engine/exec/parallel.go:62-117` | Real, order-preserving, race-free |
| JWT alg-confusion forge (RS256→HS256) | `internal/engine/token/confuse.go:38-110` | Real, tested by signature recomputation |
| CAP policy Graph-format parsing + offline evaluation | `internal/engine/cap/parser.go:30-100`, `evaluator.go` | Real model, incomplete evaluation |
| CAP history-window predictor | `internal/engine/cap/predictor.go` | Mechanically real; fabricated confidence (§17) |
| RL planner (tabular Q + linear-TD) | `internal/planner/agent.go`, `agent_linear.go`, `env.go`, `trainer.go` | Real mechanics; synthetic environment |
| Behavior pacing (Gaussian, DST-safe) | `internal/behavior/timing.go:99-160` | Correct, tested |
| Graph ingest (Entra/AWS/GCP JSON + live Entra fetch) | `internal/engine/graph/graph_builder.go:76-259` | Real but lossy (no pagination, no cross-provider edges) |
| BFS pathfinding + path qualification | `internal/engine/graph/executor.go:51-172` | Real; weights ignored; risk averaged |
| Watch diff/autopilot daemon | `internal/engine/watch/daemon.go:25-215` | Real; polls a static file; deletion-blind |
| Rollback LIFO stack | `internal/engine/rollback/stack.go` | Real, tested; **island — nothing populates it** |
| AES-256-GCM record sealing + Argon2id KDF | `internal/workspace/workspace.go:163-213` | Real primitives; deterministic salt; empty-pass mode (§12) |
| Ed25519-signed audit hash chain | `internal/store/audit.go:43-156` | Real; local-key tamper-evidence only |
| mTLS framing protocol (4-byte length + JSON) | `internal/api/protocol.go:73-108` | Real framing; no correlation IDs |
| KEV catalog fetch + path prioritization | `internal/intel/kev.go` | Real feed; arbitrary scoring |
| SARIF / ATT&CK Navigator / PDF / HTML exports | `internal/engine/validate/*`, `internal/engine/graph/visualize.go` | Real generators |
| SAML signature stripping (test harness) | `internal/protocol/saml/strip.go:12-138` | Real, careful string-level scanner |
| KKDCP length-prefixed transport | `internal/engine/pivot/cloudkerberos.go:52-96` | Real, tested |
| uTLS browser-preset TLS dialing | `internal/transport/tls.go:54-88` | Real |

---

# 4 What Has Been Verified

- **Build health:** `go build ./internal/protocol/oauth2/` and full-tree compilation succeed (verified during audit).
- **SigV4 correctness** — step-by-step spec conformance (`sigv4.go:32-156`).
- **Teamserver framing** — 8 MiB cap enforced before allocation (`protocol.go:94-96`).
- **Audit chain tamper detection** — hash linkage + signature re-verification, tested in-place tamper and deletion (`store/audit_test.go:35-87`).
- **Workspace record tamper detection** — GCM auth failure surfaces (`workspace_test.go:73-77`).
- **Parallel pool correctness** — order preservation, per-target failure isolation, cancellation (`exec/parallel_test.go`).
- **JWT forge authenticity** — HMAC recomputation in tests (`confuse_test.go:81-98`).
- **Registry install integrity when hash present** — mismatch rejected (`pkg/plugins/remote_test.go:92-113`).
- **Critical security findings verified directly:** `connect.go:114` (empty-passphrase teamserver open), `server.go:37` (RequireAndVerifyClientCert, no ClientCAs), `remote.go:158` (checksum bypass on empty field), `ccache.go:65` (zeroed keyblock).

---

# 5 What Has NOT Been Proven

- **Any live end-to-end protocol success.** All HTTP tests run against local `httptest` mocks or self-built fixtures. The Kerberos AS-REQ/AS-REP path would **fail against a real KDC** (spec errors, §16). The PRT exchange uses a **fabricated grant type** (`internal/protocol/msoapx/prt.go:115-127`) and a **placeholder proof** (`:174-182`).
- **RL planner convergence** — "verified within 100 episodes" only on a self-defined 3-action chain (`internal/planner/planner_test.go:87-108`).
- **CAP predictor confidence** — 100% is emitted with zero observations (`predictor.go:162-167`).
- **Multi-operator teamserver operation** — no test exercises concurrent command dispatch; the client API cross-delivers responses when streaming (`internal/api/client.go:78` vs `:120-130`).
- **mTLS client verification** — every test downgrades to `RequireAnyClientCert` (`teamserver_test.go:79,118`, `mesh_test.go:17`).
- **Replay determinism** — planner re-seeds RNG on policy load (`agent.go:194`, `agent_linear.go:218`) and retains epsilon ≥ 0.05; replay metadata (version/commit/graph hash) is captured nowhere.
- **Rollback of any real mutation** — zero automated push sites exist.
- **Plugin execution from installed artifacts** — installed plugins are never loaded by anything.
- **Race safety** — no `-race` runs anywhere; multiple latent races identified (§24, §27).
- **crash-safety of any persistence path** — no fsync, no atomic rename, no journaling in any store.

---

# 6 Architecture Reconstruction

```
CLI (internal/cli, 26 files, no tests)
  ↓ cobra handlers construct engines directly
┌────────────────────────────────────────────────────────────────┐
│ ORCHESTRATION (three independent spines)                       │
│  orchestrate/run.go   (sequential phases; only risk gate)      │
│  orchestrator/dag.go  (Kahn DAG; re-enters CLI root)           │
│  graph/executor.go    (qualify/runbook; output-only)           │
└────────────────────────────────────────────────────────────────┘
  ↓
ENGINES: exec (aws/azure/github/imds/sp/ztna/parallel),
         token, relay, pivot, cap, validate, planner, behavior,
         intel, watch, rollback, graph
  ↓
PROTOCOLS: oauth2, wstrust, saml, msoapx    PROVIDERS: pkg/plugins/sdk
  ↓
TRANSPORT: internal/transport (uTLS presets, jitter)
  ↓
PERSISTENCE (four uncoordinated systems):
  1. workspace record files  (AES-GCM per-file)
  2. workspace journal       (rewrite-on-append blob)
  3. store/audit.go          (Ed25519-signed JSONL; manual only)
  4. planner + rollback JSONL stores (plaintext, unlocked)
EXTERNAL: Microsoft Graph / login.microsoftonline.com / ARM / SSM /
          GitHub API / Splunk HEC / CISA KEV / plugin index (GitHub raw)
```

**Key structural facts:**
- Engines never call each other through interfaces; the CLI hand-wires everything (`internal/cli/*.go`).
- `pkg/providers/interface.go` defines a `CloudProvider` abstraction with **zero implementations and zero consumers** (dead layer).
- The teamserver does not execute commands — it journals them and returns fabricated success (`connect.go:112-119`).
- Evidence/Audit/Graph/Workspace are **not on the execution path** of any mutating command (§20).

---

# 7 Complete Component Dependency Graph

Actual (as-built) dependencies, from import analysis:

```
cmd/aether/main.go → internal/cli (SetVersion, Execute)
internal/cli → ALL engines + workspace + store + api + plugins + transport + intel
internal/engine/orchestrate → engine/token, engine/validate, engine/cap, workspace, transport
internal/engine/orchestrator → (stdlib only; command strings executed via CLI re-entry)
internal/engine/graph → (stdlib only)
internal/engine/watch → engine/graph
internal/engine/exec → transport, types, (rollback: NOTHING)
internal/engine/relay → protocol/wstrust, protocol/oauth2, transport
internal/engine/token → protocol/msoapx, protocol/saml, types
internal/engine/cap → types, transport
internal/planner → workspace (episode export via CLI, not import)
internal/workspace → paths, golang.org/x/crypto (argon2)
internal/store → (bbolt [dead], zap, crypto/ed25519)
internal/api → workspace (via handler closure), store? NO — audit chain NOT wired
pkg/plugins/* → pkg/plugins/sdk
pkg/providers/interface.go → types  (DEAD — zero impls)
```

**Missing dependencies (should exist, do not):**
- exec → rollback (mutations never recorded)
- exec → store/audit (mutations never audited)
- exec/orchestrate/orchestrator → evidence model (none exists)
- orchestrator → risk/policy (DAG nodes execute ungated)
- planner → cap (CAP state hardcoded `"medium"`, `internal/cli/v32.go:133`)
- teamserver → audit chain (commands only hit an in-memory ring)
- plugins/remote → plugins/registry (installed plugins never registered)

**Circular/inappropriate coupling:**
- `internal/cli/v3.go:108-112` — the DAG executor re-enters the CLI root to run commands: the orchestrator depends on the CLI, inverting the layering.
- `internal/cli/v3b.go:147-154` — rollback undo also re-dispatches through the root command (same inversion).

---

# 8 CLI Capability Matrix

Commands (all verified; full registration map in audit working notes):

| Command | Exists | Real impl | Gate/consent | Audit/evidence | Classification |
|---|---|---|---|---|---|
| `run` | Yes | Yes (orchestrate) | risk gate; **removed by `--auto`** | journal | FUNCTIONAL_BUT_HARDENING_REQUIRED |
| `run plan` | Yes | Yes (DAG→CLI re-entry) | **none** | none | FUNCTIONAL_BUT_HARDENING_REQUIRED |
| `plan train/export/generate` | Yes | Yes | generate: silent degradation | planner store | train/export COMPLETE; generate FUNCTIONAL_BUT_HARDENING_REQUIRED |
| `graph build/qualify/stats/correlate/visualize/generate` | Yes | Yes | none | none | COMPLETE (correlate vacuous on real data, §16) |
| `validate path/risk/profile` | Yes | Yes | warning only | none | FUNCTIONAL_BUT_HARDENING_REQUIRED |
| `validate stealth/soc` | Yes | heuristic table | none | none | PROTOTYPE |
| `exec azure/aws/github/gcp/parallel` | Yes | **Real, mutating** | **none** | **none** | FUNCTIONAL_BUT_HARDENING_REQUIRED |
| `exec imds` / `pivot imds` / `pivot verify-imds` | Yes | Real (v1-fallback) | none | none | FUNCTIONAL_BUT_HARDENING_REQUIRED (v2 handshake hybrid broken, §9) |
| `providers list/users/exec` | Yes | Real (okta/gitlab/k8s) | none | none | list COMPLETE; users PARTIAL; exec FUNCTIONAL_BUT_HARDENING_REQUIRED |
| `prt convert/show/extract/import` | Yes | convert/import real | none | import: journal | FUNCTIONAL_BUT_HARDENING_REQUIRED; show COMPLETE |
| `token show/repurpose` | Yes | Yes | none | none | COMPLETE |
| `token confuse` | Yes | forge real | none | none | **MISREPRESENTED** — `--set-claim` silently ignored (`token.go:93` vs `:127`); `--downgrade-pqc` dead (`pqcCheck` zero call sites, `v33.go:186`) |
| `token protect` | Yes | Real (msoapx) | none | none | FUNCTIONAL_BUT_HARDENING_REQUIRED |
| `pivot cloud-to-onprem` | Yes | Transport real; protocol broken | none | journal best-effort | **PROTOTYPE/MISREPRESENTED** (§16) |
| `relay mfa/devicecode/stretch/cae-handler/mex/saml-strip/ztna` | Yes | Real | none | none | FUNCTIONAL_BUT_HARDENING_REQUIRED |
| `relay fido2-downgrade` | Yes | Builds RST, **never sends** (`relay_cae.go:173-183`) | none | none | MISREPRESENTED |
| `ztna detect/exec` | Yes | detect real; **exec transmits nothing** | none | none | exec MISREPRESENTED |
| `cap parse/evaluate/matrix/predict` | Yes | Yes | none | none | parse/evaluate/matrix COMPLETE; predict PROTOTYPE |
| `cap exploit` | Yes | offline verdict = `risk<40`; probe = arbitrary URL GET | none | none | MISREPRESENTED |
| `simulate` / `simulate fuzz` | Yes | heuristic / real fuzz | none | none | PROTOTYPE / COMPLETE |
| `simulate stream` | Yes | **Real Splunk HEC injection** | **none** | none | FUNCTIONAL_BUT_HARDENING_REQUIRED (offensive, ungated) |
| `export report/graph/sarif/audit` | Yes | Yes | none | n/a | COMPLETE |
| `export executive/attck/pdf` | Yes | Yes, but `--workspace` is a **label only** — never opens the workspace | none | none | MISREPRESENTED |
| `export prioritize` | Yes | Yes (KEV) | none | none | FUNCTIONAL_BUT_HARDENING_REQUIRED |
| `export audit` / `audit audit` | Yes | Yes | none | n/a | COMPLETE / **accidental duplicate registration** (`v3c.go:409-410`) |
| `audit verify/record` | Yes | Yes (Ed25519 chain) | none | n/a | COMPLETE |
| `rollback push/undo/list` | Yes | Yes | undo: none | own plaintext JSONL | push/list COMPLETE; undo FUNCTIONAL_BUT_HARDENING_REQUIRED |
| `replay` / `replay save` | Yes | Yes | `--confirm` prompt | none | FUNCTIONAL_BUT_HARDENING_REQUIRED / COMPLETE |
| `workspace create/list/delete/report/info/rekey` | Yes | Yes | `--force` on delete | journal | FUNCTIONAL_BUT_HARDENING_REQUIRED (**`workspace info` cannot take `--passphrase` — flag not registered, `workspace.go:120` vs `:151`**) |
| `connect` | Yes | mTLS dial real | `--insecure` | server journal only | PARTIAL |
| `serve` | Yes | **Stub — fabricates `queued:` success** | none | journal | **MOCK_ONLY** |
| `watch` | Yes | static-file polling only | `--autopilot` + max-risk | journal (errors ignored) | PARTIAL (live polling unimplemented, `v3c.go:73`) |
| `dashboard` | Yes | Static snapshot over plain HTTP | **none** | none | PROTOTYPE (unauthenticated graph/event exposure) |
| `plugins search/install/installed` | Yes | Real install (hash optional) | none | install JSON only | FUNCTIONAL_BUT_HARDENING_REQUIRED |
| `tunnel` | Yes | One proxy GET | none | none | **MISREPRESENTED** (not a tunnel) |
| `doctor` | Yes | Real crypto round-trip | none | temp workspace | COMPLETE |

**Flags of note:** there is **no `--yes`, `--dry-run`, or global `--json`** anywhere. The only consent mechanisms in the entire CLI are `workspace delete --force` (`workspace.go:63-66`) and `replay --confirm` (`replay.go:60-84`). `run --auto` (`run.go:62`) is the only flag that **suppresses** a safety gate.

**Dead flag/var inventory (CLI):** `confuseClaims` bound but never read; `pqcCheck` never called; `behaviorWait` never called (`v31.go:26-32`); `execPreset` read but never bound (`exec.go:63`); `provLimit` consumed but never bound (`v3.go:347`); `runPrioritize` never read (`v33.go:224`); `fuzzRNG` created then discarded (`v3b.go:253-254`); `relay ztna` handler calls `saveTokens` on `relayOutput` which has no registered flag (`v33.go:160`) — file output unreachable.

---

# 9 Protocol Capability Matrix

| Protocol | Component | Real protocol? | Signature verification? | Classification |
|---|---|---|---|---|
| OAuth2 | grants, device code | Yes | N/A (client side) | FUNCTIONAL_BUT_HARDENING_REQUIRED |
| OAuth2/OIDC | token "validation" | Parse-only, **zero validation** (no iss/aud/exp checks, `oauth2/token.go:35-65`) | **None** | Parse-only; "validation" would be misrepresentation |
| OAuth2 | CAE claims challenge | Yes (real shape) | No `authorization_uri` validation; quoted-comma parsing bug | FUNCTIONAL_BUT_HARDENING_REQUIRED |
| OAuth2 | "PQC" support | **No PQ crypto whatsoever** — substring markers + advisory HS256-confusion suggestion (`pqc.go:116-151`) | None | **MISREPRESENTED** |
| SAML | assertion build (forgery) | Structurally valid build | N/A | PARTIAL (parser validates nothing, swallows time-parse errors `assertion.go:205-210`) |
| SAML | XML signature | **Not XML-DSig** — signs raw bytes, never signs SignedInfo, no C14N (`signature.go:42-51`); spliced signature placement is malformed XML (`engine/token/saml.go:75-81`) | `VerifyDigest` has **zero callers, zero tests** | PROTOTYPE (output cannot verify against ADFS/Entra/Shibboleth) |
| SAML | signature stripping | Real string-level stripper (defensive test harness) | N/A | FUNCTIONAL_BUT_HARDENING_REQUIRED |
| WS-Trust | RST/RSTR | Real WS-Trust 1.3 envelope; unsigned; **no TLS scheme enforcement** (`client.go:72`); extracted assertion is a re-serialization that drops namespaces (`request.go:114-145`) | None | FUNCTIONAL_BUT_HARDENING_REQUIRED |
| WS-Trust | downgrade | Real RST splicing + MEX string-matching; **never submitted** | None | PARTIAL; dead `DowngradeResult` (`downgrade.go:88-93`) |
| MS-OAPX | PRT→OAuth exchange | **Grant type `prt_sso` is not a registered grant**; proof = bare SHA256 (not HMAC/derived-key, `prt.go:174-182`); client nonce fabricated deterministically (`:186-194`) | None | PROTOTYPE |
| MS-OAPX | channel binding | Spoof injector; "tls-unique" defined as SHA-256 blob matches **no RFC** (RFC 5929 tls-unique = TLS 1.2 Finished verify_data; undefined in TLS 1.3) | N/A | PROTOTYPE |
| MS-OAPX | device registration | Real keygen/CSR; **no network flow, no TPM attestation** | N/A | PARTIAL |
| Kerberos/KKDCP | cloud-to-onprem | Transport real; **AS-REQ invalid per RFC 4120** (missing pvno/msg-type, padata tagged [1] not [3], req-body [2] not [4] — `cloudkerberos.go:260`); **AS-REP parser off-by-one on all field tags** (`:320-370`); ccache non-conformant with **zeroed session key** (`ccache.go:65`) | N/A | PROTOTYPE/MISREPRESENTED |

---

# 10 Provider Capability Matrix

| Provider | ValidateToken | Discovery | Execute | Notes |
|---|---|---|---|---|
| gcp (`pkg/plugins/gcp`) | Real REST | Real (instances, IAM SAs) | **Staged only** — returns a gcloud string (`gcp.go:132-163`) | Honest staging |
| gitlab | Real (`/api/v4/user`) | Real (projects, cap 100) | **Real pipeline dispatch** (`gitlab.go:74-102`); `target` unescaped in URL path | Real mutation, ungated |
| kubernetes | Real (SelfSubjectReview) | Real (pods) | Staged only | No kubeconfig/TLS support |
| okta | Real (`SSWS`) | Real (users, cap 200) | **Broken** — client-credentials POST with **no client auth** (`okta.go:76-93`) | Mock-grade |
| `pkg/providers.CloudProvider` | — | — | — | **DEAD CODE** — zero implementations, zero consumers |

Plugin wiring bug: all three registry providers are constructed with **the same** domain+token (`internal/cli/v3.go:365-371`).

---

# 11 Workspace / Storage Audit

**Ground truth:** The workspace is `<ConfigDir>/aether/workspaces/<Name>/` with `db/` (per-record AES-256-GCM files), `artifacts/`, `reports/` (`internal/workspace/workspace.go:47-63`).

**Critical findings:**

1. **No path validation anywhere.** Workspace names, record keys, and artifact names go straight into `filepath.Join` (`workspace.go:51,69,105,276,332`). `Create("../evil")`, `Delete("/some/path")`, artifact `..\..\..\x` all escape the root — and `Delete` **shreds** (zero-overwrite) whatever files it resolves (`workspace.go:111-161`). The only test is the empty-name case (`workspace_test.go:229-233`).
2. **Deterministic salt + empty-passphrase mode = name-derivable key.** `DeriveKey` uses `sha256("aether-salt:"+name)` as the Argon2id salt (`workspace.go:163-169`); `Open(name, "")` is legal and the teamserver uses it (`connect.go:114` — verified). "Encrypted at rest" is void for any workspace opened this way.
3. **No workspace ID** — the name is the identity. No creation timestamp, no metadata record.
4. **Journal is a rewrite-on-append blob** (`workspace.go:287-319`): O(N²), no locking (lost updates), non-atomic overwrite (crash destroys it), and **`events, _ := w.loadEvents()` silently resets a corrupt journal to one event, permanently destroying history** (`workspace.go:288`).
5. **Rekey is non-atomic** (`internal/workspace/rekey.go:9-64`): in-place per-record overwrite, mutates shared `w.pass` 2–3× per record (race), O(N) Argon2id runs (2 per record), and an empty vault accepts **any** old passphrase.
6. **No fsync, no atomic rename, no backups, no schema version, no vault-wide integrity** anywhere in the cluster. No migration path beyond a legacy directory rename whose error is swallowed (`paths.go:122`).
7. **BoltDB is dead code.** `internal/store/bolt.go` is imported by nothing but its own test. The comment "BoltDB buckets / KV namespaces" (`workspace.go:18`) is false; `DBPath()/vault.aedb` (`workspace.go:42`) is a phantom path.
8. **Four parallel persistence systems** (workspace records, journal, audit JSONL, planner/rollback JSONL) with **zero cross-process locking** anywhere in the repo (no flock/lockfile; no such dependency in `go.mod`).
9. **Artifacts are plaintext**, unencrypted by design (`workspace.go:326-327`); `SaveArtifact` has **zero production callers**; no listing/size-limit API.
10. **Audit chain weaknesses:** Ed25519 seed stored **plaintext next to the log** (`audit.go:188-205`) — tamper-evidence protects only against keyless attackers; `New()` resumes from an **unverified** tail (`audit.go:57-69`); a torn line **bricks Verify** (`audit.go:221-222`); hash canonicalization over `|`-joined fields permits field-boundary ambiguity (`audit.go:182-186`); no cross-process append lock.
11. **Config** is plaintext JSON; search order starts with `./aether.json` (cwd) — a planted cwd config hijacks settings (`paths.go:132-149`).

**Transactional? No. Deterministic? Mostly. Recoverable? No. Auditable? Partially (signed chain exists but is manual-only and local-key). Portable? No (paths, no export).**

---

# 12 Cryptography Audit

| Mechanism | Location | Verdict |
|---|---|---|
| AES-256-GCM | `workspace.go:171-213` | Correct; random 12-byte nonce from `crypto/rand`; **no algorithm/version tag** in blob (no agility) |
| Argon2id | `workspace.go:163-169` | t=3, m=64 MiB, p=4 — reasonable but **hardcoded**; **deterministic name-derived salt**; empty passphrase accepted |
| Key lifecycle | — | No key check on Open; no rotation (except manual rekey, itself unsafe); keys in env vars (`AETHER_PASSPHRASE`, `v3c.go:26`) and CLI flags |
| Ed25519 audit signatures | `store/audit.go:91-92,188-205` | Real; **seed stored unencrypted beside the log**; no rotation/escrow/external key |
| JWT handling | `oauth2/token.go:16-91` | Decode-only, **no signature verification anywhere** (by design, documented) |
| RS256→HS256 confusion forge | `engine/token/confuse.go:38-110` | Real and correct (PKIX-DER HMAC key); offline artifact only |
| XML signature | `saml/signature.go:42-61` | **Broken** — no C14N, SignedInfo never signed, digest over raw bytes; will not verify against any real verifier |
| TLS | `transport/tls.go:54-88` | uTLS presets; **no MinVersion**; `InsecureSkipVerify` plumbed but currently inert; **proxied HTTPS silently bypasses uTLS** (`transport/http.go:95-106`) |
| mTLS | `api/server.go:34-40`, `protocol.go:121-143` | Configured but **unachievable** — self-signed client certs, no `ClientCAs`, ephemeral unpinned server cert; all tests downgrade |
| Nonce/RNG | `behavior/timing.go`, `planner/rng.go`, `saml/assertion.go:31-33`, `wstrust/client.go:152-156` | `math/rand` for behavioral models (acceptable); `randomID` ignores `crypto/rand` errors (two sites); teamserver session key field declared and never used (`server.go:29`) |
| Kerberos ccache keyblock | `pivot/ccache.go:65` | **Zeroed 32 bytes** — placeholder |
| PRT session-key proof | `msoapx/prt.go:174-182` | Bare SHA256; **not the real scheme** |
| Memory hygiene | `transport/stealth.go:90-96` | `MemCleanStrings` is a **no-op placebo** (zeroes a copy; Go strings are immutable) |
| Secure delete | `workspace.go:126-161` | Single-pass zeroing; weak vs SSD/COW; has an offset/write bug at `:143` |

**Downgrade resistance:** none in any direction — modules *implement* downgrades (oauth2/pqc, wstrust/downgrade) and none detects one.

---

# 13 Teamserver Audit

- **Transport:** TLS 1.3, `RequireAndVerifyClientCert` (`server.go:34-40`) — but **cannot succeed**: client certs are self-signed (`protocol.go:138`) and `ClientCAs` is never set; every test downgrades to `RequireAnyClientCert`.
- **Authentication:** cert possession only; presented certs are **never inspected** (no `PeerCertificates()` call in the repo); `Operator` field is client-asserted and spoofable (`protocol.go:31`).
- **Authorization: complete absence.** Any client → any command in any workspace → any event stream (`server.go:91-147`). No roles, no capabilities, no ACLs.
- **Correlation: absent.** `CommandRequest`/`CommandResponse`/`Envelope` carry no request IDs (`protocol.go:28-70`). Serial per-connection processing; no multiplexing. Concurrent client use **cross-delivers responses silently** (`client.go:78` vs `:120-130`).
- **Dispatch:** `s.Run` executes **synchronously on the connection goroutine** (`server.go:98`) — a slow command blocks all subsequent frames. And the production handler is a **stub**: journals and returns `"queued: "+cmd` with `OK:true` (`connect.go:112-119`) — `connect --exec` output is fabricated success.
- **Event streaming:** per-workspace ring (100 entries) + buffered fan-out; **send-on-closed-channel panic race** between `Publish` and `unsubscribe` (`server.go:184-188` vs `:201-217`) — can crash the server.
- **Connection lock-in:** after subscribing, a connection can never issue another command (`server.go:140-147`).
- **No reconnect/resume** (no cursor/sequence in `WorkspaceUpdate`, `protocol.go:48-53`); no workspace state sync; no conflict handling.
- **DoS surface:** no read/write deadlines, no connection caps, no rate limits; attacker-chosen `WorkspaceID` grows the events map unboundedly (up to 100 × 8 MB each).
- **Dashboard:** plain HTTP, **no authentication**, serves the full identity graph and last 200 events (`dashboard.go:49-92`); "Live web UI" comment is false — it's a startup snapshot not wired to the teamserver.
- **Tests:** `TestFrameRoundTrip` and `TestFrameTooLarge` are hollow (assert arithmetic on literals; never decode a frame) (`teamserver_test.go:151-177`).

---

# 14 Plugin / Supply Chain Audit

- **Architecture:** compile-in Go packages only (no `plugin` package, no external processes). `sdk.Plugin`/`sdk.Provider` contract (`pkg/plugins/sdk/sdk.go:16-33`); Host is logging-only.
- **Registry (`registry.go`):** in-memory map; **no discovery, no metadata, no hashing, no audit**; completely disconnected from `RemoteRegistry.Installed()`.
- **Remote registry (`remote.go`):**
  - Index fetched over TLS from a single hard-coded GitHub raw URL (`internal/cli/v3c.go:432`) — whoever controls it controls manifests; **no signature over the index**, no TOFU, no pinning.
  - SHA-256 verified **only if the manifest declares one** — `if want != "" && got != want` (`remote.go:158` — verified). Missing hash = silent accept. Test fixture itself ships a hashless manifest.
  - **HASH VERIFICATION exists; SIGNED PUBLISHER AUTHENTICATION DOES NOT.** No signature field, no publisher identity anywhere. The comment "checksum matches the signed manifest" (`remote.go:147-148`) is false.
  - `MinAppVersion` parsed and **never read**. No revocation. No rollback.
  - Installed artifacts are JSON files **nothing ever loads** (`remote.go:172-173`).
- **Sandboxing/permissions: none.** A provider receives the operator's raw token and makes arbitrary HTTP. No capability declaration, no grant flow, no shutdown lifecycle.
- **Concrete plugins:** gcp/k8s stage-only (honest); gitlab performs real ungated pipeline dispatch with URL-path injection on `target`; okta Execute is broken (no client auth, unencoded form). All register with hardcoded version strings.
- **State auditable?** Minimal — install JSON records `installed_at` + `verified_sha256`, unsigned and unchained.

---

# 15 Evidence Model Audit

**Aether has no formal evidence model.**

- No evidence type, no provenance fields, no confidence, no freshness/expiration, no collection-method metadata on any type: `types/path.go`, `types/policy.go`, `types/risk.go`, `types/token.go` all lack provenance/confidence/evidence-ref dimensions.
- The `evidence` "bucket" exists as an encrypted directory (`workspace.go:20-24`) with no schema, no producer, no consumer.
- No distinction between OBSERVED / INFERRED / PREDICTED / UNKNOWN anywhere in the type system or output rendering. CAP predictor prints `confidence 100%` with zero data and no inference labeling (`predictor.go:162-167`, `:209-210`).
- Graph edges have a `Weight` field that is set at ingest and **never read** (`graph_builder.go:23-28`; pathfinding is unweighted, `executor.go:123-172`) — no per-edge confidence, evidence pointer, or timestamp.
- False-positive/false-negative risks are structural: CAP evaluator ignores `excludeUsers`/`includeLocations`/`block` controls (`evaluator.go:116-187`); ZTNA detection flags any Cloudflare-fronted site (`exec/ztna.go:38-48`); `Expired()` returns false for zero `IssuedAt` (`types/token.go:17-22`) — fail-open.

---

# 16 Graph Engine Audit

- **Node types actually produced:** user, group, sp, role (Entra), aws_user, aws_role, gcp_sa, role. **No device, application, subscription, resource, repository, cluster, or trust node is ever created.** `onprem` provider declared, never assigned.
- **Edge types:** `member_of`, `can_assume`, `has_role` produced; **`owns`, `can_exec`, `trusts` are defined and risk-scored but no ingest path ever creates them** (`executor.go:19-26` vs `graph_builder.go`).
- **Synthetic node fabrication:** Entra ingest turns `department` into a fake "group" node (`graph_builder.go:84`) — not a security group.
- **GCP ingest creates dangling edges** — binding members are ingested as bare IDs with no node created (`graph_builder.go:189-192`).
- **Live fetch truncates:** `FetchEntra` fetches users + service principals only (no role assignments) and **no `@odata.nextLink` pagination** (`graph_builder.go:218-259`) — a live-built graph never contains `has_role` edges and silently truncates at page 1. Same for `cap parse` (`cap/parser.go:73-100`) and `ServicePrincipalClient.List` (`exec/service_principal.go:43-61`).
- **No confidence/provenance/temporal validity per edge; no snapshots, versions, or diffs inside the graph package** (diffing only in watch, new-nodes-only — deletions invisible).
- **Cross-provider correlation is real traversal over an empty evidence source:** no ingest function ever creates a cross-provider edge; the correlate test injects one by hand (`correlate_test.go:14`). `graph build` + `graph correlate` in production always prints "No cross-provider paths found."
- **Pathfinding:** correct cycle-safe BFS for all shortest paths; **ignores `Weight`**; `QualifyPath` **averages** risk across steps while the comment claims 0–100 aggregation and claims to "refuse" paths over the ceiling — it only appends a note (`executor.go:88-96`).
- **Runbook commands contain placeholder secrets** and are not executable as generated (`executor.go:101-120`); `graph generate`'s DAG wires the CAE "fallback" node as a *predecessor* of step 1, inverting its semantics (`graph/plan.go:59-61`).
- **O(N²) node upsert** during bulk ingest (`graph_builder.go:37-53`); no thread safety on `IdentityGraph` (handed to the dashboard HTTP handler today, `v3c.go` dashboard path).

---

# 17 CAP Predictor Audit

- **Input:** operator-supplied observations JSON; **no collector exists in the repo** that produces it.
- **Confidence formula:** `min(distinctDays/3, 1)` (`predictor.go:120-126`) — pure sample-coverage; **100% with 3 days**, **100% with ZERO observations** (empty forecast never lowers the initialized 1.0, `:162-167` — asserted as correct in `predictor_test.go:133-142`).
- **No variance, no confidence interval, no freshness decay, no timezone normalization** (local `now` vs UTC history mixing, `v31.go:83` vs `predictor.go:151`); windows are raw medians.
- **Prediction vs certainty:** `RenderForecast` prints `confidence 100%` with no inference labeling (`predictor.go:209-210`). The system **represents prediction as certainty**, which violates the platform's stated epistemic contract.
- **Evaluator gaps feeding predictions:** excludes never checked, `includeLocations` never evaluated, `block` grant ignored, AND/OR operator ignored, sign-in/user risk conditions parsed but never evaluated (`evaluator.go:116-213`).
- **"Exploit" simulation:** offline verdict is `RiskLevel < 40` (magic threshold over arbitrary weights, `exploit.go:142`); live "probe" is a GET with `X-Forwarded-For` and `Sec-CH-UA-Platform` headers that **cannot influence real Entra CAP decisions** (edge evaluates real source IP; platform conditions ride token/device claims). The test server enforces exactly the headers the tool sends — the mirror validates the mirror.

---

# 18 RL Planner Audit

- **State space:** 4×3×3×5 = 180-state grid (token bucket, density ratio, CAP strictness, phase) (`planner/env.go:10-64`); density from a single edge/node ratio; **CAP strictness hardcoded `"medium"` in production** (`v32.go:133`) despite a full CAP evaluator existing in the repo.
- **Action space:** fixed 9-command catalog; the planner **can and will select high-risk actions** — `Critical: true` is a label, not a gate (`trainer.go:104-106`); generated plans execute via CLI re-entry with **no authorization check** (`v3.go:103-113`).
- **Reward:** correct bookkeeping with anti-farming guard (`env.go:128-171`); constants are magic; environment response is **caller-fabricated** — plan rollout assumes always-success/never-detected (`trainer.go:111`).
- **Convergence:** declared on first unchanged epoch of a single canonical state's best Q (float equality, `trainer.go:49-53`) — no policy-stability meaning; "convergence verified" in tests only on a self-defined 3-action chain (`planner_test.go:87-108`).
- **Determinism: NO by default.** `LoadPolicy` re-seeds RNG from `time.Now()` (`agent.go:194`, `agent_linear.go:218`) and retains epsilon ≥ 0.05 (`agent.go:165-170`) — same state+policy+seed is not honored unless the operator hand-sets epsilon to 0 and passes a seed. **Training randomness and production nondeterminism are conflated.**
- **Episode mapping fabricates transitions:** success = `strings.Contains(detail,"fail")` inversion; detection = kind contains "detect"; unknown kinds default to `cap evaluate`; every `command_executed` maps to `exec azure` even for AWS (`episode.go:109-156`).
- **Linear agent:** genuine linear-TD over a 13-dim one-hot φ; weight clamp ±1000 is decorative (50× max achievable Q); `Bias` field serialized, never used (`agent_linear.go:19`).
- **Dead code:** `debug_test.go` is an assertion-free 10,000-epoch print harness committed as a test; `minInt`/`RenderStates` unused; duplicate `NewEpisodeStore` instantiation (`v32.go:91-95`).

---

# 19 Behavioral Model Audit

- **timing.go: correct.** Box-Muller with underflow guard (`timing.go:99-106`), clamped delays (no negative durations; floors ≥ 5s), DST-safe hour reconstruction via `time.Date` in local location (`:144-160`), seeded determinism, context-aware waiting, tested extensively.
- **Limitations:** machine-local timezone only (a UTC server mimics a UTC human — no timezone selection); truncated-Gaussian mean shift uncompensated; persona stats are hardcoded folklore with no provenance (`:42-67`); `behaviorWait` (per-step pacing helper) is dead code (`v31.go:26-32`).
- **shaping.go: PROTOTYPE, unwired.** Header templates frozen at Chrome/Edge 120 (Dec 2023); no header ordering, no HTTP/2 pseudo-header fidelity; the "pacing metadata" doc claim is false (`shaping.go:9-11`); `Shaper` has **zero production callers** — the shipped traffic path uses `transport` presets.
- **Separation from policy:** behavior is correctly NOT an authorization input — good.

---

# 20 Run / DAG / Execution Audit

**Answer to the core question — can execution bypass policy/risk/authorization? Yes, structurally:**

1. The only risk gate (`orchestrate/run.go:160-164`) evaluates a **static policy file**, is **nullified by `--auto`** (`:94-97`, codified in `run_test.go:117-148`), and gates a chain that **performs no external mutations**.
2. The mutating executors (`exec azure/aws/github/gcp/parallel`) are reached through separate CLI paths with **zero gates** (`internal/cli/exec.go:112-156`, `v3.go:184-223`).
3. `run plan` executes arbitrary DAG node commands via CLI re-entry (`v3.go:104-112`) — any plan JSON chains ungated executors with retries and fallbacks. `rollback undo` re-enters the CLI root too (`v3b.go:147-153`) — a recorded undo is itself an ungated execution path.
4. `simulate stream` performs **real Splunk HEC telemetry injection** with no gate (`v3b.go:243-267`).
5. `plugins install` performs real artifact download with optional-only integrity (`remote.go:158`) — an unverified plugin can be installed with no consent step.

**Can execution create unaudited state? Yes — that is the default.** A full engagement (`exec parallel` against 100 targets) leaves **zero records** in any Aether store. Journal writes exist only in orchestrate phases, `prt import`, and `pivot cloud-to-onprem` (best-effort, error-ignored).

**Can it leave unrecoverable mutations? Yes.** SSM/RunCommand shell, GitHub dispatch, and (unwired) SP secret injection mutate external state with **no rollback push, no CommandId capture** (`aws.go:110-114` — CommandId formatted into a string and dropped; `GetInvocation` has zero callers so output can never be retrieved).

**DAG engine defects:** nondeterministic topo order from map iteration (`dag.go:117-121`); fallback executed inline and double-executable if also a node (`dag.go:210-220`); fallback retry accounting fabricated — the test **asserts the inflated number** (`dag.go:213-214`, `dag_test.go:84-96`); "critical abort" is post-hoc (`:279-284`); no retry backoff (`:191-201`); `Start` and `progressed` dead (`:31-35`, `:260-267`); no panic recovery in node goroutines.

**Orchestrate defects:** phases always run, failures never stop the chain (`run.go:63-103`); phasePredict is a pure simulation over hardcoded actions (`:194-205`).

---

# 21 Replay / Simulation Audit

- `replay` re-executes recorded runbook lines through the CLI root; dry-run default, `--confirm` + interactive prompt (`replay.go:59-84`).
- **Determinism is not established:** no captured replay metadata of any kind — Aether version, git commit, OS/arch, provider/plugin/schema/policy versions, graph version, planner policy hash, config hash are recorded **nowhere**.
- Planner-driven replays are nondeterministic by default (epsilon + time-seeded RNG, §18).
- Replay cannot detect environment incompatibility — there is no environment signature to compare.
- `simulate fuzz` is real telemetry mutation with seed determinism (`validate/fuzz.go`); `simulate stream` posts to live Splunk HEC ungated.
- `watch` polls a **static file** — live provider polling is explicitly unimplemented (`v3c.go:73`).

---

# 22 Rollback Audit

- **Entry contents:** reversal recipe only (`Kind/Target/Detail/Undo`) — **no before state, no requested state, no resulting state, no provider operation ID, no etag/resource-version, no idempotency token** (`rollback/stack.go:15-35`).
- **Not transactional:** JSONL append; `Pop` **rewrites the file before validating the popped line** — a corrupt last line is destroyed and its error returned (`stack.go:95-103`).
- **Failed reversals are permanently dropped** — popped, not retried, not dead-lettered (`stack.go:162-183`; "verified" by `stack_test.go:90-114`).
- **Not idempotent:** push dedup doesn't exist; IDs are `act-<UnixNano>` (`stack.go:62`).
- **Concurrency:** per-instance mutex only; cross-process append-vs-rewrite race demonstrated in its own test (`stack_test.go:135-144`).
- **Not auditable:** plaintext, unsigned, no journal/audit integration; undo outcomes printed to stdout only.
- **Island status:** zero callers outside `internal/cli/v3b.go`. No exec/orchestrate/orchestrator path records entries. Structured `UndoAction.Op` primitives (`remove_password`, `remove_member`) are **never executable** — the CLI returns "requires manual reversal" for any entry without a raw `--undo-cmd` (`v3b.go:155`). The `pipeline_dispatch`/`sp_secret` undo taxonomy anticipates mutations that nothing ever records.

**A reverse command existing ≠ reversibility. Aether today cannot roll back anything it did not manually record, and destroys records of failed reversals.**

---

# 23 Watch / Event Architecture Audit

- Polling only (`watch/daemon.go:128-160`); no provider/webhook/file-watch subscriptions; the only poller is static-file reload.
- Diff is new-nodes/new-edges set difference only — **deletions and modifications are invisible** (a revoked role assignment never generates an event) (`daemon.go:25-80`).
- "New path" discovery reduces to per-new-edge 2-node queries (`:69`); combinatorial new paths are never found.
- No dedup across ticks (flapping edges re-fire); no event store in the daemon (CLI journals to the destructible workspace journal, errors ignored — `v3c.go:104`).
- Autopilot executes synchronously inside the tick — a long action blocks polling (`:199-204`).
- The Event→Delta→Affected-Nodes→Affected-Paths→Risk-Delta pipeline **does not exist**: there is no incremental recalculation of risk or paths beyond the narrow per-edge query; every tick rebuilds a fresh engine.
- `mustLoadGraph` converts load failure into an **empty graph**, silently producing a useless autopilot run (`v3c.go:115-121`).

---

# 24 Security Threat Model

| Threat | Surface | Current state | Residual risk |
|---|---|---|---|
| Local attacker | Workspace files | AES-GCM records, but **empty-pass mode is name-derivable** (`workspace.go:163-169`, `connect.go:114`); artifacts/audit/exports/planner/rollback stores are plaintext | Full credential & engagement disclosure |
| Malicious workspace | Workspace/record/artifact names, journal, config | **No path validation** → traversal write/shred/delete (`workspace.go:51,105,276,332`); cwd config hijack (`paths.go:132-149`); corrupt journal silently destroys history | Arbitrary file destruction (shred), data loss |
| Malicious plugin | Remote registry | **Unsigned index, optional checksum, no sandbox, raw operator token handed to plugin**, installed artifacts never loaded | Full supply-chain compromise; comment claiming "signed manifest" is false (`remote.go:147-158`) |
| Compromised provider (upstream index) | Same | Same as above | Plugin metadata forgery |
| Malicious/rogue operator (local) | CLI | All mutating commands ungated, unaudited, unrollbacked | Untraceable actions; "authorized use only" is prose only (`exec.go:18`) |
| Malicious teamserver client | `internal/api` | mTLS unachievable; **zero authorization**; spoofable Operator; panic race; unbounded event map; no deadlines | Any client → any command/workspace; server crash |
| Corrupted evidence | Journal/audit/graph | Journal silently self-destructs; audit Verify bricked by one torn line; graph has no provenance to corrupt-check | Silent history loss; undetectable tamper (audit key is local) |
| Corrupted graph | Graph JSON | No integrity, no provenance, no validation on load (beyond unmarshal) | Poisoned paths/feed priorities |
| Compromised update channel | Plugin index URL, KEV URL | TLS-only; no pinning/TOFU/signatures; KEV cache poisoning possible (`intel/kev.go:79-200`) | Metadata/manifest injection |
| Insider exfil | Dashboard | **Unauthenticated plain-HTTP graph + event exposure** (`dashboard.go:49-92`) | Engagement disclosure to anyone on localhost/network |

**Assets at risk:** OAuth tokens, PRTs + session keys, ccache/Kerberos material, engagement topology (identity graph), command history, operator identity.

---

# 25 Testing Maturity

- **57 unit test files; 0 integration tests** (`test/integration/` = `.gitkeep` only); **0 `t.Parallel()`; `-race` referenced nowhere**; no fuzz targets (despite a feature named "fuzz" — that one mutates telemetry strings).
- **Best-in-repo tests:** audit chain tamper/break detection (`store/audit_test.go`), JWT forge HMAC recomputation (`confuse_test.go:81-98`), workspace ciphertext tamper (`workspace_test.go:73-77`), parallel pool cancellation (`parallel_test.go:131-145`), DAG cycle/dep validation, planner state/reward tables.
- **Tautological/hollow tests:** `TestFrameRoundTrip` never reads the frame; `TestFrameTooLarge` asserts arithmetic on a literal; `TestStripNoSignature` discards its own results; `TestRotatingDialerAgainstMock` performs no TLS handshake; `TestDebugLinearLearning` has zero assertions; `soc_predictor_test` asserts the predictor returns its own hardcoded seed data.
- **Self-fabricated-data validation:** RL convergence chain, synthetic AS-REP (encoder and decoder share the same spec error — `cloudkerberos_test.go:126-146`), mock token strings echoed back.
- **Time-fragile tests:** hardcoded 2026-09 dates and `time.Local` dependence (`timing_test.go:44-80`).
- **The only live-network code paths (FetchEntra, KKDCP send, exec backends) have zero HTTP-level tests.**
- What is NOT tested anywhere: concurrency (all), crash/interruption, corruption recovery, path traversal, cross-process locking, mTLS verification, multi-operator dispatch, torn writes, oversized records.

**Passing tests are evidence, not proof.** Multiple tests enshrine bugs (fallback retry accounting) or validate self-consistent wrongness (Kerberos tags, predictor 100% confidence).

---

# 26 Release Engineering

- **Version chaos (11 locations, 3 values):** binary/Makefile/build.sh/debian-rules = 2.0.0 (`main.go:13`, `Makefile:5`, `build.sh:10`, `debian/rules:13`); deb changelog = 1.0.0 (`debian/changelog:1`); CHANGELOG narrative = 3.2.0; SARIF hardcodes 2.0.0 (`capabilities.go:344`); **`build.ps1` injects no version at all** (`build.ps1:61`).
- **No CI/CD whatsoever** — no `.github/`, no workflows, no goreleaser, no dependabot/codeql.
- **No checksums manifest, no signing, no SBOM generation wired** — Makefile `sbom`/`sign` targets exist but depend on external tools and are invoked by nothing (`Makefile:72-83`); no `-trimpath`, no `-buildvcs` control, no `CGO_ENABLED=0` outside debian rules.
- **`bin/aether.exe` (15 MB) is committed** with no provenance, signature, or checksum.
- **Deb packaging broken:** version 1.0.0, `debian/manpages` is **0 bytes** (despite CHANGELOG/PLATFORM claims), maintainer identity mismatch, `Build-Depends: golang-go (>= 1.22)` **contradicts `go.mod` 1.26.0** (build-dep would fail), `install` file duplicates rules logic, Makefile `deb` target only echoes a hint.
- **Docs contradict build:** README "Go 1.22+" vs `go.mod` 1.26.0; PLATFORM claims FreeBSD (absent from Makefile); three build paths with three different target matrices.
- **Governance void:** no LICENSE (while `debian/copyright:6` declares MIT), no SECURITY.md, no CODEOWNERS, no CONTRIBUTING.
- **Changelog lags and leads code:** all nine "releases" dated 2026-09-09 with no artifacts/tags; features scheduled for v3.3/v3.4 (ZTNA, PQC downgrade, KEV prioritization) already exist in the tree (`v33.go:101,181,228`); the v2.0.0 impacket/ccache claim is invalidated by the code's own placeholder-keyblock comment (`ccache.go:59-65`).

**Verdict: v3.2.0 is TEST PASSING, not RELEASE READY.**

---

# 27 Root Cause Analysis

**RCA-1: Governance detached from execution.**
- Observation: mutating commands have no gates, audit, or rollback.
- Technical cause: executors are standalone CLI leaves; `orchestrate` is a separate reporting pipeline.
- Architectural cause: no shared execution spine — there is no Action abstraction that all mutations must pass through.
- Consequence: policy, risk, evidence, audit, rollback are all advisory.
- Required design: single Action pipeline (authz → risk → policy → execute → evidence → audit → rollback) as the only path from CLI to external effect.

**RCA-2: Persistence fragmentation.**
- Observation: four uncoordinated stores; journal self-destruction; no locking.
- Technical cause: per-feature JSON/JSONL files written directly; workspace API bypassed for reports/exports/engine state.
- Architectural cause: no storage contract; workspace began as BoltDB (abandoned in place) and grew ad-hoc file storage.
- Consequence: no transactionality, no recoverability, races, silent data loss.
- Required design: one storage layer (real Bolt or SQLite) behind a Workspace interface with fsync + atomic rename + file lock; all stores behind it.

**RCA-3: Teamserver built to a single-operator mental model.**
- Observation: no correlation IDs, no authz, mode lock-in, panic race.
- Technical cause: `CommandRequest` lacks identity; shared `c.Next` channel; snapshot-then-send without lifecycle coordination.
- Architectural cause: transport layer never defined request identity or operator capabilities.
- Consequence: multi-operator is structurally unsupported despite the "multi-operator" narrative.
- Required design: RequestID + dispatcher + per-request channels + operator identity extracted from client certs + capability checks per command.

**RCA-4: Tests validate mirrors.**
- Observation: synthetic fixtures, self-built AS-REPs, mock token strings, tautological assertions.
- Technical cause: no protocol conformance vectors (no captured real DER, no real XML-DSig verifier) were used.
- Architectural cause: no test-data/fixture strategy; no integration tier (`test/integration/` is empty).
- Consequence: spec errors (Kerberos, XML-DSig, PRT grant) pass CI.
- Required design: golden vectors from real protocol captures; contract tests per protocol; a mandatory `-race` and integration suite.

**RCA-5: Documentation narrative decoupled from code.**
- Observation: version chaos, false capability claims (ccache, signed manifest, PQC, tunnel, live dashboard, IMDSv2 verification).
- Technical cause: docs written per-feature-sprint without a verification gate; version not single-sourced.
- Architectural cause: no release definition (artifacts + checksums + provenance) tying docs to binaries.
- Required design: single version source via ldflags everywhere; docs CI check; capability claims require a test reference.

**RCA-6: Safety features exist but are optional islands (rollback, audit chain, authz-shaped fields).**
- Technical cause: built bottom-up as data structures, never wired top-down into the execution path.
- Required design: wiring-first roadmap (§36) before any new capability.

---

# 28 Critical Findings (top 15, ranked)

1. **[SECURITY/CORRECTNESS]** Empty-passphrase workspace keys are name-derivable and used by the teamserver — `workspace.go:163-169`, `connect.go:114`.
2. **[SECURITY]** No path validation on workspace names/record keys/artifact names → traversal write/shred/delete outside root — `workspace.go:51,105,276,332`.
3. **[SECURITY]** All mutating executors ungated, unaudited, unrollbacked — `exec.go:112-156`, `aws.go:55-115`, `azure.go:51-92`, `github.go:40-86`.
4. **[SECURITY]** Teamserver: zero authorization, no correlation IDs, spoofable operator identity, panic race — `server.go:91-147,184-217`, `protocol.go:28-40`.
5. **[SECURITY]** Teamserver mTLS cannot succeed end-to-end (self-signed client certs, no ClientCAs); all tests downgrade — `server.go:37`, `protocol.go:138`, `teamserver_test.go:79`.
6. **[SECURITY/SUPPLY_CHAIN]** Plugin checksum silently skipped when manifest omits it; unsigned index/manifest; installed plugins never loaded — `remote.go:158,172-173`.
7. **[CORRECTNESS]** Journal silently destroys all history on corrupt read; O(N²) rewrite; race-prone; non-atomic — `workspace.go:287-319`.
8. **[CORRECTNESS]** Kerberos cloud pivot: invalid AS-REQ, AS-REP parser off-by-one, unusable zero-key ccache — `cloudkerberos.go:186-370`, `ccache.go:65`.
9. **[CORRECTNESS]** SAML signing cannot verify against real XML-DSig (no C14N, SignedInfo unsigned, malformed splice) — `signature.go:42-51`, `engine/token/saml.go:63-82`.
10. **[CORRECTNESS]** CAP predictor reports 100% confidence with zero data — `predictor.go:162-167`.
11. **[SECURITY]** Unauthenticated plain-HTTP dashboard exposing the identity graph — `dashboard.go:49-92`.
12. **[RELIABILITY]** Rollback destroys records of failed reversals; non-atomic; never auto-populated — `stack.go:95-103,162-183`.
13. **[MISREPRESENTATION]** `serve` fabricates success for every command without executing — `connect.go:112-119`; `ztna exec` transmits nothing — `ztna.go:171-191`; `token confuse` advertised flags dead — `token.go:93`/`v33.go:186`; "tunnel" is one GET; `export executive/attck/pdf` `--workspace` flags never open a workspace.
14. **[RELEASE]** Version incoherence (1.0.0/2.0.0/3.2.0), no CI, no LICENSE, committed 15 MB binary, broken deb packaging.
15. **[SAFETY]** `run --auto` downgrades the risk gate to a warning — `run.go:94-97`.

---

# 29 Architectural Debt

- Three independent orchestration engines; no shared spine.
- `pkg/providers` dead layer; `pkg/plugins` registry/remote disconnect.
- BoltDB dead dependency + phantom `vault.aedb`; two divergent abandoned schemas.
- CLI-layer heuristics living in presentation code (`validate stealth` deduction table, `extras.go:75-117`).
- CLI re-entry as the execution mechanism (`v3.go:108-112`, `v3b.go:147-154`).
- Struct duplication between planner `PlanNode` and orchestrator `Node` (JSON-shape-only coupling, `env.go:210-217` vs `v3.go:66-73`).
- Duplicated helpers: `workspaceExists`, `graphDensityFromCounts`, `splitComma`/`splitLines`, `stringsTrimSpace`, `min`, hand-rolled `url.Values`.
- Transport: proxy path silently drops uTLS fingerprint; retry module dead; `MemCleanStrings` placebo.
- Frozen fingerprints (Chrome/Edge 120) across two packages with no refresh mechanism.
- Import-keeper vars papering over unused imports (`extras.go:154`, `v31.go:47`, `providers_exec.go:74`).

---

# 30 False Completion / False Confidence Findings

| Claim surface | Reality |
|---|---|
| CHANGELOG "MIT ccache for impacket/secretsdump" | Zeroed session key; non-conformant layout (`ccache.go:59-65`) |
| CHANGELOG "SHA-256 verification against the signed manifest" | Manifest unsigned; checksum optional (`remote.go:147-158`) |
| CHANGELOG "convergence verified within 100 episodes" | Self-defined 3-action chain (`planner_test.go:87-108`) |
| CHANGELOG "IMDSv2 two-step handshake verified" | AWS/Azure hybrid that matches neither cloud (`imds.go:51-74`) |
| CLI help "tunnel" | Single proxy probe GET (`v3b.go:351-372`) |
| CLI help "relay fido2-downgrade" | Builds RST, never submits (`relay_cae.go:173-183`) |
| CLI help "token confuse --set-claim / --downgrade-pqc" | Silently ignored / never called (`token.go:93`, `v33.go:186-220`) |
| "Live web UI" dashboard | Static snapshot, plain HTTP, unauthenticated (`dashboard.go:11-13,89-92`) |
| `connect --exec` output | Fabricated `queued:` success from a stub server (`connect.go:118`) |
| `export executive/attck/pdf --workspace` | Label only; workspace never opened (`v3.go:322`, `v3b.go:214`, `v3c.go:384`) |
| "workspace (BoltDB buckets)" comment | Filesystem directories; Bolt dead (`workspace.go:18`, `bolt.go`) |
| README "Go 1.22+" | `go.mod` requires 1.26.0 |
| `CapExploit` "live bypass simulation" | Arbitrary-URL GET with headers that cannot affect CAP decisions (`exploit.go:70-92,169-179`) |
| "100% confidence" CAP forecasts | 0 or 3 days of data (`predictor.go:120-167`) |
| PQC module | Zero post-quantum crypto; substring matching (`pqc.go:37-48`) |

---

# 31 Feature Gaps

**P3-class major capabilities (absent entirely):** evidence model & provenance; request-ID protocol & multiplexing; operator authn/authz model; graph provenance/confidence/temporal validity; provider ingest for device/app/subscription/repo/cluster nodes; cross-provider edge synthesis; incremental graph/risk recalculation; environment capture for replay; audit integration for all mutating commands; plugin loading + capability manifests + sandboxing; workspace export/import/backup; schema versioning & migration framework; timezone/persona configuration; integration & conformance test tiers; CI/CD and release pipeline.

**P2-class hardening gaps:** path validation; journal atomicity/locking; rekey atomicity; fsync discipline; audit key escrow/rotation + verify-on-open; pagination everywhere (Graph); panic recovery in worker goroutines; retry backoff & Retry-After; graph risk summation; CAP evaluator completeness (excludes, block, locations, operator); planner determinism controls.

---

# 32 Correlation Matrix

```
                 Stor Audit Roll Exec Pln Grph CAP  Evd TSrv Plug Prov Watch
CLI              R    R    R   R    R    R    R   -   R    R    -    R
Orchestrate      W    -    -   -    -    R    R   -   -    -    -    -
Orchestrator(DAG)-    -    -   R*   -    -    -   -   -    -    -    -
Exec engines     -    -    -   SELF -    -    -   -   -    -    -    -
Planner          R    -    -   -    SELF R    **  -   -    -    -    -
Graph engine     -    -    -   -    -    SELF -   -   -    -    -    -
CAP engine       -    -    -   -    -    -    SELF-   -    -    -    -
Watch            W    -    -   R    -    R    R   -   -    -    -    SELF
Teamserver       W    -    -   R(stub) - -    -   -   SELF -    -    -
Plugins(remote)  -    -    -   -    -    -    -   -   -    R(disconnected) -
Providers(iface) -    -    -   -    -    -    -   -   -    -    DEAD -
```
R = reads/depends, W = writes, R* = via CLI re-entry only, ** = hardcoded "medium" instead of real dependency, `-` = missing dependency that should exist.

**Missing dependencies:** Exec→Audit/Evidence/Rollback (all missing); Orchestrator→AuthZ/Risk (missing); Planner→CAP (faked); Teamserver→Audit chain (missing); Plugins-remote→Registry (missing); Everything→Evidence (evidence model doesn't exist).
**Circular/inverted:** Orchestrator↔CLI (re-entry); Rollback-undo↔CLI (re-entry).
**Duplicated responsibilities:** three orchestration engines; two provider layers (`pkg/providers` dead, `pkg/plugins/sdk` live); three journal-like stores.
**Inappropriate coupling:** CLI constructing every engine inline; validation heuristics in CLI layer (`extras.go`).

---

# 33 Maturity Scorecard

Scale: 0 nonexistent / 1 conceptual / 2 prototype / 3 functional / 4 production-grade / 5 hardened.

| Subsystem | Score | Justification |
|---|---|---|
| CLI | 3 | Broad, functional, real handlers; no gating layer, no tests, dead flags, misrepresentations |
| Transport | 3 | Real uTLS/jitter/limits; proxy-fingerprint gap, dead retry, placebo mem-clean |
| Store (audit/config/log) | 3 | Real signed chain + tamper tests; local-key trust, torn-line fragility, manual-only |
| Crypto (workspace) | 2 | Correct primitives; name-derived salt, empty-pass mode, non-atomic rekey, no agility |
| Protocol (oauth2/wstrust) | 3 | Real flows; no TLS enforcement, no pagination, mock-only tests |
| Protocol (saml/msoapx) | 2 | Forgery builders real; signature/DST of XML-DSig broken; PRT grant fabricated |
| Engines (exec) | 3 | Live-capable, correct SigV4/pool; no output retrieval, no gates, dead exports |
| Engines (graph) | 3 | Real traversal; no provenance/weights; vacuous cross-provider; ingestion lossy |
| Engines (pivot/kerberos) | 2 | Transport real; spec-invalid messages; unusable artifact |
| Workspace | 2 | Real crypto store; traversal, journal destruction, no locking/atomicity |
| Evidence | 0 | No model |
| Graph provenance | 1 | Weight field unread; no provenance/confidence/temporal |
| CAP predictor | 2 | Mechanically real; fabricated confidence; no data source |
| Planner | 3 | Real Q/linear-TD mechanics; synthetic env; nondeterministic by default; no gates |
| Behavior (timing) | 3.5 | Correct, tested, deterministic; unwired Shaper; frozen fingerprints |
| Execution (run/DAG) | 2.5 | Real machinery; gates bypassable/detached; nondeterministic ordering |
| Replay | 2 | Works mechanically; zero environment capture; no determinism guarantee |
| Rollback | 2 | Real tested stack; island; destroys failed records |
| Watch | 3 | Real daemon/gates; static poller only; deletion-blind |
| Plugins | 2 | Install real; unsigned supply chain; never loaded; no sandbox |
| Providers | 1 | sdk contract fine; pkg/providers dead; okta exec broken |
| Teamserver | 2 | Real TLS/framing; no authz/correlation; panic race; stub execution |
| Audit (as a system) | 1 | Signed chain exists; nothing feeds it automatically |
| Testing | 2 | Good negative-path unit hygiene; zero integration/race; mirror-validation |
| Release Engineering | 1 | Scripts exist; no CI, version chaos, broken packaging, committed binary |

**Weighted platform maturity: ≈ 2.3/5 — "functional prototype with production-grade islands."**

---

# 34 Prioritization

| ID | Item | Class | Priority |
|---|---|---|---|
| F1 | Name/key/artifact path validation in workspace | SECURITY | **P0** |
| F2 | Remove/flag-gate empty-passphrase mode; teamserver must not open keyless | SECURITY | **P0** |
| F3 | Unauthenticated dashboard exposure | SECURITY | **P0** |
| F4 | Teamserver panic race (publish/unsubscribe) | RELIABILITY/SECURITY | **P0** |
| F5 | Wire all mutating commands to audit chain + rollback push (before/after state) | SECURITY/AUDIT | **P0** |
| F6 | Plugin: refuse installs without checksum; sign manifests; load or delete dead install path | SUPPLY_CHAIN | **P1** |
| F7 | Single execution spine with authz→risk→policy→execute→evidence→audit→rollback | ARCHITECTURE | **P1** |
| F8 | RequestID + dispatcher + operator identity in teamserver; authz per command | ARCHITECTURE/SECURITY | **P1** |
| F9 | Workspace: single storage layer, file locking, atomic rename, journal append-only + fsync | RELIABILITY | **P1** |
| F10 | Planner determinism (persist seed, force epsilon=0 in generate; capture env hash) | CORRECTNESS | **P2** |
| F11 | Protocol conformance: fix Kerberos tags, XML-DSig C14N/SignedInfo, PRT grant/proof; golden vectors | CORRECTNESS | **P2** |
| F12 | CAP evaluator completeness + confidence semantics (never emit 100% without evidence class) | CORRECTNESS | **P2** |
| F13 | Version single-sourcing + SARIF version + Windows injection; LICENSE/SECURITY.md | RELEASE | **P2** |
| F14 | CI: build matrix, `-race`, vet, lint config; integration tier | TESTING | **P2** |
| F15 | Graph: pagination, provenance fields, cross-provider edge synthesis, weight-aware paths | ARCHITECTURE | **P3** |
| F16 | Replay environment capture (version/commit/graph/policy hashes) | CORRECTNESS | **P3** |
| F17 | Watch: deletion events, dedup, incremental risk delta | ARCHITECTURE | **P3** |
| F18 | Remove/reconcile dead code & misrepresentations (tunnel, ztna exec, pqc flags, pkg/providers, bolt, etc.) | UX/ARCHITECTURE | **P3** |
| F19 | Evidence model (typed provenance/confidence across graph/CAP/planner) | ARCHITECTURE | **P3** |
| F20 | Persona/timezone config, fingerprint refresh mechanism | ENHANCEMENT | **P4** |
| F21 | Reproducible builds (-trimpath, CGO=0), checksums+SBOM+sign wired | RELEASE | **P4** |
| F22 | HTML escaping in visualize titles; report polish | P5 | **P5** |

---

# 35 Required Architectural Changes

1. **Action spine (highest leverage).** One `Action` type (target, mutation class, risk, policy refs, evidence refs) flowing through: `AuthZ → Risk → Policy → Approval → Execute → Evidence → Audit → Rollback`. CLI mutating commands, DAG nodes, planner-generated plans, and teamserver commands all become Actions. Nothing else may touch an external system.
2. **Identity & session model.** Operators (cert-bound), capabilities per operator, per-workspace ACLs; teamserver extracts identity from client certs (requires fixing the CA hierarchy).
3. **Protocol v2 for the teamserver.** RequestID, per-request response channels, multiplexed frames, event cursors for reconnect, connection lifecycle (command+stream on one connection).
4. **Storage contract.** Single store (bbolt is already a dependency and currently dead — use it, or embed SQLite) with: file lock per workspace, transactional multi-record ops, WAL/append-only journal with fsync, atomic rename, schema version bucket, workspace/record/artifact name validation, vault-wide manifest for deletion detection.
5. **Evidence model.** `Evidence{ID, Source, Provider, CollectedAt, ExpiresAt, Confidence, Method, SubjectRef}` attached to graph nodes/edges, CAP observations, planner state, and plan steps; every report labels OBSERVED vs INFERRED vs PREDICTED.
6. **Graph upgrade.** Provenance per edge, weight-aware pathfinding, ingestion for owns/can_exec/trusts, cross-provider edge synthesis (federation/OIDC trust discovery), snapshot versioning + diff.
7. **Supply chain.** Signed manifests (Ed25519, reuse the audit key pattern — but with a proper publisher key), mandatory checksums, min-version enforcement, plugin loading or removal of the install path.
8. **Test architecture.** `test/integration` populated; protocol golden vectors; mandatory `-race`; kill the tautological tests; mock servers shared via `test/mock`.

---

# 36 Recommended Development Sequence

**Stage 1 — Stop the bleeding (no new features):**
F1–F5 (path validation, passphrase policy, dashboard auth, panic race, audit/rollback wiring for executors). Add `-race` + CI skeleton (F14). Version single-sourcing + LICENSE (F13).

**Stage 2 — Foundations:**
Storage contract (F9). Action spine (F7). Teamserver protocol v2 (F8). Kill dead layers (`pkg/providers`, bolt, phantom paths) and misrepresentation fixes (F18).

**Stage 3 — Correctness:**
Protocol conformance (F11). CAP evaluator + confidence semantics (F12). Planner determinism (F10). Graph provenance + pagination (F15).

**Stage 4 — Platform:**
Evidence model (F19). Replay environment capture (F16). Watch deltas (F17). Plugin signing/loading (F6 completion). Release engineering (F21).

Only after Stage 4 should new offensive capabilities be considered.

---

# 37 Dependency-Aware Roadmap

```
Workspace integrity ─→ Evidence model ─→ Graph provenance ─→ CAP confidence ─→ Planner explainability
        │                                                        │
        └──→ Audit-on-execute ─→ Rollback capture ─→ Replay determinism
                                          │
Action spine (F7) ──→ Teamserver v2 ─→ Multi-operator ─→ Workspace sync
        │
        └──→ Plugin sandbox ─→ Signed supply chain ─→ Plugin loading
```

Every arrow is a hard prerequisite: multi-operator is meaningless without protocol v2 and storage locking; explainable plans are meaningless without graph provenance and an evidence model; deterministic replay is meaningless without audit-on-execute.

---

# 38 v3.3 Blueprint — "Safety Wiring"

- F1–F5 complete; CI with `-race`/vet; version single-sourced; LICENSE/SECURITY.md.
- All executors: capture before/after state, push rollback entries, append audit events (reuse `store/audit.go`).
- `--auto` requires an explicit policy file + prints a prominent approval record; `--yes` added but logged.
- Dead flag cleanup (`token confuse`, `relay ztna --output`), remove `audit audit` duplicate, fix `workspace info --passphrase`.
- Exit criteria: every mutating command leaves a signed audit entry and a rollback entry in tests.

# 39 v3.4 Blueprint — "Storage & Spine"

- Single storage layer (locking, atomicity, fsync, schema version, traversal-safe names, journal append-only).
- Action spine: AuthZ → Risk → Policy → Execute → Evidence → Audit → Rollback.
- `pkg/providers` removed; sdk contract extended with capability declarations.
- Exit criteria: no production code path writes outside the storage contract; DAG nodes route through the spine.

# 40 v3.5 Blueprint — "Teamserver v2"

- RequestID protocol, multiplexed frames, per-request channels, event cursors, reconnect.
- Operator identity from client certs (fix CA chain); per-command capability checks; rate/deadline hardening; dashboard behind auth + TLS.
- `serve` executes real Actions through the spine or fails honestly.
- Exit criteria: concurrent multi-operator test suite (commands + streams + reconnect) passes under `-race`.

# 41 v3.6 Blueprint — "Protocol Truth"

- Kerberos: RFC 4120-correct AS-REQ/AS-REP with golden vectors; real ccache (or remove the impacket claim).
- XML-DSig: real exclusive-C14N + SignedInfo signing, or remove the forger; parser gains conditions/audience/signature-optional modes.
- PRT: implement the real broker flow or reclassify as research; remove the fabricated grant type.
- IMDSv2 split per-cloud; transport proxy/uTLS fix; retry wired with Retry-After.
- Exit criteria: conformance suites against captured traffic.

# 42 v3.7 Blueprint — "Evidence & Graph"

- Evidence type across the platform; graph provenance/confidence/temporal validity; weight-aware paths; owns/can_exec/trusts ingestion; cross-provider edge synthesis; graph snapshots/diff.
- CAP evaluator completeness; confidence bands (never bare 100%); observation collector.
- Exit criteria: every attack path and CAP strategy carries evidence references and labeled epistemic class.

# 43 v3.8 Blueprint — "Determinism & Replay"

- Planner: persisted seeds, epsilon=0 generation mode, environment hash in plans; replay validates env hash; watch deletion events + dedup + risk deltas.
- Exit criteria: same state+policy+seed+graph version ⇒ byte-identical plan and replay.

# 44 v3.9 Blueprint — "Supply Chain & Release"

- Signed plugin manifests, mandatory checksums, version compat enforcement, plugin loading with capability manifests.
- Reproducible builds, checksums, SBOM, signing wired into CI; goreleaser; deb fixed (versions, manpages, Go version); remove committed binary.
- Exit criteria: reproducible artifact set with provenance for every release.

# 45 v4.0 Architecture Target

The control-plane/data-plane target from the mission is achievable on top of the above: AuthZ/Risk/Policy as the control plane over the Action spine; Evidence/Graph/Store as the data plane; Planner→DAG with Simulation and Execution as the two sanctioned outlets; Events feeding Audit, Watch, and Timeline; Teamserver as a thin multiplexer over the spine. Nothing in the current codebase prevents this target — the raw machinery is largely present; what is missing is the spine, the identity model, the storage contract, and the evidence semantics.

# 46 v4.0 Release Gate

1. Zero mutating code path outside the Action spine (enforced by lint/test).
2. Every external mutation: audit entry + rollback entry + evidence record, verified by integration test.
3. Teamserver: correlation IDs, cert-bound operator identity, capability authz, `-race` multi-operator suite green.
4. Workspace: locking, atomicity, crash-recovery tests green; traversal suite green.
5. Protocol conformance suites green (OAuth2/WS-Trust/SAML/Kerberos golden vectors).
6. Planner/replay determinism tests green.
7. CI: build matrix + `-race` + vet + lint + integration; signed, SBOM'd, checksummed artifacts; single version source.
8. No misrepresentation: every CLI help claim has a test reference; docs regenerated from code.

# 47 Final Engineering Verdict

Aether v3.2.0 is a **credible offensive-engineering toolbox wrapped in an unimplemented platform**. The protocol machinery, executors, crypto primitives, and several engines are real and in places excellent (SigV4, audit chain, confuse forge, behavior pacing, parallel pool). But the properties that would make it a *platform* — authorization, evidence, auditability, determinism, recoverability, multi-operator correctness — are either absent, optional, or decorative. The system as it stands **cannot prove who authorized an action, cannot roll back what it mutates, cannot explain its confidence, and cannot safely support the next generation of capabilities.**

**Do not add features. Build the spine. Stage 1 (§36) is the only acceptable next sprint.**

---

*Report generated from read-only source audit; all citations re-verifiable at the listed `file:line` locations. Working notes from eight parallel cluster audits (CLI, workspace/store, graph/pivot/watch/rollback, CAP/planner/behavior, exec/orchestrate/token/relay, protocols/transport, teamserver/plugins, testing/release/docs) are consolidated above.*
