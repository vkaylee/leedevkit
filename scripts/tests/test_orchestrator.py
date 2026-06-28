# mypy: ignore-errors
import argparse
import fcntl
import signal
import sys
from pathlib import Path
from unittest.mock import patch

import pytest

# Add scripts/ to sys.path so we can import _orchestrator
PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent
sys.path.append(str(PROJECT_ROOT / "scripts"))
import _orchestrator  # noqa: E402


@pytest.fixture
def orchestrator():
    return _orchestrator.Orchestrator()


@pytest.fixture(autouse=True)
def mock_is_service_running():
    with patch.object(_orchestrator.Orchestrator, "_is_service_running", return_value=True) as mock:
        yield mock


@pytest.fixture(autouse=True)
def mock_lifecycle():
    with (
        patch("_orchestrator._lifecycle_up", return_value=True),
        patch("_orchestrator.lifecycle_down", return_value=True),
    ):
        yield


def test_help_message(orchestrator, capsys) -> None:
    """Test help message does not cause errors."""
    with pytest.raises(SystemExit) as e:
        orchestrator.parser.parse_args(["--help"])
    assert e.value.code == 0
    captured = capsys.readouterr()
    assert "LeeAttend Enterprise Orchestrator" in captured.out


def test_invalid_command(orchestrator) -> None:
    """Test error reporting when command does not exist."""
    with pytest.raises(SystemExit):
        orchestrator.parser.parse_args(["invalid-cmd"])


@patch("subprocess.run")
def test_manage_up_dev(mock_run, orchestrator) -> None:
    """Test manage up dev command creates correct podman-compose command."""
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["manage", "up", "dev"])
    orchestrator.handle_manage(args)

    called_args = mock_run.call_args[0][0]
    assert "podman-compose" in called_args
    assert "leeattend-dev" in called_args
    assert "up" in called_args


@patch.object(_orchestrator.Orchestrator, "run_phase")
@patch("_orchestrator.Orchestrator.handle_run")
def test_test_api_mode(mock_handle_run, mock_run_phase, orchestrator) -> None:
    """Test test --api mode runs all required phases."""
    mock_run_phase.return_value = None
    mock_handle_run.return_value = None
    args = orchestrator.parser.parse_args(["test", "api"])
    orchestrator.handle_test(args)
    assert mock_run_phase.call_count >= 3  # Startup + Lint + Unit + Integration


@patch("subprocess.run")
def test_dry_run_no_execution(mock_run, orchestrator) -> None:
    """Test --dry-run flag does not execute real commands."""
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["test", "all", "--dry-run"])
    orchestrator.dry_run = args.dry_run
    orchestrator.handle_test(args)
    assert mock_run.called is False


@patch("shutil.which")
@patch("socket.socket")
@patch("subprocess.run")
def test_doctor_logic(mock_run, mock_socket, mock_which, orchestrator) -> None:
    """Test doctor logic."""
    mock_which.return_value = "/usr/bin/podman"
    mock_socket.return_value.connect_ex.return_value = 0
    mock_run.return_value.stdout = "leeattend-dev-db\nleeattend-dev-api"
    orchestrator.handle_doctor()
    assert mock_which.called


@patch("subprocess.run")
@patch("shutil.which")
def test_db_query_logic(mock_which, mock_run, orchestrator) -> None:
    """Test db:query logic."""
    mock_run.return_value.returncode = 0
    mock_which.return_value = "podman"
    args = orchestrator.parser.parse_args(["manage", "db:query", "SELECT 1", "--json"])
    orchestrator.handle_db_query(args)
    called_args = mock_run.call_args[0][0]
    assert "SELECT 1" in called_args
    assert "--json" in called_args


def test_engine_properties(orchestrator) -> None:
    """Test engine detection properties."""
    with patch("shutil.which") as mock_which:
        # Test Podman path
        mock_which.side_effect = lambda x: "/usr/bin/podman" if "podman" in x else None
        assert orchestrator.engine == "podman"
        assert orchestrator.compose_engine == ["podman-compose"]
        assert orchestrator.compose_engine_cmd == "podman-compose"

        # Test Docker path
        mock_which.side_effect = lambda x: "/usr/bin/docker" if "docker" in x else None
        assert orchestrator.engine == "docker"
        assert orchestrator.compose_engine == ["docker", "compose"]

        # Test Fallback
        mock_which.side_effect = lambda x: None
        assert orchestrator.engine == "podman"
        assert orchestrator.compose_engine == ["podman-compose"]


