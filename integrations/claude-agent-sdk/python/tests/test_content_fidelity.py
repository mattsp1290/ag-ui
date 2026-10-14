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


@pytest.mark.asyncio
async def test_text_document_filename_and_explicit_unsupported_notice(make_input):
    from ag_ui_claude_sdk.utils import process_messages, unsupported_input_parts, frontend_result_blocks
    inp = make_input(messages=[{"id": "user", "role": "user", "content": [
        {"type": "document", "source": {"type": "data", "value": "aMOpbGxv", "mimeType": "text/plain"}, "metadata": {"filename": "notes.txt"}},
        {"type": "audio", "source": {"type": "data", "value": "AQID", "mimeType": "audio/wav"}, "metadata": {"filename": "audio.wav"}},
    ]}])
    prompt, _ = process_messages(inp)
    messages = [m async for m in prompt]
    assert messages[0]["message"]["content"] == [{"type": "document", "title": "notes.txt", "source": {"type": "text", "media_type": "text/plain", "data": "héllo"}}]
    assert unsupported_input_parts(inp)[0]["contentIndex"] == 1
    assert inp.messages[0].content[1].metadata["filename"] == "audio.wav"
    assert frontend_result_blocks([{"type": "image", "source": {"type": "data", "value": "AQID", "mimeType": "image/png"}}]) == [{"type": "image", "data": "AQID", "mimeType": "image/png"}]


@pytest.mark.asyncio
async def test_native_structured_result_and_record_identity_are_retained():
    block = SimpleNamespace(tool_use_id="original", content="ok", is_error=False)
    raw = {"structuredContent": {"chart": [1, 2]}, "_meta": {"ui": {"resourceUri": "ui://chart"}}}
    events = [e async for e in handle_tool_result_block(block, "thread", "run", None, raw, "record-original")]
    assert events[0].raw_event["tool_use_result"] == raw
    assert events[0].raw_event["uuid"] == "record-original"


@pytest.mark.asyncio
@pytest.mark.parametrize("content,error,expected", [
    ("", "denied", [{"type": "text", "text": "denied"}]),
    ("detail", "", [{"type": "text", "text": "detail"}]),
    ("detail", "denied", [{"type": "text", "text": "detail"}, {"type": "text", "text": "denied"}]),
])
async def test_frontend_proxy_preserves_failure_status_and_reason(content, error, expected):
    from ag_ui_claude_sdk.utils import convert_agui_tool_to_claude_sdk
    proxy = convert_agui_tool_to_claude_sdk({"name": "confirm"}, lambda name, args: {"content": content, "error": error})
    assert await proxy.handler({}) == {"content": expected, "is_error": True}


@pytest.mark.asyncio
async def test_frontend_proxy_accepts_rich_success_envelope():
    from ag_ui_claude_sdk.utils import convert_agui_tool_to_claude_sdk
    proxy = convert_agui_tool_to_claude_sdk({"name": "image"}, lambda name, args: {"content": [{"type": "image", "source": {"type": "data", "value": "AQID", "mimeType": "image/png"}}]})
    assert await proxy.handler({}) == {"content": [{"type": "image", "data": "AQID", "mimeType": "image/png"}]}


@pytest.mark.asyncio
async def test_frontend_error_survives_real_sdk_mcp_server():
    import anyio
    from mcp import ClientSession
    from mcp.shared.memory import create_client_server_memory_streams
    from claude_agent_sdk import create_sdk_mcp_server
    from ag_ui_claude_sdk.utils import convert_agui_tool_to_claude_sdk
    proxy = convert_agui_tool_to_claude_sdk({"name": "confirm", "parameters": {"type": "object", "properties": {}}}, lambda name, args: {"content": "", "error": "denied"})
    server = create_sdk_mcp_server(name="ag_ui", tools=[proxy])["instance"]
    async with create_client_server_memory_streams() as (client_streams, server_streams):
        async with anyio.create_task_group() as tasks:
            tasks.start_soon(server.run, *server_streams, server.create_initialization_options())
            async with ClientSession(*client_streams) as client:
                await client.initialize()
                result = await client.call_tool("confirm", {})
                assert result.model_dump(by_alias=True)["isError"] is True
                assert result.content[0].text == "denied"
            tasks.cancel_scope.cancel()
