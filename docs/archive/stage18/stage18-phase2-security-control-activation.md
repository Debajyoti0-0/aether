# Stage 18 Phase 2 — GitHub Security Control Activation and Verification

**Timestamp:** 2016-09-16
**Stage:** 18 — Production Readiness & Waiver Closure
**Scope:** Activate and verify the six GitHub security controls identified as NOT VERIFIED in Stage 17

---

## Phase 2 Objective

Activate and verify the six repository security controls that Stage 17 identified as NOT VERIFIED. Since GitHub CLI (`gh`) is not authenticated in this environment, all configurations must be performed manually via GitHub UI by a repository administrator with admin permissions.

---

## Control Activation Status

| # | Control | Required | Current State | Action Required | Owner | Deadline |
|---|---------|----------|---------------|-----------------|-------|----------|
| 1 | Branch Protection | YES | ❌ NOT VERIFIED | Enable via GitHub UI | Release Eng | 2027-03-31 |
| 2 | Required Status Checks | YES | ❌ NOT VERIFIED | Enable via GitHub UI | Release Eng | 2027-03-31 |
| 3 | Required PR Reviews | YES | ❌ NOT VERIFIED | Enable via GitHub UI | Release Eng | 2027-03-31 |
| 4 | Force-Push Restriction | YES | ❌ NOT VERIFIED | Enable via GitHub UI | Release Eng | 2027-03-31 |
| 5 | Secret Scanning | YES | ❌ NOT VERIFIED | Enable in repo settings | Security | 2027-03-31 |
| 6 | Push Protection | YES | ❌ NOT VERIFIED | Enable in repo settings | Security | 2027-03-31 |
| 7 | Dependabot Alerts | YES | ❌ NOT VERIFIED | Enable in repo settings | Security | 2027-03-31 |
| 8 | Code Scanning (CodeQL) | YES | ❌ NOT VERIFIED | Enable in repo settings | Security | 2027-03-31 |
| 9 | Tag Protection / Rulesets | YES | ❌ NOT VERIFIED | Configure via GitHub UI | Platform | 2027-03-31 |
| 10 | Release Permissions | YES | ❌ NOT VERIFIED | Configure via GitHub UI | Release Eng | 2027-03-31 |

---

## 1. Branch Protection (main branch)

### Required Configuration:
- Protected default branch: `main`
- Required pull request reviews: ✅ Required (1+ approvers)
- Required status checks: ✅ Required (CI jobs: governance, build, vet, test, lint, vuln, integration)
- Dismissal of stale approvals: ✅ Enabled
- Force-push restrictions: ✅ Enabled
- Branch deletion restrictions: ✅ Enabled
- Required CODEOWNERS review: ⚠️ If applicable
- Administrator enforcement: ✅ Enforced

### GitHub UI Path:
Settings → Branches → Branch protection rules → Add rule for `main`

### Required Settings:
```
☑ Protect this branch
☑ Require a pull request before merging
  ☑ Require approvals: 1
  ☑ Dismiss stale pull request approvals when new commits are pushed
  ☑ Require review from CODEOWNERS (if CODEOWNERS file exists)
  ☑ Require status checks to pass before merging
  ✓ governance (release files present)
  ✓ build (ubuntu-latest)
  ✓ build (windows-latest)
  ✓ build (macos-latest)
  ✓ vet
  ✓ test (-race)
  ✓ test (-race, windows)
  ✓ lint (golangci-lint)
  ✓ vuln (govulncheck)
  ✓ integration (tagged)
☑ Require branches to be up to date before merging
☑ Require conversation resolution before merging
☑ Require signed commits
☑ Require linear history
☑ Do not allow bypassing the above settings
☑ Do not allow force pushes
☑ Do not allow branch deletion
```

### Verification:
After enabling, verify via GitHub UI or (when authenticated):
```bash
gh api repos/:owner/:repo/branches/main/protection
```

---

## 2. Required Status Checks (CI)

### Required Checks (must all pass before merge):
- `governance (release files present)`
- `build (ubuntu-latest)`
- `build (windows-latest)`
- `build (macos-latest)`
- `vet`
- `test (-race)` on ubuntu-latest
- `test (-race, windows)` on windows-latest
- `lint (golangci-lint)`
- `vuln (govulncheck)`
- `integration (tagged)`

### GitHub UI Path:
Settings → Branches → Branch protection rules → main → Required status checks

### Required Contexts (exact names from CI workflow):
```
governance (release files present)
build (ubuntu-latest)
build (windows-latest)
build (macos-latest)
vet
test (-race)
test (-race, windows)
lint (golangci-lint)
vuln (govulncheck)
integration (tagged)
```

### Verification:
After enabling, verify via GitHub UI or (when authenticated):
```bash
gh api repos/:owner/:repo/branches/main/protection --jq '.required_status_checks.contexts[]'
```

---

## 3. Required PR Reviews

### Configuration:
- Required approving review count: 1
- Dismiss stale reviews: ✅ Enabled
- Require CODEOWNERS review: ⚠️ If CODEOWNERS file exists
- Dismiss stale approvals on new commits: ✅ Enabled

### GitHub UI Path:
Settings → Branches → main → Branch protection rules → Require pull request reviews

### Required Settings:
```
☑ Require pull request reviews before merging
  ☑ Required approvals: 1
  ☑ Dismiss stale pull request approvals when new commits are pushed
  ☑ Require review from CODEOWNERS (if CODEOWNERS file exists)
  ☑ Dismiss stale pull request approvals when new commits are pushed
```

