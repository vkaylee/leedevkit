---
name: parallel-agents
description: Coordinate specialized subagents for independent analysis, dependent implementation, verification, and synthesis.
allowed-tools: Read, Glob, Grep
---

# Multi-Agent Orchestration

Use the smallest capable specialist set. Run independent analysis in parallel; sequence work with dependencies; synthesize before editing shared files.

## Specialist Selection

- `security-auditor`: Threat modeling, vulnerability analysis, auth review
- `backend-specialist`: Server architecture and backend logic
- `frontend-specialist`: Web UI, accessibility, components, styling
- `api-designer`: API contracts and versioning
- `database-architect`: Schemas, migrations, indexes, queries
- `devops-engineer`: CI/CD, deployments, observability
- `test-engineer`: Tests, regression coverage, QA
- `debugger`: Root-cause analysis and reproduction
- `performance-optimizer`: Profiling and bottlenecks
- `mobile-developer`: iOS, Android, React Native, Flutter
- `explorer-agent` / `Explore`: Read-only discovery
- `general-purpose`: Multi-step work without a narrower specialist

## Delegation

Pass exact paths, constraints, success criteria, and relevant project rules. Include prior findings for dependent tasks. Keep file ownership explicit: domain agents own production code; test agents own tests; security agents review security-sensitive changes.

When several lookups are independent, delegate them concurrently. Do not delegate trivial one-file changes.

## Protocol

1. Scope the task.
2. Read applicable `.agent/rules/` files.
3. Choose the smallest capable agent set.
4. Delegate with concrete context.
5. Run relevant project checks.
6. Report findings and unresolved blockers directly.
