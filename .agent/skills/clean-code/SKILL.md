---
name: clean-code
description: Pragmatic coding standards - concise, direct, no over-engineering, no unnecessary comments
allowed-tools: Read, Write, Edit
version: 2.0
priority: CRITICAL
---

# Clean Code - Pragmatic AI Coding Standards

> **CRITICAL SKILL** - Be **concise, direct, and solution-focused**.

---

## Core Principles

| Principle | Rule |
|-----------|------|
| **SRP** | Single Responsibility - each function/class does ONE thing |
| **DRY** | Don't Repeat Yourself - extract duplicates, reuse |
| **KISS** | Keep It Simple - simplest solution that works |
| **YAGNI** | You Aren't Gonna Need It - don't build unused features |
| **Boy Scout** | Leave code cleaner than you found it |

### Dead Code

- Delete verified unused imports, variables, private functions, unreachable branches, obsolete dependencies, and commented-out implementations.
- Remove replaced code and its tests, configuration, documentation, and dependencies together.
- Before deletion, check indirect runtime usage such as reflection, framework conventions, serialization, plugins, macros, CLI entry points, and external consumers.
- Transitional code requires a reason, owner, removal condition, and review/removal date. Public APIs require deprecation before removal.
- Do not add speculative code or keep code "for later"; use version control and tracked issues.

---

## Naming Rules

| Element | Convention |
|---------|------------|
| **Variables** | Reveal intent: `userCount` not `n` |
| **Functions** | Verb + noun: `getUserById()` not `user()` |
| **Booleans** | Question form: `isActive`, `hasPermission`, `canEdit` |
| **Constants** | SCREAMING_SNAKE: `MAX_RETRY_COUNT` |

> **Rule:** If you need a comment to explain a name, rename it.

---

## Function Rules

| Rule | Description |
|------|-------------|
| **Small** | Keep functions cohesive; 20 lines is a review signal, not a hard limit |
| **One Thing** | Does one thing, does it well |
| **One Level** | One level of abstraction per function |
| **Few Args** | Prefer focused inputs; investigate long or repeatedly grouped parameter lists |
| **No Side Effects** | Don't mutate inputs unexpectedly |

---

## Code Structure

| Pattern | Apply |
|---------|-------|
| **Guard Clauses** | Early returns for edge cases |
| **Flat > Nested** | Avoid deep nesting (max 2 levels) |
| **Composition** | Small functions composed together |
| **Colocation** | Keep related code close |

---

## AI Coding Style

| Situation | Action |
|-----------|--------|
| User asks for feature | Write it directly |
| User reports bug | Fix it, don't explain |
| Behavior changes | Update tests and edge-case coverage in the same task |
| Missing decision materially changes outcome | Ask for direction |
| Repository context provides a safe answer | Proceed and state the assumption |

---

## Anti-Patterns (DON'T)

| ❌ Pattern | ✅ Fix |
|-----------|-------|
| Comment every line | Delete obvious comments |
| Helper for one-liner | Inline the code |
| Factory for 2 objects | Direct instantiation |
| utils.ts with 1 function | Put code where used |
| "First we import..." | Just write code |
| Deep nesting | Guard clauses |
| Magic numbers | Named constants |
| God functions | Split by responsibility |

---

## 🔴 Before Editing ANY File (THINK FIRST!)

**Before changing a file, ask yourself:**

| Question | Why |
|----------|-----|
| **What imports this file?** | They might break |
| **What does this file import?** | Interface changes |
| **What tests cover this?** | Tests might fail |
| **Is this a shared component?** | Multiple places affected |

**Quick Check:**
```
File to edit: UserService.ts
└── Who imports this? → UserController.ts, AuthController.ts
└── Do they need changes too? → Check function signatures
```

> 🔴 **Rule:** Edit the file + all dependent files in the SAME task.
> 🔴 **Never leave broken imports or missing updates.**

---

## Summary

| Do | Don't |
|----|-------|
| Write code directly | Write tutorials |
| Let code self-document | Add obvious comments |
| Fix bugs immediately | Explain the fix first |
| Inline small things | Create unnecessary files |
| Name things clearly | Use abbreviations |
| Keep functions small | Write 100+ line functions |
| Remove verified dead code | Keep commented-out or speculative code |

> **Remember: The user wants working code, not a programming lesson.**

---

## 🔴 Self-Check Before Completing (MANDATORY)

**Before saying "task complete", verify:**

| Check | Question |
|-------|----------|
| ✅ **Goal met?** | Did I do exactly what user asked? |
| ✅ **Files edited?** | Did I modify all necessary files? |
| ✅ **Code works?** | Did I test/verify the change? |
| ✅ **Edge cases covered?** | Did I verify boundaries, negative paths, failures, state transitions, and compatibility where applicable? |
| ✅ **No errors?** | Applicable configured lint/type checks pass? |
| ✅ **No dead code?** | Any unused, unreachable, replaced, or expired transitional code left behind? |
| ✅ **Nothing forgotten?** | Any edge cases missed? |

> 🔴 **Rule:** If ANY check fails, fix it before completing.

---

## Optional Validation Scripts

Use a skill validation script only when the project provides that script and it applies to the changed files. Discover the configured command and harness capability first; this fixed list is illustrative, not a required agent-to-script mapping. Prefer repository-configured checks for the task. Do not assume `.leedevkit/.venv` or any listed path exists.

| Area | Example script, when present and applicable |
|-------|---------------------------------------------|
| UX / accessibility | `frontend-design/scripts/ux_audit.py` or accessibility checker |
| API | `api-patterns/scripts/api_validator.py` |
| Mobile | `mobile-design/scripts/mobile_audit.py` |
| Security | `vulnerability-scanner/scripts/security_scan.py` |
| Tests / browser | `testing-patterns/scripts/test_runner.py` or `webapp-testing/scripts/playwright_runner.py` |
| Lint / types / i18n | Applicable script under the corresponding skill’s `scripts/` directory |


---

When you run an applicable script, inspect its available result, summarize relevant errors and warnings accurately, and record command and evidence when project workflow requires it. Fix in-scope failures when task authorizes those changes; otherwise report them without expanding scope. Follow harness process and output capabilities instead of assuming a particular capture mechanism.
