"""Tests for the run-admission serialization + robustness hardening.

These cover four changes (see the reviewed Notion proposal):

  Fix 1 — SERIALIZE concurrent same-thread run() invocations behind a dedicated
          per-thread run-admission lock (``_run_locks``), held from admission
          (before ``worker.query()`` / before ``RUN_STARTED``) through
          ``RUN_FINISHED`` and released on EVERY exit path. Different thread_ids
          stay concurrent.
  Fix 2 — ``query_timeout_seconds`` defaults to a generous 300s (was None →
          unbounded hang on a dead/slow worker), still overridable.
  Fix 3 — worker-death fan-out: ``SessionWorker`` signals a terminal
          WorkerError + None sentinel to ALL in-flight output queues on fatal
          worker death, so a queued/peer consumer cannot hang.
  Fix 4 — ``_per_thread_result`` is per-run, keyed by (thread_id, run_id), so a
          run's RUN_FINISHED.result reflects its OWN ResultMessage.

The dedicated ``_run_locks`` MUST be distinct from ``_state_locks`` (which is
acquired mid-stream on the state-update-tool path); reusing it would self-
deadlock the instant the model emits a state-update tool call. Scenario (c)
exercises run-lock + inner state-lock together to prove no deadlock.
"""

import asyncio

import pytest

from ag_ui.core import EventType
from ag_ui_claude_sdk.adapter import ClaudeAgentAdapter
from ag_ui_claude_sdk.config import STATE_MANAGEMENT_TOOL_FULL_NAME

from .conftest import stream_event, aiter


def _types(events):
    return [e.type for e in events]


async def _drive(adapter, inp):
    return [e async for e in adapter.run(inp)]


async def _wait_for(predicate, *, tries=2000):
    for _ in range(tries):
        if predicate():
            return True
        await asyncio.sleep(0)
    return False


# ---------------------------------------------------------------------------
# Fake workers used to drive run() deterministically without an LLM.
# ---------------------------------------------------------------------------


class _GatedTextWorker:
    """Worker whose query() streams a tiny text run, but only after a per-call
    gate is released. Tracks the order in which RUN_STARTED-able streams begin so
    a test can assert serialization ordering.

    A shared ``log`` list records (event, run_marker) tuples for ordering checks.
    """

    def __init__(self, *a, **kw):
        pass

    async def start(self):
        pass

    def is_alive(self):
        return True

    async def stop(self):
        pass


def _make_text_stream():
    return [
        stream_event({"type": "message_start"}),
        stream_event(
            {"type": "content_block_delta", "delta": {"type": "text_delta", "text": "hi"}}
        ),
        stream_event({"type": "message_stop"}),
    ]


