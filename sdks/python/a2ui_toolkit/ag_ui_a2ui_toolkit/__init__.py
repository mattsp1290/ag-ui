"""
ag-ui-a2ui-toolkit
==================

Framework-agnostic building blocks for A2UI subagent tools. Each per-
framework adapter (LangGraph, ADK, Mastra, …) composes these helpers with its
framework-specific glue (tool decorator, runtime accessor, model binding +
invoke). Nothing in this package depends on any agent framework.
"""

from __future__ import annotations

import json
from typing import Any, Optional, TypedDict


__all__ = [
    "A2UI_OPERATIONS_KEY",
    "BASIC_CATALOG_ID",
    "RENDER_A2UI_TOOL_DEF",
    "create_surface",
    "update_components",
    "update_data_model",
    "build_context_prompt",
    "find_prior_surface",
    "build_subagent_prompt",
    "assemble_ops",
    "wrap_as_operations_envelope",
    "PriorSurface",
    "EditContext",
]


A2UI_OPERATIONS_KEY = "a2ui_operations"
"""Container key the A2UI middleware looks for in tool results."""

BASIC_CATALOG_ID = "https://a2ui.org/specification/v0_9/basic_catalog.json"
"""Default catalog id used when the subagent does not specify one."""


# ---------------------------------------------------------------------------
# Op builders
# ---------------------------------------------------------------------------


def create_surface(surface_id: str, catalog_id: str) -> dict[str, Any]:
    return {
        "version": "v0.9",
        "createSurface": {"surfaceId": surface_id, "catalogId": catalog_id},
    }


def update_components(
    surface_id: str, components: list[dict[str, Any]]
) -> dict[str, Any]:
    return {
        "version": "v0.9",
        "updateComponents": {"surfaceId": surface_id, "components": components},
    }


def update_data_model(
    surface_id: str, data: Any, path: str = "/"
) -> dict[str, Any]:
    return {
        "version": "v0.9",
        "updateDataModel": {"surfaceId": surface_id, "path": path, "value": data},
    }


# ---------------------------------------------------------------------------
# Inner render_a2ui tool definition
# ---------------------------------------------------------------------------

RENDER_A2UI_TOOL_DEF: dict[str, Any] = {
    "type": "function",
    "function": {
        "name": "render_a2ui",
        "description": (
            "Render a dynamic A2UI v0.9 surface. The root component must have "
            "id 'root'. Use components from the available catalog only."
        ),
        "parameters": {
            "type": "object",
            "properties": {
                "surfaceId": {
                    "type": "string",
                    "description": "Unique surface identifier.",
                },
                "catalogId": {
                    "type": "string",
                    "description": "The catalog id for the component catalog.",
                },
                "components": {
                    "type": "array",
                    "description": (
                        "A2UI v0.9 component array (flat format). The root "
                        "component must have id 'root'."
                    ),
                    "items": {"type": "object"},
                },
                "data": {
                    "type": "object",
                    "description": (
                        "Optional initial data model for the surface (form "
                        "values, list items for data-bound components, etc.)."
                    ),
                },
            },
            "required": ["surfaceId", "components"],
        },
    },
}
"""JSON schema for the inner ``render_a2ui`` tool the subagent is forced to call."""


# ---------------------------------------------------------------------------
# State helpers
# ---------------------------------------------------------------------------


