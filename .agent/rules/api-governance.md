# API and Contract Governance

> Scope: HTTP, GraphQL, gRPC, RPC, webhook, event, message, and library contracts consumed outside their owning module.

## 1. Contract-First Design (REQUIRED)

- Define request, response, error, authentication, authorization, and compatibility behavior before or with implementation.
- Use machine-readable schemas when the protocol supports them.
- Validate inputs and outputs at trust boundaries.
- Document ownership, audience, lifecycle status, and support expectations.

## 2. Compatibility (BLOCKING)

- Changes MUST remain backward compatible unless the breaking-change process is approved.
- Adding a required field, removing or renaming a field, narrowing accepted input, changing semantics, or altering error behavior may be breaking even when the transport schema still validates.
- Consumers MUST tolerate additive fields where the protocol permits evolution.
- Deprecated contracts require a replacement path, owner, usage measurement, sunset date, and communication plan.

## 3. Collection and Query Behavior

Potentially unbounded collections MUST use pagination, streaming, or a documented safe limit. Define stable ordering, cursor semantics, maximum page size, filtering behavior, and consistency expectations.

Avoid exposing unrestricted query complexity. GraphQL depth, cost, and result size MUST be bounded for untrusted consumers.

## 4. Reliability Semantics

- Define timeouts and cancellation propagation.
- Mutating operations exposed to retries SHOULD support idempotency keys or equivalent deduplication.
- Retryable errors MUST be distinguishable from permanent errors.
- Webhooks and asynchronous messages MUST define signature verification, replay protection, delivery semantics, retry policy, and dead-letter handling.

## 5. Security and Privacy

- Enforce authentication and authorization at the receiving boundary.
- Do not expose internal identifiers, sensitive fields, stack traces, or authorization details unnecessarily.
- Apply abuse controls according to exposure and risk.
- Record security-relevant actions without logging secret or sensitive payload content.

## 6. Verification (REQUIRED)

- Schema validation and linting
- Consumer or provider contract tests for shared contracts
- Authorization and negative-path tests
- Compatibility comparison against the released contract
- Idempotency, pagination, timeout, and error-shape tests when applicable

Published documentation and generated clients MUST be updated with the contract.
