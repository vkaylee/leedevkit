# v0.7.2 — Safe DevKit Updates

## Highlights

- Preserve installed community skills across `leedevkit update` and `init` replacement flows.
- Preserve `skills.d` directory symlinks and Claude skill bridges.
- Reject unsafe release archive paths, symlinks, and special files before extraction.
- Validate downloaded `VERSION` before replacing the active installation.
- Restore the complete previous installation, including skills, when update fails.

## Upgrade

```bash
./leedevkit update --version v0.7.2
```

## Rollback

If an update fails, the previous installation is restored automatically. To pin the previous release manually:

```bash
./leedevkit update --version v0.7.1
```
