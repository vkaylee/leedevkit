# v0.3.5 — Project-agnostic custom linters

## Correctness fixes

### Custom linters skip gracefully outside the apiserver layout

The two custom Python linters previously treated a missing target directory as a
fatal error (`exit 1`), which broke `leedevkit test` lint gates in any project
that doesn't use the multi-tenant Rust apiserver layout (e.g. libraries):

- `lint_tenant_isolation.py` required `apiserver/src/repositories`
- `lint_clean_code.py` required `apiserver/src/domain`

Both linters now **skip gracefully** (`exit 0`) with an explanatory message when
their target directory is absent, since these checks are specific to the
apiserver layout and do not apply to other projects.

## Upgrade

```bash
./leedevkit update --version v0.3.5
```
