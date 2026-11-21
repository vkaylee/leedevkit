#!/usr/bin/env python3
"""Devkit configuration cascade engine.

Loads `leedevkit.toml` from the project root and merges it with the devkit
default template. Implements a strict priority chain:

    1. Project leedevkit.toml       (highest)
    2. Devkit default template    (base fallback)
    3. Auto-detected values       (lowest)

Also resolves the AI rule override manifest (`.agent/overrides.yaml`).
"""

from __future__ import annotations

import os
import re
import subprocess
import sys
from collections.abc import Mapping
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any
from urllib.parse import SplitResult, urlsplit, urlunsplit

from _runtime import enforce_project_venv

# ── Bootstrap: locate devkit root ──────────────────────────────────────────

_DEVKIT_ROOT: Path | None = None
_DEVKIT_CACHE_KEY: tuple[str, str, str, str, str] | None = None


def reset_devkit_root_cache() -> None:
    """Clear cached runtime resolution after project context changes."""
    global _DEVKIT_ROOT, _DEVKIT_CACHE_KEY
    _DEVKIT_ROOT = None
    _DEVKIT_CACHE_KEY = None


def _git_main_repo() -> Path | None:
    """Return main checkout root when running inside a Git worktree."""
    try:
        common = subprocess.run(
            ["git", "rev-parse", "--path-format=absolute", "--git-common-dir"],
            capture_output=True,
            text=True,
            check=True,
        ).stdout.strip()
    except (OSError, subprocess.CalledProcessError):
        return None
    if not common:
        return None
    common_path = Path(common).resolve()
    return common_path.parent if common_path.name == ".git" else None


def _runtime_version_matches(root: Path, expected: str) -> bool:
    """Accept runtime with a valid VERSION matching concrete project pins."""
    version_file = root / "VERSION"
    if not version_file.is_file():
        return False
    actual = version_file.read_text().strip().lstrip("v")
    return expected == "latest" or actual == str(expected).strip().lstrip("v")


def _runtime_candidate(root: Path, expected: str) -> Path | None:
    if root.is_dir() and _runtime_version_matches(root, expected):
        return root
    return None


def _find_devkit_root() -> Path:
    """Resolve runtime without reusing cache across project contexts."""
    global _DEVKIT_ROOT, _DEVKIT_CACHE_KEY
    project_root = _find_project_root()
    expected = _read_version(project_root)
    cache_key = (
        str(Path.cwd().resolve()),
        str(project_root.resolve()),
        str(expected),
        os.environ.get("DEVKIT_HOME", ""),
        os.environ.get("LEEDEVKIT_BOOTSTRAP", ""),
    )
    if _DEVKIT_ROOT is not None and _DEVKIT_CACHE_KEY == cache_key:
        return _DEVKIT_ROOT
    _DEVKIT_ROOT = None
    _DEVKIT_CACHE_KEY = cache_key
    candidates = [Path.cwd() / ".leedevkit", project_root / ".leedevkit"]
    for candidate in candidates:
        resolved = _runtime_candidate(candidate, expected)
        if resolved is not None:
            _DEVKIT_ROOT = resolved
            return resolved

    source_checkout = Path(__file__).resolve().parent.parent
    if Path.cwd().resolve() == source_checkout and _runtime_version_matches(
        source_checkout, expected
    ):
        _DEVKIT_ROOT = source_checkout
        return source_checkout

    main_repo = _git_main_repo()
    if main_repo is not None:
        candidates.append(main_repo / ".leedevkit")
    env = os.environ.get("DEVKIT_HOME")
    if env:
        candidates.append(Path(env))

    if os.environ.get("LEEDEVKIT_BOOTSTRAP") == "1":
        bootstrap_home = os.environ.get("DEVKIT_HOME")
        if bootstrap_home and (Path(bootstrap_home) / "VERSION").is_file():
            _DEVKIT_ROOT = Path(bootstrap_home)
            return _DEVKIT_ROOT
    for candidate in candidates:
        resolved = _runtime_candidate(candidate, expected)
        if resolved is not None:
            _DEVKIT_ROOT = resolved
            return resolved

    raise FileNotFoundError(
        "No compatible LeeDevKit runtime found for project version "
        f"{expected!r}. Run 'leedevkit doctor --fix' to repair or download it, "
        "or set DEVKIT_HOME to a runtime with the same VERSION."
    )


