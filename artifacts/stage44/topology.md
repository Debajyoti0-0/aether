# Stage 44 — Real Topology (Staging)

**Everything below is the architecture Aether actually has.** No OCSP, no DB/Redis/KMS/object storage, no external provider infrastructure. Those are `NOT APPLICABLE — COMPONENT ABSENT` (confirmed against source and `docs/stage3-threat-model.md`).

## Components exercised (all live, 2026-09-19)

| Component | Evidence |
|---|---|
| mTLS teamserver (`aether serve`) | bound `127.0.0.1:7788`, server cert SHA-256 `60:C8:B9:2F:AA:95:58:2B:38:79:2A:99:3B:25:A2:0E:8C:DF:6C:01:42:B4:F9:EF:5D:1C:AF:16:D9:8C:0B:D0`, CA SHA-256 `FD:CE:BC:14:62:71:38:DC:6F:B3:4A:C1:14:44:EA:DE:F8:75:25:B1:07:7C:2C:44:A9:8C:6E:AA:C5:9D:7C:CF` |
| Staging CA (`serve cert init`) | `teamserver-ca.crt/key` + `teamserver-server.crt/key`, 0600, in isolated temp PKI dir; operators `alice` (exec.azure, graph.read) and `bob` (graph.read) issued via `serve cert issue` |
| File-based revocation | `revoked.txt` beside CA cert; connection-time check (D-003 fix makes it live) |
| Sealed workspaces | `ts-ws` (AES-256-GCM + Argon2id, passphrase redacted from evidence), exclusive vault lock |
| Dashboard (`aether dashboard`) | `127.0.0.1:8443`, per-start 32-byte token, auth via `?token=` / `X-Aether-Token` / `Bearer` |
| CLI (single binary) | `bin/aether.exe` 26 top-level commands |

## Start / stop procedures used

```bash
# PKI + workspace
aether workspace create ts-ws --passphrase '<pass>'
aether serve cert init --dir /tmp/aether-ts/pki
aether serve cert issue --dir /tmp/aether-ts/pki --operator alice --caps exec.azure,graph.read
# serve (foreground; SIGINT/SIGTERM on POSIX)
aether serve --ca-cert pki/teamserver-ca.crt --server-cert pki/teamserver-server.crt \
  --server-key pki/teamserver-server.key --workspace ts-ws --listen 127.0.0.1:7788
# dashboard
aether dashboard --workspace ts-ws --graph g2k.json --port 8443
```

Teardown verified: after process termination, **ports 7788/8443 released (0 listeners), 0 aether processes, vault lock released** (`netstat`/`tasklist` evidence, 2026-09-19).
