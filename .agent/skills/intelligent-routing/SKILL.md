---
name: intelligent-routing
description: Select the smallest suitable Claude Code agent or agent set from task intent, domain, and complexity.
allowed-tools: Read, Grep, Glob
---

# Intelligent Agent Routing

Route work by risk and scope. Do not invoke agents for simple questions or one-line changes.

## Agent Selection Matrix

| Intent | Signals | Agent(s) |
|---|---|---|
| Authentication or authorization | login, auth, password, token, RBAC | `security-auditor`, `backend-specialist` |
| UI or web component | React, Vue, CSS, HTML, Tailwind, layout | `frontend-specialist` |
| Mobile app | React Native, Flutter, iOS, Android, Expo | `mobile-developer` |
| API contract | endpoint, OpenAPI, REST, GraphQL, versioning | `api-designer` |
| Backend implementation | server, handler, service, API logic | `backend-specialist` |
| Database | SQL, schema, migration, query, index, Postgres | `database-architect` |
| Bug investigation | error, crash, broken, regression | `debugger` |
| Testing | test, coverage, unit, integration, E2E | `test-engineer` |
| Deployment or infrastructure | deploy, CI/CD, Docker, Kubernetes, release | `devops-engineer` |
| Security review | vulnerability, threat model, OWASP, exploit | `security-auditor` |
| Performance | slow, latency, memory, CPU, optimize | `performance-optimizer` |
| Game development | Unity, Godot, Unreal, Phaser, multiplayer | `game-developer` |
| Planning or requirements | roadmap, backlog, acceptance criteria | `project-planner` or `product-owner` |
| Documentation | README, API docs, changelog | `documentation-writer` |

## Complexity Rules

- **Simple:** One file, one domain, clear behavior. Use one specialist or work directly.
- **Moderate:** Several files or two domains. Use a short sequential agent chain.
- **Complex:** Multiple domains, architecture, migration, or unclear scope. Use `orchestrator`.

## Invocation

When the repository is configured for specialist agents, invoke the matched agent with concrete paths and constraints. Otherwise implement directly and apply the relevant `.agent/rules/` files.

## Routing Protocol

1. Identify intent and trust boundary.
2. Choose the smallest capable agent set.
3. Read applicable `.agent/rules/*.md` files before implementation.
4. Pass concrete paths, constraints, and expected output to each agent.
5. Verify implementation with the project-configured tests.
6. Report unresolved blockers instead of silently broadening scope.

## Overrides

If the user names a specific agent, honor that choice when the agent exists. If the name is unavailable, use `general-purpose` or `orchestrator` and state the substitution.

## Examples

- `"Fix a 401 from the login endpoint"` → `debugger`, then `security-auditor` if auth handling changes.
- `"Add an OpenAPI contract for invoices"` → `api-designer`.
- `"Build a secure chat app with a web UI"` → `orchestrator` coordinating `backend-specialist`, `frontend-specialist`, `security-auditor`, and `test-engineer`.
- `"Explain how React state works"` → answer directly; no agent.