def get_devkit_root() -> Path:
    """Public accessor for the resolved devkit root."""
    return _find_devkit_root()


def _find_project_root() -> Path:
    """Walk up from cwd until we find leedevkit.toml or .git."""
    cwd = Path.cwd()
    for parent in [cwd, *cwd.parents]:
        if (parent / "leedevkit.toml").exists() or (parent / ".git").exists():
            return parent
    return cwd


# ── TOML loader (stdlib fallback) ──────────────────────────────────────────


def _load_toml(path: Path) -> dict[str, Any]:
    """Load a TOML file. Uses tomllib (3.11+) or tomli (fallback)."""
    try:
        import tomllib  # Python 3.11+

        with open(path, "rb") as f:
            return tomllib.load(f)
    except ImportError:
        pass
    try:
        import tomli  # pip install tomli

        with open(path, "rb") as f:
            return tomli.load(f)
    except ImportError:
        pass
    # Minimal inline parser — handles flat keys and strings
    return _parse_toml_minimal(path)


def _parse_toml_minimal(path: Path) -> dict[str, Any]:
    """Fallback TOML parser for environments without tomli/tomllib."""
    result: dict[str, Any] = {}
    current_section = result
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            if line.startswith("[") and line.endswith("]"):
                section = line[1:-1].strip()
                current_section = result.setdefault(section, {})
            elif "=" in line:
                key, _, val = line.partition("=")
                key = key.strip()
                val = val.strip().strip('"').strip("'")
                current_section[key] = val
    return result


# ── Merge engine ───────────────────────────────────────────────────────────


def deep_merge(base: dict, override: dict) -> dict:
    """Recursive dict merge: override keys win, nested dicts are merged."""
    result = dict(base)
    for key, value in override.items():
        if key in result and isinstance(result[key], dict) and isinstance(value, dict):
            result[key] = deep_merge(result[key], value)
        else:
            result[key] = value
    return result


@dataclass(frozen=True)
class ContainerProviderConfig:
    """Explicit container-provider settings resolved from configuration."""

    name: str
    cli: tuple[str, ...]
    compose: tuple[str, ...]
    capabilities: frozenset[str] = field(default_factory=frozenset)
    settings: Mapping[str, Any] = field(default_factory=dict)
    source: str = "default"


_CONTAINER_CAPABILITIES = frozenset(
    {
        "compose",
        "config",
        "up",
        "down",
        "ps",
        "logs",
        "exec",
        "run",
        "inspect",
        "rm",
        "build",
        "pull",
        "network",
        "volume",
        "image",
        "healthcheck",
        "project_cleanup",
        "wait",
    }
)
_SECRET_KEY = re.compile(
    r"(?:password|passwd|token|secret|private[_-]?key|api[_-]?key)", re.IGNORECASE
)


def _host_container_config_path() -> Path:
    """Return portable host-local config path without creating it."""
    root = os.environ.get("XDG_CONFIG_HOME")
    base = Path(root).expanduser() if root else Path.home() / ".config"
    return base / "leedevkit" / "config.toml"


def _container_section(config: Mapping[str, Any], source: str) -> dict[str, Any]:
    raw = config.get("container", {})
    if raw is None:
        return {}
    if not isinstance(raw, dict):
        raise ValueError(f"{source}: [container] must be a table")
    return dict(raw)


def _argv_setting(value: Any, field_name: str, source: str) -> tuple[str, ...]:
    if (
        not isinstance(value, list)
        or not value
        or any(not isinstance(item, str) or not item.strip() for item in value)
    ):
        raise ValueError(f"{source}: {field_name} must be a non-empty string list")
    return tuple(value)


