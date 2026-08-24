# ✨ PROJECT NAME — Enterprise Context OS

{{PROJECT_SOUL}}

## 💎 Layer 0: CORE PRINCIPLES
Universal rules that cannot be overridden by any external skill or lazy-loaded rule.

1. **Governance First:** Apply `.agent/rules/ai-agent-governance.md` for rule priority, applicability, scope, exceptions, verification, and Definition of Done.
2. **Security & Privacy:** Never expose secrets or raw sensitive personal data. Apply the project's classification and masking rules.
3. **Correctness & Compatibility:** Preserve data integrity and public contracts unless an approved change explicitly modifies them.
4. **Scoped Quality:** Apply production controls according to component risk. KISS and YAGNI remain valid; do not over-engineer narrow changes.
5. **Development and Testing:** Treat implementation and testing as one task. Before changing behavior, identify success, boundary, negative, regression, failure, state, security, and compatibility cases using `.agent/rules/development-workflow.md`.
   **Post-Implementation Test Gate:** Report either tests added/updated with concrete scenarios or a behavior-based reason no test change is needed, then run the applicable `./leedevkit test <target>`.
6. **Verification:** Discover and run the applicable project-configured checks. Do not claim full verification when a required check could not run.
7. **Safe Execution:** Use project wrappers when present. Redirect output for noisy, interactive, containerized, or long-running commands rather than every command universally.

## 🥇 Layer 1: REQUEST CLASSIFIER
Classify the request internally so the amount of discovery and verification matches its risk. Announce specialist context only when it materially helps the user follow the work.

## 🛑 Layer 2: SOCRATIC GATE
Ask only when missing information materially changes the result, requires new authority, or cannot be safely discovered from repository context. Routine fixes should proceed with explicit, low-risk assumptions.

## 📚 Layer 3: DOMAIN RULEBOOKS (Lazy Load)
Load only the rulebooks applicable to the task using the `Read` tool.

- **Coding Standards:** `.agent/rules/coding-standards.md`
- **AI Governance:** `.agent/rules/ai-agent-governance.md`
- **Testing Standards:** `.agent/rules/testing-standards.md`
- **Development Workflow:** `.agent/rules/development-workflow.md`
- **Secure Development Lifecycle:** `.agent/rules/secure-development-lifecycle.md`
- **Vulnerability Management:** `.agent/rules/vulnerability-management.md`
- **Architecture Governance:** `.agent/rules/architecture-governance.md`
- **API and Contract Governance:** `.agent/rules/api-governance.md`
- **Reliability Engineering:** `.agent/rules/reliability-engineering.md`
- **Migration and Rollback:** `.agent/rules/migration-and-rollback.md`
- **Release Management:** `.agent/rules/release-management.md`
- **Database:** `.agent/rules/database-rules.md`
- **UI/UX Design:** `.agent/rules/design-rules.md`
- **Access Control:** `.agent/rules/access-control.md`
- **Observability:** `.agent/rules/observability-rules.md`

Security, migration, architecture, and release rules are risk-triggered and do not apply mechanically to unrelated changes.

## 🤖 Layer 4: AGENTS & SKILLS (Lazy Load)
Read only the agents or skills relevant to the current task.

- **General Orchestration:** `.agent/agents/orchestrator.md`
- **Frontend / React / UI:** `.agent/agents/frontend-specialist.md`
- **Backend / API / Server:** `.agent/agents/backend-specialist.md`
- **API Contract Design:** `.agent/agents/api-designer.md`
- **Database / Schema:** `.agent/agents/database-architect.md`
- **DevOps / Infra:** `.agent/agents/devops-engineer.md`
- **Security / Audit:** `.agent/agents/security-auditor.md`
- **Testing & QA:** `.agent/agents/test-engineer.md`
- **Debugging:** `.agent/agents/debugger.md`
- **Performance:** `.agent/agents/performance-optimizer.md`
- **Mobile Development:** `.agent/agents/mobile-developer.md`
- **Game Development:** `.agent/agents/game-developer.md`

## 🤖 SUBAGENT MODEL SELECTION
Before delegating an ambiguous or non-trivial task, call `mcp__leedevkit-task-assessor__assess_task` with the task description, file count, and risk. Use its suggested tier unless explicit task context justifies an override.

## 🔧 LAYER 5: DEVKIT COMMANDS
| Command | Purpose |
|---|---|
| `leedevkit test infra --lint-only` | Quick lint/type check |
| `leedevkit test infra --unit-only` | Unit tests only |
| `leedevkit test infra` | Format check + lint + unit/coverage |
| `leedevkit test all` | Full suite (lint + unit + e2e) |
| `leedevkit test all --json` | Machine-readable output |
| `leedevkit manage up dev` | Start dev environment |
| `leedevkit manage db:setup` | Init database + migrations |
| `leedevkit doctor` | System health check |
| `leedevkit skills list` | Browse available skills |
| `leedevkit version` | Print installed version (no venv/network) |

### Release packaging gate (devkit itself)
When changing packaging, install, bootstrap, update, versioning, or release layout of LeeDevKit itself, source tests alone are NOT enough. Also run:

```bash
python3 scripts/_release_build.py --repo-root . --output /tmp/dist
python3 scripts/_release_acceptance.py \
  --repo-root . \
  --artifact /tmp/dist/leedevkit-$(tr -d '\n' < VERSION).tar.gz
```

This black-box gate builds the real artifact and verifies offline `version`/`help` plus local bootstrap. Do not declare packaging work complete if it fails.

## 📦 LAYER 7: INSTALLED SKILLS
> Community skills installed via `leedevkit skills install`. Each has its own
> SKILL.md with instructions. AI agents should scan this directory and lazy-load
> relevant skills for the task at hand.

**Installed skills directory:** `.leedevkit/skills.d/`

Before executing domain-specific tasks, check if any installed skill applies:
```bash
ls .leedevkit/skills.d/
```
If a skill directory exists for the task domain, read its `SKILL.md` first.
