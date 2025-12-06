# src/config.py

"""Configuration primitives for customizing ADK agent behavior."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Dict, Iterable, List, Optional


@dataclass
class PredictStateMapping:
    """Declarative mapping telling the UI how to predict state from tool args.

    This enables predictive state updates where the UI can show state changes
    in real-time as tool arguments are being streamed.

    Attributes:
        state_key: The key in the state object to update
        tool: The name of the tool that triggers this mapping
        tool_argument: The argument name from the tool that provides the value
    """

    state_key: str
    tool: str
    tool_argument: str

    def to_payload(self) -> Dict[str, str]:
        """Convert to the payload format expected by the UI."""
        return {
            "state_key": self.state_key,
            "tool": self.tool,
            "tool_argument": self.tool_argument,
        }


def normalize_predict_state(value: Optional[Iterable[PredictStateMapping]]) -> List[PredictStateMapping]:
    """Normalize predict state config into a concrete list.

    Args:
        value: A single PredictStateMapping, an iterable of them, or None

    Returns:
        A list of PredictStateMapping objects
    """
    if value is None:
        return []
    if isinstance(value, PredictStateMapping):
        return [value]
    return list(value)
