"""Unit tests for ag_ui_a2ui_toolkit's pure helpers.

Mirrors the TypeScript ``a2ui-toolkit/src/__tests__/toolkit.test.ts`` suite
so both languages stay aligned on expected behavior.
"""

from __future__ import annotations

import json
import unittest

from ag_ui_a2ui_toolkit import (
    A2UI_OPERATIONS_KEY,
    BASIC_CATALOG_ID,
    RENDER_A2UI_TOOL_DEF,
    assemble_ops,
    build_context_prompt,
    build_subagent_prompt,
    create_surface,
    find_prior_surface,
    update_components,
    update_data_model,
    wrap_as_operations_envelope,
)


class TestConstants(unittest.TestCase):
    def test_operations_key(self):
        self.assertEqual(A2UI_OPERATIONS_KEY, "a2ui_operations")

    def test_basic_catalog_id(self):
        self.assertEqual(
            BASIC_CATALOG_ID,
            "https://a2ui.org/specification/v0_9/basic_catalog.json",
        )


class TestRenderToolDef(unittest.TestCase):
    def test_shape(self):
        self.assertEqual(RENDER_A2UI_TOOL_DEF["type"], "function")
        self.assertEqual(RENDER_A2UI_TOOL_DEF["function"]["name"], "render_a2ui")

    def test_required_fields(self):
        self.assertEqual(
            RENDER_A2UI_TOOL_DEF["function"]["parameters"]["required"],
            ["surfaceId", "components"],
        )

    def test_parameter_keys(self):
        self.assertEqual(
            list(RENDER_A2UI_TOOL_DEF["function"]["parameters"]["properties"].keys()),
            ["surfaceId", "components", "data"],
        )


class TestOpBuilders(unittest.TestCase):
    def test_create_surface(self):
        self.assertEqual(
            create_surface("s1", "c1"),
            {
                "version": "v0.9",
                "createSurface": {"surfaceId": "s1", "catalogId": "c1"},
            },
        )

    def test_update_components(self):
        comps = [{"id": "root", "component": "Row"}]
        self.assertEqual(
            update_components("s1", comps),
            {
                "version": "v0.9",
                "updateComponents": {"surfaceId": "s1", "components": comps},
            },
        )

    def test_update_data_model_defaults(self):
        self.assertEqual(
            update_data_model("s1", {"items": []}),
            {
                "version": "v0.9",
                "updateDataModel": {
                    "surfaceId": "s1",
                    "path": "/",
                    "value": {"items": []},
                },
            },
        )

    def test_update_data_model_custom_path(self):
        self.assertEqual(
            update_data_model("s1", "hello", "/title"),
            {
                "version": "v0.9",
                "updateDataModel": {
                    "surfaceId": "s1",
                    "path": "/title",
                    "value": "hello",
                },
            },
        )


class TestBuildContextPrompt(unittest.TestCase):
    def test_empty_state(self):
        self.assertEqual(build_context_prompt({}), "")

    def test_described_entry(self):
        prompt = build_context_prompt(
            {
                "ag-ui": {
                    "context": [
                        {"description": "Style guide", "value": "use cards"}
                    ],
                }
            }
        )
        self.assertIn("## Style guide", prompt)
        self.assertIn("use cards", prompt)

    def test_value_only_entry(self):
        prompt = build_context_prompt(
            {"ag-ui": {"context": [{"value": "free-form note"}]}}
        )
        self.assertIn("free-form note", prompt)
        self.assertNotIn("##", prompt)

    def test_catalog_section(self):
        prompt = build_context_prompt({"ag-ui": {"a2ui_schema": "<catalog json>"}})
        self.assertIn("## Available Components", prompt)
        self.assertIn("<catalog json>", prompt)

    def test_empty_entries_dropped(self):
        prompt = build_context_prompt({"ag-ui": {"context": [{}]}})
        self.assertEqual(prompt, "")


class _ToolMessage:
    """Minimal stand-in for langchain's ToolMessage (or similar) — exposes
    ``type`` and ``content`` as attributes so the role-detection path works."""

    def __init__(self, content: str, role: str = "tool"):
        self.type = role
        self.content = content


