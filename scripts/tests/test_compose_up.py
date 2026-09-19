from unittest.mock import patch

import pytest

from _compose_up import ComposeConfigError, _compose_config, start_podman_compose


def _result(stdout: str = "", returncode: int = 0):
    return type("Result", (), {"stdout": stdout, "stderr": "", "returncode": returncode})()


def test_compose_graph_rejects_cycle_before_startup() -> None:
    config = """services:
  a:
    image: test
    depends_on:
      b:
        condition: service_started
  b:
    image: test
    depends_on:
      a:
        condition: service_started
"""
    with patch("_compose_up.subprocess.run", return_value=_result(config)):
        with pytest.raises(ComposeConfigError, match="cycle"):
            _compose_config(["podman-compose"], {})


def test_completed_job_failure_stops_downstream_start() -> None:
    config = """services:
  job:
    image: test
  app:
    image: test
    depends_on:
      job:
        condition: service_completed_successfully
"""
    calls: list[list[str]] = []

    def execute(command: list[str]) -> None:
        calls.append(command)
        if command[-1] == "job":
            raise SystemExit(23)

    with patch("_compose_up.subprocess.run", return_value=_result(config)), patch(
        "_compose_up._inspect_project_containers", return_value=[]
    ):
        with pytest.raises(SystemExit) as exc:
            start_podman_compose(["podman-compose"], {}, execute)

    assert exc.value.code == 23
    assert not any(command[-1] == "app" for command in calls)


def test_completed_job_is_run_before_downstream_service() -> None:
    config = """services:
  job:
    image: test
  app:
    image: test
    depends_on:
      job:
        condition: service_completed_successfully
"""
    calls: list[list[str]] = []

    with patch("_compose_up.subprocess.run", return_value=_result(config)), patch(
        "_compose_up._inspect_project_containers", return_value=[]
    ):
        start_podman_compose(["podman-compose"], {}, calls.append)

    assert calls == [
        ["podman-compose", "run", "--rm", "--no-deps", "job"],
        ["podman-compose", "up", "-d", "--no-deps", "app"],
    ]
