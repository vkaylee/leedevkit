# v0.3.3 — Mandatory Post-Implementation Test Gate

## What's New

### Explicit test-impact decision after every implementation

AI agents must now inspect affected tests after every code or behavior change and report one explicit outcome:

- Tests were added or updated, including the covered scenarios; or
- No test change is needed, with a concrete behavior-based justification.

The decision stays with the agent instead of being deferred to the user.

### Mandatory regression protection

Tests are now explicitly required for new or changed behavior, bug fixes, business logic, validation, authorization, persistence, API contracts, concurrency, and error paths. Bug-fix tests must fail when the fix is removed.

Documentation-only, comment-only, formatting-only, and generated-artifact changes may omit test changes, but the agent must still document why behavior cannot change.

### Consistent hermetic verification

Test guidance across rulebooks, workflows, specialist-agent context, and the base AI prompt now consistently uses the project-local wrapper:

```bash
./leedevkit test <target>
```

This removes conflicting `./test.sh` and direct `npm test` instructions from the shipped AI context.

### Regression tests

Added release tests that guard the mandatory test gate, its justified exceptions, and the canonical LeeDevKit test command in shipped templates and rules.

## Upgrade

```bash
./leedevkit update --version v0.3.3
```