def _provider_settings(raw: Any, name: str, source: str) -> ContainerProviderConfig:
    if not isinstance(raw, dict):
        raise ValueError(f'{source}: provider "{name}" must be a table')
    cli = _argv_setting(raw.get("cli"), f'[container.providers."{name}"].cli', source)
    compose = _argv_setting(
        raw.get("compose"), f'[container.providers."{name}"].compose', source
    )
    capabilities_raw = raw.get("capabilities", sorted(_CONTAINER_CAPABILITIES))
    if not isinstance(capabilities_raw, list) or any(
        not isinstance(item, str) or item not in _CONTAINER_CAPABILITIES
        for item in capabilities_raw
    ):
        raise ValueError(
            f'{source}: [container.providers."{name}"].capabilities contains unknown names; '
            f"choose from {sorted(_CONTAINER_CAPABILITIES)}"
        )
    settings = {
        key: value
        for key, value in raw.items()
        if key not in {"cli", "compose", "capabilities"}
    }
    return ContainerProviderConfig(
        name=name,
        cli=cli,
        compose=compose,
        capabilities=frozenset(capabilities_raw),
        settings=settings,
        source=source,
    )


def _mask_container_value(value: Any, key: str = "") -> Any:
    """Redact secret-like values and credentials in diagnostic structures."""
    if _SECRET_KEY.search(key):
        return "<redacted>"
    if isinstance(value, dict):
        return {str(k): _mask_container_value(v, str(k)) for k, v in value.items()}
    if isinstance(value, list):
        return [_mask_container_value(item, key) for item in value]
    if isinstance(value, tuple):
        return tuple(_mask_container_value(item, key) for item in value)
    if isinstance(value, str):
        parsed: SplitResult = urlsplit(value)
        if parsed.username or parsed.password:
            host = parsed.hostname or ""
            if parsed.port:
                host += f":{parsed.port}"
            return urlunsplit(
                (parsed.scheme, host, parsed.path, parsed.query, parsed.fragment)
            )
    return value


def resolve_container_provider(
    config: Mapping[str, Any] | None = None,
    *,
    cli_provider: str | None = None,
    environ: Mapping[str, str] | None = None,
    host_config: Mapping[str, Any] | None = None,
    host_source: str | None = None,
) -> ContainerProviderConfig:
    """Resolve explicit container settings without probing or inferring tools."""
    environ = os.environ if environ is None else environ
    if config is None:
        config = load_project_config()
    repo_section = _container_section(config, "repository configuration")
    if host_config is None:
        host_path = _host_container_config_path()
        host_config = _load_toml(host_path) if host_path.is_file() else {}
        host_source = str(host_path)
    host_section = _container_section(host_config, host_source or "host configuration")

    repo_provider = repo_section.get("provider", "docker")
    if not isinstance(repo_provider, str) or not repo_provider.strip():
        raise ValueError(
            "repository configuration: [container].provider must be a non-empty string"
        )
    host_provider = host_section.get("provider")
    if host_provider is not None and (
        not isinstance(host_provider, str) or not host_provider.strip()
    ):
        raise ValueError(
            "host configuration: [container].provider must be a non-empty string"
        )
    selected = (
        cli_provider
        or environ.get("CONTAINER_PROVIDER")
        or host_provider
        or repo_provider
    )
    if not isinstance(selected, str) or not selected.strip():
        raise ValueError(
            "container provider must be a non-empty string; set --container-provider or CONTAINER_PROVIDER"
        )
    selected = selected.strip()

    repo_providers = repo_section.get("providers", {})
    host_providers = host_section.get("providers", {})
    if not isinstance(repo_providers, dict):
        raise ValueError(
            "repository configuration: [container.providers] must be a table"
        )
    if not isinstance(host_providers, dict):
        raise ValueError("host configuration: [container.providers] must be a table")
    raw: Any = repo_providers.get(selected)
    source = "repository configuration"
    if selected in host_providers:
        raw = deep_merge(raw if isinstance(raw, dict) else {}, host_providers[selected])
        source = host_source or "host configuration"
    if raw is None:
        if selected == "docker":
            raw = {"cli": ["docker"], "compose": ["docker", "compose"]}
            source = "LeeDevKit default"
        else:
            raise ValueError(
                f'container provider "{selected}" is not defined in {source}; '
                "add [container.providers.%s] with explicit cli and compose argv"
                % selected
            )
    if cli_provider or environ.get("CONTAINER_PROVIDER"):
        source = "CLI override" if cli_provider else "CONTAINER_PROVIDER"
    provider = _provider_settings(raw, selected, source)
    if os.environ.get("LEEDEVKIT_USE_GO_CORE") == "1":
        from _go_core import resolve_provider_go

        expected = container_provider_diagnostics(provider)
        response = resolve_provider_go(expected)
        if response is None or response.get("diagnostics") != expected:
            raise RuntimeError("Go provider shadow result differs from Python result")
    return provider


