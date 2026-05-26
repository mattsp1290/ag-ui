"""Concurrency-safety proof for ``AGUIToolset.bind()`` delegation (ag-ui#1746).

Background
----------
PR #1746 replaced the ADK-1.x "swap the toolset object" strategy with a
``bind()``/``unbind()`` delegation pattern so a single ``AGUIToolset``
*instance* survives across a run (required by ADK 2.0's eager
``Runner.__init__`` tool cache — see #1389).

The defect proven here is that the delegate is stored in a **single mutable
slot on an instance that is shared by reference across concurrent runs**:

* :meth:`ADKAgent._shallow_copy_agent_tree` deliberately shares tool objects
  by reference (``copied.tools = list(tools)`` keeps the same elements) so
  every per-run copy of the agent points at the *same* construction-time
  ``AGUIToolset`` object.
* ``ADKAgent`` runs up to ``max_concurrent_executions`` (default **10**)
  background executions at once. Runs are serialized only per
  ``(thread_id, user_id)`` — two different threads/users run concurrently.
* Each run builds its *own* per-run :class:`ClientProxyToolset` (carrying that
  run's ``input.tools`` and that run's ``event_queue``) and calls
  ``AGUIToolset.bind(proxy)`` on the shared placeholder. The second bind
  clobbers the first; ``_run_adk_in_background``'s ``finally`` then calls
  ``unbind()`` on the shared placeholder regardless of which run is cleaning up.

Consequences, both reproduced below:

1. **Cross-client tool routing.** Run A's ``get_tools()`` resolves Run B's
   tool list, and the resolved :class:`ClientProxyTool` objects carry Run B's
   ``event_queue`` — so Run A's frontend tool calls are streamed to Run B's
   client.
2. **Stranded in-flight run.** When the first run to finish hits its
   ``finally`` and unbinds, a still-in-flight concurrent run loses every tool.

Version note
------------
These tests run against whichever ``google-adk`` is installed. On ADK 1.x
(``get_tools()`` resolved fresh on every ``run_async``) the cross-talk is
direct and unmasked. ADK 2.0's per-Runner ``get_tools()`` cache only *narrows*
the window for the tool-list cross-talk (an unlucky bind/Runner-construction
ordering still hits it) and does nothing for the unbind-strand defect — the
root cause (per-run state on a shared instance) is version-independent.

The tests assert the **concurrency-safe invariant** (each run sees its own
tools, on its own stream, unaffected by its peers). They therefore *fail* on
the current code and are marked ``xfail(strict=True)``; remove the marker once
each run is given an isolated delegate (e.g. a per-run placeholder copy or a
run-scoped delegate keyed off the execution rather than a single shared slot).
"""

from __future__ import annotations

import asyncio
from typing import Any, Callable, Dict, List
from unittest.mock import patch

import pytest

from ag_ui.core import Tool, UserMessage
from ag_ui.core.types import RunAgentInput
from google.adk.agents import LlmAgent

from ag_ui_adk import ADKAgent
from ag_ui_adk.adk_agent import _unbind_agui_toolsets_recursive
from ag_ui_adk.agui_toolset import AGUIToolset

_XFAIL_REASON = (
    "ag-ui#1746: AGUIToolset.bind() stores the per-run ClientProxyToolset on a "
    "single slot of an instance shared by reference across concurrent runs, so "
    "concurrent runs clobber each other's delegate. Remove this marker once each "
    "run gets an isolated delegate."
)


def _make_input(thread_id: str, tool_name: str) -> RunAgentInput:
    """A minimal new-run input for ``thread_id`` exposing exactly one frontend tool."""
    return RunAgentInput(
        thread_id=thread_id,
        run_id=f"run_{thread_id}",
        messages=[UserMessage(id=f"m_{thread_id}", role="user", content="hi")],
        context=[],
        state={},
        tools=[
            Tool(
                name=tool_name,
                description=f"the {tool_name} tool",
                parameters={"type": "object", "properties": {}},
            )
        ],
        forwarded_props={},
    )


