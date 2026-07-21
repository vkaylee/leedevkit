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

- Rollback procedures MUST be executable, owned, and compatible with data changes.
- Emergency releases may use reduced pre-release scope only under the change-management hotfix process.
- Emergency changes still require traceability, targeted verification, monitoring, and retrospective review.

## 7. Release Completion

A release is complete only when artifacts are published or deployed, verification passes, stakeholder communication is complete, temporary rollout controls have owners, and discovered issues are tracked.
