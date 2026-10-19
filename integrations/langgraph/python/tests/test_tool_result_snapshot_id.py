"""TOOL_CALL_RESULT.messageId must equal the tool message id in MESSAGES_SNAPSHOT.

Regression: at ``on_tool_end`` the ToolMessage a tool returns usually has no
id; the adapter used to emit ``message_id = tool_call_id`` while LangGraph's
``add_messages`` assigned a fresh uuid4 when it stored the message. The
snapshot then carried the tool message under a different id, so the client's
id-based snapshot merge kept the streamed copy *and* appended the snapshot's
copy after the assistant's final text (assistant(tool_call) -> assistant(text)
-> tool), which was sent back on every later turn.

These tests run a real, minimal, fully offline graph (fake chat model emitting
one tool call, prebuilt ToolNode, MemorySaver) through the adapter.
"""
import unittest
from typing import Iterator, List

from langchain_core.language_models.chat_models import BaseChatModel
from langchain_core.messages import AIMessage, AIMessageChunk, ToolMessage
from langchain_core.messages.tool import tool_call_chunk
from langchain_core.outputs import ChatGeneration, ChatGenerationChunk, ChatResult
from langchain_core.tools import InjectedToolCallId, tool
from langgraph.checkpoint.memory import MemorySaver
from langgraph.graph import START, MessagesState, StateGraph
from langgraph.prebuilt import ToolNode, tools_condition
from langgraph.types import Command
from typing_extensions import Annotated

from ag_ui.core import EventType, RunAgentInput, UserMessage
from ag_ui_langgraph import LangGraphAgent


class _FakeToolCallingModel(BaseChatModel):
    """Calls ``get_weather`` once, then answers with text after the tool result."""

    counter: int = 0

    @property
    def _llm_type(self) -> str:
        return "fake-tool-calling"

    def bind_tools(self, tools, **kwargs):
        return self

    def _plan(self, messages):
        self.counter += 1
        if isinstance(messages[-1], ToolMessage):
            return "text", f"call_{self.counter}"
        return "tool", f"call_{self.counter}"

    def _generate(self, messages, stop=None, run_manager=None, **kwargs):
        kind, call_id = self._plan(messages)
        if kind == "text":
            message = AIMessage(content="It is sunny.")
        else:
            message = AIMessage(
                content="",
                tool_calls=[{"name": "get_weather", "args": {"city": "Paris"},
                             "id": call_id, "type": "tool_call"}],
            )
        return ChatResult(generations=[ChatGeneration(message=message)])

    def _stream(self, messages, stop=None, run_manager=None, **kwargs) -> Iterator[ChatGenerationChunk]:
        kind, call_id = self._plan(messages)
        if kind == "text":
            chunks = [AIMessageChunk(content="It is "), AIMessageChunk(content="sunny.")]
        else:
            chunks = [
                AIMessageChunk(content="", tool_call_chunks=[
                    tool_call_chunk(name="get_weather", args="", id=call_id, index=0)]),
                AIMessageChunk(content="", tool_call_chunks=[
                    tool_call_chunk(name=None, args='{"city": "Paris"}', id=None, index=0)]),
            ]
        for chunk in chunks:
            generation = ChatGenerationChunk(message=chunk)
            if run_manager:
                run_manager.on_llm_new_token(chunk.content or "", chunk=generation)
            yield generation


@tool
def get_weather(city: str) -> str:
    """Get weather."""
    return f"Sunny in {city}"


@tool("get_weather")
def get_weather_command(city: str, tool_call_id: Annotated[str, InjectedToolCallId]) -> Command:
    """Get weather, returning a Command that carries a ToolMessage without an id."""
    return Command(update={"messages": [
        ToolMessage(content=f"Sunny in {city}", tool_call_id=tool_call_id)]})


@tool("get_weather")
def get_weather_command_dict(city: str, tool_call_id: Annotated[str, InjectedToolCallId]) -> Command:
    """Get weather, returning a Command whose message is a dict without an id."""
    return Command(update={"messages": [
        {"type": "tool", "content": f"Sunny in {city}", "tool_call_id": tool_call_id}]})


def _build_graph(weather_tool):
    model = _FakeToolCallingModel()

    async def agent_node(state: MessagesState):
        return {"messages": [await model.ainvoke(state["messages"])]}

    graph = StateGraph(MessagesState)
    graph.add_node("agent", agent_node)
    graph.add_node("tools", ToolNode([weather_tool]))
    graph.add_edge(START, "agent")
    graph.add_conditional_edges("agent", tools_condition)
    graph.add_edge("tools", "agent")
    return graph.compile(checkpointer=MemorySaver())


async def _run(weather_tool) -> List:
    agent = LangGraphAgent(name="tool-result-id", graph=_build_graph(weather_tool))
    run_input = RunAgentInput(
        thread_id="thread-1", run_id="run-1", state={},
        messages=[UserMessage(id="user-1", role="user", content="Weather in Paris?")],
        tools=[], context=[], forwarded_props={},
    )
    return [event async for event in agent.run(run_input) if not isinstance(event, str)]


class TestToolCallResultMatchesSnapshot(unittest.IsolatedAsyncioTestCase):

    async def _assert_result_id_matches_snapshot(self, weather_tool):
        events = await _run(weather_tool)
        results = [e for e in events if e.type == EventType.TOOL_CALL_RESULT]
        snapshots = [e for e in events if e.type == EventType.MESSAGES_SNAPSHOT]
        self.assertEqual(len(results), 1)
        self.assertTrue(snapshots, "expected a MESSAGES_SNAPSHOT")

        snapshot_tool_messages = [m for m in snapshots[-1].messages if m.role == "tool"]
        self.assertEqual(len(snapshot_tool_messages), 1)
        snapshot_tool = snapshot_tool_messages[0]

        self.assertEqual(snapshot_tool.tool_call_id, results[0].tool_call_id)
        self.assertEqual(results[0].message_id, snapshot_tool.id)
        # The id is a fresh one, not the tool_call_id fallback.
        self.assertNotEqual(results[0].message_id, results[0].tool_call_id)

        # The snapshot keeps the order the client must send back:
        # user -> assistant(tool_call) -> tool -> assistant(text).
        self.assertEqual([m.role for m in snapshots[-1].messages],
                         ["user", "assistant", "tool", "assistant"])

    async def test_plain_tool(self):
        await self._assert_result_id_matches_snapshot(get_weather)

    async def test_tool_returning_command(self):
        await self._assert_result_id_matches_snapshot(get_weather_command)

    async def test_tool_returning_command_with_dict_message(self):
        await self._assert_result_id_matches_snapshot(get_weather_command_dict)


if __name__ == "__main__":
    unittest.main()
