# v0.3.16 — Enterprise AI governance and development quality gates

## Highlights

This release upgrades LeeDevKit's shared AI rule system with deterministic governance, enterprise control domains, and a mandatory development–testing workflow designed to prevent agents from stopping at happy-path implementation.

## Development and testing

- Adds a primary development workflow that treats implementation and testing as one indivisible task.
- Requires a pre-implementation Test Impact Matrix covering success, boundaries, malformed input, negative paths, state transitions, dependency failures, timeouts, retries, compatibility, security, and resource limits.
- Requires bug fixes to include regression tests that fail without the fix when reproducible.
- Strengthens edge-case, negative-side-effect, skipped-test, and flaky-test controls.
- Requires completion reports to identify scenarios, commands, pass/fail/skip results, unverified checks, and remaining risks.

## AI governance

- Adds rule priority, applicability, severity levels, exception handling, verification discovery, and a shared Definition of Done.
- Prevents Boy Scout cleanup from broadening user authorization.
- Replaces universal over-engineering requirements with risk-based production readiness.
- Removes forced clarification questions and unconditional output redirection from the generated AI context.
- Makes project wrappers conditional on actual repository configuration.

## Enterprise rulebooks

Adds enterprise governance for:

- Secure development lifecycle and threat modeling
- Vulnerability intake, remediation targets, risk acceptance, and closure
- Architecture decisions, boundaries, review, and evolution
- API and event contracts, compatibility, idempotency, and verification
- Reliability objectives, dependency resilience, capacity, and operational readiness
- Safe migrations, backfills, rollback, roll-forward, and reconciliation
- Reproducible releases, provenance, rollout, and post-release verification

## Rule quality and portability

- Replaces product-specific project-layout assumptions with repository discovery rules.
- Removes stale references and unverifiable command assumptions.
- Standardizes technical rule content in English.
- Adds explicit dead-code prevention, verification, deprecation, and time-bound exception rules.

## Upgrade

```bash
./leedevkit update --version v0.3.16
```
