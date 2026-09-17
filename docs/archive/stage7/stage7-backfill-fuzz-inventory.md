# Stage 7 Backfill — Fuzz Target Inventory

**Timestamp:** 2026-09-16
**Authoritative Count:** 21 native fuzz targets

---

## Fuzz Target Inventory

| # | Package | Function | Target Description |
|---|---------|----------|-------------------|
| 1 | internal/api | FuzzReadFrame | Protocol frame parsing |
| 2 | internal/api | FuzzDecodePayload | Payload decoding |
| 3 | internal/api | FuzzEncodePayload | Payload encoding |
| 4 | internal/api | FuzzEnvelopeValidation | Envelope validation |
| 5 | internal/engine/exec | FuzzParseIMDSIdentityToken | IMDS identity token parsing |
| 6 | internal/engine/exec | FuzzParseInstanceMetadata | Instance metadata parsing |
| 7 | internal/engine/token | FuzzParsePRT | Primary Refresh Token parsing |
| 8 | internal/engine/token | FuzzParseOAuthTokens | OAuth token parsing |
| 9 | internal/engine/token | FuzzValidatePRT | PRT validation |
| 10 | internal/protocol/msoapx | FuzzDecodeKey | Key decoding |
| 11 | internal/protocol/msoapx | FuzzComputeSessionKeyProof | Session key proof computation |
| 12 | internal/protocol/msoapx | FuzzURLValuesEncoding | URL values encoding |
| 13 | internal/protocol/msoapx | FuzzDeriveNonce | Nonce derivation |
| 14 | internal/protocol/saml | FuzzParseAssertion | SAML assertion parsing |
| 15 | internal/protocol/saml | FuzzAssertionXMLMarshal | SAML XML marshaling |
| 16 | internal/protocol/wstrust | FuzzParseRSTR | RSTR parsing |
| 17 | internal/protocol/wstrust | FuzzExtractAssertionWS | WS-Trust assertion extraction |
| 18 | internal/protocol/wstrust | FuzzBuildRST | RST building |
| 19 | internal/protocol/wstrust | FuzzParseMEX | MEX parsing |
| 20 | internal/workspace | FuzzLoadRecord | Workspace record loading |
| 21 | internal/workspace | FuzzAuditLogEntry | Audit log entry parsing |

---

## Count Reconciliation

| Source | Claimed Count | Actual Count |
|--------|---------------|--------------|
| Stage 7 report | 19 | — |
| Stage 11 report | 6 | — |
| Stage 12 report | 7 | — |
| **Actual (grep)** | — | **21** |

**Note:** The discrepancy is due to:
- Stage 7 report (19) was an estimate/claim
- Stage 11 (6) and Stage 12 (7) counted only CI-run targets
- Actual native `func FuzzXxx(*testing.F)` targets: **21**

---

## Fuzz Evidence (Per Target)

Each target must be run for ≥60s with evidence recorded.

| Target | Executions | Interesting Inputs | Crashes | Panics | Status |
|--------|------------|-------------------|---------|--------|--------|
| FuzzReadFrame | TBD | TBD | 0 | 0 | ⏳ |
| FuzzDecodePayload | TBD | TBD | 0 | 0 | ⏳ |
| FuzzEncodePayload | TBD | TBD | 0 | 0 | ⏳ |
| FuzzEnvelopeValidation | TBD | TBD | 0 | 0 | ⏳ |
| FuzzParseIMDSIdentityToken | TBD | TBD | 0 | 0 | ⏳ |
| FuzzParseInstanceMetadata | TBD | TBD | 0 | 0 | ⏳ |
| FuzzParsePRT | TBD | TBD | 0 | 0 | ⏳ |
| FuzzParseOAuthTokens | TBD | TBD | 0 | 0 | ⏳ |
| FuzzValidatePRT | TBD | TBD | 0 | 0 | ⏳ |
| FuzzDecodeKey | TBD | TBD | 0 | 0 | ⏳ |
| FuzzComputeSessionKeyProof | TBD | TBD | 0 | 0 | ⏳ |
| FuzzURLValuesEncoding | TBD | TBD | 0 | 0 | ⏳ |
| FuzzDeriveNonce | TBD | TBD | 0 | 0 | ⏳ |
| FuzzParseAssertion | TBD | TBD | 0 | 0 | ⏳ |
| FuzzAssertionXMLMarshal | TBD | TBD | 0 | 0 | ⏳ |
| FuzzParseRSTR | TBD | TBD | 0 | 0 | ⏳ |
| FuzzExtractAssertionWS | TBD | TBD | 0 | 0 | ⏳ |
| FuzzBuildRST | TBD | TBD | 0 | 0 | ⏳ |
| FuzzParseMEX | TBD | TBD | 0 | 0 | ⏳ |
| FuzzLoadRecord | TBD | TBD | 0 | 0 | ⏳ |
| FuzzAuditLogEntry | TBD | TBD | 0 | 0 | ⏳ |

---

## Fuzz Execution Commands

```bash
# Run each target for 60s
go test -fuzz=FuzzReadFrame -fuzztime=60s ./internal/api/...
go test -fuzz=FuzzDecodePayload -fuzztime=60s ./internal/api/...
go test -fuzz=FuzzEncodePayload -fuzztime=60s ./internal/api/...
go test -fuzz=FuzzEnvelopeValidation -fuzztime=60s ./internal/api/...
go test -fuzz=FuzzParseIMDSIdentityToken -fuzztime=60s ./internal/engine/exec/...
go test -fuzz=FuzzParseInstanceMetadata -fuzztime=60s ./internal/engine/exec/...
go test -fuzz=FuzzParsePRT -fuzztime=60s ./internal/engine/token/...
go test -fuzz=FuzzParseOAuthTokens -fuzztime=60s ./internal/engine/token/...
go test -fuzz=FuzzValidatePRT -fuzztime=60s ./internal/engine/token/...
go test -fuzz=FuzzDecodeKey -fuzztime=60s ./internal/protocol/msoapx/...
go test -fuzz=FuzzComputeSessionKeyProof -fuzztime=60s ./internal/protocol/msoapx/...
go test -fuzz=FuzzURLValuesEncoding -fuzztime=60s ./internal/protocol/msoapx/...
go test -fuzz=FuzzDeriveNonce -fuzztime=60s ./internal/protocol/msoapx/...
go test -fuzz=FuzzParseAssertion -fuzztime=60s ./internal/protocol/saml/...
go test -fuzz=FuzzAssertionXMLMarshal -fuzztime=60s ./internal/protocol/saml/...
go test -fuzz=FuzzParseRSTR -fuzztime=60s ./internal/protocol/wstrust/...
go test -fuzz=FuzzExtractAssertionWS -fuzztime=60s ./internal/protocol/wstrust/...
go test -fuzz=FuzzBuildRST -fuzztime=60s ./internal/protocol/wstrust/...
go test -fuzz=FuzzParseMEX -fuzztime=60s ./internal/protocol/wstrust/...
go test -fuzz=FuzzLoadRecord -fuzztime=60s ./internal/workspace/...
go test -fuzz=FuzzAuditLogEntry -fuzztime=60s ./internal/workspace/...
```

---

## Evidence Report Template

Each target run produces:
- Executions count
- Interesting inputs count
- New coverage
- Crashes (must be 0)
- Panics (must be 0)
- Corpus size

Results saved to: `artifacts/stage7-backfill/fuzz/report.json`

---

*Generated by Stage 7 Backfill — Fuzz Inventory*