def require_container_capabilities(
    provider: ContainerProviderConfig, required: set[str]
) -> None:
    """Fail before mutation when provider lacks lifecycle capabilities."""
    missing = sorted(required - provider.capabilities)
    if missing:
        raise ValueError(
            f'container provider "{provider.name}" lacks required capabilities: '
            + ", ".join(missing)
        )


def container_provider_diagnostics(provider: ContainerProviderConfig) -> dict[str, Any]:
    """Return safe provider diagnostics for logs and machine reports."""
    return _mask_container_value(
        {
            "provider": provider.name,
            "cli": list(provider.cli),
            "compose": list(provider.compose),
            "capabilities": sorted(provider.capabilities),
            "settings": dict(provider.settings),
            "source": provider.source,
        }
    )


def load_project_config() -> dict[str, Any]:
    """Load the fully merged project configuration.

    Returns a dict with all resolved settings.
    """
    project_root = _find_project_root()
    devkit_root = _find_devkit_root()

    # Layer 1: Devkit default. Older releases used the dotted filename;
    # source checkouts and current templates use the public filename.
    default_path = devkit_root / "templates" / ".devkit.default.toml"
    if not default_path.exists():
        default_path = devkit_root / "templates" / "leedevkit.default.toml"
    config: dict[str, Any] = {}
    if default_path.exists():
        config = _load_toml(default_path)

    # Layer 2: Project override
    project_path = project_root / "leedevkit.toml"
    if project_path.exists():
        project_config = _load_toml(project_path)
        config = deep_merge(config, project_config)

    # Layer 3: Injected values
    config.setdefault("_meta", {})
    config["_meta"]["project_root"] = str(project_root)
    config["_meta"]["devkit_root"] = str(devkit_root)
    config["_meta"]["devkit_version"] = _read_version(project_root)

    return config


def _read_version(project_root: Path) -> str:
    """Read pinned devkit version from leedevkit.toml [devkit].version."""
    toml_path = project_root / "leedevkit.toml"
    if toml_path.exists():
        cfg = _load_toml(toml_path)
        return cfg.get("devkit", {}).get("version", "latest")
    return "latest"


def resolve_test_build_policy(config: dict[str, Any] | None = None) -> str:
    """Return validated test image build policy, defaulting to once per run."""
    if config is None:
        config = load_project_config()
    test_config = config.get("test", {})
    if not isinstance(test_config, dict):
        raise ValueError("[test] must be a table")

    policy = test_config.get("build_policy", "once")
    if policy not in ("once", "phase"):
        raise ValueError('[test].build_policy must be either "once" or "phase"')
    return policy


_QUALITY_PHASES = {"lint", "unit", "integration", "generated", "coverage"}
_QUALITY_FAILURE_POLICIES = {"required", "optional", "block"}
_GO_QUALITY_DEFAULTS: dict[str, dict[str, Any]] = {
    "gofmt": {
        "phase": "lint",
        "command": ["gofmt", "-l", "."],
        "mode": "check",
        "required": True,
    },
    "go-vet": {
        "phase": "lint",
        "command": ["go", "vet", "./..."],
        "required": True,
    },
    "go-test": {
        "phase": "unit",
        "command": ["go", "test", "./..."],
        "required": True,
    },
    "go-test-race": {
        "phase": "unit",
        "command": ["go", "test", "-race", "./..."],
        "required": True,
    },
    "staticcheck": {
        "phase": "lint",
        "command": ["staticcheck", "./..."],
        "required": True,
    },
    "golangci-lint": {
        "phase": "lint",
        "command": ["golangci-lint", "run", "./..."],
        "required": True,
    },
    "govulncheck": {
        "phase": "lint",
        "command": ["govulncheck", "./..."],
        "required": True,
    },
    "go-mod-verify": {
        "phase": "lint",
        "command": ["go", "mod", "verify"],
        "required": True,
    },
    "go-coverage": {
        "phase": "coverage",
        "command": [
            "go",
            "test",
            "-coverprofile=/workspace/.test_logs/coverage-go.out",
            "./...",
        ],
        "required": True,
    },
}


