"""Tests for run_inline — safe inline code execution references."""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))


class TestRunInlineReferences:
    def test_safe_run_mentions_run_inline(self):
        content = (Path(__file__).resolve().parent.parent / "_safe_run.py").read_text()
        assert "run-inline" in content

    def test_arg_sanitizer_mentions_run_inline(self):
        content = (
            Path(__file__).resolve().parent.parent / "_arg_sanitizer.py"
        ).read_text()
        assert "run-inline" in content

    def test_supported_languages_listed(self):
        content = (
            Path(__file__).resolve().parent.parent / "_arg_sanitizer.py"
        ).read_text()
        assert "python" in content.lower()
