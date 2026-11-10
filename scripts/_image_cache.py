"""Native image freshness checks for Compose-managed test images."""

from __future__ import annotations

import contextlib
import fcntl
import fnmatch
import hashlib
import json
import os
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Iterator

import yaml


FINGERPRINT_LABEL = "com.leedevkit.build-fingerprint"


@dataclass(frozen=True)
class BuildMetadata:
    """Resolved image names and fingerprints for Compose build services."""

    services: dict[str, dict[str, Any]]
    fingerprints: dict[str, str]
    images: list[str]
    override_file: Path | None


class _DockerIgnore:
    """Small Dockerignore matcher without a runtime dependency."""

    def __init__(self, lines: list[str]) -> None:
        self._patterns = [line.strip() for line in lines if line.strip()]

    @staticmethod
    def _matches(pattern: str, relative: str, is_dir: bool) -> bool:
        directory_pattern = pattern.endswith("/")
        pattern = pattern.strip("/")
        if not pattern or pattern == ".":
            return False
        candidates = [relative]
        if "/" not in pattern:
            candidates.extend(relative.split("/"))
        if directory_pattern and not is_dir:
            candidates = [
                candidate for candidate in candidates if candidate != relative
            ]
        return any(
            fnmatch.fnmatchcase(candidate, pattern) or Path(candidate).match(pattern)
            for candidate in candidates
        )

    def match(self, relative: str, is_dir: bool) -> bool:
        ignored = False
        for raw in self._patterns:
            if raw.startswith("#"):
                continue
            negated = raw.startswith("!")
            pattern = raw[1:] if negated else raw
            if self._matches(pattern, relative, is_dir):
                ignored = not negated
        return ignored


def _canonical(value: Any) -> bytes:
    return json.dumps(
        value, sort_keys=True, separators=(",", ":"), default=str
    ).encode()


def _local_path(value: Any, base: Path) -> Path | None:
    if not isinstance(value, str) or not value or "://" in value:
        return None
    path = Path(value).expanduser()
    return path if path.is_absolute() else (base / path).resolve()


def _ignore_file(context: Path, dockerfile: Path | None) -> Path | None:
    if dockerfile is not None:
        candidate = context / f"{dockerfile.name}.dockerignore"
        if candidate.exists():
            return candidate
    candidate = context / ".dockerignore"
    return candidate if candidate.exists() else None


def _read_ignore(
    context: Path, dockerfile: Path | None
) -> tuple[_DockerIgnore | None, bytes]:
    ignore_file = _ignore_file(context, dockerfile)
    if ignore_file is None:
        return None, b"missing"
    try:
        raw = ignore_file.read_bytes()
        return _DockerIgnore(raw.decode(encoding="utf-8").splitlines()), raw
    except (OSError, UnicodeError):
        return None, b"unreadable"


def _hash_context(context: Path, dockerfile: Path | None) -> tuple[str, str]:
    """Hash files sent to Compose, respecting Dockerignore exclusions."""
    digest = hashlib.sha256()
    if not context.is_dir():
        return "missing-context", "missing"
    ignore, ignore_bytes = _read_ignore(context, dockerfile)
    digest.update(b"dockerignore\0" + ignore_bytes)
    for root, dirs, files in os.walk(context, followlinks=False):
        root_path = Path(root)
        dirs[:] = [
            name
            for name in sorted(dirs)
            if ignore is None
            or not ignore.match(
                (root_path / name).relative_to(context).as_posix(), True
            )
        ]
        for name in sorted(files):
            path = root_path / name
            relative = path.relative_to(context).as_posix()
            if ignore is not None and ignore.match(relative, False):
                continue
            digest.update(b"path\0" + relative.encode() + b"\0")
            try:
                digest.update(f"mode:{path.lstat().st_mode & 0o777:o}\0".encode())
                if path.is_symlink():
                    digest.update(b"link\0" + os.readlink(path).encode())
                else:
                    digest.update(path.read_bytes())
            except (OSError, UnicodeError):
                return "inaccessible-context", "inaccessible"
    return digest.hexdigest(), ignore_bytes.hex()


def _file_hash(path: Path) -> str:
    try:
        return hashlib.sha256(path.read_bytes()).hexdigest()
    except (OSError, UnicodeError):
        return "missing-or-inaccessible"


def _build_dict(service_config: dict[str, Any]) -> dict[str, Any]:
    build = service_config.get("build")
    return {"context": build} if isinstance(build, str) else dict(build or {})


def build_fingerprint(service: str, service_config: dict[str, Any]) -> str:
    """Hash effective build settings, Dockerfile, ignored context, and target."""
    build = _build_dict(service_config)
    context = _local_path(build.get("context", "."), Path.cwd())
    dockerfile = _local_path(build.get("dockerfile"), context or Path.cwd())
    if dockerfile is None and context is not None:
        dockerfile = next(
            (
                context / name
                for name in ("Dockerfile", "Containerfile")
                if (context / name).exists()
            ),
            None,
        )
    context_hash, ignore_hash = _hash_context(context or Path("."), dockerfile)
    payload = {
        "schema": 1,
        "service": service,
        "platform": service_config.get("platform"),
        "build": build,
        "context_hash": context_hash,
        "dockerfile_hash": _file_hash(dockerfile) if dockerfile else "missing",
        "dockerfile_path": str(dockerfile) if dockerfile else None,
        "dockerignore": ignore_hash,
    }
    return hashlib.sha256(_canonical(payload)).hexdigest()


