# 📦 Supply Chain & Dependency Security (SOC 2 CC9.2 / ISO 27001 A.5.19-5.23)

## 1. Dependency Audit Policy (MANDATORY)
> 🔴 **Every dependency is an attack surface. Unaudited dependencies are unaccepted risk.**

### Automated Scanning
| Tool | Scope | Frequency | Blocking |
|------|-------|-----------|----------|
| `cargo audit` | Rust crate vulnerabilities (RustSec) | Every CI run + weekly scheduled | ✅ CI blocks on CRITICAL/HIGH |
| `npm audit` | Node.js package vulnerabilities | Every CI run + weekly scheduled | ✅ CI blocks on CRITICAL/HIGH |
| `cargo deny` | License compliance + duplicate crates | Every CI run | ✅ CI blocks on violations |
| `trivy` / `grype` | Docker image CVEs | Every image build | ✅ CI blocks on CRITICAL |

- **Weekly Report:** Generate and review a dependency vulnerability report weekly. Fix CRITICAL within **24 hours**, HIGH within **7 days**.
- **Zero-Day Response:** When a zero-day CVE is published for a dependency in use, treat as P2 security incident (per `incident-response.md`).

## 2. Approved License Policy (MANDATORY)
> 🔴 **Using dependencies with incompatible licenses is a legal liability.**

| Status | Licenses | Action |
|--------|----------|--------|
| ✅ **Allowed** | MIT, Apache-2.0, BSD-2-Clause, BSD-3-Clause, ISC, MPL-2.0, Unlicense, Zlib | Use freely |
| ⚠️ **Review Required** | LGPL-2.1, LGPL-3.0, CC-BY-4.0 | Requires team lead approval |
| ❌ **Blocked** | GPL-2.0, GPL-3.0, AGPL-3.0, SSPL, proprietary | NEVER use without legal review |
| ❓ **Unknown/Custom** | Any custom or unlicensed | NEVER use. Must have explicit license. |

- **Enforcement:** Use `cargo deny` (Rust) and `license-checker` (npm) in CI to block PRs introducing disallowed licenses.
- **New Dependency Checklist:** Before adding any new dependency, verify: license compatibility, maintenance status, download count, known vulnerabilities.

## 3. Docker Base Image Policy (MANDATORY)
- **Official Images Only:** Use ONLY official or verified publisher images from Docker Hub / container registry.
- **Pin Versions:** ALWAYS pin exact versions or SHA digests. NEVER use `:latest` tag in production Dockerfiles.
  - ✅ `FROM rust:1.82-slim-bookworm@sha256:abc123...`
  - ❌ `FROM rust:latest`
- **Minimal Images:** Use `-slim` or `-alpine` variants to reduce attack surface. Production images MUST NOT contain build tools, compilers, or debugging utilities.
- **Scanning:** ALL Docker images MUST be scanned for vulnerabilities before deployment (see Section 1).
- **Rebuild Frequency:** Production images MUST be rebuilt at least **monthly** to pick up base image security patches, even if application code hasn't changed.

## 4. Third-Party Service Integration Security
- **Security Review:** Before integrating any third-party API or SaaS service, document:
  1. What data is being sent to the service?
  2. What data classification tier? (per `data-governance.md`)
  3. Is the service SOC 2 / ISO 27001 certified?
  4. What happens if the service is unavailable? (circuit breaker plan)
  5. How is authentication handled? (API keys, OAuth)
- **Data Minimization:** Send ONLY the minimum data required for the integration. Never send full user records when only an ID is needed.
- **Egress Monitoring:** Monitor outbound API calls to detect unexpected data exfiltration.

## 5. Dependency Update Policy
| Priority | Condition | Timeline |
|----------|-----------|----------|
| 🔴 Critical | Known exploited CVE | **24 hours** |
| 🟠 High | CVE with public exploit but no known exploitation | **7 days** |
| 🟡 Medium | CVE with no public exploit | **30 days** |
| 🟢 Low | Non-security update (features, performance) | **Next release cycle** |

- **Automated Updates:** Use Dependabot or Renovate to automate dependency update PRs.
- **Lock Files:** `Cargo.lock` and `package-lock.json` (or `pnpm-lock.yaml`) MUST be committed and reviewed in PRs.
- **Peer Review:** Dependency update PRs MUST be reviewed with attention to: changelog for breaking changes, new transitive dependencies introduced, license changes.

## 6. Software Bill of Materials (SBOM)
- **Generation:** Generate an SBOM (CycloneDX or SPDX format) for every production release.
- **Storage:** SBOMs MUST be stored alongside release artifacts.
- **Purpose:** Enables rapid identification of affected systems when a new CVE is published for any dependency.
