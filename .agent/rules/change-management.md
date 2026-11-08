# Change Management

> Scope: Changes subject to project review, deployment, compatibility, or approval controls. Exact reviewers, branch protections, environments, timelines, and release gates belong to repository and organization configuration; examples elsewhere are not universal defaults.

## Review and Approval

- Use the review and approval controls configured for the project. Changes affecting authentication, authorization, cryptography, sensitive data, tenant isolation, or irreversible operations require review proportionate to their risk.
- Authors MUST NOT approve their own changes or their own risk acceptance.
- Production-destructive or irreversible operations require explicit approval from the responsible human before execution. Preserve the approval and rationale through the project’s normal change record.
- Emergency changes may use an approved expedited path, but must remain traceable, receive risk-appropriate verification and monitoring, and undergo required retrospective review.

## Deployment and Compatibility

- Follow `release-management.md` for release readiness and `migration-and-rollback.md` for data and contract changes.
- Define environment-specific approval, pre-deployment verification, rollout, abort, and recovery controls in project policy. Do not assume every project has staging, feature flags, a particular CI suite, or a timed rollback target.
- Preserve compatibility with supported consumers and mixed versions. For breaking changes, use the project’s approved communication and migration process; do not impose a universal deprecation duration or API-versioning scheme.
- For data changes, define safe rollback or roll-forward behavior. A destructive `down.sql` is not a universal requirement; see `migration-and-rollback.md`.

## Project Controls

- Keep branch protection, reviewer counts, status checks, hotfix authority, deprecation windows, and feature-flag lifecycle in the project’s configured source of truth.
- Update release notes and user/operator documentation when change impact requires it.
