# 🐘 Database Rules (Postgres / Diesel)

## 1. Multi-Tenancy Isolation (MANDATORY)
- **Tenant Isolation:** Every `SELECT`, `UPDATE`, and `DELETE` query targeting tenant data MUST explicitly include `.filter(workspace_id.eq(...))`. There are NO exceptions to this rule. Cross-tenant data leakage is a critical security vulnerability.

## 2. Performance & Zero N+1
- **Zero N+1 Queries:** NEVER execute a database query inside a loop (`for`, `while`, `map`, etc.). You MUST use bulk operations, `.filter(id.eq_any(...))` (`IN` clauses), or SQL JOINs to fetch data in a single roundtrip.
- **Connection Pool Exhaustion:** NEVER hold a database connection (`&mut PgConnection` or pool guard) while waiting for external slow I/O (e.g., HTTP requests to 3rd parties, sending emails). You must release the connection back to the pool *before* the `await` point to prevent connection starvation and deadlocks.

## 3. Transactions & Data Integrity
- **Transactional Boundaries:** Any business operation that mutates (Inserts/Updates/Deletes) more than one table or modifies related records MUST be wrapped inside a database transaction (`conn.transaction(|conn| { ... })`).
- **Soft Deletes:** NEVER use hard `DELETE` for critical business entities (e.g., Users, Employees, Payroll, Policies). Use a `deleted_at` timestamp column and filter active records via `.filter(deleted_at.is_null())`.

## 4. Schema Migrations
- **Mandatory Rollbacks:** EVERY migration MUST include a working `down.sql` script for Disaster Recovery.
- **No Destructive Actions:** NEVER `DROP TABLE` or `DROP COLUMN` in production migrations without explicit human sign-off.
- **Indexing:** Always add indexes for foreign keys (especially `workspace_id`) and frequently filtered/sorted columns.

## 5. Execution & Hermetic Wrappers
- **No Direct Cargo/Diesel Calls:** NEVER run `diesel` or `cargo` directly in the terminal to avoid PTY hangs and environment issues.
- **Hermetic CLI:** ALWAYS use the project's hermetic wrapper scripts (e.g., `./scripts/_cargo.sh test`) to ensure consistent execution environments.
