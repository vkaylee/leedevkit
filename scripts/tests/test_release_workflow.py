"""Static contract checks for automated GitHub release workflow."""

from pathlib import Path


WORKFLOW = Path(__file__).resolve().parents[2] / ".github" / "workflows" / "release.yml"


def test_release_workflow_publishes_only_after_acceptance() -> None:
    text = WORKFLOW.read_text()

    assert "tags:" in text
    assert '"v*.*.*"' in text
    assert "contents: write" in text
    assert "_release_acceptance.py" in text
    assert "gh release create" in text
    assert "release-artifact.sha256" in text
    assert "release-sbom.cdx.json" in text
    assert "if: always()" in text
    assert text.index("_release_acceptance.py") < text.index("gh release create")


def test_release_workflow_rejects_existing_release() -> None:
    text = WORKFLOW.read_text()

    assert 'gh release view "$RELEASE_TAG"' in text
    assert "GitHub Release already exists" in text
    assert "--verify-tag" in text