def build_context_prompt(state: dict) -> str:
    """Assemble the prompt prefix from AG-UI state context entries + the A2UI
    component catalog.

    Framework integrations conventionally extract the catalog into
    ``state["ag-ui"]["a2ui_schema"]`` and forward other context entries
    (generation guidelines, design guidelines) under
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


# ---------------------------------------------------------------------------
# Prior surface lookup (used for intent="update")
# ---------------------------------------------------------------------------


class PriorSurface(TypedDict, total=False):
    components: list[dict[str, Any]]
    data: Any
    catalogId: Optional[str]


def find_prior_surface(
    messages: list[Any], surface_id: str
) -> Optional[PriorSurface]:
    """Locate the most recent rendered state for ``surface_id`` in message history.

    Walks backwards looking for a ``ToolMessage``-shaped entry whose content is
    a JSON string containing ``a2ui_operations`` for the given surface.
    Returns the reconstructed ``{"components": [...], "data": ..., "catalogId": ...}``
    or ``None`` if no matching surface is found.
    """
    for msg in reversed(messages):
        role = getattr(msg, "type", None) or getattr(msg, "role", None)
        if role not in ("tool", "ToolMessage"):
            continue
        content = getattr(msg, "content", None)
        if not isinstance(content, str):
            continue
        try:
            parsed = json.loads(content)
        except (ValueError, TypeError):
            continue
        if not isinstance(parsed, dict):
            continue
        ops = parsed.get(A2UI_OPERATIONS_KEY)
        if not isinstance(ops, list):
            continue

        components: Optional[list[dict[str, Any]]] = None
        data: Any = None
        catalog_id: Optional[str] = None
        matched = False
        for op in ops:
            if not isinstance(op, dict):
                continue
            if "createSurface" in op:
                cs = op["createSurface"]
                if isinstance(cs, dict) and cs.get("surfaceId") == surface_id:
                    matched = True
                    catalog_id = cs.get("catalogId") or catalog_id
            if "updateComponents" in op:
                uc = op["updateComponents"]
                if isinstance(uc, dict) and uc.get("surfaceId") == surface_id:
                    matched = True
                    if isinstance(uc.get("components"), list):
                        components = uc["components"]
            if "updateDataModel" in op:
                ud = op["updateDataModel"]
                if isinstance(ud, dict) and ud.get("surfaceId") == surface_id:
                    matched = True
                    data = ud.get("value")
        if matched:
            return {
                "components": components or [],
                "data": data,
                "catalogId": catalog_id,
            }
    return None


# ---------------------------------------------------------------------------
# Prompt assembly
# ---------------------------------------------------------------------------


class EditContext(TypedDict, total=False):
    surfaceId: str
    prior: PriorSurface
    changes: Optional[str]


def build_subagent_prompt(
    *,
    context_prompt: str,
    composition_guide: Optional[str] = None,
    edit_context: Optional[EditContext] = None,
) -> str:
    """Compose the full subagent system prompt.

    Args:
        context_prompt: Output of ``build_context_prompt(state)``.
        composition_guide: Project-specific composition rules to append.
        edit_context: When set, instructs the subagent to edit a prior surface
            in place (used by ``intent="update"``).
    """
    parts: list[str] = []
    if context_prompt:
        parts.append(context_prompt)
    if composition_guide:
        parts.append(composition_guide)

    if edit_context:
        surface_id = edit_context.get("surfaceId")
        prior = edit_context.get("prior") or {}
        changes = edit_context.get("changes")
        edit_block = (
            "## Editing an existing surface\n"
            f"You are editing surface '{surface_id}'. Produce the FULL "
            f"updated components array and data model — not just a diff. "
            f"Preserve component ids that the user has not asked to change so "
            f"the renderer can reconcile them. Reuse the same catalogId.\n\n"
            f"### Previous components\n"
            f"{json.dumps(prior.get('components', []), indent=2)}\n\n"
            f"### Previous data\n"
            f"{json.dumps(prior.get('data'), indent=2)}\n"
        )
        if changes:
            edit_block += f"\n### Requested changes\n{changes}\n"
        parts.append(edit_block)

    return "\n".join(p for p in parts if p)


# ---------------------------------------------------------------------------
# Operations envelope
# ---------------------------------------------------------------------------


def assemble_ops(
    *,
    intent: str,
    surface_id: str,
    catalog_id: str,
    components: list[dict[str, Any]],
    data: Optional[dict[str, Any]] = None,
) -> list[dict[str, Any]]:
    """Produce the final A2UI v0.9 operation list for a render result.

    ``intent="create"`` emits ``[createSurface, updateComponents, updateDataModel?]``.
    Any other intent (e.g. ``"update"``) skips ``createSurface`` so the
    frontend reconciles the existing surface in place rather than erroring
    (per v0.9 spec, ``createSurface`` on an existing id is invalid).
    """
    ops: list[dict[str, Any]] = []
    if intent != "update":
        ops.append(create_surface(surface_id, catalog_id))
    ops.append(update_components(surface_id, components))
    if data:
        ops.append(update_data_model(surface_id, data))
    return ops


def wrap_as_operations_envelope(ops: list[dict[str, Any]]) -> str:
    """Wrap a list of A2UI operations as the JSON envelope the A2UI middleware
    looks for in tool results."""
    return json.dumps({A2UI_OPERATIONS_KEY: ops})
