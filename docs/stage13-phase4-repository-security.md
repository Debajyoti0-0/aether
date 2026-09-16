# Stage 13 Phase 4 — Repository Security Controls Assessment

**Timestamp:** 2026-09-16
**Scope:** Verify actual repository security settings vs requirements

---

## Assessment Methodology

Since GitHub CLI (`gh`) is not authenticated in this environment, repository settings **cannot be programmatically verified**. This assessment documents the **required controls** and their **verification status** based on what can be determined from workflow files and repository configuration.

**Manual verification required** via GitHub UI or authenticated `gh` CLI.

---

## Control Matrix

| Control | Required? | Current State | Evidence | Change Needed | Owner |
|---------|-----------|---------------|----------|---------------|-------|
| **Branch protection** | YES | UNKNOWN | Not verified | Likely needed | Release Eng |
| **Required status checks** | YES | UNKNOWN | Not verified | Likely needed | Release Eng |
| **PR review requirement** | YES | UNKNOWN | Not verified | Likely needed | Release Eng |
| **Force-push restriction** | YES | UNKNOWN | Not verified | Likely needed | Release Eng |
| **Secret scanning** | YES | UNKNOWN | Not verified | Likely needed | Security |
| **Push protection** | YES | UNKNOWN | Not verified | Likely needed | Security |
| **Dependabot alerts** | YES | UNKNOWN | Not verified | Likely needed | Security |
| **Dependabot auto-merge** | NO | UNKNOWN | Not verified | Optional | Security |
| **Code scanning (CodeQL)** | YES | UNKNOWN | Not verified | Likely needed | Security |
| **Tag protection / rulesets** | YES | UNKNOWN | Not verified | Likely needed | Release Eng |
| **Release permissions** | YES | UNKNOWN | Not verified | Likely needed | Release Eng |
| **Environment protection** | NO | UNKNOWN | Not verified | Optional | Release Eng |
| **OIDC trust restrictions** | YES | UNKNOWN | Not verified | Likely needed | Security |

---

## Verifiable Controls (from Workflow Files)

### ✅ Verified via Workflow Configuration

| Control | Status | Evidence |
|---------|--------|----------|
| **Action pinning** | ✅ DONE | All actions in `ci.yml` and `release.yml` pinned to commit SHAs |
| **Minimal workflow permissions** | ✅ DONE | `contents: read` default, elevated only where needed |
| **OIDC for cosign** | ✅ CONFIGURED | `id-token: write` in release.yml |
| **Attestations enabled** | ✅ CONFIGURED | `attestations: write` in release.yml |
| **Separate signing job** | ✅ CONFIGURED | Windows-sign job isolated |
| **Verification job** | ✅ CONFIGURED | Verify job with tamper tests |
| **No secret exposure in workflows** | ✅ VERIFIED | No hardcoded secrets, uses `${{ secrets.* }}` |

### ❌ Cannot Verify (Requires GitHub UI/API)

| Control | Why Not Verifiable |
|---------|-------------------|
| Branch protection rules | Requires GitHub repo settings access |
| Required status checks | Requires branch protection config |
| PR review requirements | Requires branch protection config |
| Force-push restrictions | Requires branch protection config |
| Secret scanning | Requires repo security settings |
| Push protection | Requires repo security settings |
| Dependabot configuration | Requires repo security settings |
| Code scanning (CodeQL) | Requires repo security settings |
| Tag protection | Requires repo settings |
| Release permissions | Requires repo settings |
| Environment protection | Requires repo settings |

---

## Required Actions (Manual)

### Priority 1: Branch Protection
```bash
# Via GitHub UI: Settings > Branches > Branch protection rules
# Or via gh CLI (if authenticated):
# gh api repos/Debajyoti0-0/aether/branches/main/protection \
#   --method PUT \
#   -f required_status_checks='{"strict":true,"contexts":["ci"]}' \
#   -f enforce_admins=true \
#   -f required_pull_request_reviews='{"required_approving_review_count":1}' \
#   -f restrictions=null
```

### Priority 2: Secret Scanning & Push Protection
```bash
# Via GitHub UI: Settings > Security & analysis
# Enable: Secret scanning, Push protection
```

### Priority 3: Dependabot
```bash
# Via GitHub UI: Settings > Security & analysis > Dependabot alerts
# Enable: Dependabot alerts, Dependabot security updates
```

### Priority 4: Code Scanning
```bash
# Via GitHub UI: Security > Code scanning > Set up CodeQL
```

### Priority 5: Tag Protection
```bash
# Via GitHub UI: Settings > Branches > Tag protection rules
# Or via Rulesets (newer GitHub feature)
```

---

## Gate G20 Status

| Sub-gate | Status |
|----------|--------|
| G20.1 Repository security state verified | ❌ NOT VERIFIED (requires manual check) |
| G20.2 Branch protection/rulesets assessed | ❌ NOT VERIFIED |
| G20.3 Secret scanning/push protection assessed | ❌ NOT VERIFIED |
| G20.4 Dependabot/code scanning assessed | ❌ NOT VERIFIED |
| G20.5 Release permissions assessed | ❌ NOT VERIFIED |

**G20 Status: BLOCKED — Requires manual verification via GitHub UI or authenticated gh CLI.**

---

## Recommended Next Steps

1. **Authenticate gh CLI** or access GitHub UI
2. **Enable branch protection** on `main` with:
   - Required status checks: `ci` (or specific job names)
   - Require PR review (1 approver)
   - Require linear history
   - Enforce admins
3. **Enable secret scanning + push protection**
4. **Enable Dependabot alerts**
5. **Set up CodeQL code scanning**
6. **Configure tag protection ruleset**
7. **Document all settings** in this document with screenshots/evidence

---

## Gate G20 Verdict

**G20: BLOCKED** — Repository security controls cannot be verified without manual GitHub UI access or authenticated `gh` CLI. This must be completed before Stage 13 can fully close.

**Required for Stage 13 closure:** Manual verification and configuration of all repository security controls listed above.

---

*Generated by Stage 13 Phase 4 — Repository Security Controls Assessment*