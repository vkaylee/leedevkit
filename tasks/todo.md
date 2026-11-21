# Go-only Control-Plane Completion

- [x] Go owns CLI dispatch, config/provider resolution, process execution, reports, lifecycle, builds, gates, runtime transactions, skills, debug, DB/migration commands, and release tooling.
- [x] Customer repositories own DB schema, migrations, seeds, and test DB lifecycle; `db:setup` removed.
- [x] Launcher, wrapper, bootstrap, install, release builder, manifest, SBOM, checksum, and acceptance paths are Go-only.
- [x] Retired runtime source, tests, helper scripts, dependency locks, and obsolete configuration removed.
- [x] Active documentation and test names describe Go-only ownership.
- [ ] Run final Go suite, shell syntax checks, release build, release acceptance, and archive inspection after documentation cleanup.

Final gate:

```bash
git ls-files '*.py'
git diff --check
docker run --rm -e GOPROXY=off -v "$PWD":/workspace -v "$HOME/go/pkg/mod":/go/pkg/mod -w /workspace golang:1.26.11-bookworm go test ./...
bash -n bin/leedevkit leedevkit bootstrap.sh install.sh scripts/build-release.sh scripts/build-release-go.sh
scripts/build-release-go.sh /tmp/dist-final
```