class TestFindPriorSurface(unittest.TestCase):
    @staticmethod
    def _tool(content):
        return _ToolMessage(json.dumps(content))

    def test_returns_none_when_missing(self):
        messages = [self._tool({A2UI_OPERATIONS_KEY: []})]
        self.assertIsNone(find_prior_surface(messages, "missing"))

    def test_reconstructs_state(self):
        messages = [
            self._tool(
                {
                    A2UI_OPERATIONS_KEY: [
                        create_surface("s1", "cat://x"),
                        update_components("s1", [{"id": "root", "component": "Row"}]),
                        update_data_model("s1", {"items": [1, 2]}),
                    ]
                }
            )
        ]
        prior = find_prior_surface(messages, "s1")
        self.assertEqual(prior["components"], [{"id": "root", "component": "Row"}])
        self.assertEqual(prior["data"], {"items": [1, 2]})
        self.assertEqual(prior["catalogId"], "cat://x")

    def test_prefers_latest(self):
        messages = [
            self._tool(
                {
                    A2UI_OPERATIONS_KEY: [
                        create_surface("s1", "old-cat"),
                        update_components("s1", [{"id": "root", "component": "Row"}]),
                    ]
                }
            ),
            self._tool(
                {
                    A2UI_OPERATIONS_KEY: [
                        update_components("s1", [{"id": "root", "component": "Column"}]),
                        update_data_model("s1", {"changed": True}),
                    ]
                }
            ),
        ]
        prior = find_prior_surface(messages, "s1")
        self.assertEqual(prior["components"], [{"id": "root", "component": "Column"}])
        self.assertEqual(prior["data"], {"changed": True})

    def test_ignores_non_tool(self):
        messages = [
            _ToolMessage("not a tool", role="assistant"),
            _ToolMessage("not json", role="tool"),
            self._tool({"unrelated": "payload"}),
        ]
        self.assertIsNone(find_prior_surface(messages, "s1"))

    def test_accepts_dict_style_messages(self):
        # Dict-style messages with explicit ``type`` should also work via
        # getattr fallthrough — but the toolkit reads attributes only, so
        # callers pass dicts wrapped in objects. This covers the attribute path.
        msg = _ToolMessage(
            json.dumps(
                {
                    A2UI_OPERATIONS_KEY: [
                        create_surface("s1", "c"),
                        update_components(
                            "s1", [{"id": "root", "component": "Row"}]
                        ),
                    ]
                }
            )
        )
        prior = find_prior_surface([msg], "s1")
        self.assertEqual(prior["catalogId"], "c")


class TestBuildSubagentPrompt(unittest.TestCase):
    def test_context_only(self):
        self.assertEqual(
            build_subagent_prompt(context_prompt="ctx"), "ctx"
        )

    def test_appends_composition_guide(self):
        prompt = build_subagent_prompt(
            context_prompt="ctx", composition_guide="guide"
        )
        self.assertEqual(prompt, "ctx\nguide")

    def test_edit_block(self):
        prompt = build_subagent_prompt(
            context_prompt="ctx",
            edit_context={
                "surfaceId": "s1",
                "prior": {
                    "components": [{"id": "root", "component": "Row"}],
                    "data": {"x": 1},
                },
                "changes": "make the title bigger",
            },
        )
        self.assertIn("Editing an existing surface", prompt)
        self.assertIn("'s1'", prompt)
        self.assertIn('"id": "root"', prompt)
        self.assertIn('"x": 1', prompt)
        self.assertIn("Requested changes", prompt)
        self.assertIn("make the title bigger", prompt)

    def test_omits_requested_changes_when_none(self):
        prompt = build_subagent_prompt(
            context_prompt="ctx",
            edit_context={"surfaceId": "s1", "prior": {"components": [], "data": None}},
        )
        self.assertNotIn("Requested changes", prompt)

    def test_empty_context_returns_empty(self):
        self.assertEqual(build_subagent_prompt(context_prompt=""), "")


class TestAssembleOps(unittest.TestCase):
    def test_create_intent_full_envelope(self):
        ops = assemble_ops(
            intent="create",
            surface_id="s1",
            catalog_id="cat://x",
            components=[{"id": "root", "component": "Row"}],
            data={"items": ["a"]},
        )
        self.assertEqual(len(ops), 3)
        self.assertIn("createSurface", ops[0])
        self.assertIn("updateComponents", ops[1])
        self.assertIn("updateDataModel", ops[2])

    def test_update_intent_skips_create_surface(self):
        ops = assemble_ops(
            intent="update",
            surface_id="s1",
            catalog_id="cat://x",
            components=[{"id": "root", "component": "Row"}],
            data={"items": ["a"]},
        )
        self.assertEqual(len(ops), 2)
        self.assertIn("updateComponents", ops[0])
        self.assertIn("updateDataModel", ops[1])

    def test_no_data_omits_data_model_op(self):
        ops = assemble_ops(
            intent="create",
            surface_id="s1",
            catalog_id="cat://x",
            components=[{"id": "root", "component": "Row"}],
        )
        self.assertEqual(len(ops), 2)
        self.assertIn("createSurface", ops[0])
        self.assertIn("updateComponents", ops[1])

    def test_empty_data_omits_data_model_op(self):
        ops = assemble_ops(
            intent="create",
            surface_id="s1",
            catalog_id="cat://x",
            components=[{"id": "root", "component": "Row"}],
            data={},
        )
        self.assertEqual(len(ops), 2)


class TestWrapAsOperationsEnvelope(unittest.TestCase):
    def test_serializes_under_key(self):
        ops = [create_surface("s1", "c")]
        envelope = json.loads(wrap_as_operations_envelope(ops))
        self.assertEqual(envelope, {A2UI_OPERATIONS_KEY: ops})

    def test_empty_ops(self):
        envelope = json.loads(wrap_as_operations_envelope([]))
        self.assertEqual(envelope, {A2UI_OPERATIONS_KEY: []})


if __name__ == "__main__":
    unittest.main()
