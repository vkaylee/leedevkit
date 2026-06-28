# ⚙️ Configuration Management Rules (ISO 27001 A.8.9)

## 1. Environment Variable Standards (MANDATORY)
### Naming Convention
> 🔴 **ALL environment variables MUST follow a consistent naming scheme.**

| Pattern | Scope | Examples |
|---------|-------|---------|
| `APP_<SETTING>` | Application-level settings | `APP_PORT`, `APP_ENV`, `APP_LOG_LEVEL` |
| `DB_<SETTING>` | Database connection settings | `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_PASSWORD` |
| `REDIS_<SETTING>` | Redis connection settings | `REDIS_URL`, `REDIS_PASSWORD` |
| `JWT_<SETTING>` | Authentication secrets | `JWT_SECRET`, `JWT_EXPIRY_SECONDS` |
| `S3_<SETTING>` | Object storage settings | `S3_BUCKET`, `S3_REGION`, `S3_ACCESS_KEY` |
| `SMTP_<SETTING>` | Email service settings | `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME` |
| `FF_<FEATURE>` | Feature flags | `FF_BEHALF_LEAVE_REQUESTS`, `FF_MFA_ENABLED` |

- **Boolean Values:** Use `true`/`false` (lowercase). Never `0`/`1`, `yes`/`no`.
- **Duration Values:** Include unit suffix: `JWT_EXPIRY_SECONDS=3600`, `RATE_LIMIT_WINDOW_SECS=60`.
- **List Values:** Use comma-separated: `ALLOWED_ORIGINS=https://app.leeattend.com,https://staging.leeattend.com`.

## 2. Fail-Fast Configuration Validation (MANDATORY)
> 🔴 **The application MUST fail immediately on startup if required configuration is missing or invalid.** Never silently use default values for critical settings.

### Startup Validation Rules
```rust
// ✅ CORRECT: Fail-fast with clear error message
let jwt_secret = std::env::var("JWT_SECRET")
    .expect("FATAL: JWT_SECRET environment variable is required");

// ❌ WRONG: Silent default — security risk
let jwt_secret = std::env::var("JWT_SECRET")
    .unwrap_or("default-secret".to_string());
```

| Config Type | Fail-Fast? | Fallback Allowed? |
|-------------|-----------|-------------------|
| Security secrets (JWT, DB passwords) | ✅ MUST fail | ❌ NEVER |
| Service URLs (DB host, Redis URL) | ✅ MUST fail | ❌ NEVER |
| Feature flags | ⚠️ Warn | ✅ Default to `false` (disabled) |
| Performance tuning (pool size, timeouts) | ⚠️ Warn | ✅ Sensible defaults allowed |
| Optional integrations (SMTP, S3) | ⚠️ Warn | ✅ Graceful degradation |

- **Validation Scope:** On startup, validate format (URLs are valid, ports are numeric, UUIDs parse correctly), not just presence.
- **Startup Log:** On successful startup, log all loaded configuration keys (NOT values) at INFO level to confirm what was loaded.

## 3. `.env.example` Maintenance (MANDATORY)
- **Completeness:** `.env.example` MUST contain EVERY environment variable the application reads, with a placeholder value and inline comment explaining its purpose.
- **Sync Check:** CI SHOULD verify that all `env::var("...")` calls in the codebase have a corresponding entry in `.env.example`.
- **Security:** `.env.example` MUST NEVER contain real secrets, even for development. Use obvious placeholder values like `change-me-in-production`.
- **Grouping:** Variables MUST be grouped by domain with section headers:

```bash
# === Application ===
APP_ENV=development
APP_PORT=3001

# === Database ===
DB_HOST=localhost
DB_PORT=5432
DB_NAME=leeattend_dev
DB_PASSWORD=change-me-in-production

# === Authentication ===
JWT_SECRET=change-me-in-production
JWT_EXPIRY_SECONDS=900

# === Feature Flags ===
FF_MFA_ENABLED=false
FF_BEHALF_LEAVE_REQUESTS=true
```

## 4. Environment Parity (MANDATORY)
> 🔴 **Differences between environments are a major source of production bugs.**

- **Shape Parity:** All environments (development, staging, production) MUST use the same set of environment variable keys. Staging and production MUST NOT have variables that don't exist in development (and vice versa).
- **Service Parity:** Development MUST use the same service types as production (PostgreSQL, not SQLite; Redis, not in-memory cache). Docker Compose achieves this.
- **Data Shape Parity:** Staging SHOULD contain anonymized data with the same schema and approximate scale as production for realistic testing.
- **Exception:** Debug-only variables (e.g., `RUST_LOG`, `REACT_APP_DEV_TOOLS`) are exempt from parity requirements.

## 5. Feature Flag Lifecycle (MANDATORY)
> References `change-management.md` Section 6 for governance. This section covers technical implementation.

- **Storage:** Feature flags MUST be stored in a centralized location (database or config service), NOT scattered across environment variables. Environment variables are acceptable only as an override for emergency kill switches.
- **API:** Feature flags MUST be accessible via an internal API endpoint (`GET /api/v1/feature-flags`) for the web dashboard to check client-side flags.
- **Caching:** Feature flags SHOULD be cached with a short TTL (60 seconds) to reduce database load while allowing near-real-time updates.
- **Audit:** Every feature flag state change MUST be audit-logged (per `change-management.md`).
- **Cleanup Tracking:** Maintain a list of active feature flags with their creation date and target removal date in `docs/feature-flags.md`.

## 6. Configuration Drift Detection
- **Infrastructure as Code:** ALL infrastructure configuration MUST be defined in version-controlled files (Docker Compose, Caddyfile, CI/CD pipelines). Manual configuration changes are NOT allowed.
- **Drift Detection:** If the system detects a runtime configuration that differs from the expected configuration (e.g., a manually added environment variable), log a WARNING.
- **Immutable Deployments:** Production containers MUST be immutable. Configuration is injected at startup, NEVER modified at runtime.
