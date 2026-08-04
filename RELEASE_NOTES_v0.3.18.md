# v0.3.18 — Show complete failure logs

## Highlights

This patch release improves failure diagnostics in the parallel test runner.

## Bug fixes

- Displays the complete task log when a parallel test, lint, integration, or coverage task fails instead of truncating output to the last 30 lines.
- Preserves every diagnostic line in the existing `.test_logs/` artifact and adds regression coverage for long failure logs.

## Upgrade

```bash
./leedevkit update --version v0.3.18
```

## Rollback

If needed, pin the previous release:

```bash
./leedevkit update --version v0.3.17
```