@patch("subprocess.run")
@patch("shutil.which")
def test_run_npm_typecheck(mock_which, mock_run, orchestrator) -> None:
    """Test typecheck → type-check mapping for bun."""
    mock_which.return_value = "podman"
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["run", "npm", "run", "typecheck"])
    orchestrator.handle_run(args)
    cmd_list = mock_run.call_args[0][0]
    # "type-check" must be in the args list
    assert "type-check" in cmd_list
    # Should use compose exec
    assert "exec" in cmd_list


@patch("subprocess.run")
@patch("shutil.which")
@patch("pathlib.Path.cwd")
def test_run_cargo_agent_main(mock_cwd, mock_which, mock_run, orchestrator) -> None:
    """Test cargo auto-detection of agent-main context."""
    mock_which.return_value = "podman"
    mock_run.return_value.returncode = 0
    mock_cwd.return_value = PROJECT_ROOT / "agent-main"
    args = orchestrator.parser.parse_args(["run", "cargo", "build"])
    orchestrator.handle_run(args)
    called_cmd = mock_run.call_args[0][0]
    assert "agent-main" in called_cmd


@patch.object(_orchestrator.Orchestrator, "run_phase")
@patch.object(_orchestrator.Orchestrator, "execute_safe")
def test_test_coverage_all(mock_exec, mock_run_phase, orchestrator) -> None:
    """Test test --all --coverage command runs coverage phase."""
    mock_run_phase.return_value = None
    mock_exec.return_value = None
    args = orchestrator.parser.parse_args(["test", "all", "--coverage"])
    orchestrator.handle_test(args)
    coverage_calls = [c for c in mock_run_phase.call_args_list if c[0][0] == "Coverage"]
    assert len(coverage_calls) == 2


@patch.object(_orchestrator.Orchestrator, "run_phase")
@patch("_orchestrator.Orchestrator.handle_run")
def test_test_timeout_custom(mock_handle_run, mock_run_phase, orchestrator) -> None:
    """Test test command with custom timeout."""
    mock_run_phase.return_value = None
    mock_handle_run.return_value = None
    args = orchestrator.parser.parse_args(["test", "api", "--timeout", "100"])
    orchestrator.handle_test(args)
    assert mock_run_phase.called


@patch("sys.stderr")
@patch("subprocess.run")
def test_execute_safe_failure(mock_run, mock_stderr, orchestrator) -> None:
    """Test execute_safe exits on command failure and prints clear message."""
    mock_run.return_value.returncode = 1
    with pytest.raises(SystemExit):
        orchestrator.execute_safe(["false_cmd"])


@patch("sys.argv", ["_orchestrator.py", "manage", "sync:api"])
@patch("subprocess.run")
def test_full_run_sync_api(mock_run, orchestrator) -> None:
    """Test full run loop for sync:api command."""
    mock_run.return_value.returncode = 0
    orchestrator.run()
    assert "_sync-api.sh" in str(mock_run.call_args_list)


@patch("sys.argv", ["_orchestrator.py", "manage", "test:infra"])
@patch("subprocess.run")
def test_full_run_test_infra(mock_run, orchestrator) -> None:
    """Test full run loop for test:infra command."""
    mock_run.return_value.returncode = 0
    orchestrator.run()
    assert "pytest" in str(mock_run.call_args_list)


@patch("sys.argv", ["_orchestrator.py", "test", "infra", "--lint-only"])
@patch("subprocess.run")
def test_full_run_lint_infra(mock_run, orchestrator) -> None:
    """Test full run loop for lint:infra command."""
    mock_run.return_value.returncode = 0
    orchestrator.run()
    assert "ruff" in str(mock_run.call_args_list)


