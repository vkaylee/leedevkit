---
description: Coordinate multiple agents for complex tasks. Use for multi-perspective analysis, comprehensive reviews, or tasks requiring different domain expertise.
---

# Multi-Agent Orchestration

Decompose a complex task across specialists, synthesize their findings, and verify the affected behavior. Scale agent count to the task — fewer agents fit simpler scope.

## Task to Orchestrate
$ARGUMENTS

---

## Mode Check

| Current Mode | Action |
|--------------|--------|
| plan | Proceed planning-first |
| edit, simple | Proceed directly |
| edit, complex/multi-file | Suggest plan mode |
| ask | Confirm before orchestrating |

---

## Process

1. **Analyze domains.** Identify every domain the task touches and the smallest agent set for each.
2. **Plan when warranted.** For multi-file or architectural work, create or reuse a plan; skip the ceremony for routine execution.
3. **Delegate.** Pass each agent exact paths, the user goal, prior decisions, constraints, and expected output. Run independent agents in parallel; sequence dependent ones.
4. **Verify.** Run only checks relevant to the change: tests for behavior changes, security review for trust-boundary work, build/package checks for release-layout changes.
5. **Synthesize.** Merge findings into one report with changed files, verification results, and blockers.

---

## Available Agents

| Agent | Domain |
|---|---|
| `project-planner` | Task breakdown, planning |
| `explorer-agent` | Codebase discovery |
| `frontend-specialist` | UI/UX, web components |
| `backend-specialist` | Server, API, Node.js, Python |
| `database-architect` | SQL, NoSQL, schema |
| `security-auditor` | Vulnerabilities, auth |
| `penetration-tester` | Active security testing |
| `test-engineer` | Unit, E2E, coverage |
| `devops-engineer` | CI/CD, Docker, deploy |
| `mobile-developer` | React Native, Flutter |
| `performance-optimizer` | Profiling, latency |
| `seo-specialist` | SEO, meta, rankings |
| `documentation-writer` | Docs (only when requested) |
| `debugger` | Error analysis |
| `game-developer` | Unity, Godot |
| `api-designer` | REST, GraphQL, OpenAPI |
| `orchestrator` | Coordination |

---

## Context Passing

When invoking a subagent, include:
1. The original user request
2. Decisions already made
3. Prior agent findings
4. Existing plan state, if any

---

## Output Format

```markdown
## Orchestration Report

### Task
[Summary]

### Agents Invoked
| # | Agent | Focus | Status |
|---|-------|-------|--------|
| 1 | ... | ... | ... |

### Verification
- [Check] → Pass/Fail/Not run

### Key Findings
1. [Finding]

### Deliverables
- [ ] Code implemented
- [ ] Tests passing
- [ ] Relevant verification run

### Blockers
[Any unresolved issues]
```

---

Begin orchestration. Route by domain, run independent work in parallel, run relevant verification, synthesize results.
