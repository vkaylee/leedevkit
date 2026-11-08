# Supply Chain and Dependency Security

> Scope: Projects that use third-party dependencies, build artifacts, containers, or external services. Tooling, license criteria, review cadence, severity targets, and artifact requirements are project-owned controls; examples below do not establish compliance.

## Dependency Risk

- Know and review dependencies introduced or materially updated, including transitive dependencies where tooling supports it. Consider maintenance, origin, permissions, known vulnerabilities, and fit for purpose.
- Use repository-configured vulnerability and license checks where available. Select scanning tools, blocking thresholds, cadence, and remediation targets according to project risk and contractual or legal requirements.
- Track material findings with affected versions, exposure, owner, disposition, and due date. Prioritize active exploitation and reachable high-impact vulnerabilities. Document approved risk acceptance; a scanner result alone is not a risk decision.
- Keep lockfiles and dependency source/configuration changes reviewed where the ecosystem uses them.

## License and Provenance

- Assess license obligations and compatibility with the project’s distribution and use before adopting a dependency. Escalate uncertain, conflicting, or restrictive terms to the appropriate legal or project owner.
- Maintain project-specific license allow, review, or deny criteria only where they are approved and kept current. Do not treat a generic license list as legal advice or automatic permission to use software.
- Verify dependency and artifact origins using ecosystem and repository controls where supported. Preserve provenance, checksums, signatures, attestations, or an SBOM when required by project policy or useful to the release risk.

## Build and External Services

- Pin or otherwise control build inputs according to the project’s reproducibility and update model. Use trusted base images and scan deployment artifacts when containers are part of the system and applicable controls exist.
- Minimize data shared with external services. Assess data sensitivity, authentication, retention, availability impact, and contractual obligations before integration.
- Apply updates and rebuild artifacts at a cadence appropriate to exposure and support commitments; avoid fixed universal schedules.
