"""Tests for prepare_stream interrupt-resume ordering -- fixes #1743.

The bug: the regenerate heuristic (message-count comparison) runs
*before* the interrupt check, so when a checkpoint contains an AI
message from the interrupt that the frontend never saw, prepare_stream
incorrectly enters the regenerate path, destroying the interrupt state.

The fix treats an explicit, non-None resume key as a resume command that
bypasses the regenerate heuristic. Active interrupts without a resume
still allow edit/regenerate detection before replaying interrupt events.
"""

import unittest
from dataclasses import dataclass, field
from typing import Any, List
from unittest.mock import AsyncMock, MagicMock

from langchain_core.messages import AIMessage, HumanMessage
from langgraph.types import Command

from ag_ui.core import EventType, UserMessage

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
    state.next = []
    state.metadata = {"writes": {}}
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
    inp.run_id = "run-1"
    inp.forwarded_props = forwarded_props or {}
    return inp


async def _empty_stream():
    if False:
        yield None


class TestPrepareStreamInterruptResumeOrdering(unittest.IsolatedAsyncioTestCase):
    """Interrupt resumes must bypass the regenerate heuristic (#1743)."""

    async def test_handle_stream_events_uses_forwarded_node_name_for_continue_mode(self):
        """A no-resume request with node_name should continue from that node."""
        agent = make_agent()

        checkpoint_messages = [
            HumanMessage(id="h1", content="do something"),
        ]
        initial_state = _make_state(messages=checkpoint_messages)
        agent.graph.aget_state = AsyncMock(return_value=initial_state)
        agent.graph.astream_events.return_value = _empty_stream()

        frontend_messages = [
            UserMessage(id="h1", role="user", content="do something"),
            UserMessage(id="h2", role="user", content="follow up"),
        ]
        inp = _make_input(
            messages=frontend_messages,
            forwarded_props={"node_name": "approval_node"},
        )

        collected = []
        async for event in agent._handle_stream_events(inp):
            collected.append(event)

        agent.graph.aupdate_state.assert_awaited_once()
        self.assertEqual(
            agent.graph.aupdate_state.await_args.kwargs.get("as_node"),
            "approval_node",
        )

    async def test_interrupt_none_resume_with_node_name_does_not_emit_unmatched_step(self):
        """Short-circuit interrupt replay must not start a step it never finishes."""
        agent = make_agent()

        checkpoint_messages = [
            HumanMessage(id="h1", content="do something"),
            AIMessage(
                id="ai1",
                content="",
                tool_calls=[{"id": "tc-1", "name": "approval", "args": {}}],
            ),
        ]
        initial_state = _make_state(
            messages=checkpoint_messages,
            tasks=[FakeTask(interrupts=[FakeInterrupt(value="confirm?")])],
        )
        agent.graph.aget_state = AsyncMock(return_value=initial_state)

        frontend_messages = [
            UserMessage(id="h1", role="user", content="do something"),
        ]
        inp = _make_input(
            messages=frontend_messages,
            forwarded_props={"node_name": "approval_node", "command": {"resume": None}},
        )

        events = []
        async for event in agent._handle_stream_events(inp):
            events.append(event)

        types = [getattr(event, "type", None) for event in events]
        self.assertNotIn(EventType.STEP_STARTED, types)
        self.assertNotIn(EventType.STEP_FINISHED, types)
        self.assertEqual(types.count(EventType.RUN_STARTED), 1)
        self.assertEqual(types.count(EventType.RUN_FINISHED), 1)

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

        stream_input = agent.graph.astream_events.call_args.kwargs["input"]
        self.assertIsInstance(stream_input, Command)
        self.assertEqual(stream_input.resume, "yes")

    async def test_falsy_resume_payloads_with_interrupt_are_treated_as_present(self):
        """Non-None resume payloads, not truthiness, should select Command(resume=...)."""
        falsy_payloads = [False, 0, "", {}, []]

        for payload in falsy_payloads:
            with self.subTest(payload=payload):
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
                    forwarded_props={"command": {"resume": payload}},
                )

                agent.prepare_regenerate_stream = AsyncMock()
                config = {"configurable": {"thread_id": "t1"}}

                result = await agent.prepare_stream(inp, state, config)

                agent.prepare_regenerate_stream.assert_not_awaited()
                agent.graph.aupdate_state.assert_not_called()
                self.assertIsNotNone(result.get("stream"))

                stream_input = agent.graph.astream_events.call_args.kwargs["input"]
                self.assertIsInstance(stream_input, Command)
                self.assertEqual(stream_input.resume, payload)

    async def test_none_resume_payload_with_interrupt_is_treated_as_absent(self):
        """resume=None follows the no-resume interrupt replay path."""
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
        inp = _make_input(
            messages=frontend_messages,
            forwarded_props={"command": {"resume": None}},
        )

        agent.prepare_regenerate_stream = AsyncMock()
        config = {"configurable": {"thread_id": "t1"}}

        result = await agent.prepare_stream(inp, state, config)

        agent.prepare_regenerate_stream.assert_not_awaited()
        agent.graph.astream_events.assert_not_called()
        agent.graph.aupdate_state.assert_not_called()
        self.assertIsNone(result.get("stream"))

        events = result.get("events_to_dispatch", [])
        types = [getattr(e, "type", None) for e in events]
        self.assertIn(EventType.RUN_STARTED, types)
        self.assertIn(EventType.CUSTOM, types)
        self.assertIn(EventType.RUN_FINISHED, types)

    async def test_interrupt_without_resume_still_allows_regenerate_heuristic(self):
        """Active interrupts must not globally suppress the edit/regenerate path."""
        agent = make_agent()
        agent.active_run = {"id": "run-1", "mode": "start"}

        checkpoint_messages = [
            HumanMessage(id="h1", content="original"),
            AIMessage(id="ai1", content="first answer"),
            HumanMessage(id="h2", content="regenerate from here"),
            AIMessage(
                id="ai2",
                content="",
                tool_calls=[{"id": "tc-1", "name": "approval", "args": {}}],
            ),
        ]
        state = _make_state(
            messages=checkpoint_messages,
            tasks=[FakeTask(interrupts=[FakeInterrupt(value="pending approval")])],
        )

        frontend_messages = [
            UserMessage(id="h1", role="user", content="original"),
            UserMessage(id="h-edited", role="user", content="edited earlier"),
            UserMessage(id="h2", role="user", content="regenerate from here"),
        ]
        inp = _make_input(messages=frontend_messages, forwarded_props={})

        prepared_regenerate = {
            "stream": "regenerate-stream",
            "state": {"messages": checkpoint_messages},
            "config": {"configurable": {"thread_id": "t1"}},
        }
        agent.prepare_regenerate_stream = AsyncMock(return_value=prepared_regenerate)
        config = {"configurable": {"thread_id": "t1"}}

        result = await agent.prepare_stream(inp, state, config)

        agent.prepare_regenerate_stream.assert_awaited_once()
        self.assertIs(result, prepared_regenerate)
        agent.graph.aupdate_state.assert_not_called()

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
