# v0.7.4 — Safe Source Archive Updates

## Highlights

- Allow safe relative symlinks inside downloaded archives.
- Continue rejecting absolute links and links escaping the archive root.
- Stop publishing the generated repository-root `leedevkit` symlink in source archives.
- Restore compatibility for upgrades from v0.7.3 and earlier.

## Upgrade

```bash
./leedevkit update --version v0.7.4
```

This release specifically fixes the rollback seen when updating from v0.7.2 or v0.7.3 through GitHub’s source archive.

## Rollback

If an update fails, the previous installation is restored automatically. To pin the previous release manually:

```bash
./leedevkit update --version v0.7.3
```

## Security

Archive paths remain traversal-protected. Symlinks are accepted only when their normalized target remains under an archive top-level directory; absolute and escaping targets remain rejected.