class TestSerializeSameThread:
    @pytest.mark.asyncio
    async def test_two_same_thread_runs_are_serialized(self, make_input, monkeypatch):
        # (a) Two overlapping same-thread runs: B's RUN_STARTED must be emitted
        # only AFTER A's RUN_FINISHED. The run-admission lock holds A's slot
        # across its whole run; B waits at admission.
        order = []  # records ("A"/"B", event_type)
        a_gate = asyncio.Event()  # released to let A's stream complete

        class _OrderedWorker:
            calls = 0

            def __init__(self, *a, **kw):
                pass

            async def start(self):
                pass

            def is_alive(self):
                return True

            def query(self, prompt, session_id="default"):
                idx = _OrderedWorker.calls
                _OrderedWorker.calls += 1

                async def _gen_first():
                    # A: hold the stream open so, IF B were not serialized, B
                    # would be able to emit RUN_STARTED while A is mid-run.
                    await a_gate.wait()
                    for ev in _make_text_stream():
                        yield ev

                async def _gen_second():
                    for ev in _make_text_stream():
                        yield ev

                return _gen_first() if idx == 0 else _gen_second()

            async def stop(self):
                pass

        adapter = ClaudeAgentAdapter(name="t")
        monkeypatch.setattr("ag_ui_claude_sdk.adapter.SessionWorker", _OrderedWorker)

        inp_a = make_input(thread_id="shared", run_id="A",
                           messages=[{"id": "1", "role": "user", "content": "hi"}])
        inp_b = make_input(thread_id="shared", run_id="B",
                           messages=[{"id": "2", "role": "user", "content": "yo"}])

        async def drive(inp, marker):
            async for e in adapter.run(inp):
                if e.type in (EventType.RUN_STARTED, EventType.RUN_FINISHED):
                    order.append((marker, e.type))

        t_a = asyncio.create_task(drive(inp_a, "A"))
        # Ensure A has acquired the run-lock and emitted RUN_STARTED first.
        await _wait_for(lambda: ("A", EventType.RUN_STARTED) in order)
        t_b = asyncio.create_task(drive(inp_b, "B"))

        # Give B ample scheduling opportunity; while A holds the run-lock, B must
        # NOT have emitted RUN_STARTED yet.
        for _ in range(50):
            await asyncio.sleep(0)
        assert ("B", EventType.RUN_STARTED) not in order, (
            "B's RUN_STARTED was emitted before A finished — runs are not serialized"
        )

        # Release A; it finishes, releasing the run-lock so B can proceed.
        a_gate.set()
        await asyncio.gather(t_a, t_b)

        # Both completed.
        assert ("A", EventType.RUN_FINISHED) in order
        assert ("B", EventType.RUN_FINISHED) in order
        # Ordering: A RUN_FINISHED strictly precedes B RUN_STARTED.
        idx_a_fin = order.index(("A", EventType.RUN_FINISHED))
        idx_b_start = order.index(("B", EventType.RUN_STARTED))
        assert idx_a_fin < idx_b_start, f"not serialized: {order}"

        await adapter.shutdown()

    @pytest.mark.asyncio
    async def test_different_threads_run_concurrently(self, make_input, monkeypatch):
        # (b) Two DIFFERENT-thread runs must still overlap (lock is per-thread).
        both_started = asyncio.Event()
        started = {"n": 0}
        release = asyncio.Event()

        class _ConcurrentWorker:
            def __init__(self, *a, **kw):
                pass

            async def start(self):
                pass

            def is_alive(self):
                return True

            def query(self, prompt, session_id="default"):
                async def _gen():
                    started["n"] += 1
                    if started["n"] >= 2:
                        both_started.set()
                    # Hold until both have started — proving genuine overlap. If
                    # the lock were global (not per-thread), the second run could
                    # never start and this would deadlock/time out.
                    await release.wait()
                    for ev in _make_text_stream():
                        yield ev

                return _gen()

            async def stop(self):
                pass

        adapter = ClaudeAgentAdapter(name="t")
        monkeypatch.setattr("ag_ui_claude_sdk.adapter.SessionWorker", _ConcurrentWorker)

        inp1 = make_input(thread_id="t1", run_id="r1",
                          messages=[{"id": "1", "role": "user", "content": "hi"}])
        inp2 = make_input(thread_id="t2", run_id="r2",
                          messages=[{"id": "2", "role": "user", "content": "yo"}])

        t1 = asyncio.create_task(_drive(adapter, inp1))
        t2 = asyncio.create_task(_drive(adapter, inp2))

        overlapped = await _wait_for(both_started.is_set)
        assert overlapped, "different-thread runs did not overlap — lock is not per-thread"

        release.set()
        e1, e2 = await asyncio.gather(t1, t2)
        assert EventType.RUN_FINISHED in _types(e1)
        assert EventType.RUN_FINISHED in _types(e2)

        await adapter.shutdown()

    @pytest.mark.asyncio
    async def test_state_update_tool_does_not_deadlock_with_run_lock(self, make_input, monkeypatch):
        # (c) A run whose stream includes a state-update tool call must NOT
        # deadlock: the run-lock (outer) and state-lock (inner, acquired mid-
        # stream at adapter.py state-management path) are DISTINCT locks. If the
        # run incorrectly reused _state_locks for admission, this would self-
        # deadlock the instant the state-update tool fires.
        class _StateToolWorker:
            def __init__(self, *a, **kw):
                pass

            async def start(self):
                pass

            def is_alive(self):
                return True

            def query(self, prompt, session_id="default"):
                async def _gen():
                    yield stream_event({"type": "message_start"})
                    yield stream_event({
                        "type": "content_block_start",
                        "content_block": {
                            "type": "tool_use",
                            "id": "tc1",
                            "name": STATE_MANAGEMENT_TOOL_FULL_NAME,
                        },
                    })
                    yield stream_event({
                        "type": "content_block_delta",
                        "delta": {
                            "type": "input_json_delta",
                            "partial_json": '{"state_updates": {"count": 7}}',
                        },
                    })
                    yield stream_event({"type": "content_block_stop"})
                    yield stream_event({"type": "message_stop"})

                return _gen()

            async def stop(self):
                pass

        adapter = ClaudeAgentAdapter(name="t")
        monkeypatch.setattr("ag_ui_claude_sdk.adapter.SessionWorker", _StateToolWorker)
        inp = make_input(thread_id="sd", run_id="r1", state={"count": 0},
                         messages=[{"id": "1", "role": "user", "content": "hi"}])

        # Must complete (no deadlock) within a generous bound.
        events = await asyncio.wait_for(_drive(adapter, inp), timeout=5.0)
        assert EventType.RUN_FINISHED in _types(events)
        # State-update tool path actually ran (mid-stream state-lock acquired).
        assert EventType.STATE_SNAPSHOT in _types(events)
        assert adapter._per_thread_state["sd"] == {"count": 7}

        await adapter.shutdown()

    @pytest.mark.asyncio
    async def test_run_lock_released_on_error_path(self, make_input, monkeypatch):
        # (d) A run that raises must still release the run-lock so a subsequent
        # same-thread run can proceed (not hang on a never-released lock).
        class _FailThenSucceedWorker:
            calls = 0

            def __init__(self, *a, **kw):
                pass

            async def start(self):
                pass

            def is_alive(self):
                return True

            def query(self, prompt, session_id="default"):
                idx = _FailThenSucceedWorker.calls
                _FailThenSucceedWorker.calls += 1

                async def _fail():
                    raise RuntimeError("boom")
                    yield  # pragma: no cover

                async def _ok():
                    for ev in _make_text_stream():
                        yield ev

                return _fail() if idx == 0 else _ok()

            async def stop(self):
                pass

        adapter = ClaudeAgentAdapter(name="t")
        monkeypatch.setattr("ag_ui_claude_sdk.adapter.SessionWorker", _FailThenSucceedWorker)

        inp1 = make_input(thread_id="errthread", run_id="r1",
                          messages=[{"id": "1", "role": "user", "content": "hi"}])
        events1 = await asyncio.wait_for(_drive(adapter, inp1), timeout=5.0)
        assert EventType.RUN_ERROR in _types(events1)

        # The run-lock must have been released — a second same-thread run runs.
        inp2 = make_input(thread_id="errthread", run_id="r2",
                          messages=[{"id": "2", "role": "user", "content": "yo"}])
        events2 = await asyncio.wait_for(_drive(adapter, inp2), timeout=5.0)
        assert EventType.RUN_FINISHED in _types(events2)

        await adapter.shutdown()