def _validate_quality_gate(name: str, raw: Any) -> dict[str, Any]:
    """Normalize one quality gate without interpreting command shell syntax."""
    if not isinstance(name, str) or not name.strip():
        raise ValueError("quality gate names must be non-empty strings")
    if not isinstance(raw, dict):
        raise ValueError(f'[quality.gates."{name}"] must be a table')
    command = raw.get("command")
    if (
        not isinstance(command, list)
        or not command
        or any(not isinstance(part, str) or not part for part in command)
    ):
        raise ValueError(
            f'[quality.gates."{name}"].command must be a non-empty string list'
        )
    phase = raw.get("phase", "lint")
    if not isinstance(phase, str) or phase not in _QUALITY_PHASES:
        raise ValueError(
            f'[quality.gates."{name}"].phase must be one of {sorted(_QUALITY_PHASES)}'
        )
    timeout = raw.get("timeout")
    if timeout is not None and (
        isinstance(timeout, bool)
        or not isinstance(timeout, (int, float))
        or timeout <= 0
    ):
        raise ValueError(f'[quality.gates."{name}"].timeout must be positive')
    working_dir = raw.get("working_dir", "")
    if not isinstance(working_dir, str):
        raise ValueError(f'[quality.gates."{name}"].working_dir must be a string')
    if working_dir.startswith("/") or any(
        part == ".." for part in Path(working_dir).parts
    ):
        raise ValueError(
            f'[quality.gates."{name}"].working_dir must stay inside project'
        )
    service = raw.get("service", "")
    if not isinstance(service, str):
        raise ValueError(f'[quality.gates."{name}"].service must be a string')
    required = raw.get("required", True)
    if not isinstance(required, bool):
        raise ValueError(f'[quality.gates."{name}"].required must be boolean')
    failure_policy = raw.get("failure_policy", "required" if required else "optional")
    if failure_policy not in _QUALITY_FAILURE_POLICIES:
        raise ValueError(
            f'[quality.gates."{name}"].failure_policy must be one of '
            f"{sorted(_QUALITY_FAILURE_POLICIES)}"
        )
    if failure_policy == "optional" and required:
        raise ValueError(
            f'[quality.gates."{name}"].required must be false for optional failure_policy'
        )
    condition = raw.get("condition", "always")
    if not isinstance(condition, str) or not condition.strip():
        raise ValueError(f'[quality.gates."{name}"].condition must be non-empty')
    requires = raw.get("requires", [])
    if not isinstance(requires, list) or any(
        not isinstance(item, str) or not item.strip() for item in requires
    ):
        raise ValueError(f'[quality.gates."{name}"].requires must be a list of names')
    environment = raw.get("environment", [])
    if not isinstance(environment, list) or any(
        not isinstance(key, str) or not key or "=" in key for key in environment
    ):
        raise ValueError(
            f'[quality.gates."{name}"].environment must be a list of variable names'
        )
    mode = raw.get("mode", "run")
    if not isinstance(mode, str) or mode not in {"run", "check", "fix"}:
        raise ValueError(f'[quality.gates."{name}"].mode must be run, check, or fix')
    expected_version = raw.get("expected_version")
    if expected_version is not None and (
        not isinstance(expected_version, str) or not expected_version.strip()
    ):
        raise ValueError(
            f'[quality.gates."{name}"].expected_version must be a non-empty string'
        )
    fix_command = raw.get("fix_command")
    if fix_command is not None and (
        not isinstance(fix_command, list)
        or not fix_command
        or any(not isinstance(part, str) or not part for part in fix_command)
    ):
        raise ValueError(
            f'[quality.gates."{name}"].fix_command must be a non-empty string list'
        )
    setup_command = raw.get("setup_command")
    teardown_command = raw.get("teardown_command")
    for field_name, field_value in (
        ("setup_command", setup_command),
        ("teardown_command", teardown_command),
    ):
        if field_value is not None and (
            not isinstance(field_value, list)
            or not field_value
            or any(not isinstance(part, str) or not part for part in field_value)
        ):
            raise ValueError(
                f'[quality.gates."{name}"].{field_name} must be a non-empty string list'
            )
    new_findings_flag = raw.get("new_findings_flag")
    if new_findings_flag is not None and (
        not isinstance(new_findings_flag, list)
        or not new_findings_flag
        or any(not isinstance(part, str) or not part for part in new_findings_flag)
    ):
        raise ValueError(
            f'[quality.gates."{name}"].new_findings_flag must be a non-empty string list'
        )
    version_command = raw.get("version_command")
    if version_command is not None and (
        not isinstance(version_command, list)
        or not version_command
        or any(not isinstance(part, str) or not part for part in version_command)
    ):
        raise ValueError(
            f'[quality.gates."{name}"].version_command must be a non-empty string list'
        )
    return {
        "name": name,
        "phase": phase,
        "command": list(command),
        "mode": mode,
        "working_dir": working_dir,
        "service": service,
        "timeout": timeout,
        "required": required,
        "condition": condition,
        "failure_policy": failure_policy,
        "environment": list(environment),
        "requires": list(requires),
        "expected_version": expected_version,
        "version_command": list(version_command)
        if version_command is not None
        else None,
        "fix_command": list(fix_command) if fix_command is not None else None,
        "new_findings_flag": list(new_findings_flag)
        if new_findings_flag is not None
        else None,
        "setup_command": list(setup_command) if setup_command is not None else None,
        "teardown_command": list(teardown_command)
        if teardown_command is not None
        else None,
    }


