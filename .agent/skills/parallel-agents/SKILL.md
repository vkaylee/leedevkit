---
name: parallel-agents
description: Coordinate specialized subagents and deterministic workflows using Claude Code agent tools.
allowed-tools: Read, Glob, Grep, Agent, Workflow
---

# Multi-Agent and Workflow Orchestration

Use `Agent` for independent specialist tasks. Use `Workflow` for deterministic multi-stage discovery, verification, and synthesis.

## Available Specialized Agents

- `orchestrator`: Overall task decomposition and synthesis.
- `security-auditor`: Threat modeling, vulnerability analysis, and auth review.
- `penetration-tester`: Active security verification and exploit assessment.
- `backend-specialist`: Server architecture, backend logic, and systems engineering.
- `frontend-specialist`: Web UI, accessibility, component design, and styling.
- `api-designer`: API contracts, OpenAPI, payload design, and versioning.
- `database-architect`: Schemas, migrations, indexing, and query patterns.
- `devops-engineer`: CI/CD, containerization, deployments, and observability.
- `test-engineer`: Test strategy, regression tests, and coverage.
- `qa-automation-engineer`: End-to-end automation and pipeline verification.
- `debugger`: Root-cause analysis and reproduction of failures.
- `performance-optimizer`: Profiling, latency, and resource bottlenecks.
- `mobile-developer`: Mobile applications across iOS, Android, and cross-platform.
- `game-developer`: Game systems, mechanics, and engines.
- `documentation-writer`: Technical docs, contract guides, and changelogs.
- `project-planner`: Work breakdown, dependency sequencing, and milestone mapping.
- `product-manager`: Product requirements, feature definitions, and trade-offs.
- `product-owner`: User stories, backlog refinement, and acceptance criteria.
- `seo-specialist`: Search discoverability and web performance metrics.
- `explorer-agent`: Deep structural exploration and architecture discovery.
- `code-archaeologist`: Legacy code comprehension and refactoring safety.

## Built-In Agents

- `general-purpose`: Multi-step coding or exploration when no specialist fits.
- `Explore`: Read-only discovery for symbols, file paths, and cross-references.
- `Plan`: High-level architecture and implementation planning.

## Single and Chained Invocations

Invoke agents through the Claude Code tool runner:

```text
Agent({
  description: "Audit auth middleware",
  subagent_type: "security-auditor",
  prompt: "Review src/auth/middleware.ts for session validation, token expiration, and bypass risks."
})
```

When running multiple independent lookups, send the tool calls in parallel within a single turn.

## Workflow Orchestration Pattern

Use `Workflow` when orchestrating fan-out, adversarial checks, or synthesis across many files:

```javascript
export const meta = {
  name: 'code-audit',
  description: 'Decompose codebase review across security, backend, and testing',
  phases: [{ title: 'Audit' }, { title: 'Synthesize' }],
}

const targets = ['security', 'backend', 'tests']
const reviews = await pipeline(
  targets,
  target => agent(`Review ${target} in this codebase`, { phase: 'Audit' }),
)

phase('Synthesize')
const summary = await agent(`Synthesize findings: ${JSON.stringify(reviews)}`, { phase: 'Synthesize' })
return { summary }
```

## Protocol

1. Scope the task before launching agents.
2. Read project rules from `.agent/rules/` before modifying code.
3. Pass exact paths, constraints, and success criteria into agent prompts.
4. Run project checks before declaring work complete.
5. Report findings directly with actionable next steps.
