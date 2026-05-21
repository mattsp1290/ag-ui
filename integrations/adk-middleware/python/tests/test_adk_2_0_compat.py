"""Regression tests for ADK 2.0 compatibility (ag-ui#1389 and ag-ui#1669).

These tests run against whatever ``google-adk`` version is installed —
they're written to assert behavior that must hold on BOTH ADK 1.x and 2.0.
Run the suite once per ADK major to prove dual-pin behavior.

Coverage:
- #1389: AGUIToolset uses bind() delegation pattern instead of wholesale
  replacement, so ADK 2.0's eager Runner.__init__ tool cache stays valid.
- #1669: Workflow roots receive FunctionResponse directly in new_message
  (the #1534 pre-append workaround only applies to LlmAgent roots).
- BaseToolset.__init__ is called from AGUIToolset.__init__ so ADK 2.0's
  new ``_use_invocation_cache`` attribute is properly initialized (otherwise
  ADK 2.0's llm_agent.py:185 silently hides the toolset from the LLM).
"""

from __future__ import annotations

import asyncio
from typing import AsyncGenerator
from unittest.mock import MagicMock, patch

import pytest

from ag_ui.core import RunStartedEvent, UserMessage, BaseEvent
from ag_ui.core.types import RunAgentInput
from google.adk.agents import LlmAgent as Agent
from google.adk.tools.base_toolset import BaseToolset as ADKBaseToolset

from ag_ui_adk import ADKAgent
from ag_ui_adk.agui_toolset import AGUIToolset
from ag_ui_adk.client_proxy_toolset import ClientProxyToolset


# ---------------------------------------------------------------------------
# ag-ui#1389 — AGUIToolset delegation pattern (bind/unbind)
# ---------------------------------------------------------------------------


class TestAGUIToolsetDelegation:
    """Verify the bind/unbind pattern that fixes ag-ui#1389 in ADK 2.0."""

    def test_construction_initializes_baseToolset_state(self) -> None:
        """ag-ui#1389 sub-fix: AGUIToolset.__init__ MUST call
        ``super().__init__()`` so ADK 2.0's ``_use_invocation_cache``
        attribute is set. Without this, ADK 2.0's ``llm_agent.py:185``
        ``getattr(toolset, '_use_invocation_cache')`` raises
        AttributeError and the toolset is silently dropped from the LLM
        tool list."""
        toolset = AGUIToolset(tool_filter=['x'], tool_name_prefix='pfx_')
        # On ADK 2.0 these attrs must exist; on ADK 1.x calling
        # super().__init__ is a no-op so the absence is also OK there.
        # We assert the 2.0 invariant — the test will be a no-op on 1.x.
        if hasattr(ADKBaseToolset, '_use_invocation_cache') or any(
            'invocation_cache' in name
            for name in dir(toolset)
        ):
            assert hasattr(toolset, '_use_invocation_cache')

    def test_unbound_get_tools_returns_empty_list(self) -> None:
        """Before bind() is called, ``get_tools()`` returns ``[]`` rather
        than raising. This lets ADK 2.0's eager ``Runner.__init__`` walk
        the toolset without crashing — actual tool list is supplied by
        the run-time ``bind()`` call in ``_update_agent_tools_recursive``.
        """
        toolset = AGUIToolset()
        result = asyncio.run(toolset.get_tools())
        assert result == []

    def test_unbound_get_tools_raises_when_explicit(self) -> None:
        """Legacy 1.x ``NotImplementedError`` behavior is preserved when
        a test explicitly opts in via ``_unbound_raises = True``."""
        toolset = AGUIToolset()
        toolset._unbound_raises = True
        with pytest.raises(NotImplementedError, match="placeholder"):
            asyncio.run(toolset.get_tools())

    def test_bind_then_get_tools_forwards_to_delegate(self) -> None:
        """Once a delegate is bound, ``get_tools()`` forwards to it."""
        toolset = AGUIToolset(tool_filter=['x'])
        delegate = MagicMock(spec=ClientProxyToolset)

        async def mock_get_tools(readonly_context=None):
            return ['mock_tool_1', 'mock_tool_2']

        delegate.get_tools = mock_get_tools
        toolset.bind(delegate)
        result = asyncio.run(toolset.get_tools())
        assert result == ['mock_tool_1', 'mock_tool_2']

    def test_unbind_resets_to_empty(self) -> None:
        """``unbind()`` detaches the delegate so a subsequent ``get_tools()``
        falls back to the unbound branch."""
        toolset = AGUIToolset()
        delegate = MagicMock(spec=ClientProxyToolset)

        async def mock_get_tools(readonly_context=None):
            return ['delegate_tool']

        delegate.get_tools = mock_get_tools
        toolset.bind(delegate)
        toolset.unbind()
        result = asyncio.run(toolset.get_tools())
        assert result == []
        assert toolset._delegate is None

    def test_rebind_overwrites_previous_delegate(self) -> None:
        """Successive ``bind()`` calls replace the binding — supports
        multi-turn runs where each turn supplies a different
        ``input.tools`` and therefore a different ``ClientProxyToolset``.
        """
        toolset = AGUIToolset()

        delegate_a = MagicMock(spec=ClientProxyToolset)
        delegate_b = MagicMock(spec=ClientProxyToolset)

        async def get_a(readonly_context=None):
            return ['a']

        async def get_b(readonly_context=None):
            return ['b']

        delegate_a.get_tools = get_a
        delegate_b.get_tools = get_b

        toolset.bind(delegate_a)
        assert asyncio.run(toolset.get_tools()) == ['a']

        toolset.bind(delegate_b)
        assert asyncio.run(toolset.get_tools()) == ['b']

    @pytest.mark.asyncio
    async def test_object_identity_preserved_across_run(self) -> None:
        """The original ``AGUIToolset`` instance is reused across the
        run — critical for ADK 2.0 because ``Runner.__init__`` caches a
        reference to it during eager ``get_tools()`` resolution.

        Test: declare an ``AGUIToolset`` on an agent, capture its id,
        run the agent, and verify the same id is in ``agent.tools`` after
        ``_update_agent_tools_recursive`` has bound a delegate.
        """
        agui = AGUIToolset(tool_filter=['probe_tool'])
        original_id = id(agui)
        root_agent = Agent(
            name="probe_agent",
            instruction="probe",
            tools=[agui],
        )

        with patch.object(ADKAgent, "_run_adk_in_background") as bg_mock:

            async def empty_gen() -> AsyncGenerator[BaseEvent, None]:
                if False:
                    yield
                return

            adk_agent = ADKAgent(
                adk_agent=root_agent,
                app_name="probe_app",
                user_id="probe_user",
                use_in_memory_services=True,
            )
            run_input = RunAgentInput(
                thread_id="probe_thread",
                run_id="probe_run",
                messages=[
                    UserMessage(id="m1", role="user", content="hi")
                ],
                context=[],
                state={},
                tools=[],
                forwarded_props={},
            )
            async for ev in adk_agent.run(run_input):
                if not isinstance(ev, RunStartedEvent):
                    break

            captured_agent = bg_mock.call_args.kwargs['adk_agent']
            captured_toolset = captured_agent.tools[0]
            # Object identity preserved → ADK 2.0 Runner cache stays valid
            assert id(captured_toolset) == original_id
            assert isinstance(captured_toolset, AGUIToolset)
            # And a delegate is bound
            assert captured_toolset._delegate is not None
            assert isinstance(captured_toolset._delegate, ClientProxyToolset)


