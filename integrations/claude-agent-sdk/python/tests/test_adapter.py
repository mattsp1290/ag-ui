"""Tests for ClaudeAgentAdapter event translation and option building.

The adapter's job is to translate a Claude Agent SDK message stream into the
AG-UI protocol event sequence. We drive ``_stream_claude_sdk`` directly with a
fake stream of SDK ``StreamEvent`` / message objects, so no LLM call is made.

We also test ``run()`` error handling by injecting a fake SessionWorker, and
``build_options`` merging behavior.
"""

import json

import pytest

from ag_ui.core import EventType
from ag_ui_claude_sdk.adapter import ClaudeAgentAdapter
from ag_ui_claude_sdk.config import STATE_MANAGEMENT_TOOL_FULL_NAME, AG_UI_MCP_SERVER_NAME

from ag_ui_claude_sdk.utils import extract_tool_names

from .conftest import stream_event, aiter


def _types(events):
    return [e.type for e in events]


async def _drive(adapter, stream_items, make_input, **input_kwargs):
    """Run _stream_claude_sdk over a fake message stream and collect events."""
    inp = make_input(**input_kwargs)
    frontend = set(extract_tool_names(inp.tools)) if inp.tools else set()
    # Seed per-thread state as run() would.
    adapter._per_thread_state[inp.thread_id] = inp.state
    events = []
    async for ev in adapter._stream_claude_sdk(
        aiter(stream_items), inp.thread_id, inp.run_id, inp, frontend
    ):
        events.append(ev)
    return events


class TestStreamTextMessage:
    @pytest.mark.asyncio
    async def test_streamed_text_produces_start_content_end(self, make_input):
        adapter = ClaudeAgentAdapter(name="t")
        stream = [
            stream_event({"type": "message_start"}),
            stream_event(
                {"type": "content_block_delta", "delta": {"type": "text_delta", "text": "Hello "}}
            ),
            stream_event(
                {"type": "content_block_delta", "delta": {"type": "text_delta", "text": "world"}}
            ),
            stream_event({"type": "message_stop"}),
        ]
        events = await _drive(adapter, stream, make_input)
        types = _types(events)
        assert EventType.TEXT_MESSAGE_START in types
        assert EventType.TEXT_MESSAGE_END in types
        contents = [e for e in events if e.type == EventType.TEXT_MESSAGE_CONTENT]
        assert "".join(c.delta for c in contents) == "Hello world"
        # START precedes content precedes END
        assert types.index(EventType.TEXT_MESSAGE_START) < types.index(EventType.TEXT_MESSAGE_END)

    @pytest.mark.asyncio
    async def test_messages_snapshot_emitted_at_end(self, make_input):
        adapter = ClaudeAgentAdapter(name="t")
        stream = [
            stream_event({"type": "message_start"}),
            stream_event(
                {"type": "content_block_delta", "delta": {"type": "text_delta", "text": "Hi"}}
            ),
            stream_event({"type": "message_stop"}),
        ]
        events = await _drive(adapter, stream, make_input)
        snapshots = [e for e in events if e.type == EventType.MESSAGES_SNAPSHOT]
        assert len(snapshots) == 1
        assert any(getattr(m, "content", None) == "Hi" for m in snapshots[0].messages)


class TestStreamToolCall:
    @pytest.mark.asyncio
    async def test_backend_tool_call_sequence(self, make_input):
        adapter = ClaudeAgentAdapter(name="t")
        stream = [
            stream_event({"type": "message_start"}),
            stream_event(
                {
                    "type": "content_block_start",
                    "content_block": {"type": "tool_use", "id": "tc1", "name": "mcp__srv__lookup"},
                }
            ),
            stream_event(
                {
                    "type": "content_block_delta",
                    "delta": {"type": "input_json_delta", "partial_json": '{"q":"x"}'},
                }
            ),
            stream_event({"type": "content_block_stop"}),
            stream_event({"type": "message_stop"}),
        ]
        events = await _drive(adapter, stream, make_input)
        types = _types(events)
        assert EventType.TOOL_CALL_START in types
        assert EventType.TOOL_CALL_ARGS in types
        assert EventType.TOOL_CALL_END in types
        start = next(e for e in events if e.type == EventType.TOOL_CALL_START)
        assert start.tool_call_name == "lookup"  # prefix stripped
        # exactly one END for the one tool call
        assert types.count(EventType.TOOL_CALL_END) == 1

    @pytest.mark.asyncio
    async def test_frontend_tool_halts_stream(self, make_input):
        adapter = ClaudeAgentAdapter(name="t")
        # Register a frontend tool named "confirm"
        tools = [{"name": "confirm", "description": "", "parameters": {}}]
        stream = [
            stream_event({"type": "message_start"}),
            stream_event(
                {
                    "type": "content_block_start",
                    "content_block": {"type": "tool_use", "id": "tc1", "name": "mcp__ag_ui__confirm"},
                }
            ),
            stream_event(
                {
                    "type": "content_block_delta",
                    "delta": {"type": "input_json_delta", "partial_json": "{}"},
                }
            ),
            stream_event({"type": "content_block_stop"}),
            # This message_stop must NOT be processed -- stream halts on the frontend tool
            stream_event(
                {"type": "content_block_delta", "delta": {"type": "text_delta", "text": "AFTER"}}
            ),
        ]
        events = await _drive(adapter, stream, make_input, tools=tools)
        # The post-halt text must not appear.
        contents = [e for e in events if e.type == EventType.TEXT_MESSAGE_CONTENT]
        assert all(c.delta != "AFTER" for c in contents)
        assert EventType.TOOL_CALL_END in _types(events)