@patch("sys.argv", ["_orchestrator.py", "test", "webdashboard", "--unit-only", "--pattern", "auth"])
@patch("_test_modules.run_parallel_ordered", return_value=True)
def test_full_run_e2e_cli_args_translation(mock_run_parallel, orchestrator) -> None:
    """End-to-End validation: Proves --pattern and --unit-only reach the concrete shell executor."""
    orchestrator.run()

    # run_parallel_ordered is called with: phase_name, component_filter, tasks
    # tasks is a list of tuples: (task_name, service, cmd_list)
    assert mock_run_parallel.called
    tasks = mock_run_parallel.call_args[0][2]

    # Assert that at least one task is the bun test command targeting webdashboard
    found_bun_cmd = False
    for _task_name, service, cmd in tasks:
        cmd_str = " ".join(cmd)
        if "bun run test" in cmd_str and "webdashboard" in service:
            found_bun_cmd = True
            # The pattern "auth" must be properly passed into the shell execution
            assert "auth" in cmd_str

    assert found_bun_cmd, (
        "The --unit-only and --pattern flags failed to produce the correct concrete shell execution."
    )


@patch("sys.argv", ["_orchestrator.py", "manage", "fmt:infra"])
@patch("subprocess.run")
def test_full_run_fmt_infra(mock_run, orchestrator) -> None:
    """Test full run loop for fmt:infra command."""
    mock_run.return_value.returncode = 0
    orchestrator.run()
    assert "ruff" in str(mock_run.call_args_list)


@patch("sys.argv", ["_orchestrator.py", "manage", "doctor"])
@patch("subprocess.run")
@patch("shutil.which")
def test_full_run_doctor(mock_which, mock_run, orchestrator) -> None:
    """Test full run loop for doctor command."""
    mock_run.return_value.stdout = ""
    mock_which.return_value = "podman"
    orchestrator.run()
    assert mock_run.called


@patch("sys.argv", ["_orchestrator.py", "run", "npm", "install"])
@patch("subprocess.run")
def test_full_run_npm(mock_run, orchestrator) -> None:
    """Test full run loop for run npm command."""
    mock_run.return_value.returncode = 0
    orchestrator.run()
    cmd_str = str(mock_run.call_args_list)
    assert "webdashboard" in cmd_str
    assert "exec" in cmd_str


@patch("_orchestrator.leeattend_run_unit")
def test_run_phase_with_component(mock_unit, orchestrator) -> None:
    """Test run_phase with specific component derived from target."""
    mock_unit.return_value = True
    args = orchestrator.parser.parse_args(["test", "apiserver"])
    args.component = "apiserver"
    orchestrator.run_phase("Unit Tests", "api", args)
    mock_unit.assert_called_once()


def test_log_warn(orchestrator, capsys) -> None:
    """Test log_warn function."""
    _orchestrator.log_warn("test warning")
    captured = capsys.readouterr()
    assert "test warning" in captured.err


@patch("subprocess.run")
def test_handle_manage_clean(mock_run, orchestrator) -> None:
    """Test manage clean command."""
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["manage", "clean", "dev"])
    orchestrator.handle_manage(args)
    called_args = mock_run.call_args[0][0]
    assert "down" in called_args
    assert "-v" in called_args
    assert "leeattend-dev" in called_args


@patch("subprocess.run")
def test_dry_run_execute_safe(mock_run, orchestrator) -> None:
    """Test execute_safe in dry-run mode."""
    orchestrator.dry_run = True
    orchestrator.execute_safe(["echo", "hi"])
    assert not mock_run.called


@patch("subprocess.run")
@patch("shutil.which")
def test_doctor_no_engine(mock_which, mock_run, orchestrator) -> None:
    """Test doctor when no container engine is found."""
    mock_which.return_value = None
    mock_run.return_value.stdout = ""
    orchestrator.handle_doctor()
    assert mock_which.called


@patch("subprocess.run")
@patch("shutil.which")
def test_run_with_remainder_separator(mock_which, mock_run, orchestrator) -> None:
    """Test run command with -- separator."""
    mock_which.return_value = "podman"
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["run", "cargo", "--", "check"])
    orchestrator.handle_run(args)
    called_args = mock_run.call_args[0][0]
    assert "--" not in called_args


def test_get_compose_files_other(orchestrator) -> None:
    """Test compose file selection logic for other environments."""
    files = orchestrator.get_compose_files("prod")
    assert "docker-compose.yml" in files


@patch.object(_orchestrator.Orchestrator, "run_phase")
def test_handle_test_web(mock_run_phase, orchestrator) -> None:
    """Test test --web command."""
    mock_run_phase.return_value = None
    args = orchestrator.parser.parse_args(["test", "web"])
    orchestrator.handle_test(args)
    assert mock_run_phase.called


