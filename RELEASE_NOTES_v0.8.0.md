# v0.8.0 — Replace Manage CLI Namespace

## Highlights

- Replace `leedevkit manage up dev` with `leedevkit dev up`.
- Replace `leedevkit manage down dev` with `leedevkit dev down`.
- Remove the `manage` CLI namespace.
- Promote `init`, `doctor`, and `skills` to top-level commands. Administrative database/migration handlers remain internal; the public launcher does not expose them.
- Update CLI help, wrappers, documentation, and tests.

## Breaking changes

Scripts invoking `leedevkit manage ...` MUST migrate to the new command layout. Development lifecycle commands use:

```bash
leedevkit dev up
leedevkit dev down
```

Supported commands previously nested under `manage` are now top-level. For example:

```bash
leedevkit init
leedevkit doctor
leedevkit skills list
```

The old `manage` namespace is no longer accepted.

## AI context and harness compatibility

- Reduce base context and load domain guidance only when its technology and risk apply.
- Remove blanket PTY/shell-syntax bans and mandatory log redirection; use available harness process controls and existing project isolation wrappers.
- Separate reusable safety invariants from project-owned auth, database, license, retention, and recovery policy.
- Resolve generated context and Claude hook/MCP paths for both source checkouts and installed consumer projects; preserve user-owned managed-file surroundings.
- Consolidate behavior verification and test execution rules. Dry-run success is not test evidence.

## Rollback

```bash
./leedevkit update --version v0.7.30
```

Rollback restores the previous CLI contract. Existing scripts using `manage` require no changes after rollback.

## Verification

- Focused CLI and orchestrator tests passed.
- Release acceptance tests passed.
- CLI parser smoke test passed for `dev up`, `dev down`, and top-level commands.
- `bash -n` passed for CLI wrappers.
- `git diff --check` passed.