def resolve_quality_gates(
    config: dict[str, Any] | None = None, language: str = "go"
) -> list[dict[str, Any]]:
    """Return deterministic normalized quality gates for one language.

    Existing projects get only safe Go core defaults. Extra tools run only when
    explicitly named in ``[quality.<language>]`` and defined in ``quality.gates``.
    """
    if config is None:
        config = load_project_config()
    quality = config.get("quality", {})
    if quality is None:
        quality = {}
    if not isinstance(quality, dict):
        raise ValueError("[quality] must be a table")
    raw_defs = quality.get("gates", {})
    if raw_defs is None:
        raw_defs = {}
    if not isinstance(raw_defs, dict):
        raise ValueError("[quality.gates] must be a table")

    language_config = quality.get(language, {})
    if language_config is None:
        language_config = {}
    if not isinstance(language_config, dict):
        raise ValueError(f"[quality.{language}] must be a table")

    global_generated = quality.get("generated")
    if global_generated is not None:
        if not isinstance(global_generated, dict):
            raise ValueError("[quality.generated] must be a table")
        if "command" not in global_generated:
            raise ValueError("[quality.generated].command is required")
        raw_defs = dict(raw_defs)
        raw_defs["generated"] = {
            "phase": "generated",
            **global_generated,
        }
        language_config = dict(language_config)
        language_config["generated"] = ["generated"]

    selected_lists = []
    for phase_name in ("lint", "unit", "integration", "generated"):
        phase_gates = language_config.get(phase_name, [])
        if not isinstance(phase_gates, list):
            raise ValueError(f"[quality.{language}].{phase_name} must be a list")
        selected_lists.append(phase_gates)
    selected = sum(selected_lists, [])
    if any(not isinstance(name, str) or not name.strip() for name in selected):
        raise ValueError(f"[quality.{language}] gate lists must contain names")

    definitions: dict[str, Any] = dict(_GO_QUALITY_DEFAULTS if language == "go" else {})
    definitions.update(raw_defs)
    if selected:
        names = list(dict.fromkeys(selected))
    elif language == "go":
        names = ["gofmt", "go-vet", "go-test", "go-coverage"]
    else:
        names = []

    gates: list[dict[str, Any]] = []
    for name in names:
        if name not in definitions:
            raise ValueError(f'quality gate "{name}" is not defined')
        gates.append(_validate_quality_gate(name, definitions[name]))
    return gates


def resolve_version_gate(gate: dict[str, Any]) -> dict[str, Any] | None:
    """Return validated version-check metadata for a gate, if configured."""
    expected = gate.get("expected_version")
    command = gate.get("version_command")
    if expected is None and command is None:
        return None
    if expected is None or command is None:
        raise ValueError(
            f'quality gate "{gate.get("name", "")}" needs expected_version and version_command'
        )
    return {"expected_version": expected, "version_command": list(command)}