---

## 4. Force-Push Restriction

### Configuration:
- Allow force pushes: ❌ Disabled (blocked)
- Allow force pushes by admins only: ❌ Disabled (or only for admins if policy allows)
- Block force pushes to protected branches: ✅ Enabled

### GitHub UI Path:
Settings → Branches → main → Branch protection rules → Do not allow force pushes

---

## 5. Secret Scanning

### Required Configuration:
- Secret scanning: ✅ Enabled
- Push protection: ✅ Enabled
- Alert notifications: ✅ Enabled
- Custom patterns: ⚠️ If applicable

### GitHub UI Path:
Settings → Security & analysis → Secret scanning

### Required Settings:
```
☑ Secret scanning
☑ Push protection
☑ Alert notifications
```

### Verification:
Settings → Security & analysis → Secret scanning

---

### 5. Push Protection

### Required Configuration:
- Push protection: ✅ Enabled
- Blocks pushes with detected secrets: ✅ Enabled
- Bypass for admins: ⚠️ If configured

### Verification:
Part of Secret Scanning configuration above

---

### 6. Dependabot Alerts

### Required Configuration:
- Dependabot alerts: ✅ Enabled
- Dependabot security updates: ✅ Enabled
- Grouped updates: ⚠️ If configured
- Auto-merge for security updates: ⚠️ If configured

### GitHub UI Path:
Settings → Security & analysis → Dependabot alerts

### Required Settings:
```
☑ Dependabot alerts
☑ Dependabot security updates
```

### Verification:
Settings → Security & analysis → Dependabot alerts

---

### 8. Code Scanning (CodeQL)

### Required Configuration:
- CodeQL workflow: ✅ Configured
- Languages: Go (at minimum)
- Scheduled scans: ✅ Weekly
- PR scanning: ✅ Enabled
- SARIF upload: ✅ Enabled

### GitHub UI Path:
Security → Code scanning alerts → Set up CodeQL

### Required Workflow:
`.github/workflows/codeql.yml` (must exist)

### Required Settings:
```
☑ Code scanning alerts
☑ Code scanning on pull requests
☑ Weekly scheduled scans
☑ Upload SARIF results
```

### Status:
❌ NOT VERIFIED — Requires manual configuration

---

### 8b. Dependabot Security Updates

### Required Configuration:
- Dependabot security updates: ✅ Enabled
- Automatic PR creation for security updates: ✅ Enabled

### GitHub UI Path:
Settings → Security & analysis → Dependabot security updates

### Required Settings:
```
☑ Dependabot security updates
☑ Automatic PR creation for security updates
```

### Status:
❌ NOT VERIFIED — Requires manual configuration

---

### 9. Tag Protection / Rulesets

### Required Configuration:
- Tag protection ruleset: ✅ Configured
- Tag pattern: `v*` (release tags)
- Protection rules: No deletion, no force push, required reviews
- Bypass permissions: Admin only

### GitHub UI Path:
Settings → Rulesets → New ruleset → Tag rules

### Required Ruleset Configuration:
```
Name: Release Tag Protection
Target: Tags matching pattern `v*`
Rules:
  ☑ No deletions
  ☑ No force pushes
  ☑ Required reviews (1+)
  ☑ Required status checks
  Bypass list: Admins only
```

### Verification:
Settings → Rulesets

---

### 6. Release Permissions

### Required Configuration:
- Release creation: Limited to authorized roles
- Tag protection: Enforced via ruleset
- Artifact upload: Restricted to release workflows

### GitHub UI Path:
Settings → Environments → Release environment (if used) / Repository settings

### Required Settings:
- Required reviewers for releases
- Deployment branch policy
- Required status checks for release

---

## Verification Checklist

### Verified Controls (✅ DONE):
- [ ] Branch Protection
- [ ] Required Status Checks
- [ ] Required PR Reviews
- [ ] Force-Push Restriction
- [ ] Secret Scanning
- [ ] Push Protection
- [ ] Dependabot Alerts
- [ ] CodeQL Code Scanning
- [ ] Tag Protection / Rulesets
- [ ] Release Permissions

### Currently: 0/10 Verified

---

## Gate S18-G02 Status

| Sub-gate | Status |
|----------|--------|
| G18-G02.1 | Branch protection enabled | ❌ NOT DONE |
| G18-G02.2 | Required status checks enabled | ❌ NOT DONE |
| G18-G02.2 | PR reviews enabled | ❌ NOT DONE |
| G18-G02.3 | Force-push restriction enabled | ❌ NOT DONE |
| G18-G02.4 | Secret scanning enabled | ❌ NOT DONE |
| G18-G02.5 | Push protection enabled | ❌ NOT DONE |
| G18-G02.5 | Dependabot alerts enabled | ❌ NOT DONE |
| G18-G02.6 | CodeQL code scanning enabled | ❌ NOT DONE |
| G18-G02.6 | Tag protection / rulesets enabled | ❌ NOT DONE |
| G18-G02.7 | Release permissions configured | ❌ NOT DONE |

**G18-G02 Status: BLOCKED — All 10 controls require manual GitHub UI configuration**

---

## Next Steps

1. Repository administrator with admin permissions must configure all 10 controls via GitHub UI
2. Each control must be verified after configuration
3. Document evidence (screenshots or API responses) for each control
4. Update this document with verification evidence as controls are enabled

---

*Generated by Stage 18 Phase 2 — Security Control Activation and Verification*