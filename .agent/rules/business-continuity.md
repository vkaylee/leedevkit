# 🛡️ Business Continuity & Disaster Recovery (SOC 2 A1.1-A1.3 / ISO 27001 A.5.29-5.30)

## 1. Recovery Objectives (MANDATORY)
> 🔴 **Every critical system component MUST have defined recovery targets.**

| Component | RPO (max data loss) | RTO (max downtime) | Priority |
|-----------|---------------------|---------------------|----------|
| **PostgreSQL Database** | 1 hour | 4 hours | P1 — Critical |
| **API Server** | 0 (stateless) | 15 minutes | P1 — Critical |
| **Redis Cache** | Ephemeral (acceptable loss) | 15 minutes | P2 — High |
| **Background Workers** | 0 (job queue persisted in DB) | 30 minutes | P2 — High |
| **Web Dashboard** | 0 (static assets) | 15 minutes | P2 — High |
| **File Storage** | 1 hour | 4 hours | P3 — Medium |

## 2. Backup Policy (MANDATORY)
### Database Backups
- **Frequency:** Automated daily full backups + continuous WAL archiving (point-in-time recovery).
- **Retention:** Daily backups retained for **30 days**. Weekly backups retained for **6 months**. Monthly backups retained for **3 years**.
- **Storage:** Backups MUST be stored in a different region/availability zone from the primary database.
- **Encryption:** ALL backups MUST be encrypted at rest (per `encryption-rules.md`).
- **Testing:** Backup restoration MUST be tested at least **monthly**. Document results in `docs/backup-tests/YYYY-MM.md`.

### Application State Backups
- **Configuration:** All infrastructure configuration (Docker Compose, environment templates, Caddyfile) MUST be version-controlled.
- **Secrets:** Backup of secrets/keys MUST be stored in a separate secrets manager with access limited to infrastructure team.

## 3. Failover Procedures
### Database Failover
```
1. Detect: Automated health check fails for > 30 seconds
2. Promote: Promote read replica to primary (automated if supported)
3. Redirect: Update connection string (DNS-based or environment variable)
4. Verify: Run smoke tests against new primary
5. Rebuild: Create new replica from new primary
6. Document: Log incident and failover timeline
```

### API Server Failover
```
1. Detect: Health check endpoint returns non-200 for > 15 seconds
2. Restart: Container orchestrator auto-restarts failed instance
3. Scale: If persistent failure, scale up additional instances
4. Route: Load balancer automatically routes away from unhealthy instances
```

## 4. Capacity Planning Rules
- **Database Storage:** Alert at **80%** disk usage, critical at **90%**. Plan capacity expansion at 70%.
- **Connection Pool:** Monitor active vs. max connections. Alert at **80%** utilization (per `observability-rules.md`).
- **Memory:** Detect memory leaks via trend analysis. Alert on continuous growth without plateau.
- **Horizontal Scaling:** Define scaling triggers: auto-scale API servers when CPU > 70% or request latency p99 > 2s.

## 5. Coding Rules for Resilience
> These coding practices ensure the application survives infrastructure failures gracefully.

- **Idempotent Operations:** All state-changing operations MUST be idempotent (safe to retry). Use unique request IDs or database constraints to prevent duplicate processing.
- **Retry with Backoff:** External service calls MUST implement exponential backoff with jitter (not fixed retry intervals). Maximum 3 retries.
- **Circuit Breakers:** External dependencies MUST have circuit breakers. After N consecutive failures, stop calling and return a cached/default response.
- **Graceful Shutdown:** On SIGTERM, the application MUST: stop accepting new requests, finish in-progress requests (timeout 30s), flush pending writes, close DB connections cleanly, then exit.
- **Queue Durability:** Background jobs MUST be persisted (database-backed queue) to survive process restarts. In-memory queues are NOT acceptable for production.
- **Health Endpoint Accuracy:** Health check endpoints MUST actually verify dependencies (DB connection, Redis ping) — never return 200 unconditionally.

## 6. Disaster Recovery Testing
- **Frequency:** Full DR test at least **quarterly**. Database restore test **monthly**.
- **Scope:** Simulate complete loss of primary infrastructure and verify recovery within RTO.
- **Documentation:** Each DR test MUST produce a report documenting: test date, scenario simulated, actual recovery time, issues encountered, improvements needed.
