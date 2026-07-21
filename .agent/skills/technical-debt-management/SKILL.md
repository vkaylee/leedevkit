---
name: technical-debt-management
description: Systematically analyze, assess, and resolve technical debt. Use when the user requests refactoring, legacy-code optimization, or cleanup of obsolete code or architecture.
---

# Technical Debt Management Skill

## Purpose

This skill provides a safe, systematic method for identifying, quantifying, and resolving technical debt without breaking existing behavior. It keeps the codebase aligned with enterprise-grade standards.

## Core Principles

### 1. Classify Technical Debt

Before making changes, classify the debt being addressed:

- **Code Debt:** Complex or duplicated code, DRY violations, overly long functions, magic numbers, hardcoded user-facing strings, dead code, unused dependencies, and expired feature-flag branches or compatibility shims.
- **Architecture Debt:** Cross-component dependencies, tight coupling, SOLID violations, or missing interface and repository abstractions.
- **Testing Debt:** Test coverage below 80%, missing end-to-end tests, or flaky tests.
- **Documentation/UI Debt:** Documentation or comments that no longer match behavior, or UI built with raw elements instead of the approved design system.

### 2. Quantify and Prioritize

Not all debt must be resolved immediately. Prioritize it using this matrix:

- **High Impact / Low Effort — Resolve Now:** Extract small functions, remove verified dead code, move hardcoded user-facing text to i18n, and upgrade basic UI components.
- **High Impact / High Effort — Plan First:** Replace state management or change database schemas. A documented plan and user approval are required.
- **Low Impact — Track:** Record the debt in a TODO linked to a tracked issue with an owner and review date.

### 3. Safe Refactoring Process (MANDATORY)

Technical-debt work MUST follow these steps:

1. **Protect:** Do not refactor untested behavior. Confirm the relevant tests pass or add characterization tests before changing the implementation.
2. **Refactor:** Improve the structure without changing current business behavior.
3. **Verify:** Run the applicable type checker, linter, and test suite. Required checks MUST complete with zero errors and zero warnings.

### 3.1. Safe Dead Code Removal

1. **Prove:** Check callers and indirect execution mechanisms such as reflection, framework conventions, serialization, macros, plugins, CLI entry points, and external consumers. Static references alone are not sufficient evidence.
2. **Remove Completely:** Delete verified dead code together with tests, configuration, documentation, and dependencies used only by that code. Do not preserve old implementations as comments.
3. **Preserve Compatibility:** Public APIs, migrations, and compatibility shims MUST follow the deprecation process. Transitional code requires a documented reason, owner, removal condition, and review/removal date.
4. **Verify:** Run the relevant lint, type, and test gates. Cleanup MUST preserve current behavior except for code proven to be unreachable or unused.

### 4. Enterprise-Grade Enforcement

Technical-debt cleanup MUST move the codebase toward the applicable project standards:

- Updated UI components MUST follow the approved design system, accessibility rules, and error-boundary requirements.
- Refactored APIs MUST remain type-safe and must not introduce N+1 queries.
- Cleanup MUST comply with `.agent/rules/coding-standards.md`, `.agent/rules/testing-standards.md`, and the relevant domain rulebooks.

## User Communication

When this skill is activated, briefly summarize the findings and intended safe-refactoring scope. Example:

> Found [X] technical-debt issues across [categories]. Proceeding with protected, behavior-preserving cleanup and verification.
