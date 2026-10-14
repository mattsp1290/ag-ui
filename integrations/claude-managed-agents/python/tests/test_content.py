import json
import pytest
from .fake_client import FakeClient
from .test_agent import base_input, new_agent, IDLE_END_TURN

async def run(parts, native=()):
    fake = FakeClient(streams=[[*native, IDLE_END_TURN]])
    value = base_input(state={"cart": {"total": 42}}, messages=[{"id": "original-user", "role": "user", "content": parts}])
    events = [e.model_dump(mode="json", by_alias=True, exclude_none=True) async for e in new_agent(fake).run(value)]
    return fake, value, events

@pytest.mark.parametrize("kind,mime", [("image", "image/png"), ("document", "application/pdf")])
async def test_media_sources_and_original_recorder_input(kind, mime):
    parts = [{"type": kind, "source": {"type": "data", "value": "AAEC/w==", "mimeType": mime}, "metadata": {"filename": "original.bin"}}, {"type": kind, "source": {"type": "url", "value": "https://example.test/media", "mimeType": mime}}, {"type": kind, "source": {"type": "file", "value": "file_native", "provider": "anthropic"}}]
    fake, value, events = await run(parts)
    assert events[0]["input"] == value.model_dump(mode="json", by_alias=True, exclude_none=True)
    assert events[1]["snapshot"] == value.state
    blocks = fake.sent[0]["events"][0]["content"]
    assert blocks[0]["source"] == {"type": "base64", "data": "AAEC/w==", "media_type": mime}
    assert blocks[1]["source"] == {"type": "url", "url": "https://example.test/media"}
    assert blocks[2]["source"] == {"type": "file", "file_id": "file_native"}
    assert events[-1]["type"] == "RUN_FINISHED"

@pytest.mark.parametrize("kind", ["audio", "video"])
async def test_unsupported_media_is_recorded_before_error(kind):
    fake, value, events = await run([{"type": "text", "text": "hello"}, {"type": kind, "source": {"type": "data", "value": "AAEC", "mimeType": kind + "/test"}, "metadata": {"filename": "original"}}])
    assert events[0]["input"] == value.model_dump(mode="json", by_alias=True, exclude_none=True)
    assert events[-1]["code"] == "unsupported_content"
    assert fake.sent == []
    assert fake.create_calls == []

async def test_foreign_file_handle_is_not_reinterpreted():
    fake, _, events = await run([{"type": "image", "source": {"type": "file", "value": "opaque", "provider": "openai"}}])
    assert events[-1]["code"] == "unsupported_content"
    assert fake.sent == []

async def test_full_native_results_and_unknown_rich_events():
    content = [{"type": "text", "text": "  " + "x" * 30000 + "\n"}, {"type": "image", "source": {"type": "base64", "data": "AAEC/w==", "media_type": "image/png"}}, {"type": "search_result", "title": "A &amp; B", "source": "https://example.test", "content": [{"type": "text", "text": "y" * 2000}]}]
    native = [{"type": "agent.mcp_tool_result", "id": "original-result", "mcp_tool_use_id": "original-call", "is_error": True, "content": content}, {"type": "agent.future_rich_event", "id": "rich", "payload": {"ui": [1, 2]}}]
    _, _, events = await run([{"type": "text", "text": "hello"}], native)
    assert [e["event"] for e in events if e["type"] == "RAW"] == native
    result = next(e for e in events if e["type"] == "TOOL_CALL_RESULT")
    assert result["messageId"] == "original-result"
    assert result["toolCallId"] == "original-call"
    assert [part["metadata"]["claudeManagedAgents"] for part in result["content"]] == content
    assert result["content"][1]["source"] == {"type": "data", "value": "AAEC/w==", "mimeType": "image/png"}

async def test_existing_unsupported_history_does_not_block_pending_tool_result():
    from ag_ui_claude_managed_agents import InMemorySessionStore, SessionRecord
    from .test_agent import SESSION_KEY
    fake = FakeClient(streams=[[IDLE_END_TURN]])
    store = InMemorySessionStore()
    store.set(SESSION_KEY, SessionRecord(session_id="sesn_1", tool_names=[], pending_client_tool_use_ids=["ctu_1"], last_user_message_id="u1"))
    value = base_input(messages=[
        {"id": "u1", "role": "user", "content": [{"type": "audio", "source": {"type": "url", "value": "https://example.test/audio"}}]},
        {"id": "t1", "role": "tool", "toolCallId": "ctu_1", "content": "answered"},
    ])
    events = [e.model_dump(mode="json", by_alias=True) async for e in new_agent(fake, store).run(value)]
    assert events[-1]["type"] == "RUN_FINISHED"
    assert fake.sent[0]["events"] == [{"type": "user.custom_tool_result", "custom_tool_use_id": "ctu_1", "content": [{"type": "text", "text": "answered"}], "is_error": False}]
    assert fake.create_calls == []

async def test_sdk_model_raw_event_preserves_explicit_null_and_omitted_fields():
    from anthropic.types.beta.sessions.beta_managed_agents_agent_mcp_tool_result_event import BetaManagedAgentsAgentMCPToolResultEvent
    native = BetaManagedAgentsAgentMCPToolResultEvent(id="native-result", mcp_tool_use_id="native-call", processed_at="2026-01-01T00:00:00Z", type="agent.mcp_tool_result", content=None)
    _, _, events = await run([{"type": "text", "text": "hello"}], [native])
    raw = next(e["event"] for e in events if e["type"] == "RAW")
    assert raw == native.model_dump(mode="json", by_alias=True, exclude_unset=True)
    assert "content" in raw and raw["content"] is None
    assert "is_error" not in raw
