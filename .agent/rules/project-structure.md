# Project Structure and Commits

## 1. Source of Truth

Do not assume a fixed directory layout. Discover the current project structure from:

1. Project configuration such as `leedevkit.toml`, workspace manifests, and package files
2. Existing source directories and entry points
3. Build, test, and CI configuration
4. Project-specific rule overrides and architecture decision records

Examples in shared rulebooks MUST NOT override the repository's actual structure.

## 2. Structural Principles

- Keep entry points thin and delegate behavior to cohesive modules.
- Keep domain logic independent from transport, persistence, and framework details where practical.
- Depend on stable abstractions at boundaries that require substitution or isolated testing.
- Avoid circular dependencies and undocumented cross-layer access.
- Colocate tests with the convention already used by the project.
- Place temporary files outside the repository or in an ignored project-designated directory.

Project-specific layer constraints belong in a project rule override, not in this universal rulebook.

## 3. Commit Convention

Use the repository's configured commit convention. When none is documented, use Conventional Commits:

`feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:`

Commits SHOULD be cohesive and explain the observable reason for the change. Do not commit generated, temporary, secret, or unrelated files.

## 4. Architecture Decisions

Record significant, durable architectural decisions using the repository's existing ADR convention. If no convention exists and documentation is requested, prefer `docs/adr/` with a dated decision record. Do not create architecture documentation for routine local changes.

## 5. Dependency Direction

Before changing module boundaries:

- Inspect existing imports and callers.
- Preserve established dependency direction unless the task explicitly changes the architecture.
- Update all affected consumers in the same change.
- Add or update architecture checks when the repository already enforces layer boundaries.
