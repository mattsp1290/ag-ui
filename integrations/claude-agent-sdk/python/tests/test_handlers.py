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
    async def test_missing_id_falls_back_to_generated_uuid(self):
        # A ToolUseBlock with a falsy id must not crash: the handler falls back
        # to a generated uuid. This guards against the `uuid` import living in
        # the module docstring (NameError at the str(uuid.uuid4()) fallback).
        block = ToolUseBlock(id="", name="ping", input={})
        _, gen = await handle_tool_use_block(block, _Msg(), "th", "run", None)
        events = await collect(gen)
        types = [e.type for e in events]
        assert types == [EventType.TOOL_CALL_START, EventType.TOOL_CALL_END]
        # A non-empty fallback id was generated (a uuid4 string).
        assert events[0].tool_call_id
        assert events[0].tool_call_id == events[1].tool_call_id

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
        # Exact current sequence: the parse error emits a CUSTOM event, then the
        # handler STILL emits a STATE_SNAPSHOT (with the un-updated state). That
        # trailing STATE_SNAPSHOT-after-error is a known handler bug deferred to
        # the follow-up PR; we assert reality precisely here so the test is not
        # vacuous (do NOT fix the handler logic in this PR).
        assert types == [EventType.CUSTOM, EventType.STATE_SNAPSHOT]
        custom = events[0]
        assert custom.name == "state_update_error"
        assert "error" in custom.value
        # Invalid JSON -> updates discarded -> snapshot reflects the original {} state.
        assert events[1].snapshot == {}


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
