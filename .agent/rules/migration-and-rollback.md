# Migration and Rollback

> Scope: Database schemas, stored data, APIs, events, configuration, infrastructure, authentication models, and runtime migrations.

## 1. Migration Design (REQUIRED)

- Prefer expand, migrate, verify, and contract phases.
- Maintain compatibility across mixed application versions during rolling deployment.
- Separate schema change, data backfill, behavior switch, and destructive cleanup when independent rollback is needed.
- Define owner, affected consumers, duration, capacity impact, success metrics, abort conditions, and cleanup criteria.

## 2. Data Safety (BLOCKING)

- Back up or otherwise protect non-reconstructable data before destructive operations.
- Backfills MUST be bounded, observable, restartable, and safe to run more than once where practical.
- Large migrations MUST control batch size, lock duration, transaction size, replication lag, and production load.
- Validate row counts, constraints, referential integrity, and business-level reconciliation after migration.

## 3. Compatibility

- Readers MUST tolerate both old and new representations during transition when deployments overlap.
- Writers MUST not make rollback impossible before the rollback window closes.
- Dual writes require consistency monitoring, reconciliation, an owner, and an expiry condition.
- Contracting old fields or paths requires verified consumer and data migration completion.

## 4. Rollback and Roll-Forward

Every high-risk migration MUST define:

- Last safe rollback point
- Data written by the new version and how older versions handle it
- Rollback commands or automated procedure
- Conditions where roll-forward is safer than rollback
- Recovery and reconciliation steps after partial failure

An untested destructive rollback script MUST NOT be treated as a valid rollback plan.

## 5. Execution Controls

- Test migrations against representative data volume and schema state.
- Use explicit approvals for irreversible or production-destructive steps.
- Monitor progress, errors, locks, latency, resource usage, and reconciliation signals.
- Stop when abort thresholds are reached; do not continue solely to complete the run.

## 6. Completion

A migration is complete only after data and consumers are verified, legacy paths are removed, temporary controls are retired, documentation is updated, and rollback-only artifacts are archived or deleted according to policy.
