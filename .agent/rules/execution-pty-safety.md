# ⚠️ PTY Compatibility & Execution — CRITICAL

> [!CAUTION]
> **CRITICAL PTY HANG WARNING:** 
> 1. NEVER pass strings containing literal newlines into terminal commands (like `git commit -m "multi\nline"` or heredocs). THIS WILL HANG THE AGENT FOREVER. Write multi-line content to a temp file (`/tmp/msg.txt`), then execute the command reading from that file (`-F /tmp/msg.txt` or `< /tmp/msg.txt`).
> 2. When running commands via terminal tools, child processes can inherit PTY file descriptors and prevent command completion detection.

## 1. The Hermetic Toolbox (REQUIRED WHEN CONFIGURED)
Use project-provided wrappers when they exist and apply to the selected target. Verify paths before execution; projects may use different languages and wrappers.

| Tool | Avoid when isolation is configured | Conditional wrapper example | Example target |
| :--- | :--- | :--- | :--- |
| Rust/Cargo | `cargo build` | `./scripts/_cargo.sh build` | `apiserver_test` |
| Node/NPM | `npm install` | `./scripts/_npm.sh install` | `web_test` |
| Diesel/DB | `diesel print-schema` | `./scripts/_diesel.sh print-schema` | `apiserver_test` |

## 2. Test Execution & Coverage (MANDATORY)
- **Running Tests:** NEVER run framework test commands directly on the host. ALWAYS use the project-local LeeDevKit wrapper.
  - Examples: `./leedevkit test api`, `./leedevkit test web`, `./leedevkit test all`
- **Checking Coverage:** Use the `--coverage` flag on the applicable target (e.g., `./leedevkit test api --coverage`).

## 3. Git Protocol
- **Git Protocol:** Use a project-provided Git wrapper when the repository documents one and the wrapper exists. Otherwise use non-interactive Git commands directly.
- **Multiline Strings:** Avoid embedding literal newlines in terminal arguments. Write the message with a file-editing tool to a temporary file, then use the configured Git command with `commit -F <file>`.

## 4. Background Execution & Services
- **BANNED:** Never use the bash background operator (`&`) for ANY command (tests, servers, builds). 
- **BANNED:** Never use `tail -f` or similar blocking pagers.
- *Note:* The AI's native terminal tool automatically handles long-running processes safely. You must run all commands normally without manual shell redirection or backgrounding.

## 5. Banned Inline Commands & Patterns
> **Do NOT use these via AI terminal tools.** They cause infinite PTY hangs due to unclosed quotes or waiting for stdin.
- Heredocs (`cat <<EOF` or `cat >> file <<EOF`)
- Pager commands (`git log`, `less`, `more`). Always use `--no-pager`.
- Piped commands holding PTY (`rg | awk`, `grep | sed`). Append `</dev/null`.

## 6. Safe Inline Code Runner
> **NEVER use `python -c`, `python3 -c`, `node -e`, `bash -c` for inline code.**
Use `run-inline` instead — writes code to a temp file, executes via `_safe_run.py`:
- `echo 'print(1)' | ./scripts/ai-tools/run-inline -l python`
- `./scripts/ai-tools/run-inline -l bash -c 'echo $HOME'`

## 7. Avoiding PTY Hangs on Scripts (MANDATORY)
> Containerized, interactive, or long-running child processes may retain PTY descriptors.
- **Redirect Risky Commands:** Redirect output for commands known to be noisy, interactive, containerized, or long-running:
  ✅ `> run.log 2>&1 </dev/null`
- Short deterministic commands may run normally when output capture is reliable.
- **NEVER** pipe commands together if they spawn containers or heavy child processes (e.g., `cargo check | grep` is ❌).
