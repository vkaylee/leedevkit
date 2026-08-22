"""Tests for _lifecycle.py — lifecycle_up, lifecycle_down, health checks."""

import os
import shutil
import subprocess
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent
sys.path.append(str(PROJECT_ROOT / "scripts"))

import _lifecycle  # noqa: E402
from _lifecycle import (  # noqa: E402
    _container_healthy,
    lifecycle_down,
    lifecycle_up,
    start_watchdog,
)


class TestContainerHealthy:
    def test_healthy_container(self) -> None:
        with patch("_lifecycle._run") as mock_run:
            mock_run.return_value = MagicMock(stdout="healthy", returncode=0)
            assert _container_healthy("test_apiserver_1") is True

    def test_unhealthy_container(self) -> None:
        with patch("_lifecycle._run") as mock_run:
            mock_run.return_value = MagicMock(stdout="unhealthy\n", returncode=0)
            assert _container_healthy("test_apiserver_1") is False

    def test_empty_output(self) -> None:
        with patch("_lifecycle._run") as mock_run:
            mock_run.return_value = MagicMock(stdout="", returncode=1)
            assert _container_healthy("test_apiserver_1") is False


class TestLifecycleUp:
    def test_api_mode_starts_backend(self) -> None:
        with (
            patch("_lifecycle._run") as mock_run,
            patch("_lifecycle._container_healthy", return_value=True),
            patch("_lifecycle.time.sleep"),
        ):
            result = lifecycle_up("api")
            assert result is True
            all_call_args = str(mock_run.call_args_list)
            assert "up" in all_call_args

    def test_web_mode_checks_webdashboard(self) -> None:
        with (
            patch("_lifecycle._run"),
            patch("_lifecycle._container_healthy", return_value=True),
            patch("_lifecycle.time.sleep"),
        ):
            result = lifecycle_up("web")
            assert result is True

    def test_lint_and_unit_modes_use_no_deps(self) -> None:
        """Verify that lint- and unit- modes append --no-deps so heavy infra is not started."""
        with (
            patch("_lifecycle._run") as mock_run,
            patch("_lifecycle._container_healthy", return_value=True),
            patch("_lifecycle.time.sleep"),
        ):
            # Test lint- mode
            lifecycle_up("lint-api")
            lint_call_args = " ".join(mock_run.call_args_list[-1][0][0])
            assert "--no-deps" in lint_call_args

            # Test unit- mode
            lifecycle_up("unit-api")
            unit_call_args = " ".join(mock_run.call_args_list[-1][0][0])
            assert "--no-deps" in unit_call_args

    def test_health_check_timeout_returns_false(self) -> None:
        with (
            patch("_lifecycle._run"),
            patch("_lifecycle._container_healthy", return_value=False),
            patch("_lifecycle.time.sleep"),
        ):
            result = lifecycle_up("api")
            assert result is False

    def test_no_pre_cleanup_on_up(self) -> None:
        """lifecycle_up must NOT do a pre-down — concurrent agents share containers."""
        with (
            patch("_lifecycle._run") as mock_run,
            patch("_lifecycle._container_healthy", return_value=True),
            patch("_lifecycle.time.sleep"),
        ):
            lifecycle_up("api")
            all_call_str = str(mock_run.call_args_list)
            assert "down" not in all_call_str

    def test_go_mode_checks_go_container(self) -> None:
        """mode 'go' health-check waits on the <project>_go_1 container."""
        with (
            patch("_lifecycle._run"),
            patch("_lifecycle._container_healthy", return_value=True) as mock_health,
            patch("_lifecycle.time.sleep"),
        ):
            result = lifecycle_up("go")
            assert result is True
            checked = [call[0][0] for call in mock_health.call_args_list]
            assert any("_go_1" in name for name in checked)

    def test_go_granular_modes_check_go_container(self) -> None:
        """lint-go/unit-go/int-go all health-check the go container."""
        for mode in ("lint-go", "unit-go", "int-go"):
            with (
                patch("_lifecycle._run"),
                patch(
                    "_lifecycle._container_healthy", return_value=True
                ) as mock_health,
                patch("_lifecycle.time.sleep"),
            ):
                lifecycle_up(mode)
                checked = [call[0][0] for call in mock_health.call_args_list]
                assert any("_go_1" in name for name in checked), mode

    def test_stops_containers(self) -> None:
        with patch("_lifecycle._run") as mock_run:
            mock_run.return_value = MagicMock(stdout="", returncode=0)
            lifecycle_down("all")
            all_call_str = str(mock_run.call_args_list)
            assert "down" in all_call_str

    def test_uses_remove_orphans(self) -> None:
        """With per-run isolation, lifecycle_down MUST use --remove-orphans to completely clean up."""
        with patch("_lifecycle._run") as mock_run:
            mock_run.return_value = MagicMock(stdout="", returncode=0)
            lifecycle_down("all")
            all_call_str = str(mock_run.call_args_list)
            assert "--remove-orphans" in all_call_str

    def test_force_kill_pod(self) -> None:
        """Force-kill aggressively removes the pod since it is isolated per run."""
        with patch("_lifecycle._run") as mock_run:
            mock_run.return_value = MagicMock(stdout="", returncode=0)
            lifecycle_down("web")
            all_call_str = str(mock_run.call_args_list)
            # Must remove the pod
            assert "pod" in all_call_str and "rm" in all_call_str

    def test_no_volume_cleanup(self) -> None:
        """lifecycle_down must NOT remove shared cache volumes (used by other agents).
        Specifically ensures -v and --volumes flags are NOT passed to compose down.
        """
        with patch("_lifecycle._run") as mock_run:
            mock_run.return_value = MagicMock(stdout="", returncode=0)
            lifecycle_down("all")
            all_call_str = str(mock_run.call_args_list)
            assert "volume" not in all_call_str
            assert "'-v'" not in all_call_str
            assert "'--volumes'" not in all_call_str