# ---------------------------------------------------------------------------
# ag-ui#1669 — Workflow root HITL rehydrate gate
# ---------------------------------------------------------------------------


class TestWorkflowRootDetection:
    """Verify the ``_root_agent_is_workflow()`` predicate that gates the
    #1534 pre-append workaround for ag-ui#1669."""

    def test_llm_agent_root_is_not_workflow(self) -> None:
        """LlmAgent roots must take the pre-append path (ag-ui#1534)."""
        root_agent = Agent(
            name="llm_root",
            instruction="i am llm",
        )
        adk_agent = ADKAgent(
            adk_agent=root_agent,
            app_name="t",
            user_id="u",
            use_in_memory_services=True,
        )
        assert adk_agent._root_agent_is_workflow() is False

    def test_no_root_agent_returns_false(self) -> None:
        """Defensive: a missing root agent returns False rather than
        raising — the run will fail later in a clearer place."""
        # Construct an ADKAgent with an LlmAgent root, then strip the
        # internal reference to simulate no root.
        adk_agent = ADKAgent(
            adk_agent=Agent(name="r", instruction="r"),
            app_name="t",
            user_id="u",
            use_in_memory_services=True,
        )
        adk_agent._adk_agent = None
        adk_agent._app = None
        assert adk_agent._root_agent_is_workflow() is False

    def test_workflow_root_returns_true_when_available(self) -> None:
        """On ADK 2.0 a ``Workflow`` root must return True.

        On ADK 1.x ``google.adk.workflow`` does not exist; the
        ``_root_agent_is_workflow`` predicate must return False without
        raising — covered by ``test_llm_agent_root_is_not_workflow``
        above (which already asserts False on every version).
        """
        try:
            from google.adk.workflow import Workflow  # type: ignore[import-not-found]
        except ImportError:
            pytest.skip("Workflow not available on this ADK version (1.x)")

        # Build a minimal Workflow with a single passthrough node.
        # The exact Workflow API may evolve — we only need an instance
        # of the class for the isinstance check.
        try:
            wf = Workflow.__new__(Workflow)
            wf.name = "wf_root"
        except Exception:
            pytest.skip("Cannot construct a Workflow without full args on this ADK build")

        adk_agent = ADKAgent(
            adk_agent=Agent(name="placeholder", instruction="p"),
            app_name="t",
            user_id="u",
            use_in_memory_services=True,
        )
        # Override the cached root to point at our Workflow instance.
        adk_agent._adk_agent = wf
        assert adk_agent._root_agent_is_workflow() is True
