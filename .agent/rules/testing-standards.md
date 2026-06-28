# 🧪 Testing Standards

> [!WARNING]
> **NEVER run `cargo test`, `npm test`, `npx playwright test`, or any test command directly.** 
> This project uses Podman-based test isolation. The ONLY way to run tests is through `./test.sh`.

## 1. Test Execution Commands
> [!IMPORTANT]
> **DO NOT use background execution (`&`) for tests.** The `./test.sh` script is already PTY-safe. You MUST run it directly in the foreground (e.g., `./test.sh <target> </dev/null`) so you can read the output and verify the results.

| Scope | Command | Purpose |
|---------------|---------|---------|
| **Frontend** | `./test.sh web </dev/null` | Lint, Unit (Vitest), E2E (Playwright) |
| **Backend** | `./test.sh api </dev/null` | Lint, Unit (Rust), Integration (SQLx) |
| **Full Suite** | `./test.sh all </dev/null` | Comprehensive verification |

## 2. Core Testing Principles
- **Testing Pyramid:** Prioritize Unit > Integration > E2E. 
- **Pattern:** Use AAA Pattern (Arrange-Act-Assert).
- **Code Coverage:** Minimum **80%** coverage for all new business logic.
- **Mandatory Pass:** A task is NOT complete until `./test.sh <target>` passes successfully.

## 3. Verification Pipeline
> 🔴 **Execution Priority:** Security → Lint → Schema → Tests → UX → SEO → Performance

**Mandatory Pipeline:**
1. Type Safety (`./scripts/_cargo.sh check` / `./scripts/_npm.sh run typecheck`)
2. Linting (`./scripts/_cargo.sh clippy` / `./scripts/_npm.sh run lint`)
3. Schema (`./scripts/_sqlx.sh migrate run`)
4. Tests (`./test.sh <target>`)
