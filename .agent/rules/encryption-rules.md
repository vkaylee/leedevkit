# Encryption and Key Management

> Scope: Changes that handle sensitive data, credentials, encryption, cryptographic keys, or encrypted transport. Select controls for the data classification, threat model, deployment environment, and applicable obligations. Examples are not compliance claims or universal algorithm mandates.

## Transport and Stored Data

- Protect sensitive data and credentials in transit using supported, secure transport configurations. Do not disable peer or certificate validation in production.
- Protect stored sensitive data and backups using approved storage and cryptographic controls appropriate to the threat model. Apply access controls even when storage encryption is enabled.
- Hash passwords and other values that only require verification using a maintained password-hashing scheme; do not use reversible encryption for verifiers.
- Prefer established platform cryptography and key-management services. Custom cryptographic designs or algorithms require security review.
- Configure security headers and browser protections where applicable to web-facing applications; use policy compatible with actual application behavior.

## Key and Secret Lifecycle

- Never commit secrets or expose them in logs, command output, examples, or client-visible errors. Use the project’s approved secret-management and access-control mechanisms.
- Restrict keys and secrets to the identities and operations that need them. Separate duties and environments where appropriate.
- Define rotation, overlap, revocation, recovery, and incident procedures for keys based on their use and availability impact. Do not remove old key material until dependent data or signatures no longer require it.
- Protect backups and exports containing sensitive data, including key material, under the applicable data-handling policy. Avoid storing encryption keys with the data they protect.
- Use repository-configured secret scanning or other controls where available and appropriate; do not assume a particular tool or CI configuration.