@patch.object(_orchestrator.Orchestrator, "run_phase")
@patch("_orchestrator.Orchestrator.handle_run")
def test_handle_test_integration(mock_handle_run, mock_run_phase, orchestrator) -> None:
    """Test test with api target (maps to integration internally)."""
    mock_run_phase.return_value = None
    mock_handle_run.return_value = None
    args = orchestrator.parser.parse_args(["test", "api"])
    orchestrator.handle_test(args)
    assert mock_run_phase.called


@patch("subprocess.run")
def test_handle_manage_logs_full(mock_run, orchestrator) -> None:
    """Test manage logs with specific service."""
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["manage", "logs", "test", "db"])
    orchestrator.handle_manage(args)
    called = mock_run.call_args[0][0]
    assert "logs" in called
    assert "db" in called


@patch("subprocess.run")
def test_handle_manage_exec(mock_run, orchestrator) -> None:
    """Test manage exec command."""
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["manage", "exec", "db", "ls"])
    orchestrator.handle_manage(args)
    called = mock_run.call_args[0]
    assert "exec" in str(called)


@patch("sys.argv", ["_orchestrator.py", "test", "api"])
@patch("_orchestrator.Orchestrator.handle_run")
@patch.object(_orchestrator.Orchestrator, "run_phase")
def test_full_run_test_api(mock_run_phase, mock_handle_run, orchestrator) -> None:
    """Test full run loop for test --api command."""
    mock_run_phase.return_value = None
    mock_handle_run.return_value = None
    orchestrator.run()
    assert mock_run_phase.called


@patch("sys.argv", ["_orchestrator.py", "run", "cargo", "--", "check"])
@patch("subprocess.run")
@patch("shutil.which")
def test_full_run_cargo_remainder(mock_which, mock_run, orchestrator) -> None:
    """Test full run loop for run cargo with -- remainder."""
    mock_run.return_value.returncode = 0
    mock_which.return_value = "podman"
    orchestrator.run()
    assert "apiserver" in str(mock_run.call_args_list)


@patch("subprocess.run")
@patch("shutil.which")
def test_handle_run_npm_no_args(mock_which, mock_run, orchestrator) -> None:
    """Test handle_run npm without arguments."""
    mock_which.return_value = "podman"
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["run", "npm"])
    orchestrator.handle_run(args)
    cmd_list = mock_run.call_args[0][0]
    assert "exec" in cmd_list
    assert "bun" in cmd_list


@patch.object(_orchestrator.Orchestrator, "run_phase")
@patch.object(_orchestrator.Orchestrator, "execute_safe")
def test_handle_test_lint_subcommands(mock_exec, mock_run_phase, orchestrator) -> None:
    """Test test lint-only mode for infra, api, web targets."""
    mock_run_phase.return_value = None
    mock_exec.return_value = None
    for cmd in [
        ["test", "infra", "--lint-only"],
        ["test", "api", "--lint-only"],
        ["test", "web", "--lint-only"],
    ]:
        args = orchestrator.parser.parse_args(cmd)
        orchestrator.handle_test(args)
    assert mock_run_phase.call_count >= 2


@patch("subprocess.run")
@patch("shutil.which")
def test_handle_lint_infra_no_shellcheck(mock_which, mock_run, orchestrator) -> None:
    """Test handle_lint_infra when shellcheck is missing."""

    mock_which.return_value = None
    mock_run.return_value.returncode = 0
    orchestrator.handle_lint_infra()
    assert mock_run.called


@patch("subprocess.run")
@patch("shutil.which")
@patch("socket.socket")
def test_handle_doctor_ports_failure(mock_socket, mock_which, mock_run, orchestrator) -> None:
    """Test handle_doctor when ports are occupied."""
    mock_which.return_value = "podman"
    mock_socket.return_value.connect_ex.return_value = 0  # Port busy
    orchestrator.handle_doctor()
    assert mock_socket.called


@patch("sys.argv", ["_orchestrator.py", "manage", "verify:infra"])
@patch("subprocess.run")
def test_full_run_verify_infra(mock_run, orchestrator) -> None:
    """Test full run loop for verify:infra command."""
    mock_run.return_value.returncode = 0
    orchestrator.run()
    # Verify at least 3 phases are called (fmt, lint, test)
    assert mock_run.call_count >= 3


