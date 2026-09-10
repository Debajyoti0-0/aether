# Changelog

## v3.2.0 — Reinforcement Learning Planner (Sprint 2 of the v4.0.0 roadmap) (2026-09-09)

### RL Planner (P0)
- **Discretized environment** (`internal/planner/env.go`): state space of token bucket (0-3), graph density (low/medium/high), CAP strictness (open/medium/strict), and workflow phase (recon→access→persist→execute→done); a fixed 9-action catalog with OPSEC risk and gain classification; a calibrated reward function (token gain +10, path discovery +4, execution +20, failure −10, detection −15, high-risk −5, step cost −1).
- **Tabular Q-learning agent** (`internal/planner/agent.go`): epsilon-greedy with forward-phase optimism bias, per-episode epsilon decay, policy JSON persistence/loading, and convergence verified within 100 episodes on the mock chain (unit test).
- **Episode export** (`internal/planner/episode.go`): `aether plan export` converts the workspace journal into training episodes (JSONL) by mapping event kinds to catalog actions; failures/SOC hits earn negative rewards.
- **Offline trainer** (`internal/planner/trainer.go`): `aether plan train` replays episodes for N epochs with convergence reporting (epoch at which the goal-action Q-value stabilized).
- **Policy-driven planning**: `aether plan generate --policy policy.json` walks the learned policy into a DAG plan (dependency-chained, critical flags on high-risk nodes, cycle-safe with visited-state tracking), directly consumable by `aether run plan`.

### Fixed
- **Duplicate `watch` command registration** (flagged in gap analysis): watchCmd was added to root twice; the CLI help now lists it once.

### Scheduled (later v4.0.0 sprints)
- v3.3.0: ZTNA broker exploitation (plugin-based, Zscaler PoC) + PQC signature downgrade (classic-alg fallback via the existing confuse engine).
- v3.4.0: MITRE-based target prioritization (`run --prioritize`), container escape extensions to `pivot imds`.
- v4.0.0: `forge` (synthetic activity), `deceive` (honeytokens), `collaborate` (P2P mesh), SBOM/signing release pipeline.

## v3.1.0 — Behavioral Mimicry & Predictive CAP (Sprint 1 of the v4.0.0 roadmap) (2026-09-09)

### Behavioral Mimicry (P0)
- **Human timing model** (`internal/behavior/timing.go`): Gaussian (Box-Muller) inter-action delays clamped to persona bounds, plausible session-duration draws, work-hours windows (DST-safe, half-hour timezones handled), and four shipped personas — engineer, hr, executive, analyst — with distinct rhythms. Deterministic under `--seed`.
- **`aether run --behavior realistic --persona <p>`**: phases pace themselves with persona-consistent human delays instead of machine speed; `--respect-hours` defers execution until the persona's next active window (weekends honored per persona); `--seed` reproduces a session exactly.
- **Traffic shaping** (`internal/behavior/shaping.go`): browser-realistic header sets (sec-ch-ua family, sec-fetch-*, accept-language rotated per persona locale) applied to outbound requests, with operator override precedence.

### Predictive CAP Engine (P1)
- **`internal/engine/cap/predictor.go`**: infers per-policy daily activity windows from historical activation/deactivation observations (median hour, midnight-wrapping windows, always-on detection), scores confidence by observed days, and forecasts active policies at any time.
- **`aether cap predict --history history.json --at <RFC3339>`**: prints inferred windows + forecast and recommends the least-restrictive execution window in the next 24 hours (hour-by-hour scan, ties → earliest).

### Scheduled (later v4.0.0 sprints)
- RL planner (`plan generate --rl`), ZTNA broker exploitation, post-quantum forging, threat-intel prioritization, container escape modules, `forge`/`deceive`/`collaborate`.

## v3.0.0 — The Autonomous Identity Warfare Platform (2026-09-09)

Final sprint of the v3.0.0 roadmap: continuous monitoring, signed audit trails, full GCP execution, plugin registry, live dashboard, and PDF reports.

### Continuous Monitoring & Autopilot
- **`aether watch`**: polling daemon that refreshes graph snapshots at `--interval`, diffs them against the previous state, and reports newly exploitable paths. Survives transient poll failures.
- **Graph diff engine**: new nodes/edges detection + path qualification through each new edge.
- **`--autopilot`**: automatically executes newly detected paths whose risk stays under `--max-risk`, emitting structured JSON events (detected/executed/reason) and journaling them to the workspace. Manual mode detects and reports without executing.

### Signed Audit Trail (compliance)
- **Tamper-evident chain** (`internal/store/audit.go`): every action is hashed (SHA-256 over seq+timestamp+command+result+prevHash) and signed with a workspace-persisted Ed25519 key.
- **`aether audit verify`**: walks the chain, detecting tampered entries and broken links.
- **`aether export audit`**: signed JSONL export for regulatory submission.

