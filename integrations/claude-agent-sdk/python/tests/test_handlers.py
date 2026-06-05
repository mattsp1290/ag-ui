"""Tests for the Claude SDK stream block handlers.

Exercises tool-use / tool-result block translation and the state-management
interception path. Handlers are async generators, so we collect events.
"""

import json

import pytest

from ag_ui.core import EventType
from ag_ui_claude_sdk.config import STATE_MANAGEMENT_TOOL_FULL_NAME
from ag_ui_claude_sdk.handlers import (
    handle_tool_use_block,
    handle_tool_result_block,
)

from claude_agent_sdk import ToolUseBlock, ToolResultBlock


async def collect(agen):
    return [e async for e in agen]


class _Msg:
    """Stand-in parent message carrying parent_tool_use_id."""

    def __init__(self, parent_tool_use_id=None):
        self.parent_tool_use_id = parent_tool_use_id


class TestHandleToolUseBlock:
    @pytest.mark.asyncio
    async def test_regular_tool_emits_start_args_end(self):
        block = ToolUseBlock(id="tc1", name="mcp__weather__get_weather", input={"city": "NYC"})
        state, gen = await handle_tool_use_block(block, _Msg(), "th", "run", None)
        events = await collect(gen)
        types = [e.type for e in events]
        assert types == [
            EventType.TOOL_CALL_START,
            EventType.TOOL_CALL_ARGS,
            EventType.TOOL_CALL_END,
        ]
        # Name is stripped of the MCP prefix
        assert events[0].tool_call_name == "get_weather"
        assert events[0].tool_call_id == "tc1"
        assert json.loads(events[1].delta) == {"city": "NYC"}

    @pytest.mark.asyncio
    async def test_tool_without_input_skips_args(self):
        block = ToolUseBlock(id="tc2", name="ping", input={})
        _, gen = await handle_tool_use_block(block, _Msg(), "th", "run", None)
        types = [e.type for e in await collect(gen)]
        assert EventType.TOOL_CALL_ARGS not in types
        assert types == [EventType.TOOL_CALL_START, EventType.TOOL_CALL_END]

    @pytest.mark.asyncio
    async def test_state_management_tool_emits_snapshot_and_merges(self):
        block = ToolUseBlock(
            id="tc3",
            name=STATE_MANAGEMENT_TOOL_FULL_NAME,
            input={"state_updates": {"count": 5}},
        )
        new_state, gen = await handle_tool_use_block(
            block, _Msg(), "th", "run", {"count": 1, "name": "a"}
        )
        events = await collect(gen)
        # Only a STATE_SNAPSHOT, no TOOL_CALL_* events
        assert [e.type for e in events] == [EventType.STATE_SNAPSHOT]
        assert events[0].snapshot == {"count": 5, "name": "a"}

    @pytest.mark.asyncio
    async def test_state_management_tool_json_string_updates(self):
        block = ToolUseBlock(
            id="tc4",
            name=STATE_MANAGEMENT_TOOL_FULL_NAME,
            input={"state_updates": json.dumps({"count": 9})},
        )
        _, gen = await handle_tool_use_block(block, _Msg(), "th", "run", {"count": 1})
        events = await collect(gen)
        assert events[0].snapshot == {"count": 9}

    @pytest.mark.asyncio
    async def test_state_management_invalid_json_emits_custom_error(self):
        block = ToolUseBlock(
            id="tc5",
            name=STATE_MANAGEMENT_TOOL_FULL_NAME,
            input={"state_updates": "{not valid json"},
        )
        _, gen = await handle_tool_use_block(block, _Msg(), "th", "run", {})
        events = await collect(gen)
        types = [e.type for e in events]
        assert EventType.CUSTOM in types
        custom = next(e for e in events if e.type == EventType.CUSTOM)
        assert custom.name == "state_update_error"


class TestHandleToolResultBlock:
    @pytest.mark.asyncio
    async def test_emits_tool_call_result(self):
        block = ToolResultBlock(
            tool_use_id="tc1",
            content=[{"type": "text", "text": '{"ok": true}'}],
        )
        events = await collect(handle_tool_result_block(block, "th", "run"))
        assert len(events) == 1
        assert events[0].type == EventType.TOOL_CALL_RESULT
        assert events[0].tool_call_id == "tc1"
        assert events[0].message_id == "tc1-result"
        assert json.loads(events[0].content) == {"ok": True}

    @pytest.mark.asyncio
    async def test_does_not_emit_tool_call_end(self):
        # Regression guard: result handler must NOT re-emit TOOL_CALL_END
        # (that caused "No active tool call" runtime errors).
        block = ToolResultBlock(tool_use_id="tc1", content="plain")
        events = await collect(handle_tool_result_block(block, "th", "run"))
        assert all(e.type != EventType.TOOL_CALL_END for e in events)

    @pytest.mark.asyncio
    async def test_no_tool_use_id_emits_nothing(self):
        block = ToolResultBlock(tool_use_id="", content="x")
        events = await collect(handle_tool_result_block(block, "th", "run"))
        assert events == []