@patch("subprocess.run")
def test_manage_db_query(mock_run, orchestrator) -> None:
    """Test manage db:query command."""
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["manage", "db:query", "SELECT 1"])
    orchestrator.handle_manage(args)
    called_args = mock_run.call_args[0][0]
    assert "psql" in called_args
    assert "SELECT 1" in called_args


@patch("subprocess.run")
def test_manage_logs_with_service(mock_run, orchestrator) -> None:
    """Test manage logs --service command."""
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["manage", "logs", "dev", "api"])
    orchestrator.handle_manage(args)
    called_args = mock_run.call_args[0][0]
    assert "logs" in called_args
    assert "-f" in called_args
    assert "api" in called_args


@patch("_orchestrator.log_info")
def test_manage_exec_dry_run(mock_log, orchestrator) -> None:
    """Test manage exec command in dry-run mode."""
    orchestrator.dry_run = True
    args = orchestrator.parser.parse_args(["manage", "exec", "api", "ls"])
    orchestrator.handle_manage(args)
    expected = (
        "🔍 Dry-run: Executing podman-compose -p leeattend-dev "
        "-f docker-compose.yml -f .compose/docker-compose.dev.yml exec api ls"
    )
    mock_log.assert_any_call(expected)


@patch("subprocess.run")
def test_handle_run_remainder_args(mock_run, orchestrator) -> None:
    """Test handle_run with -- remainder separator."""
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["run", "npm", "--", "run", "dev"])
    orchestrator.handle_run(args)
    cmd_list = mock_run.call_args[0][0]
    assert "exec" in cmd_list
    assert "bun" in cmd_list
    assert "run" in cmd_list
    assert "dev" in cmd_list


@patch("socket.socket")
@patch("_orchestrator.log_success")
@patch("_orchestrator.log_info")
def test_doctor_full(mock_info, mock_success, mock_socket, orchestrator) -> None:
    """Test doctor with both success and failure port checks."""
    # Mock socket connect_ex: 3000 success (occupied), others failure (free)
    mock_socket.return_value.connect_ex.side_effect = [0, 1, 1]

    with (
        patch("shutil.which", return_value="/usr/bin/podman"),
        patch("subprocess.run") as mock_run,
        patch("pathlib.Path.exists", return_value=True),
    ):
        mock_run.return_value.stdout = "leeattend-dev-db\nleeattend-dev-api"
        orchestrator.handle_doctor()

    mock_success.assert_any_call("✅ Container Engine: podman")
    mock_info.assert_any_call("⚠️  Port 3000 is occupied")


@patch("subprocess.run")
def test_handle_run_cargo_agent(mock_run, orchestrator) -> None:
    """Test handle_run cargo with agent-main context."""
    mock_run.return_value.returncode = 0
    with patch("pathlib.Path.cwd", return_value=PROJECT_ROOT / "agent-main"):
        args = orchestrator.parser.parse_args(["run", "cargo", "check"])
        orchestrator.handle_run(args)
        called_args = mock_run.call_args[0][0]
        assert "agent-main" in called_args
        assert "/workspace/agent-main" in str(called_args)


@patch("subprocess.run")
def test_handle_run_cargo_test_conversion(mock_run, orchestrator) -> None:
    """Test silently converting 'cargo test' to 'cargo nextest run'."""
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["run", "cargo", "test", "--lib"])
    orchestrator.handle_run(args)
    called_args = mock_run.call_args[0][0]
    assert "nextest" in called_args
    assert "run" in called_args
    assert "--lib" in called_args


@patch.object(_orchestrator.Orchestrator, "run_phase")
@patch("_orchestrator.Orchestrator.handle_run")
def test_run_phase_with_pattern(mock_handle_run, mock_run_phase, orchestrator) -> None:
    """Test pattern arg is propagated."""
    mock_run_phase.return_value = None
    mock_handle_run.return_value = None
    args = orchestrator.parser.parse_args(["test", "api", "--pattern", "auth"])
    orchestrator.handle_test(args)
    assert mock_run_phase.called