### GCP Execution (the Big-Three clouds complete)
- **GCP provider plugin** (`pkg/plugins/gcp`): raw Compute Engine + IAM REST (no google SDK dependency) — token validation, aggregated instance enumeration, IAM service account discovery, and exec staging with gcloud-equivalent output for OS Login/serial console paths.

### Plugin Registry
- **`aether plugins search`**: queries a remote registry index (name/provider/description search).
- **`aether plugins install`**: downloads artifacts with SHA-256 verification against the signed manifest; records the verified checksum; installs to the OS plugin directory.
- **`aether plugins installed`**: lists local installs.

### Live Dashboard
- **`aether dashboard`**: HTTP server serving the interactive graph visualization plus `/api/graph`, `/api/events` (activity stream), and `/api/health` JSON endpoints. Zero frontend dependencies — reuses the offline visualize engine.

### PDF Reports
- **`aether export pdf`**: board-ready PDF via go-fpdf (pure Go) — cover, executive summary, KPI table, top risks, remediation sections, and a MITRE ATT&CK heatmap of exercised techniques.

### Fixed
- Teamserver sub-leak and fan-out hardening (from v2.6.0 review).

### Deliberately deferred
- Live LSASS extraction remains out of scope (active credential dumping).
- Go native `.so` plugin loading stays behind OS constraints; the registry ships verified manifests and artifacts that the SDK registers.

## v2.6.0 — Crypto Abuse & Operations (2026-09-09)

Closes the v2.5.0 gap analysis: crypto-protocol abuse modules, operational efficiency commands, SIEM streaming, and MITRE integration.

### Crypto & Protocol Abuse
- **`aether token confuse`**: RS256→HS256 algorithm-confusion forgery — signs arbitrary claims with the target's JWKS RSA public key as the HMAC secret (kid preserved, `--set-claim` overrides).
- **`aether token repurpose`**: aud-confusion artifact generation — rewrites the audience claim into an unsigned (alg:none) token for verifiers that skip signature validation.
- **`aether relay saml-strip`**: strips all `ds:Signature` elements from SAML assertions/responses (namespace-tolerant exact-name matching, self-closing + nested element handling). Targets verifiers that skip validation when no signature is present.

### Operational Efficiency
- **`aether graph visualize`**: self-contained interactive HTML export (radial per-provider layout, hover inspection, zero external resources — works fully offline).
- **`aether rollback`**: deterministic engagement cleanup — persistent LIFO stack (`push`/`list`/`undo`) of reverse actions in the workspace; undo runs newest-first and survives partial failure.
- **`aether graph generate`**: auto-generates a DAG plan from a graph path — chains runbook steps into dependent nodes and wires CAE claims-handler fallback into auth steps (`--cae-fallback`).
- **`aether validate profile show|apply`**: engagement presets (banking, red-team, incident-response, purple-team, lab) that set risk ceiling, pacing, TLS preset, and audit posture in one step.
- **`aether tunnel`**: routes all aether traffic through an external HTTP/SOCKS5 forward proxy, with optional per-connection JA4 rotation.

### Integration
- **`aether export attck`**: MITRE ATT&CK Navigator layer JSON — maps exercised actions to Enterprise techniques (T1550.004 PRT replay, T1528 IMDS token theft, T1098 SP secrets, T1110 WS-Trust relay, ...) with exercised-vs-simulated scoring.
- **`aether simulate stream`**: streams fuzzed telemetry variants directly into Splunk HEC (also compatible with Elastic HEC bridges) for purple-team detection-pipeline validation.

## v2.5.0 — Autonomous & Adaptive (2026-09-09)

Implements Phase 1–2 of the v3.0.0 roadmap (adaptive engine, distributed mesh, evasion, cross-cloud synthesis, purple-team fuzzing, executive reporting, official plugins).

### Adaptive Kill-Chain Engine
- **DAG workflow engine** (`internal/engine/orchestrator/dag.go`): topological execution with dependency gating, per-node retries, fallback nodes, critical-failure abort, bounded concurrency, and cycle detection. **`aether run plan --plan plan.json`** executes JSON plans whose nodes are aether commands; the summary reports per-node status/attempts.
- The plan runner validates fallback references up-front (unknown/self fallbacks rejected before execution).

### Distributed Operator Mesh
- **Teamserver fan-out**: `Publish` now broadcasts to **all** subscribers of a workspace (previously only the last subscriber received events). Subscribers are tracked per workspace as slices; snapshot-then-stream replay removes head-of-line blocking; slow clients drop frames instead of stalling publishers.
- **`aether exec parallel`**: bounded worker pool (default 5) fanning one command across a target list (`--targets-file` or `--targets`), with per-target results, order preservation, context cancellation, and a success/failure summary. Backends: azure, aws.

