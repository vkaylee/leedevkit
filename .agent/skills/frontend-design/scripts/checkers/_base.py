"""Shared base types for composable UX audit checkers."""

from dataclasses import dataclass
from typing import Protocol


@dataclass
class AuditContext:
    """Holds pre-computed flags and file metadata for all checks."""

    filename: str
    content: str
    has_long_text: bool = False
    has_form: bool = False
    complex_elements: int = 0
    has_hero: bool = False
    nav_items: int = 0


class Checker(Protocol):
    """Protocol for individual audit checkers.

    Each checker is independent — it reads the AuditContext and returns
    a list of findings dicts.  Checks must NOT mutate shared state.
    """

    def run(self, ctx: AuditContext) -> list[dict[str, str]]:
        """Run checks and return list of findings.

        Each finding: {"severity": "issue"|"warning"|"pass", "message": str}
        """
        ...
