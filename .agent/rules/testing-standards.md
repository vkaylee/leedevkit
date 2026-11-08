# Testing Standards

> Scope: Test selection, execution, coverage, and verification evidence. `development-workflow.md` owns behavior analysis and the implementation loop.

## Select and run checks

- Discover available checks from project configuration and documentation, existing wrappers, then CI; follow `ai-agent-governance.md` for discovery order.
- MUST run applicable configured checks that verify changed behavior and its material risks before reporting task complete. Use the narrowest checks that provide adequate evidence; expand when shared infrastructure, multiple targets, public contracts, packaging, or elevated risk make focused checks insufficient.
- Treat configured test, lint, type, security, and coverage checks as gates when applicable. Do not run unrelated suites or invent commands, targets, or thresholds.
- Focused test options support iteration; they do not substitute for broader verification when broader verification is warranted.
- For LeeDevKit itself, use its documented `./leedevkit test <target>` target and the release acceptance gate for packaging, install, bootstrap, update, versioning, or release-artifact changes.

## Coverage and test quality

- Preserve repository-enforced coverage thresholds. Where no threshold exists, select meaningful tests for changed behavior and material branches/failure paths; do not impose a universal percentage.
- Prefer tests at the lowest level that meaningfully checks the contract; use integration or end-to-end checks when boundaries between components or user-visible flows are at risk.
- Assert failures and absence of unauthorized or partial side effects where relevant.
- Keep tests deterministic and isolated. Do not weaken assertions or hide failures with skips, retries, or quarantines; follow project policy for any necessary exception.

## Verification report

Report checks actually run, with exact commands/targets and pass, fail, skip, or warning results. State material checks not run and why, plus remaining verification risks. Do not describe dry-run output, code inspection, or command construction as a passing test.

When no test or runtime check is appropriate, state the concrete behavior-based reason. For high-risk changes, include relevant security, compatibility, migration, release, or other specialist evidence required by applicable rulebooks.
