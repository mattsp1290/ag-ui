"""
A2UI subagent tool factory for LangGraph agents.

Ships a ready-to-bind LangGraph tool that delegates dynamic A2UI surface
generation to a secondary LLM call. The author imports the factory, passes
their chat model in, and binds the returned tool alongside their other tools.
No further A2UI-specific code is required on the author's side.

Example usage in a chat node::

    from ag_ui_langgraph import a2ui_subagent_tool

    a2ui = a2ui_subagent_tool(model=ChatOpenAI(model="gpt-4o"))

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
from langchain_core.tools import tool as lc_tool


A2UI_OPERATIONS_KEY = "a2ui_operations"
"""Container key the A2UI middleware looks for in tool results."""

BASIC_CATALOG_ID = "https://a2ui.org/specification/v0_9/basic_catalog.json"
"""Default catalog id used when the subagent does not specify one."""


def _create_surface(surface_id: str, catalog_id: str) -> dict[str, Any]:
    return {
        "version": "v0.9",
        "createSurface": {"surfaceId": surface_id, "catalogId": catalog_id},
    }


def _update_components(
    surface_id: str, components: list[dict[str, Any]]
) -> dict[str, Any]:
    return {
        "version": "v0.9",
        "updateComponents": {"surfaceId": surface_id, "components": components},
    }


def _update_data_model(
    surface_id: str, data: Any, path: str = "/"
) -> dict[str, Any]:
    return {
        "version": "v0.9",
        "updateDataModel": {"surfaceId": surface_id, "path": path, "value": data},
    }


def _build_context_prompt(state: dict) -> str:
    """Assemble the subagent prompt prefix from AG-UI context + schema in state.

    The LangGraph AG-UI integration extracts the A2UI component schema into
    ``state["ag-ui"]["a2ui_schema"]`` and forwards any other context entries
    (generation guidelines, design guidelines, etc.) under
    ``state["ag-ui"]["context"]``.
    """
    ag_ui = state.get("ag-ui", {}) or {}
    parts: list[str] = []

    for entry in ag_ui.get("context", []) or []:
        if isinstance(entry, dict):
            desc = entry.get("description")
            value = entry.get("value")
        else:
            desc = getattr(entry, "description", None)
            value = getattr(entry, "value", None)
        if desc:
            parts.append(f"## {desc}\n{value}\n")
        elif value:
            parts.append(f"{value}\n")

    a2ui_schema = ag_ui.get("a2ui_schema")
    if a2ui_schema:
        parts.append(f"## Available Components\n{a2ui_schema}\n")

    return "\n".join(parts)


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

    @lc_tool
    def render_a2ui(
        surfaceId: str,
        catalogId: str,
        components: list[dict],
        data: dict | None = None,
    ) -> str:
        """Render a dynamic A2UI v0.9 surface.

        Args:
            surfaceId: Unique surface identifier.
            catalogId: The catalog ID for the component catalog.
            components: A2UI v0.9 component array (flat format). The root
                component must have id "root".
            data: Optional initial data model for the surface (form values,
                list items for data-bound components, etc.).
        """
        return "rendered"

    description = tool_description or (
        "Generate a dynamic A2UI surface based on the conversation. A secondary "
        "LLM designs the UI components and data. Use this when the user requests "
        "visual content (cards, forms, lists, dashboards, comparisons, etc.)."
    )

    @tool(tool_name, description=description)
    def generate_a2ui(runtime: ToolRuntime[Any]) -> str:
        # The last message is this tool call itself, not yet balanced with a
        # tool result. Strip it before passing history to the subagent so the
        # subagent does not see an unfinished tool call.
        messages = runtime.state["messages"][:-1]

        prompt_parts = [_build_context_prompt(runtime.state)]
        if composition_guide:
            prompt_parts.append(composition_guide)
        prompt = "\n".join(p for p in prompt_parts if p)

        model_with_tool = model.bind_tools(
            [render_a2ui], tool_choice="render_a2ui"
        )

        response = model_with_tool.invoke(
            [SystemMessage(content=prompt), *messages]
        )

        if not response.tool_calls:
            return json.dumps({"error": "LLM did not call render_a2ui"})

        args = response.tool_calls[0]["args"]
        surface_id = args.get("surfaceId") or default_surface_id
        catalog_id = args.get("catalogId") or default_catalog_id
        components = args.get("components") or []
        data = args.get("data") or {}

        ops: list[dict[str, Any]] = [
            _create_surface(surface_id, catalog_id),
            _update_components(surface_id, components),
        ]
        if data:
            ops.append(_update_data_model(surface_id, data))

        return json.dumps({A2UI_OPERATIONS_KEY: ops})

    return generate_a2ui
