---
name: systematic-debugging
description: 4-phase systematic debugging methodology with root cause analysis and evidence-based verification. Use when debugging complex issues.
allowed-tools: Read, Glob, Grep
---

# Systematic Debugging

## Purpose
Use a repeatable, evidence-led process. Separate what was observed from what is suspected, and do not claim a cause before evidence supports it.

## Evidence Rules
- Record each observation with an evidence reference: `path:line`, request/event ID, test name, or exact log location (timestamp plus logger/source).
- Label statements as **Observed**, **Hypothesis**, or **Conclusion**.
- Keep hypotheses falsifiable: state what result would support or disprove each one.
- Preserve exact error text, inputs, environment, and timestamps needed to reproduce the observation.

## 4-Phase Debugging Process

### Phase 1: Reproduce Before Fixing
Never change production code before attempting a reproduction. Capture expected and actual behavior, inputs, environment, reproduction rate, and evidence location.

```markdown
## Reproduction
- Steps:
  1. [Exact step]
  2. [Exact step]
  3. [Expected vs actual result]
- Input/config: [relevant values, redacted safely]
- Environment: [runtime, version, platform]
- Rate: [always/often/sometimes/rare]
- Evidence: [path:line, test name, or log timestamp/source]
```

If reproduction fails, document the attempt and narrow the environment or instrumentation before fixing. A fix without a pre-fix reproduction is unverified; use a documented limitation only when reproduction is genuinely impossible.

### Phase 2: Isolate
Reduce scope while preserving the failure. Change one variable at a time and record each result with its evidence reference.

```markdown
## Isolation Log
- Observed: [fact] — Evidence: [location]
- Hypothesis: [falsifiable explanation]
- Probe: [single change or measurement]
- Result: [fact] — Evidence: [location]
```

Ask:
- When did this start, and what changed?
- Does it occur across environments, inputs, and execution paths?
- What is the smallest failing case?
- Which component first shows incorrect state?

### Phase 3: Understand
Trace the failure to a root cause, not its visible symptom. Use stack traces, state transitions, dependency behavior, and focused experiments. Mark inferences until confirmed.

```markdown
## Root Cause Analysis
1. Observed: [fact] — Evidence: [location]
2. Hypothesis: [cause that predicts the observation]
3. Test: [probe and expected disproof/support]
4. Observed: [result] — Evidence: [location]
5. Conclusion: [root cause, scope, and why evidence supports it]
```

### Phase 4: Fix and Verify
Apply the smallest fix that addresses confirmed cause. Re-run the original reproduction first, then verify related behavior and regression coverage.

```markdown
## Fix Verification
- [ ] Original reproduction no longer fails — Evidence: [test/log location]
- [ ] Regression assertion fails without fix and passes with fix
- [ ] Related functionality still works — Evidence: [location]
- [ ] Relevant failure/edge cases checked — Evidence: [location]
- [ ] No new issue introduced — Evidence: [location]
```

## Working Checklist
- [ ] Reproduction attempted and pre-fix evidence captured
- [ ] Expected behavior stated
- [ ] Observed facts separated from hypotheses
- [ ] Every material claim has an evidence location
- [ ] Root cause confirmed, not inferred from symptom alone
- [ ] Fix verified against original reproduction
- [ ] Regression protection added or limitation documented

## Useful Evidence Sources
Use repository-native tools and commands: focused test runs, version-control history/diffs, source search, debugger traces, request captures, and application logs. Record command, input, output, and location; avoid relying on unrecorded console output or tool-specific workflows.

## Anti-Patterns
- Random changes without a falsifiable hypothesis
- Fixing before attempting reproduction
- Treating assumptions as observations
- Citing logs or code without location
- Stopping when symptom disappears without confirming cause
- Declaring success without re-running original reproduction