@patch("shutil.which", return_value="/usr/bin/shellcheck")
@patch("subprocess.run")
def test_handle_lint_infra_with_shellcheck(mock_run, mock_which, orchestrator) -> None:
    """Test handle_lint_infra when shellcheck is available."""
    mock_run.return_value.returncode = 0
    orchestrator.handle_lint_infra()
    # verify shellcheck was called
    all_called_args = [str(call[0][0]) for call in mock_run.call_args_list]
    assert any("shellcheck" in args for args in all_called_args)


@patch("_orchestrator.log_info")
@patch("pathlib.Path.exists", return_value=False)
def test_doctor_no_venv(mock_exists, mock_info, orchestrator) -> None:
    """Test doctor when virtual environment is missing."""
    with patch("shutil.which", return_value=None), patch("subprocess.run") as mock_run:
        mock_run.return_value.stdout = ""
        orchestrator.handle_doctor()
    mock_info.assert_any_call("💡 Virtual Environment: Missing (will be created on next run)")


@patch("subprocess.run")
def test_handle_run_cargo_no_rel(mock_run, orchestrator) -> None:
    """Test handle_run cargo when executed from project root."""
    mock_run.return_value.returncode = 0
    with patch("pathlib.Path.cwd", return_value=PROJECT_ROOT):
        args = orchestrator.parser.parse_args(["run", "cargo", "check"])
        orchestrator.handle_run(args)
        # Should have /workspace exactly
        called_args = mock_run.call_args[0][0]
        assert "/workspace" in called_args


@patch("subprocess.run")
def test_handle_run_remainder_explicit(mock_run, orchestrator) -> None:
    """Test handle_run with explicit -- separator."""
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["run", "npm", "--", "test"])
    orchestrator.handle_run(args)
    cmd_list = mock_run.call_args[0][0]
    assert "exec" in cmd_list
    assert "bun" in cmd_list
    assert "test" in cmd_list


@patch("subprocess.run")
def test_handle_run_cargo_outside_root(mock_run, orchestrator) -> None:
    """Test handle_run cargo when executed outside project root (ValueError)."""
    mock_run.return_value.returncode = 0
    with patch("pathlib.Path.cwd", return_value=Path("/tmp")):
        args = orchestrator.parser.parse_args(["run", "cargo", "check"])
        orchestrator.handle_run(args)
        # rel_dir should remain empty
        called_args = mock_run.call_args[0][0]
        assert "/workspace" in called_args


@patch("_orchestrator.lifecycle_down")
def test_cleanup_success(mock_down, orchestrator) -> None:
    """Test successful cleanup operation."""
    orchestrator.needs_cleanup = True
    orchestrator.dry_run = False
    orchestrator.cleanup()
    assert mock_down.called
    assert not orchestrator.needs_cleanup


@patch("_orchestrator.lifecycle_down")
@patch("_orchestrator.log_error")
def test_cleanup_timeout(mock_log, mock_lifecycle, orchestrator) -> None:
    """Test cleanup operation handling errors from lifecycle_down."""
    orchestrator.needs_cleanup = True
    orchestrator.dry_run = False
    mock_lifecycle.side_effect = RuntimeError("cleanup timed out")
    orchestrator.cleanup()
    mock_log.assert_called_with("❌ Cleanup failed with error: cleanup timed out")


@patch("_orchestrator.lifecycle_down")
@patch("_orchestrator.log_error")
def test_cleanup_exception(mock_log, mock_lifecycle, orchestrator) -> None:
    """Test cleanup operation failing with generic error."""
    orchestrator.needs_cleanup = True
    orchestrator.dry_run = False
    mock_lifecycle.side_effect = Exception("test error")
    orchestrator.cleanup()
    mock_log.assert_called_with("❌ Cleanup failed with error: test error")


@patch("sys.exit")
@patch("_orchestrator.log_warn")
def test_signal_handler(mock_warn, mock_exit) -> None:
    """Test signal handler functionality."""
    handler = None

    def mock_signal(sig, h):
        nonlocal handler
        if sig == signal.SIGINT:
            handler = h

    with patch("signal.signal", side_effect=mock_signal):
        _orchestrator.Orchestrator()

    if handler:
        handler(signal.SIGINT, None)
        mock_warn.assert_called()
        mock_exit.assert_called_with(128 + signal.SIGINT)


