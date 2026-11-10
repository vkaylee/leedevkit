"""Unit tests for native Compose image freshness metadata."""

from __future__ import annotations

import json
from pathlib import Path
from types import SimpleNamespace

from _image_cache import (
    FINGERPRINT_LABEL,
    build_fingerprint,
    fingerprints_match,
    image_build_lock,
    resolve_build_metadata,
    service_fingerprints_match,
)


def _config_result(config: dict[str, object]) -> SimpleNamespace:
    import yaml

    return SimpleNamespace(returncode=0, stdout=yaml.safe_dump(config))


def _service_config(context: Path, **build: object) -> dict[str, object]:
    return {"build": {"context": str(context), "dockerfile": "Dockerfile", **build}}


def test_context_and_dockerfile_changes_change_fingerprint(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    (tmp_path / "Dockerfile").write_text("FROM alpine\n")
    (tmp_path / "package.json").write_text("{}\n")
    service = _service_config(tmp_path)
    first = build_fingerprint("webdashboard", service)

    (tmp_path / "Dockerfile").write_text("FROM alpine:latest\n")
    assert build_fingerprint("webdashboard", service) != first

    second = build_fingerprint("webdashboard", service)
    (tmp_path / "package-lock.json").write_text("{}\n")
    assert build_fingerprint("webdashboard", service) != second


def test_service_name_does_not_change_shared_image_fingerprint(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    (tmp_path / "Dockerfile").write_text("FROM alpine\n")
    service = _service_config(tmp_path)
    assert build_fingerprint("apiserver", service) == build_fingerprint("go", service)


def test_effective_image_and_build_config_define_identity(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    (tmp_path / "Dockerfile").write_text("FROM alpine\n")
    base = _service_config(tmp_path, target="runtime", args={"MODE": "test"})
    same = dict(base)
    different_target = _service_config(
        tmp_path, target="builder", args={"MODE": "test"}
    )
    different_target["image"] = "localhost/shared"
    different_image = dict(base, image="localhost/other")
    assert build_fingerprint("one", base) == build_fingerprint("two", same)
    assert build_fingerprint("one", base) != build_fingerprint("two", different_target)
    assert build_fingerprint("one", base) != build_fingerprint("two", different_image)


def test_ignored_generated_files_do_not_change_fingerprint(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    (tmp_path / "Dockerfile").write_text("FROM alpine\n")
    (tmp_path / ".dockerignore").write_text("node_modules\n.test_logs\n.git\n")
    service = _service_config(tmp_path)
    first = build_fingerprint("webdashboard", service)
    (tmp_path / "node_modules").mkdir()
    (tmp_path / "node_modules" / "generated").write_text("x")
    (tmp_path / ".test_logs").mkdir()
    (tmp_path / ".test_logs" / "report").write_text("x")
    assert build_fingerprint("webdashboard", service) == first


def test_narrow_build_inputs_ignore_unrelated_context_changes(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    (tmp_path / "Dockerfile").write_text("FROM alpine\n")
    (tmp_path / ".dockerignore").write_text("ignored\n")
    service = _service_config(tmp_path)
    service["x-leedevkit-build-inputs"] = ["Dockerfile"]
    first = build_fingerprint("go", service)
    (tmp_path / "unrelated-source.go").write_text("package unrelated\n")
    assert build_fingerprint("go", service) == first
    (tmp_path / ".dockerignore").write_text("ignored\nother\n")
    assert build_fingerprint("go", service) != first
    second = build_fingerprint("go", service)
    (tmp_path / "Dockerfile").write_text("FROM busybox\n")
    assert build_fingerprint("go", service) != second


def test_build_target_and_args_change_fingerprint(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    (tmp_path / "Dockerfile").write_text("FROM alpine\n")
    base = _service_config(tmp_path, target="runtime", args={"NODE_ENV": "test"})
    target_changed = _service_config(
        tmp_path, target="builder", args={"NODE_ENV": "test"}
    )
    args_changed = _service_config(
        tmp_path, target="runtime", args={"NODE_ENV": "production"}
    )
    assert build_fingerprint("web", base) != build_fingerprint("web", target_changed)
    assert build_fingerprint("web", base) != build_fingerprint("web", args_changed)


def test_missing_or_malformed_label_rebuilds(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    (tmp_path / "Dockerfile").write_text("FROM alpine\n")
    service = _service_config(tmp_path)
    config = {"services": {"webdashboard": service}}
    calls: list[list[str]] = []

    def run(command, **kwargs):
        calls.append(command)
        if command[-1] == "config":
            return _config_result(config)
        return SimpleNamespace(returncode=0, stdout="not-json")

    metadata = resolve_build_metadata(["podman-compose"], [], ["webdashboard"], run)
    assert metadata.fingerprints
    assert not fingerprints_match(metadata, "podman", run)


def test_matching_label_reuses_image(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    (tmp_path / "Dockerfile").write_text("FROM alpine\n")
    config = {"services": {"webdashboard": _service_config(tmp_path)}}
    metadata_holder = {}

    def run(command, **kwargs):
        if command[-1] == "config":
            result = _config_result(config)
            metadata_holder["metadata"] = resolve_build_metadata(
                ["podman-compose"], [], ["webdashboard"], lambda *_a, **_k: result
            )
            return result
        labels = {
            FINGERPRINT_LABEL: metadata_holder["metadata"].fingerprints["webdashboard"]
        }
        return SimpleNamespace(returncode=0, stdout=json.dumps(labels))

    metadata = resolve_build_metadata(["podman-compose"], [], ["webdashboard"], run)
    assert fingerprints_match(metadata, "podman", run)


def test_shared_image_checks_once_and_reuses_fingerprint(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    (tmp_path / "Dockerfile").write_text("FROM alpine\n")
    shared = dict(_service_config(tmp_path), image="localhost/shared")
    config = {"services": {"apiserver": shared, "go": dict(shared)}}
    metadata = resolve_build_metadata(
        ["podman-compose"],
        [],
        ["apiserver", "go"],
        lambda command, **kwargs: _config_result(config),
    )
    inspected: list[str] = []

    def run(command, **kwargs):
        inspected.append(command[-1])
        return SimpleNamespace(
            returncode=0,
            stdout=json.dumps({FINGERPRINT_LABEL: metadata.fingerprints["go"]}),
        )

    assert metadata.fingerprints["apiserver"] == metadata.fingerprints["go"]
    assert service_fingerprints_match(metadata, ["apiserver"], "podman", run)
    assert service_fingerprints_match(metadata, ["go"], "podman", run)
    assert service_fingerprints_match(metadata, ["apiserver"], "podman", run)
    assert inspected == ["localhost/shared"] * 3


def test_missing_image_triggers_build(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    (tmp_path / "Dockerfile").write_text("FROM alpine\n")
    config = {"services": {"go": _service_config(tmp_path)}}

    def run(command, **kwargs):
        if command[-1] == "config":
            return _config_result(config)
        return SimpleNamespace(returncode=1, stdout="")

    metadata = resolve_build_metadata(["podman-compose"], [], ["go"], run)
    assert not service_fingerprints_match(metadata, ["go"], "podman", run)


def test_service_fingerprint_check_only_inspects_selected_services(
    tmp_path, monkeypatch
):
    monkeypatch.chdir(tmp_path)
    (tmp_path / "Dockerfile").write_text("FROM alpine\\n")
    config = {
        "services": {
            "web": _service_config(tmp_path),
            "worker": _service_config(tmp_path),
        }
    }
    metadata = resolve_build_metadata(
        ["podman-compose"],
        [],
        ["web", "worker"],
        lambda command, **kwargs: _config_result(config),
    )
    inspected = []

    def run(command, **kwargs):
        inspected.append(command[-1])
        labels = {FINGERPRINT_LABEL: metadata.fingerprints["web"]}
        return SimpleNamespace(returncode=0, stdout=json.dumps(labels))

    assert service_fingerprints_match(metadata, ["web"], "podman", run)
    assert inspected == [metadata.images[0]]


def test_concurrent_lock_uses_stable_lock_inode(tmp_path, monkeypatch):
    monkeypatch.setenv("TMPDIR", str(tmp_path))
    with image_build_lock(["localhost/test"]):
        lock_files = list(tmp_path.glob("leedevkit-image-*.lock"))
        assert len(lock_files) == 1
        inode = lock_files[0].stat().st_ino
    assert lock_files[0].stat().st_ino == inode
