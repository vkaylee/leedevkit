# Incident Response and Security Events

> Scope: Services with incident-response and security-monitoring responsibilities. Severity definitions, responders, notification duties, and timing must follow the project’s incident plan, contracts, and applicable law. Examples below do not establish compliance or legal notification requirements.

## Detection and Triage

- Maintain a project-defined severity model that considers exploitation, affected assets and tenants, sensitive-data exposure, integrity, availability, and business impact.
- Assign an incident owner, preserve relevant evidence, and record scope, timeline, decisions, and actions in the project’s established incident channel or record.
- Escalate promptly according to the approved incident plan. Do not publish response-time promises or classify an event as legally reportable without appropriate assessment.

## Containment and Recovery

- Contain active compromise using controls appropriate to the incident, such as revoking credentials, restricting access, isolating affected systems, or disabling affected functionality.
- Preserve logs and evidence; avoid destructive changes to affected data unless required to contain harm and approved under incident procedures.
- Rotate or revoke compromised credentials and keys. Restore or recover affected systems and data using approved recovery procedures; verify integrity and access controls before return to service.
- Investigate root cause, affected scope, tenant impact, and potential data exposure. Coordinate security, legal, privacy, communications, and operational owners when relevant.
- Notify users, regulators, customers, or other parties only according to confirmed project obligations and applicable law, using the designated decision process.
- Conduct a proportionate post-incident review for material incidents. Record contributing factors, response effectiveness, corrective actions, owners, and follow-up.

## Security-Relevant Telemetry

- Record authentication, authorization, privilege changes, sensitive access, exports, and destructive actions when justified by risk or project policy.
- Capture only fields needed for detection and investigation. Mask or omit credentials and sensitive personal data; apply tenant-aware access controls to incident records.
- Correlate relevant events across requests and systems where the architecture supports it. Define alert thresholds from observed behavior and the project’s response capacity, not generic example values.

## Readiness

- Keep incident ownership, escalation contacts, evidence-preservation procedures, credential revocation, containment options, and recovery references current for systems that need them.
- Use feature controls, circuit breakers, debug facilities, and automatic blocks only when the project’s architecture and threat model support them safely. Incident tooling MUST NOT bypass authorization or expose sensitive data.
