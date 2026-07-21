# Secure Development Lifecycle

> Scope: Changes that affect executable code, infrastructure, dependencies, authentication, authorization, sensitive data, or externally reachable interfaces.

## 1. Security Planning (REQUIRED)

- Identify affected assets, trust boundaries, actors, entry points, sensitive data, and abuse cases before implementing a high-risk change.
- Add explicit security acceptance criteria for authentication, authorization, cryptography, tenant isolation, file handling, command execution, external callbacks, and untrusted input.
- Reuse approved security controls. Custom cryptography, authentication protocols, and secret-storage mechanisms require security review.

## 2. Threat Modeling

A documented threat model is REQUIRED for new externally exposed services, privilege-boundary changes, sensitive-data flows, payment or financial logic, multi-tenant isolation, and high-impact AI tool execution.

At minimum, record:

- Assets and trust boundaries
- Threat actors and abuse cases
- Spoofing, tampering, repudiation, disclosure, denial-of-service, and privilege-escalation risks
- Preventive and detective controls
- Residual risks, owners, and review conditions

Update the threat model when trust boundaries or material data flows change.

## 3. Implementation Controls (BLOCKING)

- Validate untrusted input at the boundary and encode output for its destination context.
- Enforce authorization server-side before accessing protected data or performing privileged actions.
- Use parameterized data access and safe process-execution APIs.
- Prevent secrets and sensitive personal data from entering source, logs, telemetry, errors, test fixtures, or generated artifacts.
- Apply secure defaults and fail closed for access-control decisions.
- Pin, verify, and minimize third-party dependencies according to supply-chain rules.

## 4. Automated Security Verification (REQUIRED)

Use repository-configured controls applicable to the change:

- Secret scanning
- Static application security testing
- Dependency and license scanning
- Infrastructure and container scanning
- Dynamic or API security testing for exposed behavior
- Security-focused unit and integration tests

Critical and high-confidence high-severity findings block release unless an approved risk acceptance exists.

## 5. Security Review Gate

Independent security review is REQUIRED for changes involving cryptographic design, authentication protocols, authorization models, sensitive-data export, sandbox escape risk, remote code execution surfaces, or cross-tenant access.

Review evidence MUST identify scope, findings, disposition, approver, and verification performed.

## 6. Security Definition of Done

- Security acceptance criteria are satisfied.
- Required threat-model changes are recorded.
- Applicable automated checks pass.
- Findings have an owner and approved disposition.
- Security-relevant operational documentation and incident signals are updated.
