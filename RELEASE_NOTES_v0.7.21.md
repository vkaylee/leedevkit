# v0.7.21 — Ignore Generated Skill Asset Copies

## Highlights

- Stop skill discovery warnings caused by duplicate copies under plugin `cli/assets/skills/` directories.
- Preserve duplicate warnings for genuine skill source collisions.

## Verification

- Skill discovery regression tests: passed.
- Full regression suite and release acceptance gate run before publishing.

## Upgrade

```bash
./leedevkit update --version v0.7.21
```

## Rollback

```bash
./leedevkit update --version v0.7.20
```