class TestStreamReasoning:
    @pytest.mark.asyncio
    async def test_thinking_block_emits_reasoning_events(self, make_input):
        adapter = ClaudeAgentAdapter(name="t")
        stream = [
            stream_event({"type": "message_start"}),
            stream_event(
                {"type": "content_block_start", "content_block": {"type": "thinking"}}
            ),
            stream_event(
                {"type": "content_block_delta", "delta": {"type": "thinking_delta", "thinking": "hmm"}}
            ),
            stream_event(
                {"type": "content_block_delta", "delta": {"type": "signature_delta", "signature": "sig"}}
            ),
            stream_event({"type": "content_block_stop"}),
            stream_event({"type": "message_stop"}),
        ]
        events = await _drive(adapter, stream, make_input)
        types = _types(events)
        assert EventType.REASONING_START in types
        assert EventType.REASONING_MESSAGE_START in types
        assert EventType.REASONING_MESSAGE_CONTENT in types
        assert EventType.REASONING_END in types
        # signature was accumulated -> encrypted value emitted
        assert EventType.REASONING_ENCRYPTED_VALUE in types
        enc = next(e for e in events if e.type == EventType.REASONING_ENCRYPTED_VALUE)
        assert enc.encrypted_value == "sig"


class TestStreamCleanup:
    @pytest.mark.asyncio
    async def test_hanging_tool_call_closed_on_stream_end(self, make_input):
        adapter = ClaudeAgentAdapter(name="t")
        # tool_use opened but stream ends without content_block_stop
        stream = [
            stream_event({"type": "message_start"}),
            stream_event(
                {
                    "type": "content_block_start",
                    "content_block": {"type": "tool_use", "id": "tc1", "name": "lookup"},
                }
            ),
        ]
        events = await _drive(adapter, stream, make_input)
        # Cleanup must close the hanging tool call.
        assert EventType.TOOL_CALL_END in _types(events)


class TestBuildOptions:
    def test_dict_options_merged(self):
        adapter = ClaudeAgentAdapter(name="t", options={"model": "claude-x"})
        opts = adapter.build_options()
        assert opts.model == "claude-x"
        # include_partial_messages default applied
        assert opts.include_partial_messages is True

    def test_api_key_stripped(self):
        # api_key must be popped from the merged kwargs before constructing
        # ClaudeAgentOptions (it is handled via env var, and the options
        # dataclass has no such field). Build must succeed (proving the pop
        # happened — otherwise ClaudeAgentOptions(**kwargs) would raise on the
        # unexpected api_key kwarg) and the secret must be absent from vars(opts).
        adapter = ClaudeAgentAdapter(name="t", options={"api_key": "secret", "model": "m"})
        opts = adapter.build_options()
        opts_vars = vars(opts)
        assert "api_key" not in opts_vars
        assert "secret" not in opts_vars.values()
        # The non-secret kwargs still flow through.
        assert opts.model == "m"

    def test_state_adds_state_management_tool(self, make_input):
        adapter = ClaudeAgentAdapter(name="t")
        inp = make_input(state={"count": 1})
        opts = adapter.build_options(inp)
        assert STATE_MANAGEMENT_TOOL_FULL_NAME in (opts.allowed_tools or [])
        assert AG_UI_MCP_SERVER_NAME in (opts.mcp_servers or {})

    def test_state_addendum_appended_to_system_prompt(self, make_input):
        adapter = ClaudeAgentAdapter(name="t", options={"system_prompt": "BASE"})
        inp = make_input(state={"count": 1})
        opts = adapter.build_options(inp)
        assert opts.system_prompt.startswith("BASE")
        assert "Current Shared State" in opts.system_prompt


class _FakeFailingWorker:
    """A SessionWorker stand-in whose query raises immediately."""

    def __init__(self, *args, **kwargs):
        pass

    async def start(self):
        pass

    def query(self, prompt, session_id="default"):
        async def _gen():
            raise RuntimeError("boom")
            yield  # pragma: no cover

        return _gen()

    async def stop(self):
        pass


class TestRunErrorPath:
    @pytest.mark.asyncio
    async def test_run_emits_run_error_on_worker_failure(self, make_input, monkeypatch):
        adapter = ClaudeAgentAdapter(name="t")
        monkeypatch.setattr("ag_ui_claude_sdk.adapter.SessionWorker", _FakeFailingWorker)

        inp = make_input(messages=[{"id": "1", "role": "user", "content": "hi"}])
        events = [e async for e in adapter.run(inp)]
        types = _types(events)
        # RUN_STARTED then RUN_ERROR (not RUN_FINISHED)
        assert EventType.RUN_STARTED in types
        assert EventType.RUN_ERROR in types
        assert EventType.RUN_FINISHED not in types
        err = next(e for e in events if e.type == EventType.RUN_ERROR)
        assert "boom" in err.message
