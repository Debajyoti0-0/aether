# Stage 38 Completion - Campaign 6: Resilience

Baseline: master @ `5cc7756`; tag build `bb56cf3`; sandboxed
`AETHER_CONFIG_DIR`.

## C6a - Malformed environment

| Probe | Raw result |
| --- | --- |
| Empty `AETHER_PASSPHRASE` | exit 1, `Error: workspace "engage" is passphrase-protected: an empty passphrase is rejected` (clean typed error, no panic) |
| Passphrase containing NUL + lone surrogate | same clean rejection (no mangled hashing, no crash) |
| `AETHER_CONFIG_DIR=NUL:` | tool continues with an empty workspace view (exit 0, `No workspaces...`) - F-40-2: silent empty view on invalid config dir, no diagnostic |

## C6b - Read-only vault.db

| Step | Raw result |
| --- | --- |
| `attrib +R vault.db`, then `audit record` | exit 1, typed: `Error: open vault ...: Access is denied.` (no panic) |
| `attrib -R`, then `audit verify` (master) | exit 0, `VERIFIED (1 entries, 0 tampered)` |

## C6c - Read-only salt.bin

`audit verify` exit 0 (read path unaffected by read-only attr - Windows
permits open-for-read); `audit record` (master) still succeeded. No denial
vector through the salt path.

## C6d - Interrupt (kill mid-exec)

| Step | Raw result |
| --- | --- |
| Governed `exec azure` launched, `Stop-Process -Force` at 150 ms | `KILLED_MID_EXEC=True` |
| `audit verify` (master) after kill | exit 0, `VERIFIED (1 entries, 0 tampered)` - no chain damage |
| `audit record` after kill | exit 0, `Recorded seq 2` - lock released, no recovery ritual needed |
| Killed process output | no `panic`/`goroutine` text (`KILLED_PROC_PANIC=False`) |

Campaign 6 verdict: PASS - no panics, no leaks, no lock leaks; typed
errors throughout.
