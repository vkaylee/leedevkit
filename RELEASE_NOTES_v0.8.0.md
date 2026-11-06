# v0.8.0 — Replace Manage CLI Namespace

## Highlights

- Replace `leedevkit manage up dev` with `leedevkit dev up`.
- Replace `leedevkit manage down dev` with `leedevkit dev down`.
- Remove the `manage` CLI namespace.
- Promote remaining administrative commands to top-level commands, including `init`, `doctor`, `skills`, `db:setup`, and migration commands.
- Update CLI help, wrappers, documentation, and tests.

## Breaking changes

Scripts invoking `leedevkit manage ...` MUST migrate to the new command layout. Development lifecycle commands use:

```bash
leedevkit dev up
leedevkit dev down
```

Other commands previously nested under `manage` are now top-level. For example:

```bash
leedevkit init
leedevkit doctor
leedevkit skills list
leedevkit db:setup
```

The old `manage` namespace is no longer accepted.

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
