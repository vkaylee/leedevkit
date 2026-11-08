# API and Contract Governance

> Scope: HTTP, GraphQL, RPC, library, event, message, webhook, or other contracts consumed outside their owning module. Apply where a change crosses a real consumer boundary.

## Contracts and Compatibility

- Define externally observable inputs, outputs, errors, authentication, authorization, and compatibility behavior when introducing or changing a contract.
- Use machine-readable schemas and contract documentation when supported and useful to consumers.
- Preserve compatibility for supported consumers unless a breaking change follows the project’s approved process. Assess semantic and error-behavior changes, not only schema compatibility.
- Deprecation notices, migration support, versioning, and sunset dates should follow project policy and actual consumer impact; avoid promising unsupported timelines.
- Bound potentially unbounded collections with pagination, streaming, or a documented limit. Define ordering and consistency when consumers rely on them. Bound query complexity for exposed query languages where relevant.

## Reliability and Security

- Define timeout, cancellation, idempotency, and retry semantics where relevant to the contract. Do not retry non-idempotent operations without deduplication or equivalent protection.
- For webhooks and asynchronous messages, define verification, replay protection, delivery, retry, and poison-message behavior as appropriate to the protocol.
- Enforce authentication and authorization at a trusted receiving boundary. Validate untrusted data and avoid exposing secrets, sensitive data, internal diagnostics, or unnecessary identifiers.
- Apply abuse controls according to exposure and risk. Record security-relevant actions without logging secret or sensitive payload content.

## Verification

Choose schema validation, compatibility comparison, consumer/provider contract tests, authorization tests, and failure-path tests according to the changed contract and risk. Update published documentation and generated clients when affected.
