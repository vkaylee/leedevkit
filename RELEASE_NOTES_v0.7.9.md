# v0.7.9 — Reliability, Verification & Transactional Updates

## Highlights

- Fix project-local and dogfood launchers so they execute the tracked/local CLI instead of recursively invoking themselves.
- Make `infra --unit-only` execute only infrastructure tests and reject unsupported E2E selection explicitly.
- Report runtime coverage without counting `scripts/tests` as production code; add the missing `types-PyYAML` verification dependency.
- Make global installation and per-project bootstrap transactional, with staged validation, archive link rejection, and rollback preservation for existing installs and project files.
- Make validation runners fail closed when required checkers are missing or no checks execute.
- Terminate owned process trees on task timeout while preserving the `124` timeout contract and unrelated processes.
- Enforce exact Git commit pins for community skills without silently rewriting lock state after checkout failure.
- Add focused regression coverage and a GitHub Actions quality/release acceptance workflow.

## Verification

- Local source CLI version/help paths: passed.
- `./leedevkit test infra --lint-only`: passed.
- `./leedevkit test infra --unit-only`: passed.
- `./leedevkit test infra`: passed.
- Full test suite: 806 passed, 1 skipped.
- Runtime coverage: 86.18% (threshold 80%).
- Ruff format and lint: clean.
- Mypy: clean across 65 source files.
- Shell syntax checks: passed.
- Release acceptance gate: passed for `leedevkit-0.7.9.tar.gz`.

## Upgrade

```bash
./leedevkit update --version v0.7.9
```

## Rollback

```bash
./leedevkit update --version v0.7.8
```

## Known Risks

- Artifact authenticity still relies on the configured release channel; signed release metadata or an externally trusted digest is not included in this patch.
- GitHub Actions workflow validation occurs after publication in the repository CI environment; the local source and black-box gates above passed before release.