def _stable_image_name(service: str, context: Path) -> str:
    identity = hashlib.sha256(str(context.resolve()).encode()).hexdigest()[:12]
    return f"localhost/leedevkit-test-{identity}-{service}"


def _service_image(service: str, config: dict[str, Any]) -> str:
    configured = config.get("image")
    if isinstance(configured, str) and configured:
        return configured
    build = _build_dict(config)
    context = _local_path(build.get("context", "."), Path.cwd()) or Path.cwd()
    return _stable_image_name(service, context)


def _image_has_fingerprint(
    engine: str, image: str, expected: str, run: Callable[..., Any]
) -> bool:
    try:
        result = run(
            [engine, "image", "inspect", "--format={{json .Config.Labels}}", image],
            capture=True,
        )
        if getattr(result, "returncode", 1) != 0:
            return False
        labels = json.loads(str(getattr(result, "stdout", "")).strip())
        return isinstance(labels, dict) and labels.get(FINGERPRINT_LABEL) == expected
    except (OSError, TypeError, ValueError, json.JSONDecodeError):
        return False


def _compose_config(
    compose_base: list[str], profiles: list[str], run: Callable[..., Any]
) -> dict[str, Any] | None:
    try:
        result = run(compose_base + profiles + ["config"], capture=True)
        if getattr(result, "returncode", 1) != 0:
            return None
        config = yaml.safe_load(getattr(result, "stdout", ""))
        return (
            config
            if isinstance(config, dict) and isinstance(config.get("services"), dict)
            else None
        )
    except (OSError, TypeError, ValueError, yaml.YAMLError):
        return None


def _write_override(
    services: dict[str, dict[str, Any]], fingerprints: dict[str, str]
) -> Path | None:
    override_services: dict[str, dict[str, Any]] = {}
    for service, fingerprint in fingerprints.items():
        config = services[service]
        build = _build_dict(config)
        labels = build.get("labels", [])
        if isinstance(labels, dict):
            label_list = [f"{key}={value}" for key, value in labels.items()]
        else:
            label_list = [str(label) for label in labels]
        label_list = [
            label
            for label in label_list
            if not label.startswith(f"{FINGERPRINT_LABEL}=")
        ]
        label_list.append(f"{FINGERPRINT_LABEL}={fingerprint}")
        override: dict[str, Any] = {"build": {"labels": label_list}}
        if not isinstance(config.get("image"), str):
            override["image"] = _service_image(service, config)
        override_services[service] = override
    if not override_services:
        return None
    fd, name = tempfile.mkstemp(prefix="leedevkit-build-", suffix=".yml")
    path = Path(name)
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as stream:
            yaml.safe_dump({"services": override_services}, stream, sort_keys=True)
    except BaseException:
        with contextlib.suppress(OSError):
            os.close(fd)
        path.unlink(missing_ok=True)
        raise
    return path


def resolve_build_metadata(
    compose_base: list[str],
    profiles: list[str],
    services_to_check: list[str],
    run: Callable[..., Any],
) -> BuildMetadata:
    config = _compose_config(compose_base, profiles, run)
    if config is None:
        return BuildMetadata({}, {}, [], None)
    services = {
        service: config["services"][service]
        for service in dict.fromkeys(services_to_check)
        if isinstance(config["services"].get(service), dict)
        and "build" in config["services"][service]
    }
    fingerprints = {
        service: build_fingerprint(service, value)
        for service, value in services.items()
    }
    images = [_service_image(service, value) for service, value in services.items()]
    return BuildMetadata(
        services, fingerprints, images, _write_override(services, fingerprints)
    )


@contextlib.contextmanager
def image_build_lock(images: list[str]) -> Iterator[None]:
    """Serialize image inspect/build/publish without deleting lock inodes."""
    key = hashlib.sha256("\0".join(sorted(images)).encode()).hexdigest()[:24]
    path = (
        Path(os.environ.get("TMPDIR", tempfile.gettempdir()))
        / f"leedevkit-image-{key}.lock"
    )
    fd = os.open(path, os.O_CREAT | os.O_RDWR, 0o600)
    try:
        fcntl.flock(fd, fcntl.LOCK_EX)
        yield
    finally:
        fcntl.flock(fd, fcntl.LOCK_UN)
        os.close(fd)


def service_fingerprints_match(
    metadata: BuildMetadata,
    services: list[str],
    engine: str,
    run: Callable[..., Any],
) -> bool:
    """Check freshness for selected Compose services only."""
    image_by_service = {
        service: image for service, image in zip(metadata.services, metadata.images)
    }
    selected = [service for service in services if service in metadata.fingerprints]
    if len(selected) != len(set(services)):
        return False
    return bool(selected) and all(
        service in image_by_service
        and _image_has_fingerprint(
            engine, image_by_service[service], metadata.fingerprints[service], run
        )
        for service in selected
    )


def fingerprints_match(
    metadata: BuildMetadata, engine: str, run: Callable[..., Any]
) -> bool:
    """Check freshness for every image represented by metadata."""
    return service_fingerprints_match(
        metadata, list(metadata.fingerprints), engine, run
    )
