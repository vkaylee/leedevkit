# Development Workflow

> Scope: Changes that can alter observable behavior, runtime/build behavior, data, or public contracts.

## Behavior-verification loop

1. State intended behavior and important existing contracts. Inspect relevant implementation and tests.
2. Assess only material risks for this change: regression, input boundaries, state and failure paths, security/authorization, compatibility, and operational effects as applicable. Keep the plan proportionate; explain why a material risk does not apply.
3. Make the smallest cohesive change. Tests should protect observable contracts and material risks identified above; follow `testing-standards.md` for test quality, execution, coverage, and reporting.
4. Verify changed behavior and affected contracts using `testing-standards.md` execution rules.
5. Review the resulting change and verification evidence; correct failures or report limits accurately.

For a bug fix, add a regression test that demonstrates the defect and fails without the fix. If reproduction is not feasible, state why and use the strongest available alternative verification.

Documentation-only, comment-only, and formatting-only changes that cannot affect behavior do not need runtime tests. Use judgment for generated or configuration changes based on whether they can alter behavior.
