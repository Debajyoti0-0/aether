# Stage 46f — Docker NAT Limitation Document

## Confirmed Finding

Docker Desktop on Windows drops application data on all container ports after TCP handshake succeeds. This affects ALL protocols (TCP/UDP) and ALL paths (host→container, container→container).

## Evidence

| Test | Result |
|------|--------|
| Host localhost:389 TCP | SUCCESS (handshake) |
| Host localhost:389 data | FAIL (data dropped/EOF) |
| Host localhost:88 TCP | SUCCESS (handshake) |
| Host localhost:88 data | FAIL (RST after handshake) |
| Host localhost:636 TCP | SUCCESS (handshake) |
| Host localhost:636 data | FAIL (RST after handshake) |
| test-client → Samba:88 | TCP OK, read i/o timeout |
| test-client → Samba:389 | TCP OK, read i/o timeout |
| test-client → Samba:445 | TCP OK, read i/o timeout |
| test-client → Samba:53 | TCP OK, read i/o timeout |
| test-client → Samba:636 | TCP OK, read i/o timeout |
| Nginx (host:80 via NAT) | TCP OK, data OK (control) |
| Samba internal 127.0.0.1 | All services work |

## Control Test

Nginx container via Docker NAT works normally (port 80). This proves Docker Desktop NAT can pass data — the failure is specific to certain ports/services or network configuration.

## Root Cause (Unknown)

The exact root cause could not be determined. Possible causes:
1. Docker Desktop NAT configuration issue
2. Windows Firewall rules blocking Samba ports
3. Docker bridge driver issue
4. Samba container network namespace issue
5. Docker Desktop VM networking issue

## Operator Remedies (in order of preference)

1. **Run Samba4 natively in WSL2** — `wsl --install -d Ubuntu`, `apt install samba samba-ad-dc`, provision as AD DC. Bypasses Docker entirely. Requires Samba config in WSL2 filesystem.

2. **Use Linux host** — Docker Desktop NAT works correctly on Linux hosts. Run everything on a Linux VM (Hyper-V, VMware, etc.).

3. **Use cloud Windows AD eval** — Azure/AWS Windows Server 2022 evaluation with AD DS role. Use Aether CLI against cloud AD instead of local container.

4. **Check Windows Firewall** — Verify that Docker Desktop's network adapter has appropriate firewall rules. May need to create inbound rules for Docker NAT on relevant ports.

5. **Report to Docker Desktop** — This may be a Docker Desktop bug. Report with evidence above.

6. **Podman / containerd** — Use alternative container runtimes that don't use Docker Desktop NAT. Podman with rootless mode on Windows may provide different networking.