def resolve_lifecycle_dependencies(
    config: dict[str, Any] | None = None,
) -> dict[str, list[str]]:
    """Return validated Compose service dependencies by lifecycle mode."""
    if config is None:
        config = load_project_config()
    test_config = config.get("test", {})
    if not isinstance(test_config, dict):
        raise ValueError("[test] must be a table")

    dependencies = test_config.get("dependencies", {})
    if dependencies is None:
        return {}
    if not isinstance(dependencies, dict):
        raise ValueError("[test.dependencies] must be a table")

    result: dict[str, list[str]] = {}
    for mode, services in dependencies.items():
        if not isinstance(mode, str) or not mode.strip():
            raise ValueError("[test.dependencies] mode keys must be non-empty strings")
        if not isinstance(services, list) or not services:
            raise ValueError(
                f'[test.dependencies."{mode}"] must be a non-empty list of service names'
            )
        if any(
            not isinstance(service, str) or not service.strip() for service in services
        ):
            raise ValueError(
                f'[test.dependencies."{mode}"] must contain only non-empty strings'
            )
        result[mode] = services
    return result


# ── AI Rule resolver ───────────────────────────────────────────────────────


def _load_yaml(path: Path) -> dict[str, Any] | None:
    """Load YAML if available, else return None."""
    try:
        import yaml  # type: ignore[import-untyped]

        with open(path, encoding="utf-8") as f:
            return yaml.safe_load(f)
    except ImportError:
        return None


def resolve_ai_rules(project_root: Path | None = None) -> list[Path]:
    """Return the ordered list of AI rule files to load.

    Priority chain (loaded in order, later files override):
      1. devkit base rules
      2. project rules (replacing devkit where applicable)
      3. project add-only rules

    Returns: list of absolute Paths to .md rule files.
    """
    if project_root is None:
        project_root = _find_project_root()
    devkit_root = _find_devkit_root()

    # Read AI paths from leedevkit.toml, falling back to defaults
    cfg = load_project_config()
    ai_cfg = cfg.get("ai", {})
    rules_rel = ai_cfg.get("rules_dir", ".agent/rules")
    manifest_rel = ai_cfg.get("override_manifest", ".agent/overrides.yaml")

    devkit_rules_dir = devkit_root / ".agent" / "rules"
    project_rules_dir = project_root / rules_rel
    manifest_path = project_root / manifest_rel

    manifest: dict[str, Any] = _load_yaml(manifest_path) or {}
    replace: list[str] = manifest.get("replace", [])
    add: list[str] = manifest.get("add", [])
    extend: list[str] = manifest.get("extend", [])

    result: list[Path] = []

    # Devkit base rules (skip replaced ones)
    if devkit_rules_dir.exists():
        for rule_file in sorted(devkit_rules_dir.rglob("*.md")):
            rel = str(rule_file.relative_to(devkit_rules_dir))
            if rel not in replace:
                result.append(rule_file)

    # Project overrides
    if project_rules_dir.exists():
        # Project versions of devkit rules (replace/extend)
        for rule_file in sorted(project_rules_dir.rglob("*.md")):
            rel = str(rule_file.relative_to(project_rules_dir))
            if rel in replace or rel in extend:
                result.append(rule_file)

        # Project-only rules (add)
        if add:
            for rule_name in add:
                rule_path = project_rules_dir / rule_name
                if rule_path.exists():
                    result.append(rule_path)

    return result


def resolve_targets() -> list[str]:
    """Resolve valid test targets from leedevkit.toml, falling back to defaults."""
    try:
        cfg = load_project_config()
        targets = cfg.get("targets", {})
        if targets:
            return list(targets.keys())
    except Exception:
        pass
    return [
        "all",
        "api",
        "web",
        "go",
        "apiserver",
        "agent-main",
        "webdashboard",
        "infra",
    ]


