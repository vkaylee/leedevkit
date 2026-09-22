---
name: plan-writing
description: Structured task planning with clear breakdowns, dependencies, and verification criteria. Use when implementing features, refactoring, or any multi-step work.
allowed-tools: Read, Glob, Grep
---

# Plan Writing

> Source: obra/superpowers

## Overview
This skill provides a framework for breaking down work into clear, actionable tasks with verification criteria.

## Task Breakdown Principles

### 1. Small, Focused Tasks
- Each task should take 2-5 minutes
- One clear outcome per task
- Independently verifiable

### 2. Clear Verification
- How do you know it's done?
- What can you check/test?
- What's the expected output?

### 3. Logical Ordering
- Dependencies identified
- Parallel work where possible
- Critical path highlighted
- **Phase X: Verification is always LAST**

### 4. Dynamic Naming in Project Root
- Plan files are saved as `{task-slug}.md` in the PROJECT ROOT
- Name derived from task (e.g., "add auth" → `auth-feature.md`)
- Keep saved plans in the project root or repository's established plan location; never assume a vendor-specific hidden directory.

### 5. Saved-Plan Approval Gate
- For non-trivial, medium/high-risk, cross-cutting, or externally visible work, save the plan before implementation and request explicit approval.
- State approval status in the saved plan (`Status: Awaiting approval` → `Status: Approved`) and do not begin implementation while approval is pending.
- Approval is optional for trivial, low-risk, easily reversible work; proceed directly when a saved plan would add more ceremony than value.

### 6. Right-Size Ceremony
- Use a short task list for small changes; add dependencies, phases, interfaces, constraints, or a saved-plan gate only when risk or coordination requires them.
- Never create planning artifacts solely to satisfy this skill. Prefer the smallest plan that makes scope, ownership, and verification unambiguous.

## Planning Principles (NOT Templates!)

> 🔴 **NO fixed templates. Each plan is UNIQUE to the task.**

### Principle 1: Keep It SHORT

| ❌ Wrong | ✅ Right |
|----------|----------|
| 50 tasks with sub-sub-tasks | 5-10 clear tasks max |
| Every micro-step listed | Only actionable items |
| Verbose descriptions | One-line per task |

> **Rule:** If plan is longer than 1 page, it's too long. Simplify.

---

### Principle 2: Be SPECIFIC, Not Generic

| ❌ Wrong | ✅ Right |
|----------|----------|
| "Set up project" | "Run `npx create-next-app`" |
| "Add authentication" | "Install next-auth, create `/api/auth/[...nextauth].ts`" |
| "Style the UI" | "Add Tailwind classes to `Header.tsx`" |

> **Rule:** Each task should have a clear, verifiable outcome.

---

### Principle 3: Dynamic Content Based on Project Type

**For NEW PROJECT:**
- What tech stack? (decide first)
- What's the MVP? (minimal features)
- What's the file structure?

**For FEATURE ADDITION:**
- Which files are affected?
- What dependencies needed?
- How to verify it works?

**For BUG FIX:**
- What's the root cause?
- What file/line to change?
- How to test the fix?

---

### Principle 4: Scripts Are Project-Specific

> 🔴 **DO NOT copy-paste script commands. Choose based on project type.**

| Project Type | Relevant Scripts |
|--------------|------------------|
| Frontend/React | `ux_audit.py`, `accessibility_checker.py` |
| Backend/API | `api_validator.py`, `security_scan.py` |
| Mobile | `mobile_audit.py` |
| Database | `schema_validator.py` |
| Full-stack | Mix of above based on what you touched |

**Wrong:** Adding all scripts to every plan
**Right:** Only scripts relevant to THIS task

---

### Principle 5: Verification is Simple

| ❌ Wrong | ✅ Right |
|----------|----------|
| "Verify the component works correctly" | "Run `npm run dev`, click button, see toast" |
| "Test the API" | "curl localhost:3000/api/users returns 200" |
| "Check styles" | "Open browser, verify dark mode toggle works" |

---

## Plan Structure (Flexible, Not Fixed!)

```
# [Task Name]

Status: [Draft | Awaiting approval | Approved]

## Goal
One sentence: What are we building/fixing?

## Constraints and Interfaces
- **Global constraints:** [Compatibility, security, performance, scope, or tooling limits every task must respect]
- **Global interfaces:** [APIs, schemas, contracts, file boundaries, or ownership rules every task must honor]

## Tasks
- [ ] Task 1: [Specific action] → Constraints: [task-specific limits plus inherited global constraints] → Interface: [contract or surface this task changes] → Verify: [How to check]
- [ ] Task 2: [Specific action] → Constraints: [...] → Interface: [...] → Verify: [How to check]
- [ ] Task 3: [Specific action] → Constraints: [...] → Interface: [...] → Verify: [How to check]

## Done When
- [ ] [Main success criteria]

## Review Focus (Optional)
- [Up to five risks the specification implies but existing tests or checks do not cover]

## Notes
[Any important considerations]
```

> **That's it.** No phases, no sub-sections unless truly needed.
> Keep it minimal. Add complexity only when required.
> Trust explicit interfaces over task-local assumptions, and verify at the level the user will observe.

---

## Best Practices (Quick Reference)

1. **Start with goal** - What are we building/fixing?
2. **Max 10 tasks** - If more, break into multiple plans
3. **Each task verifiable** - Clear "done" criteria
4. **Project-specific** - No copy-paste templates
5. **Update as you go** - Mark `[x]` when complete

---

## When to Use

- New project from scratch
- Adding a feature
- Fixing a bug (if complex)
- Refactoring multiple files
