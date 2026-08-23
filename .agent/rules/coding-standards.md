# 💻 Coding Standards & Best Practices

> Apply universal principles to every project. Apply Rust, React, API, and domain-specific examples only when the affected component uses those technologies and project conventions.

## 1. Language Handling
- **Communication:** Respond in the language the user initiated the prompt with.
- **Code:** ALL variables, function names, and code comments MUST be in English.

## 2. Clean Code Standards
- **Development and Tests:** Follow `development-workflow.md`. Behavior-changing code and its applicable tests MUST be delivered and verified together.
- **SOLID & DRY:** Enforce Single Responsibility. Extract reusable logic.
- **Self-Documenting:** Write clear names. Minimal comments. No over-engineering.
- **Strict Encapsulation (Rust/OOP Components):** Struct and object state SHOULD be private by default when invariants require controlled access.
  - Do NOT use `pub` for struct fields.
  - State mutation and data access MUST be controlled via explicitly defined `new()` constructors and getter methods (e.g., `inner()`, `into_inner()`, `name()`).

### Dead Code Policy (MANDATORY)
- **No New Dead Code:** Do not merge unused imports, variables, private functions, unreachable branches, obsolete dependencies, commented-out implementations, or code for speculative future use. Git history is the archive.
- **Remove Replaced Code:** When a change supersedes an implementation, remove the obsolete implementation and its tests, configuration, dependencies, and documentation in the same change.
- **Verify Before Removal:** Confirm call sites and framework/runtime usage before deleting code. Reflection, dependency injection, serialization, macros, plugins, CLI entry points, and external consumers may not appear in static references.
- **Scoped Cleanup:** Remove verified dead code encountered in files touched by the task. If cleanup outside the change would materially increase risk or review size, record a technical-debt issue with an owner instead of silently leaving it undocumented.
- **Time-Bound Exceptions:** Transitional code, migrations, compatibility shims, and feature-flag branches may remain only with a documented reason, owner, removal condition, and review/removal date. Expired exceptions MUST be removed.
- **Safe Removal:** Public APIs and compatibility behavior MUST follow the project's deprecation policy before removal. Dead-code cleanup MUST preserve current behavior and pass the relevant lint, type, and test gates.
- **Enforcement:** Configure compiler and linter checks for unused or unreachable code where the language supports them. Confirmed findings MUST fail CI; suppressions require a specific justification and MUST be as narrow as possible.

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
- **Pure Domain Logic (N+1 Prevention):** To strictly prevent N+1 queries at the architectural level, do NOT pass Database Connections (`PgConnection`, `TenantAuthToken`) or Repositories into pure Business/Domain logic functions (e.g., calculation engines). Force data to be batch-fetched beforehand and passed in as memory structures.

## 8. Escalation Protocol
- **The Rule:** Retry only when new evidence or a materially different safe approach exists. After repeated failure with the same blocker, stop, summarize attempts and evidence, and request the missing input or authority. Do not retry mechanically.

## 9. Code-Level Execution Rules (Enterprise Pro Max)
- **Dependency Injection & Decoupling:** Keep domain and service logic independent from replaceable infrastructure where testing or substitution provides value. Introduce interfaces at meaningful boundaries, not for every concrete dependency.
- **Concurrency Safety (Rust):** NEVER hold an OS-level lock (`MutexGuard`, `RwLockReadGuard`) across an `await` point to avoid deadlocks. NEVER use blocking I/O calls (e.g., `std::fs`, `std::thread::sleep`) inside an async runtime; use `tokio` equivalents.
- **API Contract Design:** APIs returning potentially unbounded collections MUST implement pagination, streaming, or an explicit safe limit. Do not introduce breaking contract changes without following the deprecation and versioning policy.
- **Frontend Architecture:** Separate reusable business logic from presentation when it improves testability and reuse. Avoid network waterfalls when requests are independent; do not parallelize dependent requests.

## 10. Related Rulebooks
When working on specific concerns, also consult:
- Read `.agent/rules/ai-agent-governance.md` — rule priority, scope, exceptions, verification, and Definition of Done.
- Read `.agent/rules/database-rules.md` — multi-tenancy, N+1 prevention, migrations, and soft deletes.
- Read `.agent/rules/testing-standards.md` — test gates, coverage thresholds, and `./leedevkit test`.
- Read `.agent/rules/development-workflow.md` — implementation loop, test impact, edge cases, and evidence.
- Read `.agent/rules/secure-development-lifecycle.md` — threat modeling and security verification.
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
