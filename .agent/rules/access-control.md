# 🔐 Access Control & Authentication Rules (SOC 2 CC6.1-6.3 / ISO 27001 A.5.15-5.18)

## 1. Authorization Enforcement Pattern (MANDATORY)
> 🔴 **Every API endpoint that accesses or mutates data MUST enforce authorization BEFORE executing business logic.** No exceptions.

### Handler Authorization Flow
```
1. Extract authenticated identity (middleware — already exists)
2. Extract target workspace_id from path/token
3. Verify actor has required permission for this action (MANDATORY check)
4. Execute business logic ONLY after authorization passes
5. Log action to audit trail for state-changing operations
```

- **Default Deny:** If no explicit permission grant exists, the request MUST be denied with `AppError::Forbidden`.
- **Fail Closed:** Authorization errors (e.g., permission service unavailable) MUST result in denial, NOT bypass.
- **No Implicit Trust:** Internal service-to-service calls MUST still carry authentication context. Never assume a request is authorized because it comes from an internal service.

## 2. RBAC Coding Standards (MANDATORY)
- **Permission Granularity:** Permissions MUST be defined at the `resource:action` level (e.g., `employee:read`, `leave:approve`, `attendance:export`). Never use coarse-grained permissions like `admin:all`.
- **Role Hierarchy:** Roles aggregate permissions. A role MUST be a named collection of `resource:action` pairs. Roles can inherit from other roles (e.g., `hr_manager` inherits from `hr_staff`).
- **Permission Check Location:** Permission checks MUST happen in the **handler layer** (before calling services), NOT inside services or repositories. Services MUST be permission-agnostic to remain testable.
- **New Endpoint Checklist:** When creating a new API endpoint:
  1. Define the required permission in the permissions domain
  2. Add permission check in the handler
  3. Add the permission to relevant default roles
  4. Document the permission in OpenAPI spec
  5. Add test for unauthorized access (expect 403)

## 3. Authentication Rules
### JWT Token Management
- **Access Token Expiry:** Short-lived, maximum **15 minutes** for web, **60 minutes** for mobile.
- **Refresh Token Expiry:** Maximum **7 days** for web, **30 days** for mobile.
- **Token Payload:** MUST contain only: `sub` (user_id), `exp`, `iat`, `workspace_id`. NEVER include PII (email, name, role names) in JWT payload.
- **Token Revocation:** The system MUST support immediate token revocation (via Redis blocklist) for security incidents. Check blocklist on every request.
- **Token Rotation:** Refresh tokens MUST be rotated on use (one-time use). Reuse of a refresh token MUST revoke all tokens for that user (potential token theft).

### Password Policy
- **Minimum Length:** 8 characters (NIST SP 800-63B guideline).
- **Hashing:** Use `argon2id` with recommended parameters. NEVER use MD5, SHA-1, or plain bcrypt for new implementations.
- **Breach Check:** Passwords SHOULD be checked against known breach databases (e.g., HaveIBeenPwned API) during registration/change.
- **Rate Limiting:** Login attempts MUST be rate-limited to **5 attempts per 15 minutes per account**, with exponential backoff.

### Session Security
- **Concurrent Sessions:** Track active sessions. Allow users to view and revoke their own sessions.
- **Session Invalidation on Sensitive Actions:** Changing password or email MUST invalidate all other sessions for that user.
- **Idle Timeout:** Web sessions SHOULD timeout after **30 minutes** of inactivity.

## 4. Service-to-Service Authentication
- **Internal API Calls:** Use signed JWTs with a dedicated service identity (e.g., `sub: "service:agent-main"`). Never use shared secrets or API keys for internal calls.
- **Background Workers:** Workers MUST authenticate using a scoped service token, NOT a user token. The service token MUST have only the permissions needed for that worker's tasks.
- **Webhook Delivery:** Outgoing webhooks MUST include an HMAC signature header for payload verification by the receiver.

## 5. Multi-Factor Authentication (MFA)
- **Admin Accounts:** MFA SHOULD be enforced for all users with admin-level permissions.
- **Implementation:** Support TOTP (Time-based One-Time Password) as the primary MFA method.
- **Recovery Codes:** Generate 10 single-use recovery codes during MFA setup. Store hashed (bcrypt). Display ONLY once during setup.
- **MFA Bypass:** NEVER provide a "skip MFA" option in production. Recovery MUST go through recovery codes or admin account recovery.

## 6. API Key Management (for External Integrations)
- **Issuance:** API keys MUST be scoped to a specific workspace and permission set. Never issue workspace-agnostic keys.
- **Storage:** Store API keys as hashed values (SHA-256). Display the raw key ONLY once at creation.
- **Rotation:** API keys MUST support rotation with a grace period (old key valid for 24 hours after new key is generated).
- **Revocation:** API keys MUST be immediately revocable. Revocation MUST be audit-logged.
- **Rate Limiting:** API key requests MUST have separate (stricter) rate limits from interactive user requests.
