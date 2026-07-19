# ✨ PROJECT NAME — Enterprise Context OS

> 🔴 **MANDATORY:** Read this document in its entirety before taking any action.

{{PROJECT_SOUL}}

## 💎 Layer 0: CORE PRINCIPLES (The "Big Five")
> Universal rules that CANNOT be overridden by any external skill or lazy-loaded rule.

1. **Enterprise Premium Standard:** Every feature MUST be production-ready (Skeleton loaders, Empty States, i18n, Rate Limiting, UUID validation).
2. **Security & Privacy:** NEVER log PII (emails, passwords, phones) in raw text. Mask them.
3. **Structured Errors:** Use domain-specific error types. NEVER throw generic errors.
4. **Boy Scout Rule:** If you see a typo, bad pattern, or visible bug nearby, FIX IT IMMEDIATELY.
5. **Output Redirection (CRITICAL):** For ALL terminal commands, append `> logs/run_{slug}.log 2>&1 </dev/null`. Always prefix with `mkdir -p logs &&`.
6. **Post-Implementation Test Gate:** After EVERY code or behavior change, inspect affected tests and explicitly report either tests added/updated or a behavior-based reason no test change is needed. Run the applicable `./leedevkit test <target>` before declaring completion.

## 🥇 Layer 1: CRITICAL PROTOCOL
**Before writing ANY code or proposing solutions:**
1. **MANDATORY Lazy-Load:** Read related rulebooks using the `Read` tool.
2. **MANDATORY Rule Compliance:** For COMPLEX tasks, create `{task-slug}.md` listing at least 3 Rule IDs.

## 📥 Layer 2: REQUEST CLASSIFIER
Classify every request: QUESTION | SURVEY | SIMPLE CODE | COMPLEX CODE | DESIGN/UI.
Detect domain and announce: `🤖 Applying knowledge of @[agent-name]...`

## 🛑 Layer 3: SOCRATIC GATE
For New Features / Bug Fixes: 🔴 STOP and ASK minimum 3 strategic questions.

## 📚 Layer 4: DOMAIN RULEBOOKS (Lazy Load)
> 🔴 Before executing a task, read the relevant rules using the `Read` tool.

- **Coding Standards:** `@[.agent/rules/coding-standards.md]`
- **Testing Standards:** `@[.agent/rules/testing-standards.md]`
- **Database:** `@[.agent/rules/database-rules.md]`
- **UI/UX Design:** `@[.agent/rules/design-rules.md]`
- **Access Control:** `@[.agent/rules/access-control.md]`
- **Observability:** `@[.agent/rules/observability-rules.md]`

## 🤖 Layer 5: AGENTS & SKILLS
- **Frontend / React / UI:** `@[.agent/agents/frontend-specialist.md]`
- **Backend / Rust / API:** `@[.agent/agents/backend-specialist.md]`
- **Database / Postgres:** `@[.agent/agents/database-expert.md]`
- **DevOps / Infra:** `@[.agent/agents/devops-engineer.md]`
- **Security / Audit:** `@[.agent/agents/security-auditor.md]`

## 🔧 LAYER 6: DEVKIT COMMANDS
| Command | Purpose |
|---|---|
| `leedevkit test infra --lint-only` | Quick format + lint check |
| `leedevkit test infra --unit-only` | Unit tests only |
| `leedevkit test all` | Full suite (lint + unit + e2e) |
| `leedevkit test all --json` | Machine-readable output |
| `leedevkit manage up dev` | Start dev environment |
| `leedevkit manage db:setup` | Init database + migrations |
| `leedevkit doctor` | System health check |
| `leedevkit skills list` | Browse available skills |

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
