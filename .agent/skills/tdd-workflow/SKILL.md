---
name: tdd-workflow
description: Test-Driven Development workflow principles. RED-GREEN-REFACTOR cycle.
allowed-tools: Read, Write, Edit, Glob, Grep, Bash
---

# TDD Workflow

> Write tests first, code second. Verify observable behavior, not implementation details.

---

## 1. The TDD Cycle

```
🔴 RED → Write failing test with falsifiable behavior assertion
    ↓
🟢 GREEN → Write minimal code to pass the test
    ↓
🔵 REFACTOR → Clean code while keeping tests green
    ↓
🏁 GREEN GATE → Full project test suite passes before completion
    ↓
   Repeat...
```

---

## 2. The Three Laws of TDD

1. Write production code only to make a failing test pass.
2. Write only enough test to demonstrate failure of a specific behavior.
3. Write only enough production code to make the failing test pass.

---

## 3. RED Phase Principles

### What to Write

| Focus | Example | Assert What |
|-------|---------|-------------|
| Behavior | "should calculate discounted price" | Output value, returned contract |
| Edge cases | "should handle empty input array" | Boundary return or specific error |
| Error states | "should reject expired session token" | Rejection type, status, or message |
| Invariants | "balance cannot fall below zero" | State constraint across transitions |

### RED Phase Rules

- **Watch it fail first:** The test must fail before writing production code, and for the expected reason (not syntax/import errors).
- **Falsifiable behavior assertions:** Assertions must test observable outcomes (return values, state changes, external contracts) and must fail on plausible defects.
- **Reject change-detector tests:** Do not assert source text, AST structure, internal variable names, private method calls, or mock call counts that merely mirror implementation code.
- **One behavior per test:** Test name describes the observable requirement, not the internal mechanism.

---

## 4. Test Design: Behavior vs Implementation

### Falsifiable Assertions vs Fragile Checks

| Quality | Practice | Why |
|---------|----------|-----|
| ✅ **Falsifiable** | Assert output, observable state, or contract error | Fails when behavior breaks; passes when behavior holds |
| ✅ **Refactor-safe** | Test public interface and behavior boundaries | Allows internal refactoring without breaking tests |
| ❌ **Source-text check** | Searching file contents or AST for specific tokens | Breaks on syntax-preserving changes; proves no behavior |
| ❌ **Change-detector** | Mocking internals and asserting call sequence | Pins implementation, couples tests to internal wiring |
| ❌ **Vacuous test** | Bare `not-throw`, tautological comparisons | Never fails on plausible bugs; provides false confidence |

---

## 5. GREEN Phase Principles

### Minimum Code

| Principle | Meaning |
|-----------|---------|
| **YAGNI** | You Aren't Gonna Need It — build only what test demands |
| **Simplest thing** | Write the minimum code to pass the failing test |
| **No early optimization** | Make it correct first, optimize in refactor phase |

### GREEN Phase Rules

- Do not write unrequested features or unneeded abstractions.
- Do not write speculative code "for later".
- Make the failing test pass, nothing more.

---

## 6. REFACTOR Phase Principles

### What to Improve

| Area | Action |
|------|--------|
| Duplication | Extract common logic and shared constants |
| Naming | Make intent and domain concepts clear |
| Structure | Simplify control flow, eliminate nesting |
| Complexity | Delete weightless abstractions and dead branches |

### REFACTOR Rules

- All tests must stay green during refactoring.
- Small incremental changes; run tests after each change.
- Do not add new behavior during refactoring.

---

## 7. The Project-Suite Green Gate

Passing only the newly written test is not sufficient for completion.

### Green Gate Requirements

1. **Local test passes:** The new/modified test passes consistently.
2. **Regression check passes:** Original bug reproduction fails pre-fix and passes post-fix.
3. **Full suite green:** The entire project test suite (unit, integration, regression) runs and passes.
4. **No silenced tests:** No tests skipped, retried, commented out, or weakened to force a green result.
5. **Clean environment:** No leftover test artifacts, mocks leaking between runs, or leaked background processes.

---

## 8. AAA Pattern

Every test follows Arrange-Act-Assert:

| Step | Purpose | Rule |
|------|---------|------|
| **Arrange** | Set up test data and preconditions | Minimal context needed for the scenario |
| **Act** | Execute code under test | Single observable action or invocation |
| **Assert** | Verify expected outcome | Falsifiable check on observable result or state |

---

## 9. When to Use TDD

| Scenario | TDD Value | Approach |
|----------|-----------|----------|
| New feature | High | Specify behavior with failing test, then implement |
| Bug fix | High | Reproduce bug with failing test first, then fix |
| Complex business logic | High | Drive edge cases and boundary conditions with tests |
| Exploratory spike | Low | Prototype first, discard/keep learnings, then TDD |
| Visual/declarative layout | Low | Prefer visual inspection; unit test logic only |

---

## 10. Anti-Patterns

| ❌ Don't | ✅ Do |
|----------|-------|
| Skip the RED phase | Watch test fail before writing implementation |
| Write tests after implementation | Drive implementation from test requirements |
| Test source text / AST contents | Test observable runtime behavior |
| Create change-detector tests | Test public contracts; let internals refactor freely |
| Write vacuous assertions (tautology, bare not-throw) | Assert specific observable state or error contracts |
| Stop at isolated test pass | Pass the full project-suite green gate |
| Weaken existing assertions to pass | Fix implementation or address root regression |
| Over-engineer initial implementation | Keep implementation minimal until refactor phase |

---

## 11. Collaborative TDD Workflow

When dividing TDD tasks across roles or sessions:

| Stage | Focus | Acceptance |
|-------|-------|------------|
| **Test Author** | Write failing tests for required behaviors | Test fails for expected behavioral reason; assertions falsifiable |
| **Implementer** | Write minimal code to pass failing test | Target test passes; no unrequested abstractions |
| **Refactorer** | Improve code quality and structure | Code clean; full project-suite green gate holds |

---

> **Remember:** The test is an executable specification of observable behavior. If a test breaks when implementation changes but behavior stays correct, the test is defective.
