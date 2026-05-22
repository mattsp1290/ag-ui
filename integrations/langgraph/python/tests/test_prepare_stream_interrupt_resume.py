"""Tests for prepare_stream interrupt-resume ordering — fixes #1743.

The bug: the regenerate heuristic (message-count comparison) runs
*before* the interrupt check, so when a checkpoint contains an AI
message from the interrupt that the frontend never saw, prepare_stream
incorrectly enters the regenerate path, destroying the interrupt state.

The fix moves interrupt handling before the regenerate heuristic so
that resume requests bypass the message-count comparison entirely.
"""

import unittest
from dataclasses import dataclass, field
from typing import Any, List
from unittest.mock import AsyncMock, MagicMock

from langchain_core.messages import AIMessage, HumanMessage

from ag_ui.core import EventType, RunAgentInput, UserMessage

from tests._helpers import make_agent


@dataclass
class FakeInterrupt:
    value: Any


@dataclass
class FakeTask:
    interrupts: List[FakeInterrupt] = field(default_factory=list)


def _make_state(messages, tasks=None):
    """Build a mock agent_state with messages and optional tasks."""
    state = MagicMock()
    state.values = {"messages": messages}
    state.tasks = tasks or []
    return state


def _make_input(
    messages,
    thread_id="t1",
    forwarded_props=None,
):
    """Build a RunAgentInput-compatible mock."""
    inp = MagicMock()
    inp.thread_id = thread_id
    inp.messages = messages
    inp.state = {}
    inp.tools = []
    inp.context = []
    inp.forwarded_props = forwarded_props or {}
    return inp


class TestPrepareStreamInterruptResumeOrdering(unittest.IsolatedAsyncioTestCase):
    """Interrupt resumes must bypass the regenerate heuristic (#1743)."""

    async def test_resume_with_interrupt_does_not_regenerate(self):
        """Core regression: checkpoint has more messages than frontend
        sent (the AI tool-call from the interrupt), and a resume value is
        present. The old code would enter prepare_regenerate_stream; the
        fix must skip it and produce a Command(resume=...) stream."""
        agent = make_agent()
        agent.active_run = {"id": "run-1", "mode": "start"}

        checkpoint_messages = [
            HumanMessage(id="h1", content="do something"),
            AIMessage(
                id="ai1",
                content="",
                tool_calls=[{"id": "tc-1", "name": "approval", "args": {}}],
            ),
        ]
        state = _make_state(
            messages=checkpoint_messages,
            tasks=[FakeTask(interrupts=[FakeInterrupt(value={"question": "Approve?"})])],
        )

        frontend_messages = [
            UserMessage(id="h1", role="user", content="do something"),
        ]
        inp = _make_input(
            messages=frontend_messages,
            forwarded_props={"command": {"resume": "yes"}},
        )

        agent.prepare_regenerate_stream = AsyncMock()
        config = {"configurable": {"thread_id": "t1"}}

        result = await agent.prepare_stream(inp, state, config)

        agent.prepare_regenerate_stream.assert_not_awaited()
        self.assertIsNotNone(result.get("stream"))

    async def test_interrupt_without_resume_dispatches_interrupt_events(self):
        """When there's an active interrupt but no resume value, the agent
        must dispatch interrupt events (not enter regenerate)."""
        agent = make_agent()
        agent.active_run = {"id": "run-1", "mode": "start"}

        checkpoint_messages = [
            HumanMessage(id="h1", content="do something"),
            AIMessage(
                id="ai1",
                content="",
                tool_calls=[{"id": "tc-1", "name": "approval", "args": {}}],
            ),
        ]
        state = _make_state(
            messages=checkpoint_messages,
            tasks=[FakeTask(interrupts=[FakeInterrupt(value="confirm?")])],
        )

        frontend_messages = [
            UserMessage(id="h1", role="user", content="do something"),
        ]
        inp = _make_input(messages=frontend_messages, forwarded_props={})

        agent.prepare_regenerate_stream = AsyncMock()
        config = {"configurable": {"thread_id": "t1"}}

        result = await agent.prepare_stream(inp, state, config)

        agent.prepare_regenerate_stream.assert_not_awaited()
        self.assertIsNone(result.get("stream"))

        events = result.get("events_to_dispatch", [])
        types = [getattr(e, "type", None) for e in events]
        self.assertIn(EventType.RUN_STARTED, types)
        self.assertIn(EventType.CUSTOM, types)
        self.assertIn(EventType.RUN_FINISHED, types)

    async def test_no_interrupt_normal_flow_produces_stream(self):
        """Without active interrupts, the normal streaming path must
        still work — the fix must not break standard message flow."""
        agent = make_agent()
        agent.active_run = {"id": "run-1", "mode": "start"}

        checkpoint_messages = [
            HumanMessage(id="h1", content="hello"),
        ]
        state = _make_state(messages=checkpoint_messages, tasks=[FakeTask()])

        frontend_messages = [
            UserMessage(id="h1", role="user", content="hello"),
            UserMessage(id="h2", role="user", content="follow up"),
        ]
        inp = _make_input(messages=frontend_messages, forwarded_props={})

        config = {"configurable": {"thread_id": "t1"}}

        result = await agent.prepare_stream(inp, state, config)

        self.assertIsNotNone(result.get("stream"))

    async def test_resume_with_no_interrupt_proceeds_normally(self):
        """A resume value without active interrupts should not crash;
        the resume path at the bottom of prepare_stream handles it."""
        agent = make_agent()
        agent.active_run = {"id": "run-1", "mode": "start"}

        checkpoint_messages = [
            HumanMessage(id="h1", content="do something"),
        ]
        state = _make_state(messages=checkpoint_messages, tasks=[FakeTask()])

        frontend_messages = [
            UserMessage(id="h1", role="user", content="do something"),
        ]
        inp = _make_input(
            messages=frontend_messages,
            forwarded_props={"command": {"resume": "yes"}},
        )

        config = {"configurable": {"thread_id": "t1"}}

        result = await agent.prepare_stream(inp, state, config)

        self.assertIsNotNone(result.get("stream"))
