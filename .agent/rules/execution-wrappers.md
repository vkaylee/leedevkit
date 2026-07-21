# Execution Wrappers & Hermetic Execution (CRITICAL)

> **REQUIRED WHEN PRESENT:** Use project-provided wrappers for the tools they cover. First verify that the wrapper exists and applies to the current project and target.

## 🚫 The Problem: Host Environment Pollution & PTY Hangs
If you run `cargo test` or `npm install` directly on the host machine, it will:
1. Break the hermetic environment (using host tools instead of the correct container versions).
2. Fail to connect to `db` or `redis` because the network stack is isolated.
3. Cause PTY Hangs if the command generates too much output or blocks IO.

## ✅ The Solution: Orchestrator Wrapper Scripts
When present, use the documented wrapper scripts located in the `scripts/` directory. If a listed wrapper is absent, discover the configured command from project configuration or CI instead of inventing the path.

| Action / Tool | Avoid when isolation is configured | Conditional wrapper example |
|---------------|---------------------------|----------------------|
| **Rust / Cargo** | `cargo test`, `cargo build` | `./scripts/_cargo.sh test`, `./scripts/_cargo.sh build` |
| **Node / NPM** | `npm install`, `npm run dev` | `./scripts/_npm.sh install`, `./scripts/_npm.sh run dev` |
| **Diesel CLI** | `diesel setup`, `diesel migration run` | `./scripts/_diesel.sh setup`, `./scripts/_diesel.sh migration run` |
| **Python** | Host execution when the project requires isolation | `./scripts/_python.sh ...` when provided, otherwise the documented project environment |

## ⚠️ PTY Safety & Redirection (MANDATORY)
Redirect output when a command is noisy, long-running, containerized, or known to retain terminal descriptors. Short deterministic commands may run normally when the execution environment captures output safely.

**Rule:** `> [logfile] 2>&1 </dev/null`

**Examples when those wrappers exist:**
- `RUST_LOG=info ./scripts/_cargo.sh test > test.log 2>&1 </dev/null`
- `./scripts/_npm.sh run lint > lint.log 2>&1 </dev/null`
- `./scripts/_diesel.sh migration run > migration.log 2>&1 </dev/null`

After a redirected command, read the complete relevant log and report failures accurately. Do not hide exit codes or rely only on truncated output.
