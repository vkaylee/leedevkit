# 📡 Observability & Monitoring Rules (SOC 2 CC7.1-7.3 / ISO 27001 A.8.15-8.16)

## 1. Structured Logging Standard (MANDATORY)
> 🔴 **ALL log output MUST be structured JSON in production.** Human-readable format is allowed ONLY in local development.

### Log Entry Required Fields
Every log entry MUST include these fields:

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `timestamp` | ISO 8601 | UTC timestamp | `2026-06-04T15:00:00.000Z` |
| `level` | enum | Log severity | `INFO`, `WARN`, `ERROR` |
| `message` | string | Human-readable description | `"Leave request approved"` |
| `service` | string | Originating service name | `"apiserver"`, `"agent-main"` |
| `request_id` | UUID | Unique per-request correlation ID | `"550e8400-..."` |
| `workspace_id` | UUID | Tenant context (if applicable) | `"7c9e6679-..."` |
| `actor_id` | UUID | User/system performing action (if applicable) | `"a1b2c3d4-..."` |
| `span_id` | string | OpenTelemetry span ID (if tracing enabled) | `"abc123"` |
| `error.type` | string | Error classification (on ERROR level) | `"AppError::Validation"` |

### Log Level Classification
> 🔴 **Incorrect log levels create noise and hide real issues.** Follow this strictly:

| Level | When to Use | Examples |
|-------|------------|---------|
| **ERROR** | System cannot fulfill request, requires human attention | DB connection failure, unhandled panic, auth service down |
| **WARN** | Degraded operation, auto-recovered, or suspicious activity | Rate limit hit, cache miss fallback, retry succeeded, invalid token |
| **INFO** | Significant business events (audit-worthy) | User login, leave approved, employee created, export generated |
| **DEBUG** | Detailed technical context for troubleshooting | Query parameters, cache hit/miss, computed values |
| **TRACE** | Extremely verbose, framework-level detail | HTTP header dumps, serialization steps (NEVER in production) |

- **Production:** `INFO` and above. `DEBUG` enabled only via runtime config for targeted troubleshooting.
- **TRACE:** NEVER enabled in production. Strip from release builds where possible.

## 2. Request Correlation (MANDATORY)
- **Request ID Propagation:** Every incoming HTTP request MUST be assigned a unique `X-Request-Id` (UUID v7 preferred for sortability). If the client sends one, use it; otherwise, generate one in middleware.
- **Propagation:** The `request_id` MUST be propagated to ALL downstream calls (DB queries, Redis operations, background job enqueues, inter-service calls).
- **Response Header:** The `X-Request-Id` MUST be included in the HTTP response headers so clients can reference it in bug reports.
- **Background Jobs:** When a background job is spawned from an HTTP request, the originating `request_id` MUST be attached as metadata to enable end-to-end tracing.

## 3. PII Scrubbing in Logs (MANDATORY)
> 🔴 **Enforcement of `data-governance.md` classification in logs.**
- **T1 Fields:** NEVER appear in any log statement at any level. Use `[REDACTED]`.
- **T2 Fields:** MUST be masked using standardized patterns:
  - Email: `u***@domain.com`
  - Phone: `+84***789`
  - Name: First 1 char + `***` (e.g., `N***`)
- **Implementation:** Use the `tracing` subscriber's `MaskingLayer` or a centralized `mask_pii()` function. Do NOT rely on developers remembering to mask manually.

## 4. Monitoring & Alerting Thresholds (MANDATORY)
> 🔴 **Monitoring without alerting is just watching things break in slow motion.**

### Application-Level Alerts

| Metric | Warning Threshold | Critical Threshold | Action |
|--------|-------------------|-------------------|--------|
| **5xx Error Rate** | > 1% of requests in 5 min | > 5% of requests in 5 min | Page on-call engineer |
| **Request Latency (p99)** | > 2 seconds | > 5 seconds | Investigate slow queries |
| **Rate Limit Triggers** | > 50/min from single identity | > 200/min from single identity | Potential DDoS/abuse |
| **Failed Auth Attempts** | > 10/min from single IP | > 50/min from single IP | Potential brute force |
| **Background Job Failures** | > 3 consecutive failures | > 10 in 1 hour | Worker health degradation |
| **DB Connection Pool** | > 80% utilization | > 95% utilization | Connection exhaustion risk |

### Infrastructure-Level Alerts

| Metric | Warning | Critical |
|--------|---------|----------|
| **CPU Usage** | > 70% sustained 10 min | > 90% sustained 5 min |
| **Memory Usage** | > 80% | > 95% |
| **Disk Usage** | > 80% | > 90% |
| **DB Replication Lag** | > 5 seconds | > 30 seconds |

## 5. Health Check Endpoints (MANDATORY)
- **Liveness:** `GET /health` — Returns 200 if the process is alive. No dependency checks. Used by container orchestrator restart logic.
- **Readiness:** `GET /health/ready` — Returns 200 only if ALL critical dependencies (DB, Redis, external services) are reachable. Used by load balancer to route traffic.
- **Response Format:**
```json
{
  "status": "healthy",
  "version": "1.2.3",
  "uptime_seconds": 86400,
  "checks": {
    "database": "ok",
    "redis": "ok",
    "worker": "ok"
  }
}
```

## 6. Audit Log Observability
- **Security-Relevant Events** that MUST be logged to both application logs AND the `audit_logs` table:
  - User login/logout (success and failure)
  - Permission changes (role assignment, revocation)
  - Data exports (CSV, XLSX)
  - Employee PII access (view detail pages of T2 data)
  - Configuration changes (workspace settings, policies)
  - Bulk operations (mass import, bulk delete)
- **Audit Log Retention:** Per `data-governance.md` Section 3 — minimum 3 years.

## 7. Log Retention & Storage
- **Application Logs:** Minimum 90 days in searchable storage (ELK, Loki, CloudWatch).
- **Security Logs:** Minimum 1 year in cold/archive storage.
- **Audit Logs:** Minimum 3 years in database (per data-governance.md).
- **Cost Optimization:** Logs older than 30 days MAY be moved to cold storage (S3, GCS) with reduced query capability.
