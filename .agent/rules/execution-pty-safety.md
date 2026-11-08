# Command Execution

Use commands supported by the current harness and project. This rule does not guarantee that a specific terminal adapter accepts every shell form; follow that adapter’s documented restrictions. Do not assume PTY hangs or impose blanket bans on newlines, heredocs, inline interpreters, pipelines, or shell backgrounding based only on presumed hangs.

- Quote shell arguments, especially values containing whitespace or user-controlled data; preserve command exit status and report failures accurately.
- Prefer non-interactive options when available. Do not add redirection merely to hide output or avoid presumed hangs.
- Use harness-managed processes/services when available. Otherwise run commands in the foreground; use shell job control only when supported and needed, and retain a way to inspect, stop, and collect the process result.
- Use project wrappers when documented, present, and applicable to the target; discover the configured command instead of guessing a wrapper path. See `execution-wrappers.md`.

## Git messages

Use non-interactive Git commands unless the project documents an applicable wrapper. For multiline messages, pass a safely quoted argument or use a message file with `git commit -F <file>` when that fits the tool.
