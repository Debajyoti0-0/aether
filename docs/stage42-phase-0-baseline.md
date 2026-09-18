# Stage 42 — Phase 0 Baseline

| Field | Value |
| --- | --- |
| Locked at | 2026-09-18 (execution window 15:47-19:30 +0530, UTC 09:33-14:00) |
| Master HEAD at lock | `c923fac` (Stage 40 close commit; verified `git rev-parse` at lock) |
| VERSION | `4.2.0-rc1` (verified `type VERSION`) |
| Binary under test | `bin/aether-42.exe` (clean-room `C:\dev\clean-room-42`, clone of `c923fac`, ldflags `-X github.com/Debajyoti0-0/aether/internal/version.Version=4.2.0-rc1`) |
| Version gate | `aether-42.exe --version` → `aether version 4.2.0-rc1` |
| Tags at lock | `v4.0.0-rc2` → `5cd008be`, `v4.1.0-rc2` → `bc674af1`, `v4.3*` absent |
| Tag immutability | verified at Phase 0, at close, and in certification |
| Untracked at lock | `ad_sampledata/` (suite residue, removal blocked by safety guard — operator item), `test_graph.json`, `test_path.json`, `testws-report.md`, `.kilo/` (tooling) |
| Time gate state | UTC 2026-09-18T09:33:09Z < 2026-10-17T16:55:20Z → Stage 3 retry NOT ELAPSED (re-deferred) |
| Prior release tags unchanged | yes (`git rev-list -n1` re-verified at Phase 0 and at close) |

## Blocker inventory (source of truth for this stage)

The Stage 42 charter's gap analysis lists 30+ blockers. This stage's disposition of
each bucket:

| Bucket | Items | Disposition |
| --- | --- | --- |
| Code (C1, H1-H5, M1-M5, L1-L3, L5, CLI-0) | 15 + S42-1 (surfaced during verification) | ALL FIXED in Stage 42 — see Phase 2 register |
| Design (M6) | 1 | DEFERRED-WITH-OWNER (needs signed checkpoint; out of scope) |
| Clone-only hygiene (L6) | 1 | DISMISSED-STALE (this tree is clean) |
| Docs (charter section 2 rows C39-1..C39-5) | 5 | LANDED in Stage 40; VERIFIED in Phase 4 of this stage |
| CI (F-32-3, F-30-3, SGN-1, C39-2, F-32-2) | 5 | BLOCKED-WITH-OWNER (`gh auth status` not logged in) — Phase 5 |
| Procurement (SGN-2 Authenticode, B4 EV cert) | 2 | BLOCKED-WITH-OWNER (needs EV certificate) |
| External (B1 live Entra, B2 live IMDS, B5 Azure KV, LIVE-1 live IdP, ARM64 host) | 5 | WAIVED with owner + expiry (existing waivers B-series + LIVE-1) |

30 blockers inventoried → 30 dispositioned. Zero unconfirmed.