class TestQueryTimeoutDefault:
    def test_default_query_timeout_is_non_none(self):
        # Fix 2: constructed with no query_timeout_seconds → a non-None default
        # (300s) so a dead/slow worker cannot hang a run forever.
        adapter = ClaudeAgentAdapter(name="t")
        assert adapter._query_timeout_seconds is not None
        assert adapter._query_timeout_seconds == 300

    def test_query_timeout_override_still_honored(self):
        adapter = ClaudeAgentAdapter(name="t", query_timeout_seconds=12.0)
        assert adapter._query_timeout_seconds == 12.0
        # Explicit None still disables it.
        adapter2 = ClaudeAgentAdapter(name="t", query_timeout_seconds=None)
        assert adapter2._query_timeout_seconds is None

    @pytest.mark.asyncio
    async def test_unresponsive_worker_times_out_not_hang(self, make_input, monkeypatch):
        # A worker that never yields must surface RUN_ERROR (timeout), not hang.
        # Use a short override to keep the test fast.
        class _HangingWorker:
            def __init__(self, *a, **kw):
                pass

            async def start(self):
                pass

            def is_alive(self):
                return True

            def query(self, prompt, session_id="default"):
                async def _gen():
                    await asyncio.sleep(3600)  # never responds within the test
                    yield  # pragma: no cover

                return _gen()

            async def stop(self):
                pass

        adapter = ClaudeAgentAdapter(name="t", query_timeout_seconds=0.05)
        monkeypatch.setattr("ag_ui_claude_sdk.adapter.SessionWorker", _HangingWorker)
        inp = make_input(thread_id="slow", run_id="r1",
                         messages=[{"id": "1", "role": "user", "content": "hi"}])
        events = await asyncio.wait_for(_drive(adapter, inp), timeout=5.0)
        types = _types(events)
        assert EventType.RUN_ERROR in types
        assert EventType.RUN_FINISHED not in types

        await adapter.shutdown()