def _build_agent() -> tuple[ADKAgent, AGUIToolset]:
    """An ADKAgent whose root LlmAgent declares a single (unfiltered) AGUIToolset.

    Returns the wrapper and the construction-time placeholder instance so tests
    can assert on the object that gets shared across runs.
    """
    placeholder = AGUIToolset()  # no tool_filter -> every client tool passes through
    root = LlmAgent(name="root", instruction="be helpful", tools=[placeholder])
    agent = ADKAgent(
        adk_agent=root,
        app_name="concurrency_app",
        user_id="shared_user",
        use_in_memory_services=True,
    )
    return agent, placeholder


def _patch_background_noop() -> tuple[Any, List[Dict[str, Any]]]:
    """Patch ``_run_adk_in_background`` with an async no-op that records its kwargs.

    The no-op deliberately does NOT run the real ``finally`` unbind, so a test can
    observe the binding state produced by ``_start_background_execution`` for each
    run (and drive the cleanup itself when it wants to model a specific interleaving).
    """
    captured: List[Dict[str, Any]] = []

    async def _noop(self, **kwargs):  # bound as a method -> receives self
        captured.append(kwargs)
        return None

    return patch.object(ADKAgent, "_run_adk_in_background", _noop), captured


async def _await_tasks(*execs) -> None:
    await asyncio.gather(*(e.task for e in execs), return_exceptions=True)


