# Aether v4.2.0-ga

> **Authorized-testing engineering platform for hybrid identity fabrics**

Aether is a unified, low-level protocol toolkit for modern identity fabrics (Entra ID, OAuth2, OIDC, SAML, WS-Trust, and cloud APIs). It is designed for **authorized penetration testing and red team operations** against hybrid cloud environments.

## Stage 3: Teamserver V2 — trusted distributed control

Remote execution is architecturally subordinate to the same governance boundary as local execution:

```
operator cert (CA-issued, URI SAN aether:operator:<name>)
  → mTLS (TLS 1.3, ClientCAs, revocation list)
  → cert-derived operator identity + capability set
  → intent whitelist → ACTION SPINE (authz → risk → policy → approval → execute → evidence → audit → rollback)
  → correlated response (RequestID ⇄ ActionID)
```

- `serve cert init` bootstraps the CA + server cert; `serve cert issue --operator alice --caps exec.azure` mints credentials; `serve cert revoke` fails closed on new connections.
- `serve` refuses to start without the PKI files; it attaches exactly one passphrase-protected workspace.
- Every remote action produces the same signed audit entries (with `actor=operator:<name>`), evidence record, and rollback registration as local execution.
- Protocol v2: `Version` + `RequestID` on every frame (v1 rejected — no downgrade); 8 in-flight commands per connection; event subscriptions resume from a persisted, sequence-numbered store.
- `connect --operator-cert alice.crt --operator-key alice.key --server-ca teamserver-ca.crt`; `--insecure` requires `--i-know-what-im-doing` and prints a loud warning.

Known limitations (truthful): revocation is connection-time file-based (no OCSP); request idempotency is not provided — retried mutating commands may double-execute (audit-visible); event payloads are transport-authenticated, with the signed audit chain as the integrity anchor. See `docs/stage3-threat-model.md`.

## Stage 2: Storage & Spine

Every mutation flows through one **Action spine** (`internal/engine/spine`):

```
AUTHZ → RISK → POLICY → APPROVAL → before-state → AUDIT(before)
      → ROLLBACK REGISTRATION → EXECUTE → after-state
      → EVIDENCE → AUDIT(after) → journal
```

- Automation (`approval auto`) removes only the human prompt — no stage is ever skipped, and `approval_mode` is recorded in every signed audit entry.
- DAG plans (`run plan`), rollback undo, and replay/watch mutating intents execute through the same spine via a strict intent whitelist; unrepresentable commands fail closed.
- Every Action produces: an Action ID, two signed audit entries, one typed evidence record (epistemic class: observed/inferred/predicted/unknown), and a journal entry.
- `cap predict` never claims certainty: zero observations render `confidence 0% (UNKNOWN)`; predictions cap at 66%.

All workspace state persists through one **canonical storage contract** (`internal/store/vault.go`):

```
workspaces/<name>/
├── vault.db      # bbolt: meta | records | journal | audit | rollback | rollback_failed
├── salt.bin      # random Argon2id salt + HMAC tag (Stage 1)
├── KEYLESS       # marker only for explicitly keyless workspaces (Stage 1)
├── artifacts/    # large raw artifacts
└── reports/      # exported reports
```

- ACID transactions, cross-process file locking (two processes cannot open the same workspace), schema versioning (newer vaults refuse to open), and crash-safe pages.
- The journal is append-only with monotonic sequence numbers; records and journal entries stay per-record AES-256-GCM sealed.
- Rollback `Pop` is atomic; failed reversals are retained in `rollback_failed` and surfaced — never silently discarded.
- Legacy `db/`-directory workspaces migrate idempotently on first open (original directory preserved as `db.pre-vault-imported`).

## Core Modules

| Module | Purpose |
|--------|---------|
| `aether token` | Token Protection & channel binding operations (`token protect`, `token show`) |
| `aether prt` | PRT → OAuth (MS-OAPX), PRT import/extract from dumps |
| `aether relay` | WS-Trust/SAML relay, device code, session stretch, CAE handler, MEX discovery |
| `aether cap` | Parse + offline-evaluate Conditional Access policies, `cap matrix`, history-based `cap predict` |
| `aether exec` | Azure RunCommand, AWS SSM (SigV4), GitHub Actions, `exec imds`, `exec parallel` — all governed (audit + rollback records) |
| `aether validate` | BloodHound paths, OPSEC risk scoring |
| `aether pivot` | Cloud→onprem Kerberos TGT via MS-KKDCP (writes a ccache with a placeholder session key — readable by `klist`, NOT usable for authentication), IMDS |
| `aether graph` | Identity graph build/qualify/stats, visualization |
| `aether workspace` | Encrypted engagement workspaces (AES-256-GCM + Argon2id + random per-workspace salt), `workspace rekey` |
| `aether run` | Kill-chain orchestration with risk gates |
| `aether replay` | Dry-run or confirmed replay of saved operation runbooks |
| `aether export` | Engagement reports (markdown/PDF), SARIF 2.1.0, ATT&CK Navigator, graph topology |
| `aether simulate` | SOC telemetry emulation and fuzzing for detection engineering |
| `aether serve` / `aether connect` | mTLS teamserver v2: CA-issued operator certificates (`serve cert init/issue/revoke`), cert-bound identity, per-operator capability authorization, RequestID-correlated multiplexed commands, resumable event subscriptions, spine-routed execution |
| `aether audit` | Tamper-evident Ed25519-signed audit chain (`verify`/`record`) |
| `aether rollback` | Rollback stack for reversible mutations |
| `aether plugins` | Plugin registry search/install (SHA-256 checksum when the manifest declares one; manifests are currently unsigned) |
| `aether providers` | Okta/GitLab/Kubernetes provider plugins |
| `aether dashboard` | Token-authenticated web dashboard (loopback by default; TLS required off-loopback) |

