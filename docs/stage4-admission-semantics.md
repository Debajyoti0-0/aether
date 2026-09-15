# Stage 4 Backfill — Admission-Time Semantics (B4-G12 / WS5)

Status: **DOCUMENTED + ENFORCED + TESTED**

## 1. Enforced semantics (from `internal/engine/spine/spine.go`)

Every externally-visible mutation flows through `Spine.Run` with the
canonical stage pipeline:

```text
AUTHZ → RISK → POLICY → APPROVAL → before-state → AUDIT(before) →
ROLLBACK REGISTRATION → EXECUTE → after-state → EVIDENCE → AUDIT(after)
```

**Authorization, risk, and policy are evaluated at admission time —
strictly before execution** (and before the `AUDIT(before)` write).
Rejection at any admission stage means the mutation never executes.

## 2. Enforcement rules (code-verified)

| Rule | Mechanism |
|---|---|
| Authorization rejection ⇒ no execution | `Capabilities.Has` gate (`StageAuthZ`); teamserver derives caps from the client certificate only (`internal/api/identity.go`); unknown capability ⇒ deny |
| Risk rejection ⇒ no execution | `RiskScore` vs `MaxRisk` (config `MaxRiskThreshold`, default 50); abort at `StageRisk` |
| Policy rejection ⇒ no execution | `PolicyEvaluator` deny rules (`StagePolicy`) |
| Audit chain unavailable ⇒ no execution | fail-closed, inherited Stage 1 |
| Rollback registration failure (reversible mutation) ⇒ no execution | abort `aborted_rollback_registration` |
| Auto-approval removes ONLY the human prompt | `ApprovalAuto` never bypasses authz/risk/policy/audit/rollback/evidence; `ApprovalInteractive` with nil approver refuses (fail closed) |
| Every stage decision is written to the signed audit trail | chain append per stage outcome |

**Both-time semantics:** admission gates run before execution;
post-execution failures produce `completed_state_unknown` or rollback
statuses — never a bare success. Provider-side authorization is
independent and is never inferred from local policy approval.

## 3. Enforcement evidence (executable)

- `TestEndToEnd_SpineRiskAbort` — 70-risk action at max 10 aborted at
  `StageRisk`, mutation not executed, abort journaled.
- `TestEndToEnd_SpinePolicyDeny` — deny-rule match aborts before
  execution.
- `TestEndToEnd_SpineAuditChain` — full governed flow produces 2 signed
  audit entries + evidence + rollback registration, chain VERIFIED.
- `test/integration/spine_storage_test.go` (build tag `integration`),
  all passing (`artifacts/stage4-backfill/`).
