# Data Governance and Privacy

> Scope: Changes that collect, store, transform, expose, transfer, retain, or delete data. Classifications, retention periods, residency, and individual-rights processes are project-owned decisions based on data sensitivity, purpose, risk, contracts, and applicable law. This guidance is not a compliance claim or legal advice.

## Data Handling

- Identify sensitive and personal data at relevant design boundaries. Use project classification levels where defined; do not treat illustrative categories below as a substitute for the project’s approved inventory or legal analysis.
- Collect and retain only data needed for documented purposes. Restrict access by role and purpose; authorize access server-side and preserve tenant isolation where applicable.
- Do not expose credentials, secrets, or unnecessary sensitive data in logs, errors, URLs, analytics, test data, or external responses. Mask or redact personal data when operational detail does not require it.
- Return sensitive fields only to authorized callers and only when needed. Apply access controls to searches, exports, bulk operations, and derived or cached data as well as ordinary reads.
- Audit sensitive access, exports, administrative changes, and deletions when warranted by risk or project obligations. Logs must not reproduce the protected data itself.

## Retention, Deletion, and Requests

- Define retention and disposal requirements for data categories based on purpose, recovery needs, and applicable legal or contractual obligations. Record approved periods and exceptions in the project’s existing data inventory or policy.
- Implement deletion, anonymization, archival, and backup expiry consistently with retention decisions and integrity constraints. Soft deletion is optional; when used, define access filtering, restoration, and final purge behavior.
- Establish processes for applicable access, correction, export, or erasure requests based on actual obligations. Verify requester authority, preserve required records, and avoid promising timelines not established by project policy or law.
- Protect cross-region transfers and subprocessors according to the project’s data-location and contractual requirements. Minimize transferred data and document material transfers where required.

## Inventory and Change

- Keep data ownership, sensitivity, purpose, access, and lifecycle information in the project’s designated inventory when one exists or when risk requires one.
- Reassess classification and handling when schema, use, audience, or data flows materially change.
