# Stage 46R Baseline

## Git Status
 M VERSION
 M cmd/aether/main.go
 M internal/cli/governance.go
 M internal/cli/root.go
?? ad.test.exe
?? admin.keytab
?? artifacts/stage45/
?? artifacts/stage45b/
?? artifacts/stage45c/
?? artifacts/stage46/
?? artifacts/stage46r/
?? artifacts/stage47/
?? artifacts/track-b-recovery/
?? cmd/asn1probe/
?? docs/stage45-certification.md
?? docs/stage45-ws0-structure.md
?? docs/stage45-ws0-summary.md
?? docs/stage45-ws1-kerberos-protocol.md
?? docs/stage45-ws2-engines.md
?? docs/stage45-ws3-cli.md
?? docs/stage45-ws4-spine.md
?? docs/stage45-ws5-test-harness.md
?? docs/stage45b-integration-tests.md
?? docs/stage45b-live-qualification.md
?? docs/stage45b-samba4-harness.md
?? internal/cli/ad/
?? internal/engine/ad/
?? internal/protocol/kerberos/
?? internal/protocol/ldap/
?? scripts/ad-lab/
?? test/integration/ad/
?? test_integration_ad.exe
?? test_integration_ad_linux
?? tmp/
?? tmp_test_asreq.go
?? tmp_test_asreq2.go

## HEAD
a887a0ed9c9fcdbf0400a5d59ccad162cfcac202

## Branch
master

## Recent Commits
a887a0e chore(artifacts): rename stage*/ to release/, audit/, handoff/
264f04c chore(artifacts): remove PROCESS-HISTORY pointer file; fix RELEASING.md references
127103f chore(cleanup): drop temporary hash scratch files
af6bf86 chore(cleanup): prune process history under artifacts/
682f3d6 chore: remove obsolete development artifacts
1c3ec25 chore(cleanup): phases 0-3 complete — inventory, classification, dead-file proof, approval proposal PENDING
9ebebf0 final: release status locked — governance blocked, ceremony not executed
20e8412 docs: RELEASING.md — standing operator runbook + resume prompt for v3.4.0-ga
4c5d0b2 stage49: operator intake — no new external inputs; blocker status recorded, RELEASE GOVERNANCE BLOCKED
ce9ee7b stage48: governance blocker closure — handoff packages complete, RELEASE GOVERNANCE BLOCKED
d0f2d2e stage47: GA blocker closure attempt — cross-platform runtime upgraded, gate matrix, RELEASE BLOCKED (4 process blockers)
eeb86e7 stage44-47 (re-based, World B): operational qualification, supply chain, 3 defect fixes, evidence package
0dae3ae docs: Stage 7 G0/G26 — Baseline reconciliation and lineage resolution
b69a152 review: development-cycle forensic review — actual baseline is Stage 3 (3.4.0-stage3); zero native fuzz targets; fixture-only interop; M3 verified-engineering / M1 release process verdict; evidence index E1-E15
d5d91eb docs: Stage 6 Phase 9 — Final release-candidate certification

## Version
5.0.0-alpha1

## Go Version
go version go1.27.1 windows/amd64