---
name: brainstorming
description: Targeted clarification and concise user communication for ambiguous or high-impact work.
allowed-tools: Read, Glob, Grep
---

# Brainstorming & Targeted Clarification

Clarify before implementing only when ambiguity changes architecture, trust boundaries, data safety, or user-visible outcomes. Proceed with low-risk defaults for routine tasks.

## Socratic Gate

Ask questions when:
- Requirements are missing and multiple valid architectures exist
- The change involves destructive actions, migrations, or security boundaries
- Public API or data contracts would change irreversibly

Proceed without blocking when:
- The task has an obvious standard implementation in the codebase
- Missing details can be safely inferred with a sensible default stated in the response

## Dynamic Questioning

When questions are necessary, focus on decisions and trade-offs rather than generic templates:
- State the decision point clearly
- Provide the recommended option with trade-offs
- Name the low-risk default that will be used if not specified

## Actionable Error Handling

When operations fail:
1. State the exact failure and cause
2. Propose concrete recovery options with trade-offs
3. Execute the safest non-destructive path or ask if irreversible

## Completion Reporting

- Summarize the concrete changes made
- Provide the exact verification command to test the result
- State the immediate next step