class TestStartWatchdog:
    def test_spawns_detached_process(self) -> None:
        with patch("subprocess.Popen") as mock_popen:
            mock_proc = MagicMock()
            mock_proc.pid = 99999
            mock_popen.return_value = mock_proc
            result = start_watchdog(60, 12345)
            assert result == 99999
            kwargs = mock_popen.call_args.kwargs
            assert kwargs["start_new_session"] is True
            assert kwargs["stdin"] == subprocess.DEVNULL
            assert kwargs["stdout"] == subprocess.DEVNULL
            assert kwargs["stderr"] == subprocess.DEVNULL

    def test_uses_correct_parent_pid(self) -> None:
        with patch("subprocess.Popen") as mock_popen:
            mock_proc = MagicMock()
            mock_proc.pid = 1
            mock_popen.return_value = mock_proc
            start_watchdog(60, 54321)
            cmd_str = " ".join(mock_popen.call_args.args[0])
            assert "54321" in cmd_str


class TestLifecycleComposeCommand:
    """Verify lifecycle_up/down always use correct compose base (bug: bun_init leak)."""

    def test_lifecycle_down_uses_base_not_env_override(self) -> None:
        """lifecycle_down must use DOCKER_COMPOSE_BASE with -p and -f in its down command."""
        with (
            patch("_lifecycle._run") as mock_run,
            patch.dict(
                "os.environ",
                {
                    "DOCKER_COMPOSE_CMD": "podman-compose",
                    "COMPOSE_PROJECT_NAME": "leeattend-test",
                },
            ),
        ):
            mock_run.return_value = MagicMock(stdout="", returncode=0)
            lifecycle_down("web")
            # The down command is no longer the very last call (due to sweep_stale_environments)
            down_cmd = ""
            for call in mock_run.call_args_list:
                cmd = " ".join(call[0][0])
                if "down" in cmd:
                    down_cmd = cmd
                    break
            assert "-p" in down_cmd
            assert "leeattend-test" in down_cmd
            assert "-f" in down_cmd
            assert "docker-compose.test.yml" in down_cmd

    @patch("_lifecycle.sweep_stale_environments")
    def test_lifecycle_up_web_uses_task_profiles_not_frontend(
        self, mock_sweep: MagicMock
    ) -> None:
        """lifecycle_up(web) must use lint-web, unit-web, e2e-web (not frontend)."""
        with (
            patch("_lifecycle._run") as mock_run,
            patch("_lifecycle._container_healthy", return_value=True),
            patch("_lifecycle.time.sleep"),
            patch.dict(os.environ, {"PROFILES": ""}),
        ):
            mock_run.return_value = MagicMock(stdout="", returncode=0)
            lifecycle_up("web")
            # Extract arguments from the first call to _run (which should be the up -d command)
            first_call_args = mock_run.call_args_list[0]
            up_cmd = " ".join(first_call_args[0][0])
            assert "web" in up_cmd

            # Verify that silent=True is passed to prevent PTY hang
            kwargs = first_call_args[1]
            assert kwargs.get("silent") is True

    def test_run_executes_subprocess(self) -> None:
        """Verify _run executes subprocess with correct args and handles check correctly."""
        from _lifecycle import _run

        with patch("subprocess.run") as mock_sub_run:
            mock_sub_run.return_value = MagicMock(returncode=0, stdout="test")
            result = _run(["echo", "test"], capture=True, check=False, silent=False)
            assert result.stdout == "test"
            assert mock_sub_run.called

            # Test silent
            _run(["echo", "test"], silent=True)
            assert mock_sub_run.call_args[1].get("stdout") == subprocess.DEVNULL

    @patch("_lifecycle.sweep_stale_environments")
    def test_lifecycle_up_has_correct_compose_base(self, mock_sweep: MagicMock) -> None:
        """lifecycle_up must use DOCKER_COMPOSE_BASE with -p and -f."""
        with (
            patch("_lifecycle._run") as mock_run,
            patch("_lifecycle._container_healthy", return_value=True),
            patch("_lifecycle.time.sleep"),
            patch.dict(
                "os.environ",
                {
                    "DOCKER_COMPOSE_CMD": "wrong-value-only",
                    "COMPOSE_PROJECT_NAME": "leeattend-test",
                },
            ),
        ):
            mock_run.return_value = MagicMock(stdout="", returncode=0)
            lifecycle_up("api")
            up_cmd = " ".join(mock_run.call_args_list[0][0][0])
            # Must use correct compose file, not env override
            assert "docker-compose.test.yml" in up_cmd
            assert "leeattend-test" in up_cmd


