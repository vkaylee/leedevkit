# 🧪 Testing Standards

## 1. Post-Implementation Test Gate (MANDATORY)

> 🔴 **Every code implementation or behavior change MUST pass this gate before the task is reported complete.**

After implementing a change, the AI agent MUST:

1. Identify the new or changed observable behaviors, failure paths, and regression risks.
2. Inspect the existing tests that cover those behaviors.
3. Decide autonomously whether tests must be added or updated; do not defer this engineering decision to the user.
4. Record one explicit outcome in the final report:
   - **Tests added/updated:** list the scenarios covered; or
   - **No test change needed:** give a concrete, behavior-based justification.
5. Run the applicable LeeDevKit test target and report the exact result. If verification cannot run, state why and leave the task unverified—not complete.

Tests MUST be added or updated for:
- New or changed business logic or externally observable behavior
- Bug fixes and regressions (the test MUST fail without the fix)
- Validation, authorization, persistence, API contract, concurrency, and error-path changes

A test change MAY be unnecessary for documentation-only, comment-only, formatting-only, or generated-artifact changes that cannot alter behavior. The agent must still state this conclusion and its reason.

## 2. Test Execution Commands

> [!WARNING]
> **NEVER run `cargo test`, `npm test`, `npx playwright test`, or another framework test command directly.** Use the project-local LeeDevKit wrapper so execution remains hermetic.

| Scope | Command | Purpose |
|---|---|---|
| **Fast feedback** | `./leedevkit test <target> --unit-only` | Focused unit-test iteration |
| **Applicable target** | `./leedevkit test <target>` | Required verification before completion |
| **Coverage** | `./leedevkit test <target> --coverage` | Coverage verification for changed logic |
| **Full suite** | `./leedevkit test all` | Cross-target regression verification |

Use the narrowest target that fully covers the change. Run the full suite when the change crosses targets or affects shared infrastructure. Do not use background execution (`&`). Follow `execution-pty-safety.md` for output handling.

## 3. Core Testing Principles

- **Testing Pyramid:** Prioritize Unit > Integration > E2E.
- **Pattern:** Use AAA Pattern (Arrange-Act-Assert).
- **Behavior First:** Test public behavior and regression risk, not implementation details.
- **Code Coverage:** Minimum **80%** coverage for all new business logic; critical paths target 100%.
- **Mandatory Pass:** A code task is NOT complete until the Post-Implementation Test Gate passes.

## 4. Verification Pipeline

> 🔴 **Execution Priority:** Security → Lint → Schema → Tests → UX → SEO → Performance

The LeeDevKit target command orchestrates the applicable type, lint, schema, and test phases:

```bash
./leedevkit test <target>
```

Use `--lint-only`, `--unit-only`, `--e2e-only`, `--pattern`, or `--coverage` only for focused iteration. Focused checks do not replace the applicable target verification required before completion.
