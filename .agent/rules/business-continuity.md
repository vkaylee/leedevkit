# Business Continuity and Recovery

> Scope: Production systems and data for which the project has recovery responsibilities. Define component-specific recovery objectives, backups, retention, and exercises from business impact, legal/contractual duties, architecture, and available operations. Values below must be project decisions, not assumed defaults. This guidance does not establish regulatory compliance.

## Recovery Planning

- Identify critical user journeys, services, data stores, dependencies, and owners. Define and approve recovery point (RPO) and recovery time (RTO) objectives where applicable; document assumptions, dependencies, and trade-offs.
- Select backup frequency, retention, location, encryption, access, and deletion controls to meet approved recovery objectives and applicable obligations. Protect backups from the same failure or compromise as primary data where feasible.
- Back up non-reconstructable data and configuration needed for recovery. Keep secrets in the approved secret-management system; do not create an unprotected duplicate as a backup.
- Verify restoration and recovery procedures at a cadence appropriate to risk and change. Record results, gaps, owners, and follow-up actions.

## Recovery Operations

- Document who may declare an incident, authorize failover or restore, communicate status, and validate recovered service and data.
- Define detection, decision points, dependency order, failover or restore steps, reconciliation, and safe return to normal operation for systems with recovery targets.
- Before restoring or promoting data, assess split-brain, stale writes, integrity, and data-loss risks. Preserve evidence where incident response requires it.
- Escalate when recovery is likely to exceed approved objectives or cause irreversible data loss. Do not claim objectives have been met without measured verification.

## Resilience in Code

- Bound retries and use backoff where retry is safe; do not retry non-idempotent operations without deduplication or equivalent safeguards.
- Define failure behavior for critical dependencies. Fail closed for authentication, authorization, tenant isolation, and integrity decisions; graceful degradation is appropriate only when it cannot weaken these controls or corrupt data.
- Persist or otherwise protect work that must survive process restart according to product requirements. Select queues, health checks, shutdown behavior, and circuit breakers based on actual architecture and failure modes.
- Capacity targets, alert thresholds, test frequency, and operational procedures are project-owned choices. Set them for services where impact justifies them and keep them aligned with approved objectives.