class TestPerRunResult:
    @pytest.mark.asyncio
    async def test_run_finished_result_reflects_own_result_message(self, make_input, monkeypatch):
        # Fix 4: RUN_FINISHED.result must reflect THIS run's own ResultMessage,
        # not a shared per-thread slot clobbered by another run.
        from claude_agent_sdk import ResultMessage

        class _ResultWorker:
            calls = 0

            def __init__(self, *a, **kw):
                pass

            async def start(self):
                pass

            def is_alive(self):
                return True

            def query(self, prompt, session_id="default"):
                idx = _ResultWorker.calls
                _ResultWorker.calls += 1

                async def _gen():
                    yield stream_event({"type": "message_start"})
                    yield stream_event({
                        "type": "content_block_delta",
                        "delta": {"type": "text_delta", "text": "hi"},
                    })
                    yield stream_event({"type": "message_stop"})
                    yield ResultMessage(
                        subtype="success",
                        duration_ms=idx,  # distinct per run
                        duration_api_ms=1,
                        is_error=False,
                        num_turns=idx + 1,
                        session_id="sess",
                        total_cost_usd=0.0,
                        usage={},
                        result="hi",
                    )

                return _gen()

            async def stop(self):
                pass

        adapter = ClaudeAgentAdapter(name="t")
        monkeypatch.setattr("ag_ui_claude_sdk.adapter.SessionWorker", _ResultWorker)

        inp1 = make_input(thread_id="shared", run_id="r1",
                          messages=[{"id": "1", "role": "user", "content": "hi"}])
        events1 = await _drive(adapter, inp1)
        fin1 = next(e for e in events1 if e.type == EventType.RUN_FINISHED)
        assert fin1.result is not None
        assert fin1.result["duration_ms"] == 0
        assert fin1.result["num_turns"] == 1

        inp2 = make_input(thread_id="shared", run_id="r2",
                          messages=[{"id": "2", "role": "user", "content": "yo"}])
        events2 = await _drive(adapter, inp2)
        fin2 = next(e for e in events2 if e.type == EventType.RUN_FINISHED)
        assert fin2.result is not None
        # Run 2 gets its OWN result, not run 1's.
        assert fin2.result["duration_ms"] == 1
        assert fin2.result["num_turns"] == 2

        await adapter.shutdown()

    @pytest.mark.asyncio
    async def test_two_serialized_runs_each_get_own_result(self, make_input, monkeypatch):
        # Two serialized same-thread runs each carry their own ResultMessage even
        # when launched overlapping (serialize keeps them ordered; result must
        # not bleed across).
        from claude_agent_sdk import ResultMessage

        class _SeqResultWorker:
            calls = 0

            def __init__(self, *a, **kw):
                pass

            async def start(self):
                pass

            def is_alive(self):
                return True

            def query(self, prompt, session_id="default"):
                idx = _SeqResultWorker.calls
                _SeqResultWorker.calls += 1

                async def _gen():
                    yield stream_event({"type": "message_start"})
                    yield stream_event({
                        "type": "content_block_delta",
                        "delta": {"type": "text_delta", "text": "x"},
                    })
                    yield stream_event({"type": "message_stop"})
                    yield ResultMessage(
                        subtype="success",
                        duration_ms=100 + idx,
                        duration_api_ms=1,
                        is_error=False,
                        num_turns=1,
                        session_id="sess",
                        total_cost_usd=0.0,
                        usage={},
                        result="x",
                    )

                return _gen()

            async def stop(self):
                pass

        adapter = ClaudeAgentAdapter(name="t")
        monkeypatch.setattr("ag_ui_claude_sdk.adapter.SessionWorker", _SeqResultWorker)

        inp_a = make_input(thread_id="shared", run_id="A",
                           messages=[{"id": "1", "role": "user", "content": "hi"}])
        inp_b = make_input(thread_id="shared", run_id="B",
                           messages=[{"id": "2", "role": "user", "content": "yo"}])

        t_a = asyncio.create_task(_drive(adapter, inp_a))
        t_b = asyncio.create_task(_drive(adapter, inp_b))
        events_a, events_b = await asyncio.gather(t_a, t_b)

        fin_a = next(e for e in events_a if e.type == EventType.RUN_FINISHED)
        fin_b = next(e for e in events_b if e.type == EventType.RUN_FINISHED)
        # Each run has a distinct, own result (the two calls produced 100 / 101).
        assert {fin_a.result["duration_ms"], fin_b.result["duration_ms"]} == {100, 101}

        await adapter.shutdown()