## Stage 1 Safety Wiring

Every command that mutates external state (**`exec azure`, `exec aws`,
`exec github`, `exec gcp`, `exec parallel`, `providers exec`,
`simulate stream`, `plugins install`, `prt import`,
`pivot cloud-to-onprem`**) now:

1. **requires `--workspace`** — it refuses to run otherwise;
2. records **signed audit entries (before + after)** in the workspace's
   Ed25519 hash chain — verify with `aether audit verify`;
3. registers a **rollback entry before execution** when a reversal
   recipe exists (irreversible mutations are recorded as irreversible
   in the audit chain instead of pretending to be reversible);
4. **fails closed**: if the audit chain or rollback stack cannot be
   written, the mutation does not run.

Workspace integrity hardening:

- workspace names, record keys, and artifact names are validated
  (path traversal, absolute paths, UNC/drive letters, Windows reserved
  names, symlink escapes are rejected);
- workspaces use a **random per-workspace salt** (`salt.bin`,
  tamper-evident) — the key is never derived from the name alone;
- an empty passphrase is rejected unless the workspace was explicitly
  created with `--allow-empty-passphrase` (KEYLESS mode warns on every
  open); legacy workspaces must be migrated with
  `aether workspace rekey`;
- the teamserver refuses to attach a workspace without a passphrase.

Dashboard security:

- binds to `127.0.0.1` by default and prints a one-time access token;
- every route requires the token (`?token=`, `X-Aether-Token`, or
  `Authorization: Bearer`);
- non-loopback binds require `--tls-cert`/`--tls-key` or the dashboard
  refuses to start.

## Architecture

```
CLI (Cobra) → Action spine (authz → risk → policy → approval → audit → rollback → execute → evidence)
            → Protocol layer (OAuth2/OIDC, SAML, WS-Trust, MS-OAPX)
            → Transport (uTLS browser presets, HTTP/2)
            → Vault (bbolt: sealed records, append-only journal, signed audit, rollback)
```

## Build

```bash
go mod tidy
make build          # produces ./bin/aether (host platform)
make test           # unit tests
make test-race      # race detector (requires CGO / a C toolchain)
make build-all      # linux + macOS + Windows, amd64 + arm64
aether doctor       # verify the host environment
```

See [docs/PLATFORM.md](docs/PLATFORM.md) for the full OS support matrix (Windows/macOS/Linux/BSD), per-OS directory layout, signals, and install instructions.

Requires **Go 1.26+** (see `go.mod`). No CGO for release binaries.

## Usage Examples

```bash
# Workspaces: everything belongs to a passphrase-protected workspace
aether workspace create ClientX --passphrase 'op-passphrase'

# Mutating commands require --workspace (audit + rollback are mandatory)
aether prt import --prt prt.bin --tls-binding binding.bin --workspace ClientX
aether exec azure --token <arm-token> --subscription-id <sub> --resource-group rg --vm-id vm-01 --cmd "id" --workspace ClientX
aether exec aws --access-key AKIA... --secret-key ... --instance-id i-123 --cmd "whoami" --workspace ClientX

# Verify what was recorded
aether audit verify --workspace ClientX
aether rollback list --workspace ClientX

# Relay / MEX discovery
aether relay mex --domain corp.com --mex mex.xml
aether relay cae-handler --refresh-token <rt> --tenant <tenant>

# Graph + validation (situational awareness)
aether graph build --input users.json,aws.json --kind entra:users,aws --output graph.json
aether graph qualify --graph graph.json --path u1,role:xyz
aether validate path --bh-json path.json --risk-threshold 30

# Token-authenticated dashboard (loopback default)
aether dashboard --workspace ClientX --graph graph.json

# Teamserver (passphrase required when attaching a workspace)
aether serve --listen 127.0.0.1:7788 --workspace ClientX
aether connect --server 127.0.0.1:7788 --workspace ClientX

# Exit: report + secure delete
aether workspace report --workspace ClientX --output report.md
aether workspace delete ClientX --force
```

## Configuration

`aether.json` in the working directory, or `~/.config/aether/config.json`:

```json
{
  "log_level": "info",
  "timeout": 30,
  "browser_preset": "chrome",
  "max_risk_threshold": 50
}
```

`browser_preset` selects the uTLS fingerprint: `chrome`, `edge`, or `firefox`.

## Known Limitations

- The Kerberos ccache produced by `pivot cloud-to-onprem` carries a
  placeholder session key: readable by `klist`, **not** usable for
  Kerberos authentication.
- `token confuse` is an offline artifact forge; `--set-claim` overrides
  apply to the forged token only (no live token manipulation).
- `relay fido2-downgrade` builds and previews an RST; it does not send
  anything.
- `ztna exec` performs a broker-routed reachability probe; it does not
  execute commands.
- Remote commands execute through the Action spine with cert-bound
  capability checks; request idempotency is not provided — retried
  mutating commands may double-execute (audit-visible).
- Plugin manifests are verified by SHA-256 only when a checksum is
  declared; there is no publisher signature yet.

## Legal

This tool is provided for authorized security assessments only. Unauthorized access to computer systems is illegal. The authors accept no liability for misuse. See [SECURITY.md](SECURITY.md) for the vulnerability disclosure policy.