class TestAGUIToolsetConcurrencySafety:
    """Two concurrent runs must not corrupt each other's frontend toolset."""

    @pytest.mark.xfail(strict=True, reason=_XFAIL_REASON)
    async def test_concurrent_bind_routes_first_runs_tools_to_second_runs_stream(self) -> None:
        """Run A, started first and still in flight, must keep seeing *its own*
        tools on *its own* event stream after a concurrent Run B starts.

        On the current code both runs share one ``AGUIToolset``; Run B's
        ``bind()`` overwrites Run A's delegate, so Run A's Runner resolves Run
        B's tool list and Run B's ``event_queue`` — a cross-client leak.
        """
        agent, placeholder = _build_agent()
        bg_patch, captured = _patch_background_noop()

        with bg_patch:
            # Run A starts and is now in flight (background task created, not finished).
            exec_a = await agent._start_background_execution(_make_input("thread-A", "toolA"))
            # Run B (a different thread => not serialized) starts while A is in flight.
            exec_b = await agent._start_background_execution(_make_input("thread-B", "toolB"))
            await _await_tasks(exec_a, exec_b)

        tree_a, tree_b = captured[0]["adk_agent"], captured[1]["adk_agent"]
        toolset_a = tree_a.tools[0]  # the AGUIToolset Run A's Runner will resolve through
        queue_a = captured[0]["event_queue"]
        queue_b = captured[1]["event_queue"]

        # Root cause (reported in the failure message, not asserted, so a fix that
        # isolates delegates without un-sharing the instance still flips this test):
        root_cause = (
            f"shared placeholder across runs: {toolset_a is tree_b.tools[0]}; "
            f"placeholder is construction-time obj: {toolset_a is placeholder}"
        )

        # What Run A's Runner sees when it resolves tools (fresh per run_async on ADK 1.x):
        resolved_a = await toolset_a.get_tools()
        resolved_names = [t.name for t in resolved_a]

        assert resolved_names == ["toolA"], (
            f"Run A's LLM was offered Run B's tools: got {resolved_names}, "
            f"expected ['toolA']. ({root_cause})"
        )
        assert resolved_a[0].event_queue is queue_a, (
            "Run A's frontend tool call would be delivered on Run B's AG-UI event "
            "stream (the shared delegate carries Run B's event_queue) — a "
            f"cross-client leak. ({root_cause})"
        )
        assert resolved_a[0].event_queue is not queue_b

    @pytest.mark.xfail(strict=True, reason=_XFAIL_REASON)
    async def test_inflight_run_not_stranded_by_other_runs_cleanup(self) -> None:
        """When the first run to finish unbinds in its ``finally``, a concurrent
        still-in-flight run must keep its tools.

        On the current code ``_run_adk_in_background``'s ``finally`` calls
        ``_unbind_agui_toolsets_recursive`` on the *shared* placeholder, so Run
        A finishing strands in-flight Run B with an empty tool list.
        """
        agent, _placeholder = _build_agent()
        bg_patch, captured = _patch_background_noop()

        with bg_patch:
            exec_a = await agent._start_background_execution(_make_input("thread-A", "toolA"))
            exec_b = await agent._start_background_execution(_make_input("thread-B", "toolB"))
            await _await_tasks(exec_a, exec_b)

        tree_a, tree_b = captured[0]["adk_agent"], captured[1]["adk_agent"]
        toolset_b = tree_b.tools[0]  # Run B is still in flight

        # Run A finishes first and runs exactly what its real `finally` block runs:
        _unbind_agui_toolsets_recursive(tree_a)

        resolved_b = [t.name for t in await toolset_b.get_tools()]
        assert resolved_b == ["toolB"], (
            f"In-flight Run B lost its tools (got {resolved_b}) because concurrent "
            "Run A's cleanup unbound the shared AGUIToolset placeholder."
        )

    @pytest.mark.xfail(strict=True, reason=_XFAIL_REASON)
    async def test_real_concurrent_runs_each_resolve_their_own_tools(self) -> None:
        """Same defect under genuine concurrent asyncio scheduling.

        Two background executions run as real overlapping tasks. A barrier makes
        the interleaving deterministic: Run A resolves its tools (as its Runner
        would, mid-flight) only *after* Run B has bound. Each run must observe
        its own tools/stream.
        """
        agent, _placeholder = _build_agent()

        release = asyncio.Event()
        started: Dict[str, asyncio.Event] = {"thread-A": asyncio.Event(), "thread-B": asyncio.Event()}
        resolved: Dict[str, Dict[str, Any]] = {}

        async def runner_fake(self, *, input, adk_agent, event_queue, client_proxy_toolsets, **kwargs):
            label = input.thread_id
            started[label].set()
            await release.wait()  # park until both runs have bound
            toolset = adk_agent.tools[0]
            tools = await toolset.get_tools()  # what this run's Runner resolves
            resolved[label] = {
                "names": [t.name for t in tools],
                "own_queue": event_queue,
                "resolved_queue": tools[0].event_queue if tools else None,
            }

        async def _wait_until(pred: Callable[[], bool], timeout: float = 5.0) -> None:
            deadline = asyncio.get_event_loop().time() + timeout
            while not pred():
                if asyncio.get_event_loop().time() > deadline:
                    raise AssertionError("condition not met within timeout")
                await asyncio.sleep(0.005)

        with patch.object(ADKAgent, "_run_adk_in_background", runner_fake):
            exec_a = await agent._start_background_execution(_make_input("thread-A", "toolA"))
            await asyncio.wait_for(started["thread-A"].wait(), 5)  # A bound & parked
            exec_b = await agent._start_background_execution(_make_input("thread-B", "toolB"))
            await asyncio.wait_for(started["thread-B"].wait(), 5)  # B bound (clobbers A)
            release.set()
            await _wait_until(lambda: {"thread-A", "thread-B"} <= resolved.keys())
            await _await_tasks(exec_a, exec_b)

        assert resolved["thread-A"]["names"] == ["toolA"], (
            f"Run A resolved {resolved['thread-A']['names']} under concurrent "
            "scheduling — Run B's concurrent bind() overwrote Run A's tools."
        )
        assert resolved["thread-A"]["resolved_queue"] is resolved["thread-A"]["own_queue"], (
            "Run A's tool calls would be routed to Run B's event stream."
        )
