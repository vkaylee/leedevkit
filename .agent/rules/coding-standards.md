# 💻 Coding Standards & Best Practices

> Apply universal principles to every project. Apply Rust, React, API, and domain-specific examples only when the affected component uses those technologies and project conventions.

## 1. Language Handling
- **Communication:** Respond in the language the user initiated the prompt with.
- **Code:** ALL variables, function names, and code comments MUST be in English.

## 2. Clean Code Standards
- **Development and Tests:** Follow `development-workflow.md` for behavior analysis and implementation. `testing-standards.md` defines check execution, coverage, and reporting.
- **SOLID & DRY:** Keep responsibilities focused; extract reusable logic when reuse or separation improves clarity.
- **Self-Documenting:** Use clear names, minimal comments, and no speculative abstraction.
- **Encapsulation:** Use the language and framework's established visibility and access patterns. Restrict state access when needed to protect invariants; public fields, constructors, and accessors are design choices governed by the component's API and project conventions, not universal requirements.
### Dead Code Policy (MANDATORY)
- **No New Dead Code:** Do not merge unused imports, variables, private functions, unreachable branches, obsolete dependencies, commented-out implementations, or code for speculative future use. Git history is the archive.
- **Remove Replaced Code:** When a change supersedes an implementation, remove the obsolete implementation and its tests, configuration, dependencies, and documentation in the same change.
- **Verify Before Removal:** Confirm call sites and framework/runtime usage before deleting code. Reflection, dependency injection, serialization, macros, plugins, CLI entry points, and external consumers may not appear in static references.
- **Scoped Cleanup:** Remove verified dead code encountered in files touched by the task. If cleanup outside the change would materially increase risk or review size, record a technical-debt issue with an owner instead of silently leaving it undocumented.
- **Time-Bound Exceptions:** Transitional code, migrations, compatibility shims, and feature-flag branches may remain only with a documented reason, owner, removal condition, and review/removal date. Expired exceptions MUST be removed.
- **Safe Removal:** Public APIs and compatibility behavior MUST follow the project's deprecation policy before removal. Dead-code cleanup MUST preserve current behavior and satisfy applicable verification defined in `testing-standards.md`.
- **Enforcement:** Use repository-configured compiler and linter checks for unused or unreachable code where available. Keep suppressions narrowly scoped and specifically justified.

## 3. Error Handling & Observability
- **Structured Errors:** Use the project's domain-specific error type when one exists. In projects defining `AppError`, `I18nKey`, and `AppResult<T>`, preserve those conventions rather than introducing generic errors or raw user-facing strings.
- **Error Boundaries:** User-interface components with independent failure modes SHOULD use the framework's supported error isolation pattern. Do not wrap every trivial leaf component mechanically.
- **Telemetry Baseline:** Production backend request boundaries MUST participate in the project's tracing and metrics conventions when observability is configured.

## 4. Security & Compliance
- **Data Privacy:** NEVER log Personally Identifiable Information (PII) like emails, passwords, phone numbers in raw text. Mask them (e.g., `user_***@domain.com`).
- **Secrets:** NEVER hardcode secrets. Use environment variables.

## 5. Boy Scout Rule
Improve nearby code when the issue is verified, low-risk, behavior-preserving, and naturally coupled to the requested change. Report or track unrelated, risky, or review-expanding issues separately. This rule does not broaden user authorization.

## 6. Production Readiness (REQUIRED)
- **Risk-Based Application:** Production-facing behavior MUST meet the security, correctness, accessibility, operability, and compatibility requirements applicable to its risk. Internal tools, prototypes, documentation, and narrow fixes apply only relevant controls.
- **Frontend:** User-facing asynchronous flows SHOULD include appropriate loading, empty, error, and accessible interaction states. i18n is REQUIRED when the project supports multiple locales. Decorative micro-interactions are optional.
- **Backend:** Validate untrusted input, use explicit error handling, and add transaction boundaries or audit logging when data integrity or accountability requires them.
- **Avoid Over-Engineering:** KISS and YAGNI remain valid. Do not introduce abstractions, infrastructure, feature flags, or operational machinery without a current requirement or demonstrated risk.

## 7. Performance & Resource Optimization
- **Performance Budget:** Use repository-configured budgets. For user-facing web applications without configured budgets, LCP < 2.5s and a 200KB gzipped initial JS target are RECOMMENDED baselines, not universal blocking gates.
- **Scalability:** N+1 queries MUST NOT be introduced in data-access paths. Public APIs exposed to untrusted or high-volume callers MUST use an appropriate abuse-control strategy; rate limiting is not required for every internal API.
- **Data Access:** Keep persistence and domain concerns separate where it improves testability, reuse, or query efficiency. Batch data when needed to avoid N+1 access; follow the project's established boundaries and data-access conventions.

## 8. Escalation Protocol
- **The Rule:** Retry only when new evidence or a materially different safe approach exists. After repeated failure with the same blocker, stop, summarize attempts and evidence, and request the missing input or authority. Do not retry mechanically.

## 9. Technology-Specific Practices
- **Dependency Boundaries:** Keep domain and service logic independent from replaceable infrastructure where testing or substitution provides value. Introduce interfaces at meaningful boundaries, not for every concrete dependency.
- **Async runtimes:** In components using an async runtime, avoid blocking executor threads and holding blocking locks across suspension points. Follow the selected runtime's and project's supported APIs; justify exceptions that risk deadlocks or starvation.
- **API contracts:** For APIs that can return unbounded collections, use pagination, streaming, or an explicit safe limit when appropriate. Follow project compatibility and versioning policy for contract changes.
- **Frontend:** Separate reusable business logic from presentation when that improves testability and reuse. Avoid waterfalls for independent requests; preserve dependencies between ordered requests.

## 10. Related Rulebooks
When working on specific concerns, also consult:
- Read `.agent/rules/ai-agent-governance.md` — rule priority, scope, exceptions, verification, and Definition of Done.
- Read `.agent/rules/database-rules.md` — multi-tenancy, N+1 prevention, migrations, and soft deletes.
- Read `.agent/rules/testing-standards.md` — check execution, coverage, and reporting.
- Read `.agent/rules/development-workflow.md` — behavior analysis and implementation loop.
- Read `.agent/rules/architecture-governance.md` — ADRs, boundaries, and architecture gates.
- Read `.agent/rules/api-governance.md` — public contracts and compatibility.
- Read `.agent/rules/reliability-engineering.md` — SLOs, resilience, capacity, and operations.
- Read `.agent/rules/migration-and-rollback.md` — safe evolution and rollback.
- Read `.agent/rules/release-management.md` — artifacts, rollout, and release verification.
- Read `.agent/rules/vulnerability-management.md` — triage, remediation, and closure evidence.
- Read `.agent/rules/access-control.md` — RBAC, JWT, and permissions.
- Read `.agent/rules/data-governance.md` — PII masking, classification, and retention.
- Read `.agent/rules/design-rules.md` — UI/UX, responsive design, and accessibility.
- Read `.agent/rules/observability-rules.md` — tracing, logging, and metrics.
- Read `.agent/rules/encryption-rules.md` — secrets, TLS, and key management.
- Read `.agent/rules/configuration-management.md` — environment variables and fail-fast validation.
- Read `.agent/rules/execution-wrappers.md` — hermetic build and test scripts.
- Read `.agent/rules/project-structure.md` — component map, commits, and directory layout.
