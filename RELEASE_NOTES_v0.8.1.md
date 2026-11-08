# v0.8.1 — Leaner AI Context and Project Python Commands

## Highlights

- Reduce base AI context and load applicable project rules only when needed, improving compatibility with modern agent harnesses.
- Use each project's `.venv/bin/python3` in skill instructions and examples so commands run with project-installed dependencies.

## Compatibility

- No CLI command or configuration migration required.
- Python skill commands now use the project-local virtual environment; create it with `python3 -m venv .venv` when absent.

## Rollback

```bash
./leedevkit update --version v0.8.0
```

Rollback restores the previous LeeDevKit runtime. Python commands documented in skills may need the global `python3` form if the project virtual environment is absent.

## Verification

- Full regression passed: 856 passed, 1 skipped; production coverage 83.28%.
- Release artifact build and acceptance passed for `leedevkit-0.8.1.tar.gz`.