### Intelligent Evasion
- **JA4 rotation pool** (`internal/transport/ja4_pool.go`): thread-safe fingerprint rotation across Chrome/Edge/Firefox/randomized profiles; `NewUTLSClientWithPool` builds HTTP clients that rotate the TLS fingerprint **per connection** via the dialer.

### Cross-Cloud Path Synthesis
- **`graph.SynthesizeChains`**: provider-aware, cycle-safe DFS finding attack chains traversing **3+ providers** (e.g., entra → aws → gcp), with runbooks, risk scoring, provider ordering, and a max-chain bound. Seeded from every provider (chains may start in any cloud).

### Proactive SOC Simulation
- **`aether simulate fuzz`**: generates 1–100 mutated variants of a seed telemetry record (timestamp skew, key casing, extra/missing fields, unicode escapes, nested injection, NBSP substitution) for detection-rule robustness testing. Deterministic under seed; NDJSON output for SIEM replay.

### Enterprise Reporting
- **`aether export executive`**: board-ready report with KPIs (paths analyzed/valid, success rate, average/peak risk, critical findings, detection exposure), top risks, and prioritized P0–P2 remediation guidance derived from the actual edge types (DCSync, AdminTo, HasSession, ResetPassword). Markdown + JSON output.

### First-Class Plugins
- **Plugin SDK** (`pkg/plugins/sdk`): `Plugin` + `Provider` contracts with an adapter.
- **Official plugins**: Okta (`SSWS` API — token validation, user enumeration, token exchange), GitLab (PAT — project listing, pipeline dispatch), Kubernetes (service-account token — SelfSubjectReview validation, pod listing, exec staging).
- **Thread-safe registry** with typed provider lookup; **`aether providers list|users|exec`** CLI.

### Fixed
- Teamserver goroutine/FD leak: replaced-subscriber channels are now closed; unsubscribe no longer double-closes.
- `pkg/plugins` legacy registry removed (dead code per v2.1.0 review); superseded by the SDK.

### Deliberately deferred
- Live LSASS extraction (v3.0 W13–14): rejected as an active credential-dumping implant; PRT acquisition remains with the operator's beacon, and `prt extract` (offline dump scanning) stays the supported path.

## v2.1.0 — The Crown Jewels (2026-09-09)

### New commands (closing the v2.0.0 vision gaps)
- **`aether token protect`**: dedicated Token Protection bypass — converts a PRT while presenting the original host's `tls-unique` channel binding (`x-client-bound`). `token show` renders the binding headers.
- **`aether prt extract`**: scans registry/credential dumps for PRT-shaped JSON and raw base64url cookies (entropy-heuristic filtering), emits normalized PRT JSON.
- **`aether cap exploit`**: live CAP bypass simulation — derives the spoof set from the offline strategy, applies spoofed headers (UA, Sec-CH-UA, X-Forwarded-For), optionally probes a URL. `cap matrix` builds the user×app policy decision matrix.
- **`aether graph correlate`**: cross-provider attack path discovery (Entra → AWS → GCP), each with an executable runbook.
- **`aether run --auto`**: autonomous kill chain — continues past risk gates (recorded warnings), uses the workspace-stored PRT, and adds a PRT-conversion phase to the chain.
- **`aether replay`**: dry-run (default) or confirmed replay of saved runbooks; `replay save` exports a graph path as an operation file.
- **`aether simulate`**: SOC telemetry emulation with JSON output for detection engineering.
- **`aether export`**: engagement reports (markdown), SARIF 2.1.0 path findings, graph topology.
- **`aether workspace rekey`**: rotates the workspace passphrase (re-encrypts all records + journal).
- **`aether validate stealth`**: stealth posture scoring (jitter, TLS fingerprint, SOC exposure, timing).
- **Aliases**: `relay cae` → `cae-handler`, `relay fido2` → `fido2-downgrade`, `exec imds` → IMDS flow.

### Fixes
- **Token Protection is now real**: the channel binding is injected into the actual MS-OAPX exchange (`x-client-bound` on the token request) instead of being loaded and discarded.
- MS-OAPX client supports a base-URL override for testing; binding can be set client-wide or per-request.

## v2.0.1 — Platform Hardening (2026-09-09)

