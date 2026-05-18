"""
Dynamic A2UI tool: LLM-generated UI from conversation context.

A secondary LLM generates v0.9 A2UI components via a structured tool call.
The generate_a2ui tool wraps the output as a2ui_operations, which the
middleware detects in the TOOL_CALL_RESULT and renders automatically.
"""

import json
import os
from typing import Any, List

from langchain.tools import tool, ToolRuntime
from langchain_core.messages import SystemMessage
from langchain_core.tools import tool as lc_tool
from langchain_core.runnables import RunnableConfig
from langchain_openai import ChatOpenAI
from langgraph.graph import StateGraph, END, MessagesState
from langgraph.prebuilt import ToolNode
from ag_ui_langgraph import get_a2ui_tools


base_model = ChatOpenAI(model="gpt-4o")

TOOLS = [get_a2ui_tools(model=base_model)]


class AgentState(MessagesState):
    tools: List[Any]
    copilotkit: dict  # CopilotKit context (actions, etc.)

# LangGraph requires state keys declared in the schema.
# "ag-ui" uses a hyphen which isn't valid as a Python identifier,
# so we patch it into the annotations directly.
AgentState.__annotations__["ag-ui"] = dict


SYSTEM_PROMPT = """You are a helpful assistant that creates rich visual UI on the fly.

When the user asks for visual content (product comparisons, dashboards, lists, cards, etc.),
use the generate_a2ui tool to create a dynamic A2UI surface.
IMPORTANT: After calling the tool, do NOT repeat the data in your text response. The tool renders UI automatically. Just confirm what was rendered."""


async def chat_node(state: AgentState, config: RunnableConfig):
    model = base_model.bind_tools(TOOLS, parallel_tool_calls=False)

    response = await model.ainvoke([
        SystemMessage(content=SYSTEM_PROMPT),
        *state["messages"],
    ], config)

    return {"messages": [response]}


def route_after_chat(state: AgentState):
    last_message = state["messages"][-1]
    if hasattr(last_message, "tool_calls") and last_message.tool_calls:
        return "tool_node"
    return END


workflow = StateGraph(AgentState)
workflow.add_node("chat_node", chat_node)
workflow.add_node("tool_node", ToolNode(tools=TOOLS))
workflow.set_entry_point("chat_node")
workflow.add_conditional_edges("chat_node", route_after_chat)
workflow.add_edge("tool_node", "chat_node")

is_fast_api = os.environ.get("LANGGRAPH_FAST_API", "false").lower() == "true"

if is_fast_api:
    from langgraph.checkpoint.memory import MemorySaver
    memory = MemorySaver()
    graph = workflow.compile(checkpointer=memory)
else:
    graph = workflow.compile()
