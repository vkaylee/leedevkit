# 🔄 Change Management Rules (SOC 2 CC8.1 / ISO 27001 A.8.32)

## 1. Code Review Policy (MANDATORY)
> 🔴 **No code reaches `main` without review. No exceptions.**

- **Minimum Reviewers:** Every pull request MUST receive at least **1 approved review** before merge.
- **Security-Sensitive Changes:** PRs touching authentication, authorization, encryption, PII handling, or financial calculations require **2 approved reviews**, at least one from a senior engineer.
- **Self-Merge Ban:** The PR author MUST NOT approve their own PR. Emergency hotfixes may use a fast-track process (see Section 4).
- **Review Scope:** Reviewers MUST verify: correctness, security implications, test coverage, backward compatibility, and adherence to coding standards.
- **Stale Reviews:** If the PR is updated after approval, the approval MUST be re-requested if the changes are substantive (not just formatting/comments).

## 2. Branch Protection Rules (MANDATORY)
| Branch | Direct Push | Required Reviews | Status Checks | Force Push |
|--------|-------------|-----------------|---------------|------------|
| `main` | ❌ Blocked | 1 (2 for security) | All CI checks pass | ❌ Blocked |
| `staging` | ❌ Blocked | 1 | All CI checks pass | ❌ Blocked |
| `develop` | ❌ Blocked | 1 | Lint + Type check | ❌ Blocked |
| Feature branches | ✅ Allowed | N/A | N/A | ✅ Allowed |

- **Branch Naming:** Feature branches MUST follow `<type>/<ticket-id>-<short-description>` (e.g., `feat/LA-123-behalf-leave-request`).
- **Branch Lifecycle:** Feature branches MUST be deleted after merge. Stale branches (> 30 days without commits) SHOULD be pruned.

## 3. Deployment Approval Gates (MANDATORY)
| Environment | Required Approvals | Pre-Deploy Checks | Rollback Plan |
|-------------|-------------------|--------------------|---------------|
| **Development** | 0 (auto-deploy on PR merge to `develop`) | Lint + Type check | Auto-revert on failure |
| **Staging** | 1 engineer | Full CI suite + E2E tests | Redeploy previous version |
| **Production** | 1 engineer + 1 PM/lead (for breaking changes) | Full CI + Staging verification + Smoke test | Rollback within 15 min |

### Deployment Checklist (Production)
```
- [ ] All CI checks pass (lint, type, unit, integration, E2E)
- [ ] Staging verified with no regressions
- [ ] Database migrations tested on staging
- [ ] Migration rollback (`down.sql`) verified
- [ ] Feature flags configured for gradual rollout (if applicable)
- [ ] Monitoring alerts reviewed and thresholds set
- [ ] Rollback plan documented and tested
- [ ] CHANGELOG.md updated
```

## 4. Emergency Hotfix Process
> For P1/P2 security incidents or critical production bugs ONLY.

| Step | Normal Process | Emergency Hotfix |
|------|---------------|-----------------|
| Review | 1-2 reviewers | 1 reviewer (any senior engineer) |
| Testing | Full CI suite | Targeted tests for the fix only |
| Staging | Full staging verification | Skip (direct to production) |
| Deploy | Scheduled window | Immediate |
| Post-deploy | Standard monitoring | Enhanced monitoring for 2 hours |
| Follow-up | None | Full review within 24 hours, post-mortem |

- **Emergency Access:** Emergency deploys MUST still be audit-logged with the deploying engineer, justification, and incident reference.
- **Retroactive Review:** Emergency PRs MUST receive a full review within 24 hours of deployment and any additional fixes applied.

## 5. Breaking Change Policy
> 🔴 **Breaking changes require advance communication and migration support.**

- **Definition:** A breaking change is any change that requires API consumers to modify their integration (removed/renamed endpoints, changed request/response schema, removed fields, behavior changes).
- **API Versioning:** When a breaking change is unavoidable, increment the API version (e.g., `/api/v1/` → `/api/v2/`). The old version MUST continue functioning for a deprecation period.
- **Deprecation Period:** Minimum **90 days** between deprecation announcement and removal. Shorter periods require PM approval and direct communication to affected users.
- **Deprecation Headers:** Deprecated endpoints MUST return a `Deprecation` header with the sunset date and a `Link` header pointing to the new version.
- **Database Migrations:** Schema changes that break backward compatibility (column removal, type change) MUST use a multi-step migration: add new → migrate data → remove old.

## 6. Feature Flag Governance
- **When Required:** Any user-facing feature with risk of regression or that needs gradual rollout MUST use a feature flag.
- **Naming Convention:** `ff_<feature_area>_<feature_name>` (e.g., `ff_leave_behalf_requests`).
- **Lifecycle:** Feature flags MUST be cleaned up within **30 days** of full rollout (100% enabled). Stale flags are tech debt.
- **Kill Switch:** Critical features MUST have a feature flag that can disable them without deployment for incident containment.
- **Audit:** Feature flag changes (enable/disable/percentage change) MUST be audit-logged.

## 7. Rulebook and Standard Changes (AI Proposals)
- AI agents and developers MUST NOT modify rulebooks or core standard files in `.agent/rules/` and `GEMINI.md` autonomously without direct, explicit user approval.
- If a technical debt resolution or refactoring reveals a gap or an outdated policy in the core rules:
  1. The AI agent MUST outline a recommended addition or modification in the final walkthrough report or pull request comments.
  2. The changes to `.agent/rules/` or `GEMINI.md` may only be merged and applied after Kỹ sư trưởng (Lead Engineer) reviews and approves the proposal.
