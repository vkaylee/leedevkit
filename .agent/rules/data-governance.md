# 🔒 Data Governance & Privacy (SOC 2 CC6.5 / ISO 27001 A.5.12-5.13)

## 1. Data Classification Tiers (MANDATORY)
Every data field handled by the system MUST be classified into one of these tiers. When designing new tables, APIs, or UI components, explicitly document the tier of each field.

| Tier | Label | Examples | Storage Rules | Access Rules |
|------|-------|----------|---------------|--------------|
| **T1** | **Restricted** | Passwords, API keys, JWT secrets, SSN, bank accounts | Encrypted at rest (AES-256), never cached, never logged | Service-only access, zero human visibility in prod |
| **T2** | **Confidential** | Emails, phone numbers, salary, medical/leave reasons, home addresses | Encrypted at rest, masked in logs (`***@domain.com`), short retention | Role-gated access (HR Manager+), audit-logged on read |
| **T3** | **Internal** | Employee names, department, job title, attendance records, shift schedules | Standard DB storage, masked in external-facing logs | Workspace-scoped access, standard RBAC |
| **T4** | **Public** | Company name, workspace slug, published policy names | No special handling | Authenticated users within workspace |

## 2. PII Handling Rules (MANDATORY)
> 🔴 **NEVER violate these rules. PII breaches carry legal liability.**
- **Logging:** T1 data MUST NEVER appear in logs. T2 data MUST be masked (e.g., `user_***@domain.com`, `+84***789`). Use the `mask_pii()` utility for all log statements involving user data.
- **API Responses:** T1 data MUST NEVER be returned in API responses (e.g., never return password hashes). T2 data is returned ONLY to authorized roles, NEVER in list/collection endpoints — only in detail views.
- **Search & Filters:** NEVER allow free-text search on T1 fields. T2 fields may be filtered by exact match only (no LIKE/fuzzy).
- **Exports (CSV/XLSX):** ALL exports containing T2+ data MUST be audit-logged with the exporting user, timestamp, and row count.
- **Error Messages:** NEVER include T1/T2 data in error messages returned to clients.

## 3. Data Retention Schedule (MANDATORY)
> 🔴 **Every entity type MUST have a defined retention period.** Data hoarding is a liability.

| Entity Type | Retention Period | Purge Method | Legal Basis |
|-------------|-----------------|--------------|-------------|
| **Audit Logs** | 3 years | Automated batch delete (`housekeeping` worker) | SOC 2 CC7.3, labor law compliance |
| **Attendance Records** | 5 years after employee termination | Soft-delete → hard purge after retention | Labor law record-keeping |
| **Leave Requests** | 5 years after employee termination | Same as attendance | Labor law |
| **Employee PII** | 1 year after termination (or upon erasure request) | Anonymize → purge | GDPR Art. 17 / PDPA |
| **Session Tokens / JWTs** | Auto-expire (configurable, default 24h) | Redis TTL auto-cleanup | Security best practice |
| **Failed Login Attempts** | 90 days | Automated cleanup | Security monitoring |
| **Temporary Files / Uploads** | 7 days after processing | Cron job cleanup | Storage hygiene |

- **Implementation:** The `housekeeping` background worker MUST include retention-based cleanup jobs for each entity type above.
- **Soft Delete Grace Period:** Soft-deleted records MUST be retained for **30 days** before hard purge to allow recovery from accidental deletions.

## 4. Right to Erasure (GDPR Art. 17 / PDPA)
- **Scope:** When an employee or user requests data erasure, the system MUST anonymize or delete ALL T1 and T2 data associated with that individual.
- **Anonymization Pattern:** Replace PII fields with deterministic hashes or generic placeholders (e.g., `DELETED_USER_<hash>`, `deleted@example.com`). Do NOT use NULL — maintain referential integrity.
- **Retained Data:** Aggregated/anonymized records (e.g., attendance summaries, headcount stats) MAY be retained as they no longer constitute personal data.
- **Audit Trail:** The erasure request itself and its completion MUST be audit-logged (without including the erased PII).
- **Timeline:** Erasure MUST be completed within **30 calendar days** of the request.

## 5. Cross-Border Data Transfer
- **Default:** All data MUST be stored in the region specified by the workspace's configuration.
- **Transfers:** If data must cross borders (e.g., multi-region deployment), ensure compliance with applicable data transfer mechanisms (Standard Contractual Clauses, adequacy decisions).
- **Logging:** Cross-border data transfers MUST be logged in the audit trail.

## 6. Data Inventory
- **Requirement:** Maintain a living document (`docs/data-inventory.md`) mapping every database table to its data classification tier, retention period, and owning service.
- **Update Trigger:** This inventory MUST be updated whenever a new migration adds columns containing T1/T2 data.
