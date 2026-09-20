# v0.7.12 — Nested Go Toolchain Configuration

## Highlights

- Inject `[services.go].go_version` even when the Go module is nested below the project root.
- Preserve configured nested Go workdir support from v0.7.10.
- Keep the Go container build fix from v0.7.11.

## Verification

- Nested Go version regression: 38 passed.
- Go 1.24 image build and smoke command passed.
- Release artifact acceptance must pass before publication.

## Upgrade

```bash
./leedevkit update --version v0.7.12
```

## Rollback

```bash
./leedevkit update --version v0.7.11
```
