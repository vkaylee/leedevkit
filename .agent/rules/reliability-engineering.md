# Reliability Engineering

> Scope: Production services, scheduled jobs, asynchronous workers, critical user journeys, and shared infrastructure.

## 1. Service Objectives

Define service-level indicators and objectives for services or journeys where the project’s business impact and operational model justify them. Objectives and targets are project-owned decisions, not generic SLAs.

- Assign an owner and measurement source to objectives the project adopts.
- Alerts SHOULD reflect user impact and operational capacity rather than raw infrastructure noise.
- Use adopted objectives to inform reliability work and release risk where appropriate.

## 2. Dependency Resilience (REQUIRED)

- Every remote call MUST have a bounded timeout.
- Retries require a retryable failure classification, bounded attempts, exponential backoff, and jitter.
- Do not retry non-idempotent operations without deduplication or idempotency protection.
- Use circuit breaking, concurrency limits, load shedding, or bulkheads when dependency failure could cause cascading failure.
- Define degraded behavior for non-critical dependencies.

## 3. Capacity and Resource Safety

- Bound queues, batches, payloads, concurrency, memory growth, and work per request.
- Capacity plans MUST consider peak demand, growth, failover, and dependency quotas.
- Autoscaling MUST use signals that represent actual saturation or demand and MUST define safe minimum and maximum capacity.

## 4. Failure and Recovery

- Critical operations MUST define retry, replay, reconciliation, and recovery behavior.
- Jobs and consumers SHOULD be resumable and idempotent.
- Poison work items MUST not block healthy processing indefinitely.
- Recovery procedures MUST identify data-loss risk, recovery point, recovery time, owner, and validation steps.

## 5. Reliability Verification

High-risk changes require applicable evidence from load, stress, soak, failover, dependency-failure, or recovery testing. Tests MUST use representative limits and failure modes rather than only happy-path traffic.

## 6. Operational Readiness Gate

Before production release of a new critical service or journey, confirm:

- Ownership, SLOs, dashboards, and actionable alerts
- Health and readiness behavior
- Capacity assumptions and dependency quotas
- Runbook, rollback, and recovery procedures
- Known failure modes and degraded behavior
- On-call or escalation path