class TestStaleSweep:
    @patch("pathlib.Path.exists")
    @patch("os.open")
    @patch("fcntl.flock")
    @patch("os.close")
    @patch("pathlib.Path.unlink")
    def test_is_project_locked(
        self,
        mock_unlink: MagicMock,
        mock_close: MagicMock,
        mock_flock: MagicMock,
        mock_open: MagicMock,
        mock_exists: MagicMock,
    ) -> None:
        from _lifecycle import _is_project_locked

        # Test 1: File does not exist -> Not locked
        mock_exists.return_value = False
        assert _is_project_locked("proj") is False

        # Test 2: File exists but os.open fails -> Not locked
        mock_exists.return_value = True
        mock_open.side_effect = OSError
        assert _is_project_locked("proj") is False
        mock_open.side_effect = None

        # Test 3: Can acquire lock -> Not locked
        mock_open.return_value = 123
        assert _is_project_locked("proj") is False
        assert mock_unlink.called

        # Test 4: Cannot acquire lock -> Locked
        mock_flock.side_effect = OSError
        assert _is_project_locked("proj") is True

    @patch("_lifecycle._run")
    @patch("_lifecycle._get_engine")
    @patch("_lifecycle._is_project_locked")
    def test_sweep_stale_environments_podman(
        self, mock_is_locked: MagicMock, mock_engine: MagicMock, mock_run: MagicMock
    ) -> None:
        from _lifecycle import sweep_stale_environments
        from unittest.mock import MagicMock

        mock_engine.return_value = "podman"
        mock_is_locked.return_value = False

        mock_run.return_value = MagicMock(
            stdout="pod_leeattend-test-12345678\nleeattend-test-87654321_db_system_1"
        )
        sweep_stale_environments()
        # Should call pod rm -f and rm -f
        assert mock_run.call_count == 4  # ps, rm pods, ps, rm containers

    @patch("_lifecycle._run")
    @patch("_lifecycle._get_engine")
    @patch("_lifecycle._is_project_locked")
    def test_sweep_stale_environments_docker(
        self, mock_is_locked: MagicMock, mock_engine: MagicMock, mock_run: MagicMock
    ) -> None:
        from _lifecycle import sweep_stale_environments
        from unittest.mock import MagicMock

        mock_engine.return_value = "docker"
        mock_is_locked.return_value = False

        mock_run.return_value = MagicMock(stdout="leeattend-test-12345678_db_system_1")
        sweep_stale_environments()
        assert mock_run.call_count == 2  # ps, rm containers

    @patch("_lifecycle._run")
    def test_sweep_stale_environments_exception(self, mock_run: MagicMock) -> None:
        from _lifecycle import sweep_stale_environments

        mock_run.side_effect = Exception("Test error")
        # Should catch and ignore
        sweep_stale_environments()


