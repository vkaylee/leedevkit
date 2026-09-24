# LeeDevKit — Technical Debt Remediation Plan

Status: Approved

## Goal

Restore trustworthy verification, reproducible acquisition, and release evidence without changing consumer-project contracts or silently repinning installed projects.

## Current evidence

- Source `VERSION`: `0.7.13`; local `.leedevkit/VERSION`: `0.3.8`. Source/local identity drift remains unresolved.
- `./leedevkit test infra`: 814 passed, 1 skipped, 86.59% coverage.
- `./leedevkit test infra --lint-only`: Ruff and Mypy pass.
- `python3 scripts/_release_acceptance.py --repo-root .`: pass.
- `.agent/scripts/verify_all.py .` crashes at line 397 because parser omits `no_e2e`.
- `.agent/scripts/checklist.py .` reports UX and SEO failures for this CLI repository; applicability model is wrong.
- CI and `_ensure-venv.sh` install unpinned dependencies. `leedevkit.lock` is empty.
- Integrity manifest hashes files but does not authenticate release origin. No SBOM/signature/provenance gate found.
- Remote download and Git operations lack one bounded timeout contract.
- `technical-debt-remediation.md` baseline is stale (`v0.7.8`).

## Constraints and contracts

- Preserve public CLI behavior except truthful failures for checks that cannot run.
- No automatic consumer-project repinning; preserve config, installed skills, and existing rollback state.
- Use behavior-first regression tests for failure, timeout, rollback, applicability, and compatibility paths.
- Keep one shared downloader/archive validator and one reviewed dependency source.
- Signing/trusted digest distribution requires a separate ADR before implementation; do not invent a trust anchor.
- Approval required before implementation because work changes installation, verification, and release trust boundaries.

## Work packages

1. **Freeze baseline and protection.** Update this register with owner, issue links, severity, and acceptance criteria; add minimal reproductions for verifier crash, CLI applicability, timeout, invalid lock pin, failed download, and post-activation rollback. Verify each reproduction fails for the current defect and passes only after its fix.
2. **Repair verification contract.** Fix `.agent/scripts/verify_all.py` parser/options and applicable-check selection; refactor `.agent/scripts/checklist.py` so optional/inapplicable checks are explicit and exit codes distinguish pass, fail, skip, and zero executed checks. Fix `scripts/_test_handler.py` phase flags and report production coverage separately from test-source coverage. Verify empty project, missing required checker, optional skip, checker failure, `--lint-only`, and `--unit-only`.
3. **Harden acquisition and rollback.** Unify `scripts/_download.py`, `install.sh`, `bootstrap.sh`, `scripts/_init_handler.py`, and `scripts/_update_handler.py` around bounded network operations, safe archive extraction, version validation, and atomic activation. Test download, extraction, validation, config/wrapper failure, existing install preservation, and successful reinstall.
4. **Make skill updates transactional.** Audit `scripts/_skills_manager.py`; invalid locked commits, failed clone/fetch, partial multi-skill updates, and sync failures MUST preserve prior skills and `leedevkit.lock`. Keep explicit `skills update` semantics. Verify valid pins resolve exact commits and invalid pins fail closed.
5. **Make dependencies reproducible.** Add one constraints/lock source for supported Python versions; consume it from CI and `_ensure-venv.sh`; avoid installing Playwright for commands that do not need it; invalidate damaged environments on interpreter/dependency changes. Verify clean setup, offline `version`/`help`, recovery, and warm-start behavior.
6. **Close release supply-chain gaps.** Add applicable dependency/license/container scanning, release SBOM generation, artifact checksum publication, and CI evidence retention. Create an ADR for provenance/signing/trusted digest design before implementation. Document that the in-artifact manifest is tamper detection, not publisher authenticity.
7. **Close reliability and operability gaps.** Prove owned process-tree cleanup on timeout/cancellation and unrelated-process survival; add bounded subprocess/network behavior and actionable operator errors. Refresh runbooks/release notes, explain the existing skip, and reduce central coupling only where it lowers verified maintenance risk without broad refactoring.
8. **Integration and delivery gate.** Refresh documentation and this register; run source format/lint/type/unit gates, full `./leedevkit test infra`, release build plus acceptance, real CLI smoke tests, and all temporary failure reproductions. Record pass/fail/skip/warnings and do not declare complete with unresolved P1 findings.

## Dependencies

Package 1 precedes all others. Packages 2–4 can proceed independently after protection. Package 5 owns `_ensure-venv.sh` and must not overlap dependency changes in Package 2. Package 6 follows the dependency design. Package 7 follows timeout/acquisition changes. Package 8 is final and runs after all implementation packages.

## Done when

- Verification cannot produce green output after a missing required check, checker failure, or zero applicable checks.
- Source and installed engine identity are explicit and mismatch is actionable.
- Install, bootstrap, update, skill changes, and timeout failures preserve prior usable state.
- CI/runtime dependency graph is pinned and release evidence includes security and provenance status.
- All focused regressions, `./leedevkit test infra`, and release acceptance pass; skips have reason, owner, and expiry.
- Register, release notes, and operator documentation match current behavior.

## Implementation evidence

- Verification contract repaired: `checklist.py` and `verify_all.py` now distinguish pass, fail, optional skip, and inapplicable checks; CLI smoke exits 0 with truthful skips.
- Infra phase selection and coverage split repaired: `./leedevkit test infra` passed 831 tests, 1 skipped, 83.87% production coverage; test-source coverage is non-gating and reported separately.
- Acquisition and rollback hardened: focused acquisition tests passed 32; release acceptance passed after bootstrap staging fix.
- Skill transactions and lock pins hardened: focused skill tests passed 45; invalid pins preserve prior state.
- Timeout ownership hardened: focused timeout/safe-run tests passed 32 combined.
- Dependency/release evidence added: `scripts/requirements.lock`, deterministic CycloneDX SBOM, supply-chain evidence, artifact checksums, and authenticity ADR.
- Remaining operational follow-up: explain the existing skipped test with owner/expiry, decide publisher signing/trusted digest design, and update release notes only with the next versioned release.
