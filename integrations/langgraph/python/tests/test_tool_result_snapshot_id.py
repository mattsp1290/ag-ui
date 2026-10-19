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
import asyncio
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


@tool("get_weather")
def get_weather_command_multi(city: str, tool_call_id: Annotated[str, InjectedToolCallId]) -> Command:
    """Get weather, returning a Command that carries several tool messages without ids.

    Besides its own result, the Command records a follow-up lookup the tool made
    (an assistant tool call plus its result), so one Command yields two
    TOOL_CALL_RESULT events.
    """
    inner_call_id = f"{tool_call_id}_forecast"
    return Command(update={"messages": [
        ToolMessage(content=f"Sunny in {city}", tool_call_id=tool_call_id),
        AIMessage(content="", tool_calls=[{"name": "get_forecast", "args": {"city": city},
                                           "id": inner_call_id, "type": "tool_call"}]),
        {"type": "tool", "content": f"Sunny all week in {city}",
         "tool_call_id": inner_call_id, "name": "get_forecast"},
    ]})


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


def _make_agent(weather_tool, emit_subagent_events: bool = False) -> LangGraphAgent:
    return LangGraphAgent(name="tool-result-id", graph=_build_graph(weather_tool),
                          emit_subagent_events=emit_subagent_events)


async def _run(weather_tool, *, agent: LangGraphAgent = None, messages=None,
               run_id: str = "run-1", delay: float = 0.0) -> List:
    """Drain one run. ``delay`` > 0 makes the consumer slow (as an SSE write to
    a slow client is): the graph keeps running while each event is handled."""
    agent = agent or _make_agent(weather_tool)
    run_input = RunAgentInput(
        thread_id="thread-1", run_id=run_id, state={},
        messages=messages or [UserMessage(id="user-1", role="user", content="Weather in Paris?")],
        tools=[], context=[], forwarded_props={},
    )
    events = []
    async for event in agent.run(run_input):
        if isinstance(event, str):
            continue
        events.append(event)
        if delay:
            await asyncio.sleep(delay)
    return events


class TestToolCallResultMatchesSnapshot(unittest.IsolatedAsyncioTestCase):

    async def _assert_result_id_matches_snapshot(self, weather_tool, *, delay: float = 0.0,
                                                 emit_subagent_events: bool = False):
        agent = _make_agent(weather_tool, emit_subagent_events)
        events = await _run(weather_tool, agent=agent, delay=delay)
        results = [e for e in events if e.type == EventType.TOOL_CALL_RESULT]
        snapshots = [e for e in events if e.type == EventType.MESSAGES_SNAPSHOT]
        self.assertTrue(results, "expected a TOOL_CALL_RESULT")
        self.assertTrue(snapshots, "expected a MESSAGES_SNAPSHOT")

        snapshot_tools = {m.tool_call_id: m for m in snapshots[-1].messages if m.role == "tool"}
        self.assertEqual(len(snapshot_tools), len(results))
        for result in results:
            snapshot_tool = snapshot_tools[result.tool_call_id]
            self.assertEqual(result.message_id, snapshot_tool.id)
            # The id is a fresh one, not the tool_call_id fallback.
            self.assertNotEqual(result.message_id, result.tool_call_id)

        # The snapshot keeps the order the client must send back:
        # user -> assistant(tool_call) -> tool(s) -> assistant(text).
        roles = [m.role for m in snapshots[-1].messages]
        self.assertEqual(roles[:3], ["user", "assistant", "tool"])
        self.assertEqual(roles[-1], "assistant")
        return agent, results, snapshots[-1]

    async def test_plain_tool(self):
        await self._assert_result_id_matches_snapshot(get_weather)

    async def test_tool_returning_command(self):
        await self._assert_result_id_matches_snapshot(get_weather_command)

    async def test_tool_returning_command_with_dict_message(self):
        await self._assert_result_id_matches_snapshot(get_weather_command_dict)

    async def test_tool_returning_command_with_several_messages(self):
        _, results, _ = await self._assert_result_id_matches_snapshot(get_weather_command_multi)
        self.assertEqual(len(results), 2)


class TestToolCallResultMatchesSnapshotWithSlowConsumer(unittest.IsolatedAsyncioTestCase):
    """The same contract when the consumer lags behind the graph.

    ``astream_events`` runs the graph in its own task and does not wait for the
    consumer, so by the time the adapter handles ``on_tool_end`` ToolNode may
    already have deep-copied a returned Command and LangGraph stored the copy
    under an id of its own. The streamed id must match the snapshot anyway, and
    the client sending the snapshot back must not duplicate tool messages.
    """

    DELAY = 0.02

    async def _assert_two_turns(self, weather_tool, emit_subagent_events: bool):
        agent = _make_agent(weather_tool, emit_subagent_events)
        events = await _run(weather_tool, agent=agent, delay=self.DELAY)
        results = [e for e in events if e.type == EventType.TOOL_CALL_RESULT]
        snapshot = [e for e in events if e.type == EventType.MESSAGES_SNAPSHOT][-1]
        snapshot_tools = {m.tool_call_id: m for m in snapshot.messages if m.role == "tool"}
        self.assertTrue(results)
        self.assertEqual(len(snapshot_tools), len(results))
        streamed = {r.tool_call_id: r.message_id for r in results}
        for call_id, message_id in streamed.items():
            self.assertEqual(snapshot_tools[call_id].id, message_id)

        # Second turn: the client sends its history (the snapshot) back.
        second_input = [*snapshot.messages,
                        UserMessage(id="user-2", role="user", content="And tomorrow?")]
        events2 = await _run(weather_tool, agent=agent, messages=second_input,
                             run_id="run-2", delay=self.DELAY)
        snapshot2 = [e for e in events2 if e.type == EventType.MESSAGES_SNAPSHOT][-1]
        results2 = [e for e in events2 if e.type == EventType.TOOL_CALL_RESULT]

        # No tool message was duplicated in graph state.
        state = await agent.graph.aget_state({"configurable": {"thread_id": "thread-1"}})
        stored_tool_call_ids = [m.tool_call_id for m in state.values["messages"]
                                if isinstance(m, ToolMessage)]
        self.assertEqual(len(stored_tool_call_ids), len(set(stored_tool_call_ids)),
                         stored_tool_call_ids)
        self.assertEqual(len(stored_tool_call_ids), len(results) + len(results2))

        # Nor in the snapshot, and turn 1's tool messages keep the ids the
        # client already holds; turn 2's match their streamed results too.
        snapshot2_tools = [m for m in snapshot2.messages if m.role == "tool"]
        self.assertEqual(len(snapshot2_tools), len({m.tool_call_id for m in snapshot2_tools}))
        snapshot2_ids = {m.tool_call_id: m.id for m in snapshot2_tools}
        streamed.update({r.tool_call_id: r.message_id for r in results2})
        self.assertEqual(snapshot2_ids, streamed)

    async def _assert_both_modes(self, weather_tool):
        for emit_subagent_events in (False, True):
            with self.subTest(emit_subagent_events=emit_subagent_events):
                await self._assert_two_turns(weather_tool, emit_subagent_events)

    async def test_plain_tool(self):
        await self._assert_both_modes(get_weather)

    async def test_tool_returning_command(self):
        await self._assert_both_modes(get_weather_command)

    async def test_tool_returning_command_with_dict_message(self):
        await self._assert_both_modes(get_weather_command_dict)

    async def test_tool_returning_command_with_several_messages(self):
        await self._assert_both_modes(get_weather_command_multi)


if __name__ == "__main__":
    unittest.main()
