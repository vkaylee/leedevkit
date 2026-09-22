---
name: code-review-checklist
description: Code review guidelines covering code quality, security, and best practices.
allowed-tools: Read, Glob, Grep
---

# Code Review Checklist

## Reviewer Operating Rules
- Review read-only. Do not modify files, rewrite tests, or silently apply fixes.
- Review the requested behavior and its boundaries, not personal style preference.
- Cite every finding with an evidence location: `path:line`, test name, request/event ID, or exact log location.
- Separate observed facts from hypotheses. Mark unverified concerns as questions or follow-up checks.
- Review the diff plus relevant callers, tests, configuration, documentation, and dependency usage. Do not infer safety from the changed lines alone.

## Review Scope

### Spec-Compliance Verdict
Determine whether change meets explicit requirements and acceptance criteria.

- [ ] Requested behavior implemented end to end
- [ ] Inputs, outputs, errors, and compatibility match stated contract
- [ ] All named files/symbols/callers updated or intentionally unchanged
- [ ] Required tests, docs, configuration, and migrations included
- [ ] Scope constraints and non-goals respected

Verdict: **SPEC COMPLIANT**, **SPEC NON-COMPLIANT**, or **UNVERIFIED**.
Record evidence locations and blocking gaps.

### Quality Verdict
Assess maintainability and risk independently of spec compliance.

- [ ] Correctness and edge cases handled
- [ ] Error handling prevents data loss or unsafe partial state
- [ ] Security boundaries validated: injection, XSS/CSRF, authorization, secrets, unsafe deserialization, and sensitive logging
- [ ] Performance and resource use reasonable: queries, loops, caching, bundle/runtime cost, timeouts
- [ ] Naming, structure, abstraction level, and duplication support maintenance
- [ ] No verified dead code: unused imports/variables/private functions, unreachable branches, obsolete dependencies, or commented-out implementations
- [ ] Replaced code and related tests, configuration, documentation, and dependencies removed together
- [ ] Transitional code has reason, owner, removal condition, and review/removal date
- [ ] Indirect runtime and external usage checked before deletion; public APIs follow deprecation policy

Verdict: **QUALITY SOUND**, **QUALITY CONCERNS**, or **QUALITY BLOCKED**.
Record evidence locations, severity, and remediation.

## Behavioral Review

### Reasonable-User and Unnamed-Input Review
Exercise the feature as a reasonable user would, including inputs the specification does not name explicitly. Do not treat unnamed input as permission to invent requirements; check safe, unsurprising behavior and document assumptions.

- [ ] Typical valid input works
- [ ] Empty, missing, null-like, whitespace-only, and boundary values behave safely where applicable
- [ ] Malformed, unexpected, oversized, repeated, or out-of-order input fails safely or is handled predictably
- [ ] User-visible errors are actionable without exposing secrets or internals
- [ ] Authorization, tenant, locale, time-zone, and compatibility boundaries remain enforced where applicable
- [ ] Unnamed-input behavior is either reasonable and evidenced, or called out as an explicit question

Evidence: [path:line, test name, request/event ID, or log location]

## Testing Review
- [ ] Tests assert observable behavior and meaningful boundaries
- [ ] Bug fixes include regression coverage that fails without fix, or documented reproduction limitation
- [ ] Applicable negative, authorization, dependency-failure, timeout, partial-state, compatibility, and resource-limit cases covered
- [ ] Assertions verify absence of unauthorized or partial side effects where relevant
- [ ] No source-text, AST, implementation-name, or change-detector tests substitute for behavior assertions
- [ ] No assertions weakened; no tests skipped, retried, or quarantined without tracked, time-bound reason
- [ ] Test Impact Matrix dimensions considered; non-applicable dimensions have concrete reasons
- [ ] Project test suite result recorded with command and evidence location when required by project workflow

## Findings Format
For each finding, report:

```text
[BLOCKING|IMPORTANT|NIT|QUESTION] path:line
Observed: [specific fact]
Impact: [user, security, correctness, or maintenance consequence]
Evidence: [test/log/spec location]
Action: [required change or question]
```

Do not block on nits. Do block on security, data loss, broken contract, unsafe failure, or missing required behavior. End review with separate verdicts:

```text
Spec compliance: [SPEC COMPLIANT | SPEC NON-COMPLIANT | UNVERIFIED]
Quality: [QUALITY SOUND | QUALITY CONCERNS | QUALITY BLOCKED]
Blocking findings: [none or evidence-linked list]
Open questions: [none or evidence-linked list]
```

## Anti-Patterns to Flag
- Random or speculative behavior outside the request
- Silent scope expansion or unrelated cleanup
- Assumptions presented as evidence
- Source-text assertions that only detect implementation changes
- Deep nesting, long functions, magic numbers, untyped escape hatches, or verified dead paths
- Hardcoded secrets, unsanitized input, injection sinks, unsafe prompt/data boundaries, or sensitive output leakage
- Missing checks for empty, malformed, unauthorized, timeout, partial, or resource-limit cases

## Review Completion
- [ ] Findings cite evidence locations
- [ ] Spec-compliance and quality verdicts are separate
- [ ] Reviewer made no repository changes
- [ ] Reasonable-user unnamed-input behavior considered
- [ ] Blocking findings distinguish required fixes from questions and nits
