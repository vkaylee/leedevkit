# Database and Persistence

> Scope: Changes to database schema, queries, transactions, or persistence behavior. Apply to technologies used by this project; examples are illustrative, not stack requirements.

## Tenant Isolation and Access

- Every query and operation involving tenant-owned data MUST enforce tenant scope through a trusted authorization context. Keep isolation intact across reads, writes, joins, background jobs, exports, caches, and indirect identifiers. Use database-enforced isolation when configured and appropriate; it complements rather than replaces authorization.
- Use parameterized queries or safe query builders. Grant application identities only required database privileges.

## Integrity and Performance

- Use transactions or equivalent atomicity for changes whose partial completion would violate data integrity or business invariants.
- Keep database connections out of slow external I/O where possible. Bound query work and avoid N+1 access patterns when they materially affect performance.
- Add indexes based on constraints and observed or expected query patterns; account for write and storage cost.
- Protect data from unintended loss. Destructive operations require explicit approval from the responsible human and an appropriate recovery plan.
- Choose deletion, archival, and retention behavior according to product, legal, and recovery requirements. Soft deletion is not universally required; where used, define purge and restore semantics.

## Schema Changes

- Follow `migration-and-rollback.md` for compatibility, data protection, approval, and recovery. Prefer reversible or expand-and-contract changes when deployment overlap or rollback needs it.
- A `down.sql` or equivalent rollback is useful only when it safely preserves data and restores a supported state. Do not require destructive rollback for every migration; document when roll-forward, restore, or reconciliation is safer.

## Repository Commands

- Discover and use applicable repository wrappers for database tools and project commands when they exist. Do not assume a wrapper, prohibit direct tool use categorically, or bypass a documented project execution environment.