class TestSequentialStateReset:
    @pytest.mark.asyncio
    async def test_run2_fresh_state_replaces_run1(self, make_input, monkeypatch):
        # Regression guard: run 1 then run 2 (sequential) on the same thread,
        # where run 2 sends fresh input_data.state. Run 2's state must REPLACE
        # run 1's (documented reset). Serialize must not turn the per-run re-seed
        # into "inherit/ignore".
        class _NoopWorker:
            def __init__(self, *a, **kw):
                pass

            async def start(self):
                pass

            def is_alive(self):
                return True

            def query(self, prompt, session_id="default"):
                async def _gen():
                    for ev in _make_text_stream():
                        yield ev

                return _gen()

            async def stop(self):
                pass

        adapter = ClaudeAgentAdapter(name="t")
        monkeypatch.setattr("ag_ui_claude_sdk.adapter.SessionWorker", _NoopWorker)

        inp1 = make_input(thread_id="shared", run_id="r1", state={"count": 1},
                          messages=[{"id": "1", "role": "user", "content": "hi"}])
        await _drive(adapter, inp1)
        assert adapter._per_thread_state["shared"] == {"count": 1}

        inp2 = make_input(thread_id="shared", run_id="r2", state={"other": 99},
                          messages=[{"id": "2", "role": "user", "content": "yo"}])
        await _drive(adapter, inp2)
        # Fresh state from run 2 REPLACED run 1's (reset semantics preserved).
        assert adapter._per_thread_state["shared"] == {"other": 99}

        await adapter.shutdown()


class TestWorkerDeathFanout:
    @pytest.mark.asyncio
    async def test_waiting_consumer_gets_terminal_signal_on_worker_death(self):
        # Fix 3: SessionWorker must fan out WorkerError + None to ALL in-flight
        # output queues on fatal worker death, so a queued/peer consumer does not
        # hang. Drive the REAL SessionWorker with a scripted ClaudeSDKClient that
        # dies in connect() AFTER queries have been enqueued — the fatal-error
        # branch must terminate every registered consumer.
        import claude_agent_sdk
        from ag_ui_claude_sdk.session import SessionWorker

        connect_gate = asyncio.Event()

        class _DyingClient:
            def __init__(self, options=None, **kwargs):
                self.options = options

            async def connect(self):
                # Wait until consumers have enqueued their queries, THEN die.
                await connect_gate.wait()
                raise RuntimeError("client connect boom")

            async def query(self, prompt, session_id="default"):  # pragma: no cover
                pass

            async def receive_response(self):  # pragma: no cover
                if False:
                    yield None

            async def disconnect(self):
                pass

            async def interrupt(self):
                pass

        orig = claude_agent_sdk.ClaudeSDKClient
        claude_agent_sdk.ClaudeSDKClient = _DyingClient
        try:
            worker = SessionWorker("th", options=None)
            await worker.start()

            # Enqueue TWO queries while the worker is still blocked in connect().
            # Both register their output queues; on worker death BOTH must get a
            # terminal signal (without the fan-out, the second hangs forever).
            async def consume():
                got_error = False
                try:
                    async for _ in worker.query("p", session_id="th"):
                        pass
                except Exception:
                    got_error = True
                return got_error

            c1 = asyncio.create_task(consume())
            c2 = asyncio.create_task(consume())

            # Let both queries land on the input queue before the worker dies.
            await _wait_for(lambda: worker._input_queue.qsize() >= 2)
            connect_gate.set()

            # Both consumers must terminate (error or clean end) — neither hangs.
            results = await asyncio.wait_for(asyncio.gather(c1, c2), timeout=5.0)
            assert all(r is True for r in results), (
                "a waiting consumer did not receive a terminal error on worker death"
            )
        finally:
            claude_agent_sdk.ClaudeSDKClient = orig
            await worker.stop()
