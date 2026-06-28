# 🚨 Incident Response & Security Events (SOC 2 CC7.3-7.4 / ISO 27001 A.5.24-5.28)

## 1. Security Incident Classification (MANDATORY)
> 🔴 **Every developer MUST understand these severity levels and their obligations.**

| Severity | Name | Definition | Examples | Response Time |
|----------|------|------------|----------|---------------|
| **P1** | **Critical** | Active data breach, unauthorized access to T1/T2 data, system compromise | PII data leak, SQL injection exploited, auth bypass, ransomware | **Immediate** (< 15 min acknowledgment) |
| **P2** | **High** | Vulnerability with high exploit potential, service-wide outage | Unpatched CVE with public exploit, DDoS taking down service, credential exposure in logs | **< 1 hour** |
| **P3** | **Medium** | Security weakness discovered, degraded security posture | Missing rate limiting on endpoint, weak password allowed, CORS misconfiguration | **< 24 hours** |
| **P4** | **Low** | Minor security improvement, hardening opportunity | Verbose error message, missing security header, outdated non-critical dependency | **Next sprint** |

## 2. Security Event Logging (MANDATORY for Code)
> 🔴 **The following events MUST be logged as structured security events** with log level `WARN` or `ERROR` and the field `event_type: "security"`.

### Authentication Events

| Event | Log Level | Required Fields | Alerting |
|-------|-----------|-----------------|----------|
| Successful login | INFO | `actor_id`, `ip_address`, `user_agent` | No |
| Failed login attempt | WARN | `attempted_email` (masked), `ip_address`, `failure_reason` | Alert if > 10/min per IP |
| Account locked (too many failures) | WARN | `actor_id`, `ip_address`, `lock_duration` | Always alert |
| Password changed | INFO | `actor_id`, `ip_address` | No |
| Token refresh | DEBUG | `actor_id` | No |
| Invalid/expired token used | WARN | `ip_address`, `token_age` | Alert if > 20/min |

### Authorization Events

| Event | Log Level | Required Fields | Alerting |
|-------|-----------|-----------------|----------|
| Permission denied (403) | WARN | `actor_id`, `resource`, `required_permission` | Alert if > 5/min per user |
| Role escalation (user granted higher role) | INFO | `actor_id`, `target_user_id`, `old_role`, `new_role` | Always audit-log |
| Cross-workspace access attempt | ERROR | `actor_id`, `source_workspace`, `target_workspace` | Always alert |
| Admin action performed | INFO | `actor_id`, `action`, `target` | Always audit-log |

### Data Access Events

| Event | Log Level | Required Fields | Alerting |
|-------|-----------|-----------------|----------|
| Bulk data export | INFO | `actor_id`, `export_type`, `row_count` | Alert if > 1000 rows |
| PII data accessed (T2 detail view) | DEBUG | `actor_id`, `target_user_id` | No (audit-log only) |
| Data deletion (soft or hard) | INFO | `actor_id`, `entity_type`, `entity_id` | Always audit-log |
| Mass operation (bulk import/update) | INFO | `actor_id`, `operation`, `affected_count` | Alert if > 500 records |

## 3. Incident Response Procedure
When a security incident is detected (by monitoring, user report, or code review):

### Step 1: Triage (< 15 minutes for P1)
```
1. Classify severity (P1-P4) using table above
2. Assign incident owner (person, not team)
3. Create incident channel/thread for communication
4. Document initial findings: What happened? When? What's affected?
```

### Step 2: Contain (P1: immediately, P2: < 1 hour)
```
1. Isolate affected systems if active breach (revoke tokens, block IPs, disable endpoint)
2. Use Feature Flags to disable affected functionality without full deploy
3. Preserve evidence (do NOT delete logs, do NOT modify affected data)
4. Rotate compromised credentials immediately
```

### Step 3: Investigate & Fix
```
1. Identify root cause using audit logs, application logs, and request traces
2. Determine blast radius: which users/data were affected?
3. Develop and test fix in isolated environment
4. Deploy fix with expedited review (P1: single-reviewer fast-track allowed)
```

### Step 4: Notify (MANDATORY for P1/P2)
```
1. Internal stakeholders: within 1 hour of confirmed incident
2. Affected users: within 72 hours (GDPR Art. 33 requirement)
3. Regulatory bodies: within 72 hours if personal data breach (GDPR)
4. Notification MUST include: what happened, what data was affected, 
   what actions were taken, what users should do
```

### Step 5: Post-Incident Review (MANDATORY for P1/P2)
```
1. Conduct blameless post-mortem within 5 business days
2. Document: timeline, root cause, impact, response effectiveness
3. Identify systemic improvements (not just patches)
4. Create follow-up tickets for preventive measures
5. Update rules/runbooks if gaps were found in process
6. Store in docs/incidents/YYYY-MM-DD-<slug>.md
```

## 4. Coding Rules for Incident Readiness
> These coding practices ensure the system is READY for incident investigation when one occurs.

- **Correlation IDs:** Every request MUST carry a `request_id` (per `observability-rules.md`). Incident investigators will use this to trace the full request path.
- **Feature Flags:** Critical features MUST have a feature flag that can disable them without deployment. This is the fastest containment mechanism.
- **Circuit Breakers:** External service integrations MUST implement circuit breakers to prevent cascade failures during incidents.
- **Graceful Degradation:** If a non-critical dependency fails (e.g., Redis cache), the system MUST continue operating in degraded mode rather than returning 500 errors.
- **Debug Mode:** The system MUST support a per-request debug mode (via header `X-Debug-Token`) that returns detailed timing/trace information for authorized incident responders.

## 5. Escalation Protocol (Code-Level)
> 🔴 **Extends the existing Escalation Protocol in `coding-standards.md` Section 8.**

| Scenario | Auto-Escalation Action |
|----------|----------------------|
| > 10 failed login attempts from single IP in 1 min | Auto-block IP for 15 min, log as P3 |
| > 50 rate limit violations from single identity in 5 min | Auto-block identity for 30 min, alert on-call |
| Database connection pool > 95% | Log P2 alert, reject new connections gracefully |
| Background job fails > 3 consecutive times | Pause job queue for that job type, alert on-call |
| Unhandled panic in production | Log P2 with full stack trace, service auto-restarts |
