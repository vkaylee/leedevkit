# Development Workflow

> Scope: Every implementation, bug fix, refactor, migration, configuration change, and generated-code change that can alter runtime or build behavior.

## 1. Primary Execution Gate (BLOCKING)

Development and testing form one indivisible task. An implementation is not complete when the code is written; it is complete only when the changed behavior and its material risks are verified.

The agent MUST NOT defer ordinary test design to the user, omit tests to save time, or report completion based only on compilation, linting, manual inspection, or the happy path.

## 2. Before Implementation: Test Impact Matrix (REQUIRED)

Before editing behavior, identify the test impact in the working plan or reasoning record:

| Dimension | Required analysis |
|---|---|
| Changed behavior | What observable result is new or different? |
| Existing behavior | What must remain unchanged? |
| Inputs and boundaries | Empty, missing, minimum, maximum, malformed, duplicate, and unexpected values |
| State | Initial, existing, partial, stale, concurrent, and already-completed states |
| Failures | Dependency error, timeout, cancellation, retry exhaustion, partial success, and rollback |
| Security | Unauthorized, forbidden, cross-tenant, tampered, replayed, and excessive input |
| Compatibility | Old clients, old data, mixed versions, deprecated fields, and additive schema changes |
| Operations | Logging, metrics, tracing, resource limits, and recovery behavior |

Mark a dimension not applicable only when there is a concrete reason.

## 3. Implementation Loop (REQUIRED)

For each behavior change:

1. Establish protection with an existing test or a new failing test when practical.
2. Implement the smallest cohesive change that satisfies the requirement.
3. Run focused tests covering the changed behavior.
4. Add missing boundary, negative, regression, and failure-path cases revealed by the implementation.
5. Run the applicable target verification.
6. Inspect the complete result; do not rely only on an exit-code summary when logs contain warnings or skipped checks.

For bug fixes, a regression test MUST fail without the fix unless reproduction is impossible in the available test environment. Any exception requires a concrete explanation and alternative verification.

## 4. Mandatory Test Categories

Tests MUST cover categories applicable to the change:

- Primary success behavior
- Boundary and validation behavior
- Negative and authorization behavior
- Regression for the reported or discovered defect
- Dependency and error propagation
- State transition and persistence behavior
- Idempotency, retry, concurrency, or ordering behavior
- Compatibility and migration behavior
- Resource or size limits

Do not duplicate tests mechanically. Each test SHOULD protect a distinct contract, risk, or failure mode.

## 5. Test Quality

- Assert observable behavior and externally meaningful state, not private implementation details.
- Make failure messages and scenario names reveal the protected contract.
- Keep tests deterministic, isolated, and independent of execution order.
- Control time, randomness, network, and external dependencies.
- Do not weaken assertions, remove coverage, add arbitrary sleeps, or mark tests skipped merely to make the suite pass.
- A flaky test is a defect. Quarantine requires an owner, issue, reason, and expiry date.

## 6. Change-Type Requirements

| Change type | Minimum evidence |
|---|---|
| New behavior | Success, boundary, negative, and relevant failure-path tests |
| Bug fix | Reproducing regression test plus adjacent boundary cases |
| Refactor | Existing behavior tests pass; add characterization tests when protection is insufficient |
| API or event contract | Schema, compatibility, authorization, and error-shape tests |
| Persistence or migration | Forward migration, mixed-version behavior, reconciliation, and recovery evidence |
| Concurrency or retry | Deterministic race, idempotency, ordering, timeout, and exhaustion tests as applicable |
| Documentation only | Structural validation; explain why runtime tests are not applicable |

## 7. Completion Evidence (BLOCKING)

The final report for a code task MUST state:

- Behaviors implemented or preserved
- Tests added or updated, grouped by success, edge, negative, regression, and failure cases
- Exact verification commands or targets run
- Pass, fail, skip, and warning results
- Checks that could not run and why
- Remaining risks or explicitly non-applicable test dimensions

The phrases “tests pass” or “covered edge cases” are insufficient without scenarios and verification evidence.
