"""Shared base types for composable mobile audit checkers."""

from dataclasses import dataclass
from typing import Protocol


@dataclass
class MobileAuditContext:
    """Holds pre-computed flags and file metadata for all mobile checks."""

    filename: str
    content: str
    is_react_native: bool = False
    is_flutter: bool = False


class Checker(Protocol):
    """Protocol for individual mobile audit checkers."""

    def run(self, ctx: MobileAuditContext) -> list[dict[str, str]]:
        """Run checks and return list of findings.

        Each finding: {"severity": "issue"|"warning"|"pass", "message": str}
        """
        ...