class TestLifecycleDependencies:
    @staticmethod
    def _compose_result(stdout="", returncode=0):
        return subprocess.CompletedProcess([], returncode, stdout=stdout, stderr="")

    def test_int_go_starts_and_checks_dependency(self):
        def run(cmd, **kwargs):
            if "config" in cmd:
                return self._compose_result("go\npostgres\n")
            if "ps" in cmd and "-q" in cmd:
                return self._compose_result("postgres-container-id\n")
            return self._compose_result()

        with (
            patch("_lifecycle._run", side_effect=run) as mock_run,
            patch(
                "_lifecycle.resolve_lifecycle_dependencies", return_value=["postgres"]
            ),
            patch("_lifecycle._container_healthy", return_value=True) as mock_health,
            patch("_lifecycle.time.sleep"),
            patch.dict(os.environ, {"COMPOSE_PROJECT_NAME": "run-int-go"}),
        ):
            assert lifecycle_up("int-go") is True

        up_cmds = [
            call.args[0] for call in mock_run.call_args_list if "up" in call.args[0]
        ]
        assert up_cmds[0][-2:] == ["up", "-d"]
        assert up_cmds[1][-1] == "postgres"
        assert "--no-deps" not in up_cmds[0]
        assert "--network" not in up_cmds[1]
        assert "depends_on" not in up_cmds[1]
        assert "postgres-container-id" in [
            call.args[0] for call in mock_health.call_args_list
        ]

    def test_unit_go_does_not_start_or_check_configured_int_go_dependency(self):
        def dependencies(mode):
            return ["postgres"] if mode == "int-go" else []

        with (
            patch("_lifecycle._run") as mock_run,
            patch(
                "_lifecycle.resolve_lifecycle_dependencies", side_effect=dependencies
            ),
            patch("_lifecycle._container_healthy", return_value=True) as mock_health,
            patch("_lifecycle.time.sleep"),
        ):
            assert lifecycle_up("unit-go") is True

        up_cmd = mock_run.call_args_list[0].args[0]
        assert "--no-deps" in up_cmd
        assert "postgres" not in up_cmd
        assert all(
            "postgres" not in call.args[0] for call in mock_health.call_args_list
        )

    def test_multiple_dependencies_are_started_and_checked(self):
        def run(cmd, **kwargs):
            if "config" in cmd:
                return self._compose_result("go\npostgres\nredis\n")
            if "ps" in cmd and "-q" in cmd:
                return self._compose_result(f"{cmd[-1]}-id\n")
            return self._compose_result()

        with (
            patch("_lifecycle._run", side_effect=run) as mock_run,
            patch(
                "_lifecycle.resolve_lifecycle_dependencies",
                return_value=["postgres", "redis", "postgres"],
            ),
            patch("_lifecycle._container_healthy", return_value=True) as mock_health,
            patch("_lifecycle.time.sleep"),
        ):
            assert lifecycle_up("int-go") is True

        up_cmds = [
            call.args[0] for call in mock_run.call_args_list if "up" in call.args[0]
        ]
        assert up_cmds[0][-2:] == ["up", "-d"]
        assert up_cmds[1][-2:] == ["postgres", "redis"]
        checked = [call.args[0] for call in mock_health.call_args_list]
        assert "postgres-id" in checked
        assert "redis-id" in checked

    def test_unknown_dependency_fails_when_compose_metadata_available(self):
        with (
            patch(
                "_lifecycle._run",
                return_value=self._compose_result("go\nredis\n"),
            ),
            patch(
                "_lifecycle.resolve_lifecycle_dependencies", return_value=["postgres"]
            ),
        ):
            with pytest.raises(ValueError, match="postgres.*int-go"):
                lifecycle_up("int-go")

    def test_timeout_identifies_dependency_service(self, capsys):
        def run(cmd, **kwargs):
            if "config" in cmd:
                return self._compose_result("go\npostgres\n")
            if "ps" in cmd and "-q" in cmd:
                return self._compose_result(f"{cmd[-1]}-container-id\n")
            if cmd[1:3] == ["ps", "-a"]:
                return self._compose_result("postgres-container-id\n")
            return self._compose_result()

        with (
            patch("_lifecycle._run", side_effect=run),
            patch(
                "_lifecycle.resolve_lifecycle_dependencies", return_value=["postgres"]
            ),
            patch("_lifecycle._container_healthy", side_effect=[True] + [False] * 60),
            patch("_lifecycle.time.sleep"),
        ):
            assert lifecycle_up("int-go") is False

        output = capsys.readouterr().out
        assert "dependency service 'postgres'" in output
        assert "postgres-container-id" in output

    @pytest.mark.parametrize("compose_tool", ["docker compose", "podman-compose"])
    def test_compose_tool_paths_preserve_base_command(self, compose_tool):
        with (
            patch(
                "_lifecycle.bootstrap_env",
                return_value={
                    "DOCKER_COMPOSE_BASE": f"{compose_tool} -p run -f compose.yml"
                },
            ),
            patch("_lifecycle._run") as mock_run,
            patch("_lifecycle.resolve_lifecycle_dependencies", return_value=[]),
            patch("_lifecycle._container_healthy", return_value=True),
            patch("_lifecycle.time.sleep"),
        ):
            assert lifecycle_up("go") is True

        assert (
            mock_run.call_args_list[0].args[0][: len(compose_tool.split())]
            == compose_tool.split()
        )

    def test_real_compose_dependency_lifecycle(self, tmp_path, monkeypatch):
        if os.environ.get("LEEDEVKIT_RUN_COMPOSE_INTEGRATION") != "1":
            pytest.skip(
                "set LEEDEVKIT_RUN_COMPOSE_INTEGRATION=1 to run Compose integration"
            )

        from _bootstrap import detect_compose_cmd

        compose_cmd = detect_compose_cmd()
        if any(shutil.which(part) is None for part in compose_cmd):
            pytest.skip("Docker Compose or Podman Compose is unavailable")

        compose_file = tmp_path / "compose.yml"
        compose_file.write_text(
            """services:
  go:
    image: busybox:1.36
    command: [\"sh\", \"-c\", \"sleep 60\"]
    healthcheck:
      test: [\"CMD-SHELL\", \"true\"]
      interval: 1s
      timeout: 1s
      retries: 10
  dep:
    image: busybox:1.36
    command: [\"sh\", \"-c\", \"sleep 60\"]
    healthcheck:
      test: [\"CMD-SHELL\", \"true\"]
      interval: 1s
      timeout: 1s
      retries: 10
"""
        )
        project = f"leedevkit-lifecycle-{os.getpid()}"
        base = [
            *compose_cmd,
            "-p",
            project,
            "-f",
            str(compose_file),
        ]
        env = {
            "COMPOSE_PROJECT_NAME": project,
            "PODMAN_COMPOSE_PROJECT_NAME": project,
            "DOCKER_COMPOSE_BASE": " ".join(base),
        }
        monkeypatch.setenv("COMPOSE_PROJECT_NAME", project)
        monkeypatch.setattr(_lifecycle, "bootstrap_env", lambda mode="all": env)
        monkeypatch.setattr(_lifecycle, "resolve_lifecycle_profiles", lambda mode: [])
        monkeypatch.setattr(
            _lifecycle, "resolve_lifecycle_dependencies", lambda mode: ["dep"]
        )
        try:
            assert lifecycle_up("go") is True
        finally:
            subprocess.run(
                base + ["down", "--volumes", "--remove-orphans"],
                check=False,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                timeout=120,
            )
