# Release Management

> Scope: Versioned artifacts, deployable services, libraries, command-line tools, containers, schemas, and customer-visible releases.

## 1. Release Identity and Reproducibility (REQUIRED)

- Every release MUST have a unique immutable version or digest traceable to source revision and CI execution.
- Artifacts MUST be built by the approved pipeline from reviewed source, not from an unverified developer workspace.
- Record dependency lockfiles, build environment, checksums, provenance, and software bill of materials when supported.
- Published artifacts MUST NOT be overwritten under the same immutable version.

## 2. Versioning and Release Notes

- Follow the repository's versioning policy; use semantic versioning when no stronger domain policy exists.
- Classify breaking, feature, fix, security, and operational changes accurately.
- Release notes MUST describe customer or operator impact, migration requirements, deprecations, known risks, and rollback considerations.

## 3. Release Readiness Gate

Before approval, confirm:

- Required review and CI checks passed
- Security and vulnerability gates passed or have approved risk acceptance
- Compatibility and migration plans are complete
- Artifacts and configuration were validated in a representative environment
- Monitoring, support, rollback, and ownership are ready
- Documentation and change communication are prepared

## 4. Deployment Strategy

Use a strategy proportional to risk: rolling, canary, blue-green, phased, or feature-controlled rollout. High-risk changes SHOULD limit initial exposure and define automated or explicit promotion criteria.

Deployment and feature exposure are separate controls. A deployed feature MUST NOT be assumed safe merely because it is disabled by default.

## 5. Post-Release Verification

- Verify artifact version, configuration, migrations, health, critical journeys, error rate, latency, and security signals.
- Compare against a documented baseline and rollout thresholds.
- Pause or roll back when abort criteria are met.
- Record the outcome and any follow-up work.

## 6. Rollback and Emergency Releases

- Define an executable recovery path, owner, and verification criteria appropriate to release risk. Rollback is one option; when data changes make rollback unsafe, document and use the approved roll-forward, restore, or reconciliation path from `migration-and-rollback.md`.
- Emergency releases may use reduced pre-release scope only under the change-management hotfix process.
- Emergency changes still require traceability, targeted verification, monitoring, and retrospective review.

## 7. LeeDevKit Release Procedure (REQUIRED)

A request to release LeeDevKit means the complete customer-visible workflow, not only a commit or Git tag:

1. Run the release readiness gate: format, lint, type checks, tests, coverage, and `git diff --check`.
2. Build and accept the customer artifact:
   ```bash
   .venv/bin/python3 scripts/_release_build.py --repo-root . --output /tmp/leedevkit-release
   .venv/bin/python3 scripts/_release_acceptance.py \
     --repo-root . \
     --artifact /tmp/leedevkit-release/leedevkit-$(tr -d '\n' < VERSION).tar.gz
   ```
3. Update `VERSION` and `RELEASE_NOTES_vX.Y.Z.md`; commit the release changes.
4. Push `main` to `origin`.
5. Create and push immutable annotated tag `vX.Y.Z`.
6. Create the public GitHub Release **with the tarball attached**:
   ```bash
   version="$(tr -d '\n' < VERSION)"
   gh release create "v${version}" \
     --repo vkaylee/leedevkit \
     --notes-file "RELEASE_NOTES_v${version}.md" \
     "/tmp/leedevkit-release/leedevkit-${version}.tar.gz"
   ```
7. Verify `gh release view vX.Y.Z` reports public, non-draft, non-prerelease, includes `leedevkit-X.Y.Z.tar.gz`, and that `releases/latest` resolves to the new tag.

Do not report release completion until all seven steps pass. LeeDevKit customer updates read GitHub Releases, not tags alone; a tag without a GitHub Release and attached tarball is incomplete.

## 8. Release Completion

A release is complete only when artifacts are published or deployed, verification passes, stakeholder communication is complete, temporary rollout controls have owners, and discovered issues are tracked.
