# Execution Wrappers & Hermetic Execution (CRITICAL)

> **MANDATORY:** AI Agents MUST use these wrapper scripts instead of raw commands (e.g., `cargo`, `npm`, `diesel`, `python`) to ensure hermetic execution within the Docker Orchestrator. 

## 🚫 The Problem: Host Environment Pollution & PTY Hangs
If you run `cargo test` or `npm install` directly on the host machine, it will:
1. Break the hermetic environment (using host tools instead of the correct container versions).
2. Fail to connect to `db` or `redis` because the network stack is isolated.
3. Cause PTY Hangs if the command generates too much output or blocks IO.

## ✅ The Solution: Orchestrator Wrapper Scripts
You MUST use the provided wrapper scripts located in the `scripts/` directory for ALL relevant commands. These scripts proxy your command into the correct Docker container.

| Action / Tool | ❌ INK CORRECT (DO NOT USE) | ✅ CORRECT (MUST USE) |
|---------------|---------------------------|----------------------|
| **Rust / Cargo** | `cargo test`, `cargo build` | `./scripts/_cargo.sh test`, `./scripts/_cargo.sh build` |
| **Node / NPM** | `npm install`, `npm run dev` | `./scripts/_npm.sh install`, `./scripts/_npm.sh run dev` |
| **Diesel CLI** | `diesel setup`, `diesel migration run` | `./scripts/_diesel.sh setup`, `./scripts/_diesel.sh migration run` |
| **Python** | `python script.py`, `pip install` | `./scripts/_python.sh script.py`, `./scripts/_python.sh -m pip install` |

## ⚠️ PTY Safety & Redirection (MANDATORY)
Even when using wrapper scripts, you **MUST ALWAYS** redirect the output to a log file to prevent PTY hangs. 

**Rule:** `> [logfile] 2>&1 </dev/null`

**Examples:**
- `RUST_LOG=info ./scripts/_cargo.sh test > test.log 2>&1 </dev/null`
- `./scripts/_npm.sh run lint > lint.log 2>&1 </dev/null`
- `./scripts/_diesel.sh migration run > migration.log 2>&1 </dev/null`

After executing the command, use the `view_file` or `cat` tool to read the log file and report the results. NEVER run these wrapper scripts without redirection.