def resolve_test_targets(target: str = "all") -> list[str]:
    """Expand a logical target into configured target names without losing services."""
    config = load_project_config()
    targets = config.get("targets", {})
    if not isinstance(targets, dict):
        return [target]
    if target != "all":
        return [target]

    all_services = targets.get("all", [])
    if not isinstance(all_services, list):
        return ["all"]
    selected: list[str] = []
    covered_services: set[str] = set()
    normalized_services = [str(service) for service in all_services]
    for name, services in targets.items():
        if name == "all" or not isinstance(services, list):
            continue
        configured_services = {str(service) for service in services}
        covered = configured_services.intersection(normalized_services)
        if covered:
            selected.append(str(name))
            covered_services.update(covered)
    for service_name in normalized_services:
        if service_name not in covered_services and service_name not in selected:
            selected.append(service_name)
            covered_services.add(service_name)

    result = selected or ["all"]
    if os.environ.get("LEEDEVKIT_USE_GO_CORE") == "1":
        from _go_core import resolve_test_targets_go

        response = resolve_test_targets_go(
            {
                "schema_version": 1,
                "target": target,
                "all_services": normalized_services,
                "aliases": [
                    {
                        "name": str(name),
                        "services": [str(service) for service in services],
                    }
                    for name, services in targets.items()
                    if name != "all" and isinstance(services, list)
                ],
            }
        )
        if response is None or response.get("targets") != result:
            raise RuntimeError(
                "Go test target shadow result differs from Python result"
            )
    return result


# ── Mode map & Rust version injection (extracted from Orchestrator) ────────


def build_mode_map(project_root: Path | None = None) -> dict[str, str]:
    """Build mode_map from leedevkit.toml services.

    Maps each service to its test mode: Rust → 'api', TypeScript → 'web'.
    Falls back to hardcoded defaults for backward compatibility.
    """
    if project_root is None:
        project_root = _find_project_root()

    mode_map: dict[str, str] = {"all": "all"}
    configured_services: list[dict[str, str]] = []
    try:
        config_toml = project_root / "leedevkit.toml"
        if config_toml.exists():
            cfg = _load_toml(config_toml)
            services = cfg.get("services", {})
            for name, svc in services.items():
                if isinstance(svc, dict):
                    lang = svc.get("lang", "")
                    configured_services.append(
                        {"name": str(name), "language": str(lang)}
                    )
                    if lang == "rust":
                        mode_map[name] = "api"
                    elif lang in ("typescript", "javascript"):
                        mode_map[name] = "web"
                    elif lang == "go":
                        mode_map[name] = "go"
    except (OSError, ValueError, KeyError):
        pass
    # Backward-compat fallbacks
    mode_map.setdefault("apiserver", "api")
    mode_map.setdefault("agent-main", "api")
    mode_map.setdefault("webdashboard", "web")
    mode_map.setdefault("api", "api")
    mode_map.setdefault("web", "web")
    mode_map.setdefault("go", "go")
    if os.environ.get("LEEDEVKIT_USE_GO_CORE") == "1":
        from _go_core import resolve_modes_go

        response = resolve_modes_go(
            {
                "schema_version": 1,
                "services": configured_services,
            }
        )
        if response is None or response.get("modes") != mode_map:
            raise RuntimeError("Go mode map shadow result differs from Python result")
    return mode_map


def inject_rust_version_env(project_root: Path | None = None) -> None:
    """Read rust_version from leedevkit.toml and set RUST_VERSION env var.

    Defaults to '1.85' if not configured. Users can override with:
      [services.<name>]
      rust_version = "1.83"
    or via environment: RUST_VERSION=1.83 ./leedevkit test all
    """
    if "RUST_VERSION" in os.environ:
        return  # Explicit env override wins
    if project_root is None:
        project_root = _find_project_root()
    try:
        config_toml = project_root / "leedevkit.toml"
        if config_toml.exists():
            cfg = _load_toml(config_toml)
            services = cfg.get("services", {})
            for _svc in services.values():
                if isinstance(_svc, dict) and "rust_version" in _svc:
                    os.environ["RUST_VERSION"] = str(_svc["rust_version"])
                    return
    except (OSError, ValueError, KeyError):
        pass
    os.environ.setdefault("RUST_VERSION", "1.85")


# ── CLI ────────────────────────────────────────────────────────────────────

if __name__ == "__main__":  # pragma: no cover
    enforce_project_venv()
    action = sys.argv[1] if len(sys.argv) > 1 else "config"
    if action == "config":
        import json

        cfg = load_project_config()
        print(json.dumps(cfg, indent=2, default=str))
    elif action == "rules":
        for p in resolve_ai_rules():
            print(p)
