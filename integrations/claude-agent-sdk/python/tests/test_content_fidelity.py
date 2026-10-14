import json
from types import SimpleNamespace

import pytest
from ag_ui_claude_sdk.utils import build_agui_tool_message
from ag_ui_claude_sdk.handlers import handle_tool_result_block


@pytest.mark.asyncio
@pytest.mark.parametrize("blocks", [
    [{"type": "text", "text": "caption"}, {"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": "AQID"}}, {"type": "text", "text": "second"}],
    [{"type": "text", "text": "value", "annotations": {"audience": ["user"]}}],
])
async def test_rich_result_survives_event_and_snapshot(blocks):
    message = build_agui_tool_message("original", blocks)
    events = [event async for event in handle_tool_result_block(SimpleNamespace(tool_use_id="original", content=blocks, is_error=False), "thread", "run")]
    assert [part.metadata["claude"]["block"] for part in message.content] == blocks
    assert events[0].content == message.content
    assert events[0].tool_call_id == message.tool_call_id == "original"


@pytest.mark.asyncio
async def test_frontend_proxy_returns_exact_answer():
    from ag_ui_claude_sdk.utils import convert_agui_tool_to_claude_sdk
    answer = ' { "approved": false, "reason": "No" } '
    async def resolve(name, args):
        assert name == "confirm"
        assert args == {"title": "original"}
        return answer
    proxy = convert_agui_tool_to_claude_sdk({"name": "confirm"}, resolve)
    assert await proxy.handler({"title": "original"}) == {"content": [{"type": "text", "text": answer}]}
    proxy = convert_agui_tool_to_claude_sdk({"name": "confirm"})
    with pytest.raises(RuntimeError, match="matched deferred tool call"):
        await proxy.handler({})


@pytest.mark.asyncio
async def test_error_event_and_snapshot_have_identical_rich_payload():
    blocks = [{"type": "text", "text": "failed"}, {"type": "resource", "resource": {"uri": "ui://chart", "mimeType": "text/html", "text": "<div>chart</div>"}}]
    message = build_agui_tool_message("original", blocks, True)
    events = [event async for event in handle_tool_result_block(SimpleNamespace(tool_use_id="original", content=blocks, is_error=True), "thread", "run")]
    assert [part.metadata["claude"]["block"] for part in message.content] == blocks
    assert all(part.metadata["claude"]["isError"] for part in message.content)
    assert events[0].content == message.content


@pytest.mark.asyncio
async def test_native_message_id_survives_stream_and_completion(make_input):
    from claude_agent_sdk import AssistantMessage
    from claude_agent_sdk.types import TextBlock, ThinkingBlock
    from ag_ui_claude_sdk.adapter import ClaudeAgentAdapter
    from .conftest import stream_event, aiter
    inp = make_input()
    adapter = ClaudeAgentAdapter(name="test")
    stream = [stream_event({"type": "message_start", "message": {"id": "msg-native"}}),
              stream_event({"type": "content_block_delta", "delta": {"type": "text_delta", "text": "Hi"}}),
              stream_event({"type": "message_stop"}),
              AssistantMessage(content=[TextBlock(text="Hi"), ThinkingBlock(thinking="reason", signature="signature")], model="test")]
    events = [e async for e in adapter._stream_claude_sdk(aiter(stream), inp.thread_id, inp.run_id, inp, set())]
    snapshot = next(e for e in events if e.type == "MESSAGES_SNAPSHOT")
    assert len(snapshot.messages) == 1
    assert snapshot.messages[0].id == "msg-native"
    raw = next(e for e in events if getattr(e, "name", None) == "claude:assistant_content")
    assert raw.value["messageId"] == "msg-native"
    assert raw.value["content"][1]["signature"] == "signature"


def test_state_is_shallow_merged_and_rejects_invalid_updates():
    from ag_ui_claude_sdk.utils import merge_state_update
    before = {"keep": 1, "nested": {"old": True}, "values": [1]}
    assert merge_state_update(before, {"state_updates": {"nested": {"next": True}, "values": []}}) == {"keep": 1, "nested": {"next": True}, "values": []}
    assert before["nested"] == {"old": True}
    with pytest.raises(ValueError, match="must be an object"):
        merge_state_update(before, {"state_updates": []})