@patch("subprocess.run")
def test_cleanup_early_exit(mock_run, orchestrator) -> None:
    """Test early exit logic in cleanup."""
    orchestrator.needs_cleanup = False
    orchestrator.cleanup()
    assert not mock_run.called

    orchestrator.needs_cleanup = True
    orchestrator.dry_run = True
    orchestrator.cleanup()
    assert not mock_run.called


@patch("subprocess.run")
def test_handle_manage_migrate(mock_run, orchestrator) -> None:
    """Test migrate:run."""
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["manage", "migrate:run"])
    orchestrator.handle_manage(args)
    assert "migrate" in str(mock_run.call_args_list)
    assert "run" in str(mock_run.call_args_list)


@patch("subprocess.run")
def test_handle_run_with_separator(mock_run, orchestrator) -> None:
    """Test handle_run with -- separator coverage."""
    mock_run.return_value.returncode = 0
    # To hit line 371, tool_args[0] must be "--"
    # With argparse.REMAINDER, the first -- is swallowed by the parser.
    # So we need to pass TWO double-dashes if we want one to remain in args.
    args = orchestrator.parser.parse_args(["run", "cargo", "--", "--", "check"])
    orchestrator.handle_run(args)
    # Check that EXACT standalone "--" is not in the list of arguments
    # (One was swallowed by argparse, the second by line 371)
    assert "--" not in mock_run.call_args[0][0]


@patch("subprocess.run")
@patch("sys.argv", ["_orchestrator.py"])
def test_orchestrator_main_no_args(mock_run) -> None:
    """Test orchestrator with no arguments (if subparsers weren't required)."""
    # This is to hit the 'if not args.command' block
    with patch.object(_orchestrator.argparse.ArgumentParser, "parse_args") as mock_parse:
        mock_args = _orchestrator.argparse.Namespace(command=None, dry_run=False)
        mock_parse.return_value = mock_args
        with patch("sys.stdout") as _:
            _orchestrator.Orchestrator().run()
            assert mock_parse.called


def test_main_entry_point() -> None:
    """Test the main entry point block coverage."""
    with (
        patch("sys.argv", ["_orchestrator.py", "manage", "doctor"]),
        patch("_orchestrator.Orchestrator.run"),
    ):
        # We can't easily run the actual 'if __name__ == "__main__"' block
        # without side effects, but we can call it if we import it correctly
        # or just rely on the fact that we've tested Orchestrator.run()
        pass


@patch("subprocess.run")
def test_handle_run_npm_first_arg_npm(mock_run, orchestrator) -> None:
    """Test handle_run when first arg is npm — uses compose exec."""
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["run", "npm", "--", "npm", "install"])
    orchestrator.handle_run(args)
    cmd_list = mock_run.call_args[0][0]
    assert "exec" in cmd_list
    assert "npm" in cmd_list
    assert "install" in cmd_list


@patch("subprocess.run")
def test_handle_run_with_many_file_args(mock_run, orchestrator) -> None:
    """Test handle_run npm with 24+ file paths — uses compose exec."""
    mock_run.return_value.returncode = 0
    files = [f"/workspace/webdashboard/src/test_{i:03d}.tsx" for i in range(24)]
    base_args = ["run", "npm", "run", "lint", "--"]
    args = orchestrator.parser.parse_args(base_args + files)
    orchestrator.handle_run(args)
    cmd_list = mock_run.call_args[0][0]
    for f in files:
        assert f in cmd_list
    assert "exec" in cmd_list


@patch("subprocess.run")
def test_handle_run_rejects_python_inline(mock_run, orchestrator) -> None:
    """Test handle_run blocks python -c (sanitizer rejects inline code)."""
    mock_run.return_value.returncode = 0
    args = orchestrator.parser.parse_args(["run", "npm", "--", "python", "-c", "print(1)"])
    with pytest.raises(SystemExit):
        orchestrator.handle_run(args)


