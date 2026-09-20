# v0.7.11 — Go Container Build Fix

## Highlights

- Fix the built-in Go test image so its Dockerfile validation command exits successfully.
- Keep nested Go module workdir support from v0.7.10.

## Verification

- Go 1.24 container image build passed.
- Go image smoke command passed.
- Release artifact acceptance must pass before publication.

## Upgrade

```bash
./leedevkit update --version v0.7.11
```

## Rollback

```bash
./leedevkit update --version v0.7.10
```
