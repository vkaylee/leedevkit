# Observability and Monitoring

> Scope: Services and workflows where operational or security telemetry supports reliability, incident response, or user-impact detection. Choose formats, fields, retention, thresholds, and tooling according to project architecture, data sensitivity, operational capacity, and applicable obligations. Examples do not establish compliance.

## Logging and Correlation

- Emit enough structured context to diagnose failures and investigate security events where useful and supported. Do not require JSON, a specific logging library, or identical fields from every component.
- Use a request, trace, or job correlation identifier when it helps follow work across system boundaries. Do not trust a client-provided identifier for security decisions; validate or replace it as appropriate.
- Select log levels to distinguish failures, degraded operation, significant events, and diagnostic detail. Avoid high-volume or sensitive debug output in production.
- Never log secrets. Minimize personal and tenant data, and redact or mask sensitive values where needed. Follow `data-governance.md` and `incident-response.md`.

## Metrics, Alerts, and Health

- Measure signals that reflect user impact and critical system behavior. Define thresholds and alert routing for systems with on-call or operational response; tune them to actual baselines and capacity.
- Separate process liveness from dependency readiness when the deployment or orchestrator uses health probes. Report dependency status accurately without exposing sensitive details or causing unsafe request routing.
- Include latency, errors, saturation, queue or job health, and data freshness where relevant to a service’s objectives.
- Avoid metrics labels or diagnostics that leak secrets, personal data, or unbounded user-controlled values.

## Audit and Retention

- Keep security audit records distinct from ordinary diagnostic logs where required for access control, integrity, or retention.
- Audit security-sensitive access, privilege changes, exports, and destructive actions when required by project policy or risk. Record the minimum useful metadata; never copy protected payloads or credentials into the audit trail.
- Set retention, archival, access, and deletion rules according to data sensitivity, investigation needs, cost, contracts, and applicable law. Do not apply generic retention periods without project approval.
