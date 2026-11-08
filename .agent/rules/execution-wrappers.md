# Execution Wrappers

Use a project-provided wrapper when it is documented, exists, and applies to the selected tool and target—for example, when it selects a required isolated environment. Verify the path and target before use. Otherwise use the project’s documented command or environment; do not invent wrapper paths or assume host execution is invalid.

| Tool | Conditional wrapper example |
| :--- | :--- |
| Rust/Cargo | `./scripts/_cargo.sh test` or `build`, if configured for this target |
| Node/NPM | `./scripts/_npm.sh install` or `run dev`, if configured for this target |
| Diesel CLI | `./scripts/_diesel.sh migration run`, if configured for this target |
| Python | `./scripts/_python.sh ...`, if configured; otherwise use the documented project environment |

When wrapper configuration is unclear, inspect project documentation or CI before choosing a command.
