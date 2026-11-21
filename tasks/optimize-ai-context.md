# Optimize AI Context for Modern Harnesses

Status: Implemented

## Goal and boundaries

Keep project invariants explicit; remove obsolete executor workarounds, duplicated gates, and universalized product policy. Preserve user-owned context, configured isolation, data safety, authorization, compatibility controls, and Go-only runtime behavior.

## Completed

- Project policy ownership separated from harness controls.
- Execution rules use discovered wrappers and preserve quoting and exit status.
- Development workflow owns behavior verification; testing standards own execution, coverage, and evidence.
- Source and installed context resolution use current Go runtime paths.
- Base context and rulebooks reduced without removing required safety controls.
- Go package tests and release acceptance are the active verification gates.

## Verification

```bash
./leedevkit --help
./leedevkit test all --json
git diff --check
scripts/build-release-go.sh /tmp/leedevkit-context-dist
```

Dry-run output is command-construction evidence only, never a test pass.

## Out of scope

Changing production auth or DB behavior, enforcing new operational SLAs, exposing additional CLI commands, or changing project-owned application policy.