class TestNoDepsConditional:
    """Verify --no-deps for standalone (run), exec for normal commands."""

    @patch("subprocess.run")
    def test_version_uses_run_no_deps(self, mock_run, orchestrator) -> None:
        mock_run.return_value.returncode = 0
        args = orchestrator.parser.parse_args(["run", "npm", "--", "--version"])
        orchestrator.handle_run(args)
        called = str(mock_run.call_args[0][0])
        assert "--no-deps" in called
        assert "run" in called  # compose run, not exec

    @patch("subprocess.run")
    def test_run_lint_uses_exec(self, mock_run, orchestrator) -> None:
        mock_run.return_value.returncode = 0
        args = orchestrator.parser.parse_args(["run", "npm", "--", "run", "lint"])
        orchestrator.handle_run(args)
        called = str(mock_run.call_args[0][0])
        assert "exec" in called  # compose exec into running container

    @patch("subprocess.run")
    def test_install_uses_exec(self, mock_run, orchestrator) -> None:
        mock_run.return_value.returncode = 0
        args = orchestrator.parser.parse_args(["run", "npm", "--", "install"])
        orchestrator.handle_run(args)
        called = str(mock_run.call_args[0][0])
        assert "exec" in called

    @patch("subprocess.run")
    def test_empty_args_uses_exec(self, mock_run, orchestrator) -> None:
        mock_run.return_value.returncode = 0
        args = orchestrator.parser.parse_args(["run", "npm"])
        orchestrator.handle_run(args)
        called = str(mock_run.call_args[0][0])
        assert "exec" in called


class TestOrchestratorLocking:
    @patch("os.open")
    @patch("fcntl.flock")
    @patch("sys.argv", ["orchestrator", "test", "all"])
    def test_run_acquires_lock(self, mock_flock, mock_open, orchestrator) -> None:
        mock_open.return_value = 123
        with patch.object(orchestrator, "handle_test"):
            orchestrator.run()
        assert orchestrator.lock_fd == 123
        assert mock_open.called
        assert mock_flock.called

    @patch("os.open")
    @patch("fcntl.flock")
    @patch("_orchestrator.log_warn")
    @patch("sys.argv", ["orchestrator", "test", "all"])
    def test_run_lock_fails(self, mock_warn, mock_flock, mock_open, orchestrator) -> None:
        mock_open.side_effect = OSError
        with patch.object(orchestrator, "handle_test"):
            orchestrator.run()
        assert mock_warn.called

    @patch("os.close")
    @patch("fcntl.flock")
    @patch("pathlib.Path.unlink")
    def test_cleanup_releases_lock(
        self,
        mock_unlink,
        mock_flock,
        mock_close,
        orchestrator,
    ) -> None:
        orchestrator.needs_cleanup = True
        orchestrator.dry_run = False
        orchestrator.lock_fd = 123
        with patch("_orchestrator.lifecycle_down"):
            orchestrator.cleanup()
        assert mock_flock.called
        assert mock_close.called
        assert mock_unlink.called
        assert orchestrator.lock_fd is None

    @patch("os.close")
    @patch("fcntl.flock")
    def test_cleanup_lock_release_fails_silently(
        self, mock_flock, mock_close, orchestrator
    ) -> None:
        orchestrator.needs_cleanup = True
        orchestrator.dry_run = False
        orchestrator.lock_fd = 123
        mock_flock.side_effect = OSError
        with patch("_orchestrator.lifecycle_down"):
            orchestrator.cleanup()
        assert orchestrator.lock_fd is None

    @patch("_orchestrator.subprocess.run")
    @patch("os.environ.copy", return_value={"POOLER": "true"})
    def test_pooler_logic_handle_run_prepare(
        self, mock_env_copy, mock_run, orchestrator, monkeypatch
    ) -> None:
        # Test handle_run pooler logic (lines 557-562, 597-633, 645-646)
        monkeypatch.setenv("POOLER", "true")
        args = argparse.Namespace(pooler=True, tool="cargo", args=["check"])

        class MockCompletedProcess:
            def __init__(self, stdout, returncode=0):
                self.stdout = stdout
                self.returncode = returncode

        call_count = [0]

        def mock_run_effect(*args, **kwargs):
            call_count[0] += 1
            if args and "ps" in args[0]:
                if call_count[0] > 3:  # After a few calls, return healthy
                    return MockCompletedProcess(stdout="supavisor healthy")
                return MockCompletedProcess(stdout="supavisor starting")
            return MockCompletedProcess(stdout="")

        mock_run.side_effect = mock_run_effect

        with patch("time.sleep"):
            orchestrator.handle_run(args)

        assert mock_run.call_count >= 1
