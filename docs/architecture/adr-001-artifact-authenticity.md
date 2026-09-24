# ADR-001: Artifact authenticity and provenance

## Status
Accepted

## Context
LeeDevKit release artifacts contain `devkit.manifest.json`, which records SHA-256 hashes. Hashes detect changes after download, but a publisher signature or trusted digest distribution is not available. Treating this manifest as authentication would give consumers a false origin guarantee.

## Decision
Keep the in-artifact SHA-256 manifest as tamper detection only. Release CI emits a deterministic CycloneDX SBOM and retains dependency/license/container evidence. CI records unavailable scanners explicitly instead of claiming a clean scan. Release provenance remains visible as unsigned/unverified until a trust anchor is approved.

Do not add a signing key, embedded public key, remote digest endpoint, or automatic consumer-project repinning in this change.

## Rationale
- Hashes provide useful post-download integrity detection without inventing trust material.
- Deterministic SBOM and retained evidence expose dependency and provenance state.
- A future signature design needs an independently distributed trust anchor and key-rotation/revocation process.

## Consequences
- Positive: artifact contents are hash-verifiable; dependency inventory travels with release; CI status cannot hide unavailable tools.
- Negative: consumers cannot establish publisher authenticity from the artifact alone.
- Mitigation: publish CI evidence and checksum files; approve trusted signing/digest distribution before relying on it.

## Revisit trigger
Adopt signing only after an owner approves key custody, independent trust-anchor distribution, rotation, revocation, and offline verification procedures.
