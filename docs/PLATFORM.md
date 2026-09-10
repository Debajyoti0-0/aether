# Aether — Platform Support Matrix

Aether compiles to a **static, CGO-free binary** for every major platform. The same codebase runs identically everywhere; OS differences (paths, signals) are resolved at runtime by `internal/paths` and build-tagged signal handlers.

## Supported Platforms

| OS | Arch | Build target | Status |
|----|------|--------------|--------|
| Linux (Debian/Kali/Ubuntu/Fedora/Arch/Alpine) | amd64, arm64 | `GOOS=linux GOARCH=amd64|arm64` | ✅ First-class |
| macOS (Intel) | amd64 | `GOOS=darwin GOARCH=amd64` | ✅ First-class |
| macOS (Apple Silicon) | arm64 | `GOOS=darwin GOARCH=arm64` | ✅ First-class |
| Windows 10/11, Server | amd64, arm64 | `GOOS=windows GOARCH=amd64|arm64` | ✅ First-class |
| FreeBSD | amd64 | `GOOS=freebsd GOARCH=amd64` | ✅ Community |

Any other `GOOS/GOARCH` Go 1.22 supports also compiles — `go build` has no platform-specific dependencies (pure Go, no CGO).

## Per-OS Directory Layout

Override any of these with `AETHER_CONFIG_DIR`, `AETHER_DATA_DIR`, or `AETHER_CACHE_DIR` (useful for USB/portable installs and CI).

| Purpose | Windows | macOS | Linux/BSD |
|---------|---------|-------|-----------|
| Config | `%AppData%\aether` | `~/Library/Application Support/aether` | `$XDG_CONFIG_HOME/aether` (`~/.config/aether`) |
| Workspaces | `%AppData%\aether\workspaces` | `.../aether/workspaces` | `~/.config/aether/workspaces` |
| Data | `%AppData%\aether` | `.../aether` | `$XDG_DATA_HOME/aether` (`~/.local/share/aether`) |
| Cache/Logs | `%LocalAppData%\aether\cache` | `~/Library/Caches/aether` | `$XDG_CACHE_HOME/aether` (`~/.cache/aether`) |
| System config | `%ProgramData%\aether` | `/etc/aether` | `/etc/aether` |

Legacy v1.x workspaces in `~/.config/aether/workspaces` are **migrated automatically** on first run.

## Signals

| Signal | Windows | macOS / Linux |
|--------|---------|---------------|
| Ctrl-C | `CTRL_C_EVENT` ✅ | `SIGINT` ✅ |
| `kill <pid>` / systemd stop | n/a | `SIGTERM` ✅ |

## Building

```bash
# Host platform
make build                # or: scripts/build.sh / scripts\build.ps1

# Every release target
make build-all            # linux/mac/windows × amd64/arm64
# Windows-only: .\scripts\build.ps1 -All

# Verify your environment
aether doctor
```

## Installing

**Kali/Debian/Ubuntu:** `dpkg -i aether_2.0.0_amd64.deb` (installs man pages + completions)

**macOS:** place the binary in `/usr/local/bin` (or `brew install` once tapped)

**Windows:** place `aether.exe` in any `PATH` directory; completions via:
```powershell
aether completion powershell | Out-String | Invoke-Expression
```

**Shell completions (all OSes):**
```bash
aether completion bash    > ~/.local/share/bash-completion/completions/aether
aether completion zsh     > "${fpath[1]}/_aether"
aether completion fish    > ~/.config/fish/completions/aether.fish
```

## Runtime Guarantees

- **Zero CGO** — no libc coupling; the Linux binary runs on musl (Alpine) and glibc distros alike.
- **File permissions** — `0600`/`0700` on unix; ACL-inherited on Windows (no chmod errors).
- **Path separators** — all file handling goes through `path/filepath`; no `/`-string joins.
- **UTF-8 output** — safe on Windows terminals via Go's default console handling.
- **Self-test** — `aether doctor` validates every platform-specific behavior on the host and prints the resolution of each directory.
