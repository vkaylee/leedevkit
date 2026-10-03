# v0.7.27 — Prefer Project Runtime During Self-Update

## Highlights

- Make `./leedevkit update` update the project-local `.leedevkit/` runtime when run from a LeeDevKit checkout/project.
- Prevent repeated `leedevkit.bak` chains caused by treating the project root as the installed runtime.
- Make the committed launcher prefer `.leedevkit/` over source-checkout files when a project runtime exists.
- Keep update downloads on manifest-verified release artifacts.

## Verification

- Runtime selection and release artifact URL regression tests passed.
- Full infrastructure suite and release acceptance gate passed.

## Upgrade

```bash
./leedevkit update --version v0.7.27
```

## Rollback

```bash
./leedevkit update --version v0.7.26
```