### Cross-platform
- **OS-aware paths**: new `internal/paths` package resolves config/data/cache/workspace directories per OS (`%AppData%` on Windows, `~/Library/Application Support` on macOS, XDG on Linux/BSD), with `AETHER_CONFIG_DIR` / `AETHER_DATA_DIR` / `AETHER_CACHE_DIR` overrides for portable installs.
- **Legacy migration**: v1.x `~/.config/aether` workspaces migrate automatically on first run.
- **`aether doctor`**: host-environment self-test — writable dirs, config discovery, encrypted workspace roundtrip, OS notes.
- **Signals**: build-tagged handlers (`signals_unix.go` / `signals_windows.go`) — SIGTERM on unix, CTRL_C on Windows.
- **Version injection**: `-ldflags "-X main.version=…"` now drives `aether --version`.
- **Build system**: cross-platform Makefile (no `rm -rf`/`mkdir -p`), `scripts/build.ps1` (Windows), `scripts/build.sh` (unix), `make man` (cobra man-page tree), `make completions` (bash/zsh/fish/powershell).
- **Cross-compile verification**: windows/linux/darwin/freebsd × amd64/arm64 all build from one host.
- **Packaging**: debian rules now install man pages + shell completions; static CGO-free binary runs on musl and glibc distros.

## v2.0.0 — The Supremacy Upgrade (2026-09-09)

### Sprint 0 — Fortify
- **Token Protection bypass**: `internal/protocol/msoapx/channel_binding.go` — spoofs the `tls-unique` channel binding (`x-client-bound` header) from a Linux host. Load bindings via `aether prt convert --tls-binding binding.bin` or `aether prt import --tls-binding`.
- **CAE claims handler**: `internal/protocol/oauth2/cae_handler.go` — detects `WWW-Authenticate` claims challenges on 401s, refreshes with the demanded claims, retries once. `aether relay cae-handler`.

### Sprint 1 — Downgrade
- **FIDO2/passkey downgrade**: `internal/protocol/wstrust/downgrade.go` — builds WS-Trust RSTs that pin the authentication context class to `password`, plus device-claim spoofing. `aether relay fido2-downgrade`.
- **MEX parser**: parses WS-MetadataExchange documents, discovers `usernamemixed` endpoints, and reports downgrade feasibility. `aether relay mex`.

### Sprint 2 — Pivot
- **Cloud-to-onprem Kerberos**: `internal/engine/pivot/cloudkerberos.go` — pure-Go AS-REQ construction with Azure AD Kerberos PA-DATA, MS-KKDCP HTTPS transport, AS-REP parsing. `aether pivot cloud-to-onprem` writes a MIT-format ccache (`aether.ccache`) for impacket/secretsdump.
- **MIT ccache writer**: `internal/engine/pivot/ccache.go`.

### Sprint 3 — Graph
- **Cross-provider identity graph**: `internal/engine/graph/graph_builder.go` — merges Entra (users/SPs/role assignments), AWS IAM, and GCP IAM exports; live Graph fetch supported.
- **Executable paths**: `internal/engine/graph/executor.go` — BFS shortest paths, token-availability checks, per-edge risk, and concrete Aether runbooks. `aether graph build|qualify|stats`.

### Sprint 4 — Workload
- **Azure IMDS exploitation**: `internal/engine/exec/imds.go` — IMDSv2 (token) with v1 fallback, managed identity token hijack (client_id/object_id pinning), instance metadata reconnaissance. `aether pivot imds`.
- **Service principal abuse**: `internal/engine/exec/service_principal.go` — SP enumeration, secret injection (`addPassword`), owned-object pivoting.

### Sprint 5 — Platform
- **mTLS teamserver**: `internal/api/` — length-prefixed JSON RPC over TLS 1.3 (ExecuteCommand + workspace event streaming; same contract as the blueprint's gRPC definition, without requiring protoc). Self-signed server/operator cert generation. `aether serve` + `aether connect`.

### Sprint 6 — OPSEC
- **SOC signal predictor**: `internal/engine/validate/soc_predictor.go` — pre-seeded database of Entra sign-in log IDs (50126, 50074, 50177, 53003, 50142, …), Sentinel rules, and Defender alerts with environment-aware probability escalation. `aether validate soc`.
- **Stealth modes**: `internal/transport/stealth.go` — `--low-slow` human-paced jitter (1–5s), `MemClean` zeroization of token/PRT buffers before exit.

### Platform
- **Workspaces**: `~/.config/aether/workspaces/<name>/` with AES-256-GCM encrypted records and Argon2id key derivation (64MB, 3 iterations). `aether workspace create|list|info|report|delete` — delete shreds files with zeros first.
- **Kill-chain orchestration**: `aether run` — CAP evaluation → path validation → SOC prediction → report, gated by `--risk-threshold` and paced by `--low-slow`.

## v1.0.0 — Initial Release (2026-09-09)

- PRT → OAuth conversion via MS-OAPX
- WS-Trust/SAML relay, device code flow, session stretching
- Conditional Access parse + offline evaluation
- Azure RunCommand / AWS SSM / GitHub Actions execution
- BloodHound path validation + OPSEC risk scoring
- uTLS JA3/JA4 spoofing (chrome/edge/firefox), retry with jitter
- BoltDB persistence, JSON config, zap logging with redaction
