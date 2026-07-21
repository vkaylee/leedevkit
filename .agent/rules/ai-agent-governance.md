# AI Agent Governance

> This rulebook defines how AI agents interpret and apply all other project rules.

## 1. Rule Priority

When instructions conflict, apply them in this order:

1. Safety, security, privacy, legal obligations, and explicit user authorization
2. Correctness, data integrity, and backward compatibility
3. Explicit task requirements and acceptance criteria
4. Project and domain rulebooks applicable to the changed component
5. Maintainability, performance, testing, and operability
6. Style and preference

A lower-priority rule MUST NOT be used to violate a higher-priority rule. Report unresolved conflicts instead of silently choosing one.

## 2. Requirement Levels

| Level | Meaning |
|---|---|
| **BLOCKING** | Must be satisfied. Stop and report when it cannot be satisfied safely. |
| **REQUIRED** | Default requirement. A documented, narrow exception is allowed when justified by project context. |
| **RECOMMENDED** | Apply when it improves the result without disproportionate cost or scope expansion. |
| **ADVISORY** | Consider based on context; omission does not block completion. |

Words such as `MUST`, `NEVER`, and `MANDATORY` are reserved for BLOCKING or REQUIRED rules. Numeric thresholds are enforcement gates only when the repository has a configured check for them; otherwise they are targets requiring engineering judgment.

## 3. Applicability

Apply a rule only when its technology, component, risk, and lifecycle scope match the task. Examples and paths from another project are informative, not executable requirements.

Before acting, determine:

- Task type: answer, diagnose, implement, refactor, review, release, or operate
- Affected component and language
- Observable behavior and contracts at risk
- Applicable rulebooks and verification targets

Do not load or apply unrelated specialist guidance merely because it exists.

## 4. Scope and Autonomy

- Make reasonable, reversible implementation decisions within the requested outcome.
- Fix an adjacent issue only when it is verified, low-risk, behavior-preserving, and necessary for or naturally coupled to the requested change.
- Report or track unrelated, risky, architectural, or review-expanding issues separately. Do not use the Boy Scout Rule to broaden authorization.
- Ask for user direction only when a missing decision materially changes the outcome, requires new authority, or creates significant external impact.
- Do not force discovery questions for routine fixes when repository context provides a safe answer.

## 5. Exceptions and Suppressions

An exception to a REQUIRED rule MUST be narrow and recorded near the affected code or in a tracked decision. Include:

- Rule or check being excepted
- Concrete reason
- Scope of the exception
- Owner or responsible team
- Review or removal condition and date when temporary

Never use broad file-level or project-wide suppressions when a smaller suppression is possible.

## 6. Verification Discovery

Do not assume a command or wrapper exists. Discover verification in this order:

1. Project configuration and documented project-local commands
2. Existing wrapper scripts or task runners
3. CI workflow definitions
4. Ecosystem-native commands only when no project wrapper exists and host execution is safe

Use the narrowest verification that covers the change. Expand to broader checks when shared infrastructure, public contracts, packaging, or multiple targets are affected.

Safe deterministic formatting may be applied automatically to files in scope. Semantic auto-fixes, dependency changes, and fixes outside scope require review before application.

## 7. Definition of Done

A task may be reported complete only when:

- The requested outcome and acceptance criteria are met.
- Scope stayed within user authorization.
- Relevant security, privacy, compatibility, and data-integrity risks were addressed.
- Applicable lint, type, test, build, and packaging checks passed, or any unverified check is explicitly reported with its reason.
- Tests were added or updated for changed behavior, or a behavior-based reason explains why no test change was needed.
- The Test Impact Matrix in `development-workflow.md` was addressed, including applicable edge, negative, regression, and failure-path cases.
- Documentation, configuration, dependencies, and public contracts were updated when affected.
- No verified dead code, unexplained suppression, or expired transitional code was introduced.
- The final report distinguishes completed work, verification performed, and remaining risks.

Unverified work may be handed off as implemented, but MUST NOT be represented as fully verified.

## 8. Rulebook Quality

Rulebooks MUST:

- Be written in English.
- Use existing repository paths and commands, or clearly label examples as conditional.
- Avoid product-specific assumptions in universal rules.
- State scope, exceptions, and measurable verification for blocking requirements.
- Prefer one unambiguous requirement per bullet.
- Be updated only with explicit user or designated maintainer approval.
