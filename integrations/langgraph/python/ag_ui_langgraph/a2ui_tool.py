"""
A2UI subagent tool factory for LangGraph agents.

Thin adapter over ``ag-ui-a2ui-toolkit`` — the heavy lifting (op builders,
prompt assembly, history walkers, output envelope) lives in the toolkit so
each new framework adapter (ADK, Mastra, Strands, …) only owns the
framework-specific glue: tool decorator, runtime state access, model
binding + invoke.

Example usage in a chat node::

    from ag_ui_langgraph import get_a2ui_tools

    a2ui = get_a2ui_tools(model=ChatOpenAI(model="gpt-4o"))

    model_with_tools = chat_model.bind_tools(
        [*state["tools"], a2ui],
        parallel_tool_calls=False,
    )
"""

from __future__ import annotations

import json
from typing import Any, Optional

from langchain.tools import tool, ToolRuntime
from langchain_core.language_models import BaseChatModel
from langchain_core.messages import SystemMessage

from ag_ui_a2ui_toolkit import (
    A2UI_OPERATIONS_KEY,
    BASIC_CATALOG_ID,
    RENDER_A2UI_TOOL_DEF,
    assemble_ops,
    build_context_prompt,
    build_subagent_prompt,
    find_prior_surface,
    wrap_as_operations_envelope,
)


# Re-export the toolkit constants for callers that previously imported them
# from this package — keeps the public surface stable.
__all__ = [
    "get_a2ui_tools",
    "A2UI_OPERATIONS_KEY",
    "BASIC_CATALOG_ID",
]


def get_a2ui_tools(
    model: BaseChatModel,
    *,
    composition_guide: Optional[str] = None,
    default_surface_id: str = "dynamic-surface",
    default_catalog_id: str = BASIC_CATALOG_ID,
    tool_name: str = "generate_a2ui",
    tool_description: Optional[str] = None,
):
    """Build a LangGraph tool that delegates A2UI surface generation to a subagent.

    The returned tool is decorated with ``@langchain.tools.tool`` and is
    ready to bind into a chat model alongside any other tools.

    Args:
        model: Chat model the subagent will invoke for structured A2UI output.
            Using the same provider/model as the main agent is fine.
        composition_guide: Optional extra rules appended to the subagent's
            system prompt (e.g. project-specific component usage rules).
        default_surface_id: Surface id used when the subagent omits ``surfaceId``.
        default_catalog_id: Catalog id used when the subagent omits ``catalogId``.
        tool_name: Name advertised to the main agent's planner.
        tool_description: Description shown to the main agent's planner.

    Returns:
        A LangGraph tool callable suitable for ``bind_tools(...)``.
    """

    description = tool_description or (
        "Generate or update a dynamic A2UI surface based on the conversation. "
        "A secondary LLM designs the UI components and data. "
        "Use intent='create' (default) when the user requests new visual content "
        "(cards, forms, lists, dashboards, comparisons, etc.). "
        "Use intent='update' with target_surface_id to modify a surface you "
        "previously rendered (e.g. 'change the second card's price', "
        "'add a Buy button', 'use red instead of blue')."
    )

    @tool(tool_name, description=description)
    def generate_a2ui(
        runtime: ToolRuntime[Any],
        intent: str = "create",
        target_surface_id: Optional[str] = None,
        changes: Optional[str] = None,
    ) -> str:
        """Generate or edit an A2UI surface.

        Args:
            intent: Either ``"create"`` to render a new surface, or ``"update"``
                to modify a surface previously rendered in this conversation.
            target_surface_id: Required when ``intent="update"``. The surface
                id of the prior render to modify.
            changes: Optional natural-language description of the changes to
                apply when ``intent="update"``.
        """
        messages = runtime.state["messages"][:-1]

        is_update = intent == "update" and bool(target_surface_id)
        prior = (
            find_prior_surface(messages, target_surface_id)  # type: ignore[arg-type]
            if is_update
            else None
        )
        if is_update and prior is None:
            return json.dumps(
                {
                    "error": (
                        f"intent='update' requested target_surface_id="
                        f"'{target_surface_id}' but no prior render of that "
                        f"surface was found in conversation history"
                    )
                }
            )

        prompt = build_subagent_prompt(
            context_prompt=build_context_prompt(runtime.state),
            composition_guide=composition_guide,
            edit_context=(
                {"surfaceId": target_surface_id, "prior": prior, "changes": changes}
                if prior is not None
                else None
            ),
        )

        model_with_tool = model.bind_tools(
            [RENDER_A2UI_TOOL_DEF], tool_choice="render_a2ui"
        )

        response = model_with_tool.invoke(
            [SystemMessage(content=prompt), *messages]
        )

        if not response.tool_calls:
            return json.dumps({"error": "LLM did not call render_a2ui"})

        args = response.tool_calls[0]["args"]
        surface_id = (
            target_surface_id
            if is_update
            else (args.get("surfaceId") or default_surface_id)
        )
        catalog_id = (prior or {}).get("catalogId") or default_catalog_id
        components = args.get("components") or []
        data = args.get("data") or {}

        ops = assemble_ops(
            intent="update" if is_update else "create",
            surface_id=surface_id,
            catalog_id=catalog_id,
            components=components,
            data=data,
        )

        return wrap_as_operations_envelope(ops)

    return generate_a2ui
