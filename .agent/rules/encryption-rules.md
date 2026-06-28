# 🔑 Encryption & Key Management Rules (SOC 2 CC6.7 / ISO 27001 A.8.24)

## 1. Encryption in Transit (MANDATORY)
> 🔴 **ALL data in transit MUST be encrypted. No exceptions.**

- **External Traffic:** TLS 1.2+ required for all client-facing connections. TLS 1.3 preferred. TLS 1.0/1.1 MUST be disabled.
- **Internal Traffic:** Service-to-service calls within the same network SHOULD use TLS. MUST use TLS if crossing network boundaries (VPCs, subnets).
- **Database Connections:** All database connections MUST use `sslmode=require` (or `verify-full` in production). Never connect to Postgres over plaintext.
- **Redis Connections:** Redis connections MUST use TLS in production if Redis is accessed over a network (not localhost/unix socket).
- **HSTS Header:** The web dashboard MUST serve the `Strict-Transport-Security` header with `max-age=31536000; includeSubDomains`.
- **Certificate Validation:** NEVER disable certificate validation in production code (`danger_accept_invalid_certs`, `NODE_TLS_REJECT_UNAUTHORIZED=0`). This is acceptable ONLY in local development with self-signed certs.

## 2. Encryption at Rest (MANDATORY for T1/T2 Data)
> 🔴 **Data classified as T1 (Restricted) or T2 (Confidential) per `data-governance.md` MUST be encrypted at rest.**

- **Database Level:** Use database-level encryption (e.g., PostgreSQL TDE, AWS RDS encryption, or volume-level encryption). This covers all data at the storage layer.
- **Application-Level Encryption:** For T1 fields (passwords, API keys, secrets), apply application-level encryption BEFORE storing in the database. This provides defense-in-depth even if the database is compromised.
- **Hashing (One-Way):** Passwords and tokens that only need verification MUST be hashed (not encrypted) using `argon2id`.
- **Encryption Algorithm:** Use AES-256-GCM for symmetric encryption. Use RSA-2048+ or Ed25519 for asymmetric operations.
- **File Storage:** Uploaded files (profile photos, documents) MUST be stored in an encrypted storage bucket (S3 SSE, GCS CMEK).
- **Backups:** Database backups MUST be encrypted. An unencrypted backup of an encrypted database is a data breach waiting to happen.

## 3. Key Management Lifecycle (MANDATORY)
> 🔴 **Cryptographic keys are the crown jewels. Mismanaging them negates all encryption.**

| Key Type | Storage | Rotation Period | Access |
|----------|---------|----------------|--------|
| **JWT Signing Secret** | Environment variable (never in code/config files) | Every 90 days | API server only |
| **Database Encryption Key** | KMS (AWS KMS, GCP KMS, HashiCorp Vault) | Annually or after incident | Infrastructure team only |
| **API Encryption Keys** | KMS or secrets manager | Every 180 days | Service account only |
| **Backup Encryption Key** | Separate from primary keys, offline copy | Annually | Infrastructure team only |
| **Webhook HMAC Secrets** | Per-workspace, stored hashed in DB | On customer request or annually | API server only |

### Key Rotation Rules
- **Grace Period:** When rotating keys, the OLD key MUST remain valid for a transition period (24 hours for JWT, 7 days for API keys) to prevent service disruption.
- **Rotation Automation:** Key rotation SHOULD be automated where possible. Manual rotation MUST be documented as a runbook.
- **Post-Rotation Verification:** After rotation, verify that all services are using the new key and the old key is no longer in use before removing it.

## 4. Secrets Management (MANDATORY)
- **Never Hardcode:** Secrets MUST NEVER appear in source code, configuration files, or Docker images. Use environment variables or a secrets manager.
- **`.env` Files:** `.env` files MUST be in `.gitignore`. The `.env.example` file MUST contain placeholder values only (e.g., `JWT_SECRET=change-me-in-production`).
- **CI/CD Secrets:** Pipeline secrets MUST be stored in the CI/CD platform's secrets manager (GitHub Secrets, GitLab CI Variables). Never echo secrets in build logs.
- **Logging:** NEVER log secret values. Log only that a secret was loaded (e.g., `"JWT secret loaded from environment"`), not its content.
- **Secret Detection:** Use pre-commit hooks (e.g., `gitleaks`, `detect-secrets`) to prevent accidental secret commits. Configure in CI pipeline as a blocking check.

## 5. Security Headers (MANDATORY for Web Dashboard)
The web application MUST serve these security headers:

| Header | Value | Purpose |
|--------|-------|---------|
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` | Force HTTPS |
| `X-Content-Type-Options` | `nosniff` | Prevent MIME sniffing |
| `X-Frame-Options` | `DENY` | Prevent clickjacking |
| `Content-Security-Policy` | Restrictive policy (no `unsafe-inline` for scripts) | Prevent XSS |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Limit referrer leakage |
| `Permissions-Policy` | Restrict unused features (camera, mic, geolocation) | Minimize attack surface |
| `X-XSS-Protection` | `0` | Deprecated but set to 0 to prevent IE filter issues |
