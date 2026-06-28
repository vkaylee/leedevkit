import sys
from pathlib import Path
from unittest.mock import patch

import pytest

# Add scripts to path
PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent
sys.path.append(str(Path(__file__).resolve().parent.parent))

import _orchestrator  # noqa: E402


@pytest.fixture
def orchestrator() -> _orchestrator.Orchestrator:
    return _orchestrator.Orchestrator()


def test_test_no_mode_shows_help(
    orchestrator: _orchestrator.Orchestrator, capsys: pytest.CaptureFixture[str]
) -> None:
    """Test that test command with target 'all' works."""
    with (
        patch.object(_orchestrator.Orchestrator, "run_phase"),
        patch.object(_orchestrator.Orchestrator, "execute_safe"),
        patch.object(_orchestrator.Orchestrator, "handle_run"),
        patch("_orchestrator._lifecycle_up"),
    ):
        args = orchestrator.parser.parse_args(["test", "all"])
        orchestrator.handle_test(args)


def test_manage_no_subcommand_raises_error(orchestrator: _orchestrator.Orchestrator) -> None:
    """Test that manage command without subcommand raises argparse error."""
    with pytest.raises(SystemExit) as e:
        orchestrator.parser.parse_args(["manage"])
    assert e.value.code != 0


def test_no_garbage_flags_in_test_parser(orchestrator: _orchestrator.Orchestrator) -> None:
    """Ensure every flag defined in test parser is actually accessed in handle_test."""
    import argparse
    import typing

    class TrackingNamespace(argparse.Namespace):
        def __init__(self, **kwargs: typing.Any) -> None:
            super().__init__(**kwargs)
            super().__setattr__("_accessed", set())

        def __getattribute__(self, name: str) -> typing.Any:
            if not name.startswith("_") and name != "command":
                self._accessed.add(name)
            return super().__getattribute__(name)

    subparsers = next(
        action
        for action in orchestrator.parser._actions
        if isinstance(action, argparse._SubParsersAction)
    )
    test_parser = subparsers.choices["test"]

    defined_dests = {
        action.dest
        for action in test_parser._actions
        if action.dest != "help" and not action.dest.startswith("dry_run")
    }

    # Set all boolean flags to False so that no short-circuiting skips evaluation
    args_dict: dict[str, typing.Any] = dict.fromkeys(defined_dests, False)
    args_dict["target"] = "all"
    args_dict["component"] = ""
    args_dict["pattern"] = "auth"
    args_dict["command"] = "test"
    args_dict["coverage"] = False
    args_dict["timeout"] = 100

    args = TrackingNamespace(**args_dict)

    with (
        patch("_orchestrator._lifecycle_up"),
        patch("_orchestrator.leeattend_run_lint"),
        patch("_orchestrator.leeattend_run_unit"),
        patch("_orchestrator.leeattend_run_integration"),
        patch("_orchestrator.leeattend_run_coverage"),
        patch.object(_orchestrator.Orchestrator, "handle_verify_infra"),
        patch.object(_orchestrator.Orchestrator, "print_test_summary"),
        patch.object(_orchestrator.Orchestrator, "handle_run"),
    ):
        orchestrator.handle_test(args)

    unaccessed = defined_dests - args._accessed
    assert not unaccessed, f"Garbage flags detected in test usage that are never used: {unaccessed}"
