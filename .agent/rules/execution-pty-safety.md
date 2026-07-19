# ⚠️ PTY Compatibility & Execution — CRITICAL

> [!CAUTION]
> **CRITICAL PTY HANG WARNING:** 
> 1. NEVER pass strings containing literal newlines into terminal commands (like `git commit -m "multi\nline"` or heredocs). THIS WILL HANG THE AGENT FOREVER. Write multi-line content to a temp file (`/tmp/msg.txt`), then execute the command reading from that file (`-F /tmp/msg.txt` or `< /tmp/msg.txt`).
> 2. When running commands via terminal tools, child processes can inherit PTY file descriptors and prevent command completion detection.

## 1. The Hermetic Toolbox (MANDATORY)
All standard development tools run inside isolated Podman containers. You MUST execute them via these wrapper scripts instead of using native commands on the host:

| Tool | ❌ WRONG (Direct on Host) | ✅ RIGHT (Inside Container) | Target Container |
| :--- | :--- | :--- | :--- |
| Rust/Cargo | `cargo build` | `./scripts/_cargo.sh build` | `apiserver_test` |
| Node/NPM | `npm install` | `./scripts/_npm.sh install` | `web_test` |
| Diesel/DB | `diesel print-schema` | `./scripts/_diesel.sh print-schema` | `apiserver_test` |

## 2. Test Execution & Coverage (MANDATORY)
- **Running Tests:** NEVER run framework test commands directly on the host. ALWAYS use the project-local LeeDevKit wrapper.
  - Examples: `./leedevkit test api`, `./leedevkit test web`, `./leedevkit test all`
- **Checking Coverage:** Use the `--coverage` flag on the applicable target (e.g., `./leedevkit test api --coverage`).

## 3. Git Protocol
- **Git Protocol:** NEVER run `git` directly. ALWAYS use the `./scripts/git.sh` wrapper.
- **Multiline Strings:** NEVER use multi-line strings in commit messages. Create `/tmp/msg.txt` using file-write tools, then run `./scripts/git.sh commit -F /tmp/msg.txt`.

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
> **CRITICAL:** When running ANY script or command that spawns child processes (`./leedevkit test`, `./scripts/_cargo.sh`, `npm run`, etc.) inside the AI terminal, the PTY may hang indefinitely waiting for child processes (like docker-compose or rustc) to close their file descriptors.
- **UNIVERSAL RULE - ALWAYS REDIRECT OUTPUT:** You MUST ALWAYS redirect script output to a file and read it via your native `view_file` tool, regardless of how fast you think it will run:
  ✅ `> run.log 2>&1 </dev/null`
- **NEVER** run commands normally without this redirection. Do not guess execution time!
- **NEVER** pipe commands together if they spawn containers or heavy child processes (e.g., `cargo check | grep` is ❌).
