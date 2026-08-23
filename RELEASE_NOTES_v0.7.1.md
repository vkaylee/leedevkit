# v0.7.1 — Release Tarball Symlink Protection

## Highlights

- Remove committed runtime `.claude` symlinks that caused `AbsoluteLinkError` during GitHub archive extraction on Python 3.14 (`tarfile` security filters).
- Add `.claude/` to `.gitignore` so generated bridge artifacts remain project-local.

## Upgrade

```bash
./leedevkit update --version v0.7.1
```
