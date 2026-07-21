---
name: lint-and-validate
description: Automatic quality control, linting, and static analysis procedures. Use after every code modification to ensure syntax correctness and project standards. Triggers onKeywords: lint, format, check, validate, types, static analysis.
allowed-tools: Read, Glob, Grep, Bash
---

# Lint and Validate Skill

> **REQUIRED:** Run the applicable repository-configured validation after code changes. Documentation-only changes require structural checks, not unrelated language toolchains.

### Procedures by Ecosystem

#### Node.js / TypeScript
1. **Discover:** Prefer a project wrapper or configured package script.
2. **Lint:** Run lint without semantic auto-fixes first. Safe formatting may be applied to files in scope.
3. **Types:** Run the configured type-check command.
4. **Security:** Run dependency auditing when dependencies or release inputs changed.

#### Python
1. **Linter:** Use the configured Ruff command or project wrapper; inspect findings before applying semantic fixes.
2. **Security (Bandit):** `bandit -r "path" -ll`
3. **Types (MyPy):** `mypy "path"`

## The Quality Loop
1. **Write/Edit Code**
2. **Run Audit:** Use the project-configured lint and type targets.
3. **Analyze Report:** Check the "FINAL AUDIT REPORT" section.
4. **Fix & Repeat:** Submitting code with "FINAL AUDIT" failures is NOT allowed.

## Error Handling
- If `lint` fails: Fix the style or syntax issues immediately.
- If `tsc` fails: Correct type mismatches before proceeding.
- If no tool is configured: inspect project configuration and CI. Report the missing validation capability; do not introduce a new toolchain unless requested or necessary to the implementation.

---
**Strict Rule:** Do not report code as fully verified when an applicable configured check failed or could not run. State unverified checks and their reasons.

---

## Scripts

| Script | Purpose | Command |
|--------|---------|---------|
| `.agent/skills/lint-and-validate/scripts/lint_runner.py` | Unified lint check | Run through the documented project Python environment |
| `.agent/skills/lint-and-validate/scripts/type_coverage.py` | Type coverage analysis | Run through the documented project Python environment |
