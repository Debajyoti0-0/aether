# Security Policy

## Supported versions

Only the latest tagged release of Aether receives security fixes. The
current supported version is recorded in the `VERSION` file at the repo
root and reported by `aether --version`.

## Authorized use only

Aether is an engineering platform for **authorized** security assessment
of identity fabrics and hybrid cloud environments you own or are
contractually permitted to test. Vulnerability reports must relate to the
tool itself (its code, supply chain, packaging, or local security
properties) — not to attack techniques against third-party systems.

## Reporting a vulnerability

Report suspected vulnerabilities privately — do **not** open a public
GitHub issue for security reports.

- Contact: open a private GitHub security advisory against this repository
  (Repository → Security → Report a vulnerability).
- Include: affected version (`aether --version`), component, reproduction
  steps, and impact assessment.
- Do not include live credentials, session keys, tokens, or engagement
  data in reports.

## Response targets

| Severity        | Acknowledgment | Fix target |
| --------------- | -------------- | ---------- |
| Critical        | 48 hours       | 7 days     |
| High            | 72 hours       | 14 days    |
| Medium          | 7 days         | 30 days    |
| Low / hardening | 14 days        | best effort |

## Scope

In scope:

- Workspace integrity (path traversal, key handling, journal/audit tampering)
- Teamserver/API security (authentication, authorization, transport)
- Dashboard exposure
- Plugin/supply-chain integrity
- Cryptographic misuse in Aether's own persistence and transport
- Build/release integrity (version injection, packaging)

Out of scope:

- Attacks against Entra ID/AWS/GCP/GitHub/Okta endpoints themselves
- Social engineering of operators
- Reports requiring physical access to an operator's unlocked machine
