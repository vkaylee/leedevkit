# Access Control and Authentication

> Scope: Changes to authentication, authorization, identity, sessions, credentials, or access to protected resources. Select mechanisms and limits for the project’s threat model, platform, and applicable obligations; examples below are not architecture mandates.

## Authorization

- Enforce authorization at a trusted server-side boundary before protected data is returned or privileged actions occur. Put checks in the layer that can reliably enforce the invariant across all callers; do not rely only on UI checks or a single handler when other access paths exist.
- Deny by default. If identity, policy, tenant scope, or an authorization dependency cannot be verified, fail closed.
- Every resource access and mutation MUST be scoped to the authenticated actor’s authorized tenant/resource. Derive or validate tenant scope from trusted identity and policy, not an untrusted request field alone. Prevent cross-tenant reads, writes, exports, and indirect references.
- Internal calls and background work MUST use an authenticated, least-privilege identity or equivalent trusted context. Do not infer authorization from network location.
- Model permissions at a granularity appropriate to the product and threat model. Avoid blanket grants where narrower permissions are practical.
- For a new protected operation, cover permitted and denied paths, tenant boundaries, and relevant audit behavior with tests or other applicable verification.

## Authentication and Sessions

- Use established, maintained platform or project-approved authentication mechanisms. Do not design custom cryptography or token protocols without security review.
- Select credential strength, token/session lifetime, revocation, rotation, and rate limits according to threat model, client constraints, and applicable requirements. Document project decisions where they materially affect risk.
- Keep credentials and tokens out of logs, URLs, source, and client-visible errors. Store verifiers safely; use an appropriate modern password-hashing scheme for passwords.
- Invalidate or rotate sessions/credentials after compromise and on sensitive account changes where required by the chosen session model.
- Require stronger authentication, such as MFA, for privileged or sensitive actions when risk or project policy warrants it. Recovery must not become an authentication bypass.

## Service and Integration Credentials

- Scope service identities and API keys to the minimum required resources and actions. Store and transmit credentials using approved secret-management controls; support rotation and revocation appropriate to the integration.
- Verify signatures and protect against replay for signed callbacks or messages when the protocol and threat model require them.
- Audit security-relevant grants, revocations, privilege changes, and sensitive access without recording secrets or unnecessary personal data.
