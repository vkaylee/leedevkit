# Go-only Control-Plane Completion

## Status

Completed 2026-11-21. Historical migration notes remain in git history and changelog; this plan records current ownership and verification.

## Ownership

- Go owns CLI dispatch, TOML/config loading, provider resolution, process supervision, reports, lifecycle, builds, gates, runtime transactions, skills, debug, database query/migration commands, and release manifest verification.
- Compose owns service graph, dependencies, health, containers, networks, volumes, and service identity.
- Consumer projects own database schemas, migrations, seeds, and test database lifecycle.
- Shell launchers only locate and delegate to Go binaries.
- Unsupported commands fail closed. No alternate control-plane runtime exists.

## Completed slices

- Run planning, argv-only process execution, timeout/signal cleanup, ownership, locks, and reports.
- Target/mode selection, scheduler, gates, lifecycle, readiness, build policy, image verification, metadata publication, and provider/TOML resolution.
- Go-only `dev`, `doctor`, `init`, `update`, `skills`, `db:*`, `migrate:*`, `debug`, `sync:api`, `coverage`, and release tooling.
- Customer-owned database lifecycle: `db:setup` removed.
- Go-only launcher, bootstrap/install paths, release builder, manifest, SBOM, checksum, and acceptance evidence.
- Retired runtime source, tests, helper scripts, dependency locks, and obsolete configuration removed.

## Verification

```bash
git ls-files '*.py'
git diff --check
docker run --rm -e GOPROXY=off -v "$PWD":/workspace -v "$HOME/go/pkg/mod":/go/pkg/mod -w /workspace golang:1.26.11-bookworm go test ./...
bash -n bin/leedevkit leedevkit templates/leedevkit-wrapper.sh bootstrap.sh install.sh scripts/build-release.sh scripts/build-release-go.sh
scripts/build-release-go.sh /tmp/dist-final
```

Release acceptance must inspect artifact members, manifest, checksum, SBOM, offline `version|help`, bootstrap local mirror behavior, and failed bootstrap rollback.

## Future work

Add only new Go behavior or project-owned checks. Do not restore an alternate runtime or retired tooling.
