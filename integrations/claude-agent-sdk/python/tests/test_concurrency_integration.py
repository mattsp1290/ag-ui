"""Thread-level concurrency integration tests for the Claude Agent SDK adapter.

Unlike ``test_adapter.py`` — whose ``TestWorkerLifecycle`` /
``TestPoisonedWorkerCache`` suites monkeypatch the whole ``SessionWorker`` class
with ``_Fake*Worker`` stand-ins — these tests drive the **real** adapter +
the **real** :class:`ag_ui_claude_sdk.session.SessionWorker`. Only the leaf
``ClaudeSDKClient`` (the thing that would actually spawn the Claude CLI and hit
the Anthropic API) is substituted.

Why this matters: the white-box fakes replace ``SessionWorker.query`` directly,
so they never exercise the worker's background task, its input/output queue
plumbing, ``client.connect()`` / ``client.query()`` / ``client.receive_response()``,
or its ``start()`` / ``stop()`` lifecycle. The per-thread ``active_runs`` refcount
hardening (PR #1878, "item 7") is therefore proven today only against fakes.
These tests close that gap: two genuinely-concurrent ``run()`` invocations share
one real worker through the full adapter stack, with the LLM substituted at the
SDK-client boundary (the same boundary the dojo e2e mocks via aimock +
``ANTHROPIC_BASE_URL``, just pushed down into the process instead of over HTTP).

LLM substitution mechanism
---------------------------
``SessionWorker._run`` does ``from claude_agent_sdk import ClaudeSDKClient`` at
call time, so monkeypatching ``claude_agent_sdk.ClaudeSDKClient`` swaps the real
client for a scripted one while leaving the worker (and the adapter) entirely
real. The fake client implements the exact surface the worker uses:
``connect()``, ``query()``, ``receive_response()``, ``disconnect()``,
``interrupt()`` — and streams back real ``claude_agent_sdk`` message objects
(``StreamEvent`` / ``ResultMessage``), so the adapter's translation layer runs
for real too.
"""

import asyncio

import pytest

from ag_ui.core import EventType
from ag_ui_claude_sdk.adapter import ClaudeAgentAdapter
from ag_ui_claude_sdk import session as session_module
from ag_ui_claude_sdk.session import SessionWorker

from .conftest import stream_event


def _types(events):
    return [e.type for e in events]


# ---------------------------------------------------------------------------
# Scripted ClaudeSDKClient — the ONLY substituted component. Everything above
# it (SessionWorker queues/lifecycle, adapter run()) is real.
# ---------------------------------------------------------------------------


class _ScriptedClient:
    """Stand-in for ``claude_agent_sdk.ClaudeSDKClient``.

    Streams a minimal but real Claude SDK message sequence (a couple of
    streaming text deltas wrapped in ``StreamEvent`` + a terminal
    ``ResultMessage``). A per-instance ``release`` event lets a test hold the
    stream open to force genuine overlap between two concurrent runs sharing
    one worker.

    Each instance records that it was constructed/connected so a test can prove
    the **real** ``SessionWorker._run`` path executed (a fake worker never
    constructs a ClaudeSDKClient at all).
    """

    def __init__(self, *, instances, options=None, fail=False, release=None):
        self.options = options
        self._fail = fail
        self._release = release
        self.connected = False
        self.disconnected = False
        self.query_calls = []
        instances.append(self)

    async def connect(self):
        self.connected = True

    async def query(self, prompt, session_id="default"):
        self.query_calls.append((prompt, session_id))

    async def receive_response(self):
        from claude_agent_sdk import ResultMessage

        # Optionally block so a peer run can be proven mid-stream on the SAME
        # shared worker before this one completes.
        if self._release is not None:
            await self._release.wait()

        if self._fail:
            raise RuntimeError("scripted client boom")

        msg_id_event = stream_event({"type": "message_start"})
        text_start = stream_event(
            {
                "type": "content_block_delta",
                "delta": {"type": "text_delta", "text": "hello "},
            }
        )
        text_more = stream_event(
            {
                "type": "content_block_delta",
                "delta": {"type": "text_delta", "text": "world"},
            }
        )
        msg_stop = stream_event({"type": "message_stop"})
        for ev in (msg_id_event, text_start, text_more, msg_stop):
            yield ev

        yield ResultMessage(
            subtype="success",
            duration_ms=1,
            duration_api_ms=1,
            is_error=False,
            num_turns=1,
            session_id="sess",
            total_cost_usd=0.0,
            usage={},
            result="hello world",
        )

    async def disconnect(self):
        self.disconnected = True

    async def interrupt(self):
        pass


def _install_scripted_client(monkeypatch, instances, *, fail_when=None, release_when=None):
    """Patch ``claude_agent_sdk.ClaudeSDKClient`` with a factory that produces
    ``_ScriptedClient`` instances.

    ``fail_when`` / ``release_when`` are callables ``(index) -> bool`` keyed on
    construction order, letting a test designate which worker's client fails or
    blocks. (One worker per thread_id, so for a single shared thread the index
    maps to run order.)
    """
    import claude_agent_sdk

    counter = {"n": 0}
    releases = []

    def factory(options=None, **kwargs):
        idx = counter["n"]
        counter["n"] += 1
        release = None
        if release_when is not None and release_when(idx):
            release = asyncio.Event()
            releases.append(release)
        return _ScriptedClient(
            instances=instances,
            options=options,
            fail=bool(fail_when and fail_when(idx)),
            release=release,
        )

    monkeypatch.setattr(claude_agent_sdk, "ClaudeSDKClient", factory)
    return releases


async def _drive(adapter, inp):
    return [e async for e in adapter.run(inp)]


async def _wait_for(predicate, *, tries=400):
    """Cooperatively yield until ``predicate()`` is truthy (or give up)."""
    for _ in range(tries):
        if predicate():
            return True
        await asyncio.sleep(0)
    return False


class TestRealWorkerConcurrency:
    """Drives the REAL SessionWorker + adapter; only ClaudeSDKClient is faked."""

    @pytest.mark.asyncio
    async def test_scenario_a_two_overlapping_runs_share_one_real_worker(
        self, make_input, monkeypatch
    ):
        # (a) Two overlapping run() invocations on the SAME thread_id stream
        # concurrently; both complete, the shared REAL worker is reused (not
        # duplicated, not torn down): active_runs reaches 2 then drains to 0
        # and the worker survives throughout.
        instances = []
        # The worker is created lazily on the FIRST run; the 2nd run reuses it.
        # Only one ClaudeSDKClient is constructed (index 0). Hold its stream
        # open so BOTH runs are provably in-flight on the one shared worker.
        # NOTE: a single worker serves queries serially via its queue, so we
        # release as soon as both runs have incremented the refcount.
        releases = _install_scripted_client(
            monkeypatch, instances, release_when=lambda i: i == 0
        )

        adapter = ClaudeAgentAdapter(name="t")
        inp = make_input(
            thread_id="shared", messages=[{"id": "1", "role": "user", "content": "hi"}]
        )

        t1 = asyncio.create_task(_drive(adapter, inp))
        t2 = asyncio.create_task(_drive(adapter, inp))

        # Both runs in-flight => refcount 2 on the single shared worker.
        reached_two = await _wait_for(
            lambda: (adapter._workers.get("shared") or {}).get("active_runs", 0) >= 2
        )
        assert reached_two, "two concurrent runs never both became in-flight"

        entry = adapter._workers["shared"]
        assert entry["active_runs"] == 2
        assert entry["active"] is True
        # PROOF the REAL worker ran: it's an actual SessionWorker with a live
        # background task. (A fake worker would never be a SessionWorker.)
        assert isinstance(entry["worker"], SessionWorker)
        assert entry["worker"].is_alive() is True

        # The worker's background task constructs + connects exactly ONE real
        # ClaudeSDKClient (lazily, when _run is scheduled). Wait for it: a fake
        # worker would construct none. This proves the real connect()/query()/
        # receive_response() lifecycle executed, not a bypassed stub.
        constructed = await _wait_for(lambda: len(instances) == 1)
        assert constructed, "real SessionWorker never constructed its ClaudeSDKClient"
        assert instances[0].connected is True

        # Release the held stream so both runs drain. Wait for the release Event
        # to be created on the (lazily-constructed) client first.
        await _wait_for(lambda: len(releases) >= 1)
        for r in releases:
            r.set()
        events1, events2 = await asyncio.gather(t1, t2)

        assert EventType.RUN_FINISHED in _types(events1)
        assert EventType.RUN_FINISHED in _types(events2)
        # Real translation layer ran: streamed text surfaced as AG-UI events.
        assert EventType.TEXT_MESSAGE_CONTENT in _types(events1)
        assert EventType.TEXT_MESSAGE_CONTENT in _types(events2)

        # (c) After all runs finish: refcount 0, worker idle/evictable, no leak,
        # and still the SAME single worker (never duplicated).
        entry = adapter._workers["shared"]
        assert entry["active_runs"] == 0
        assert entry["active"] is False
        assert len(instances) == 1, "worker was duplicated instead of reused"

        await adapter.shutdown()

    @pytest.mark.asyncio
    async def test_scenario_b_erroring_run_does_not_evict_live_peer(
        self, make_input, monkeypatch
    ):
        # (b) Two concurrent same-thread runs; ONE raises mid-stream. The
        # surviving peer completes normally and its (shared, real) worker is NOT
        # evicted by the erroring run (item-7 error-path invariant); the erroring
        # run surfaces RUN_ERROR.
        #
        # Both runs share ONE worker (same thread_id). That worker's single
        # ClaudeSDKClient is constructed once (index 0). The worker serves the
        # two queued queries serially: we make the FIRST served query block then
        # raise (the failer A), while the SECOND completes (the survivor B). We
        # gate so the failer raises only once both runs are in-flight.
        instances = []
        gate = asyncio.Event()       # released to let the failer (A) raise
        b_streaming = asyncio.Event()  # set once the survivor (B) is streaming
        b_release = asyncio.Event()    # released to let B finish after the assert

        import claude_agent_sdk

        # A single client instance serves both queries off the worker's queue.
        # Track query invocations so the first served query fails and the second
        # succeeds, all on the one real shared worker.
        class _SharedClient:
            def __init__(self, options=None, **kwargs):
                self.options = options
                self.connected = False
                self.disconnected = False
                self._served = 0
                instances.append(self)

            async def connect(self):
                self.connected = True

            async def query(self, prompt, session_id="default"):
                pass

            async def receive_response(self):
                from claude_agent_sdk import ResultMessage

                served = self._served
                self._served += 1
                if served == 0:
                    # Failer A: wait until both runs are in-flight, then raise
                    # mid-stream while the peer (B) is still queued on this
                    # shared worker.
                    await gate.wait()
                    raise RuntimeError("scripted client boom")
                    yield  # pragma: no cover
                # Survivor B: begin streaming, then HOLD the stream open so B is
                # provably still in-flight on the shared worker when the test
                # inspects the post-error invariant. (The worker serves queries
                # serially, so B only starts after A's failed query is drained.)
                yield stream_event({"type": "message_start"})
                yield stream_event(
                    {
                        "type": "content_block_delta",
                        "delta": {"type": "text_delta", "text": "ok"},
                    }
                )
                b_streaming.set()
                await b_release.wait()
                yield stream_event({"type": "message_stop"})
                yield ResultMessage(
                    subtype="success",
                    duration_ms=1,
                    duration_api_ms=1,
                    is_error=False,
                    num_turns=1,
                    session_id="sess",
                    total_cost_usd=0.0,
                    usage={},
                    result="ok",
                )

            async def disconnect(self):
                self.disconnected = True

            async def interrupt(self):
                pass

        monkeypatch.setattr(claude_agent_sdk, "ClaudeSDKClient", _SharedClient)

        adapter = ClaudeAgentAdapter(name="t")
        inp = make_input(
            thread_id="shared", messages=[{"id": "1", "role": "user", "content": "hi"}]
        )

        # Start A (failer, first to enqueue) then B (survivor).
        t_a = asyncio.create_task(_drive(adapter, inp))
        await _wait_for(
            lambda: (adapter._workers.get("shared") or {}).get("active_runs", 0) >= 1
        )
        t_b = asyncio.create_task(_drive(adapter, inp))
        reached_two = await _wait_for(
            lambda: (adapter._workers.get("shared") or {}).get("active_runs", 0) >= 2
        )
        assert reached_two, "second concurrent run never became in-flight"

        entry = adapter._workers["shared"]
        assert entry["active_runs"] == 2
        # PROOF: one real shared SessionWorker, one real client constructed.
        assert isinstance(entry["worker"], SessionWorker)
        assert len(instances) == 1

        # Let A raise. The worker drains A's failed query then dequeues B, which
        # streams a chunk and parks on b_release — so B is provably mid-stream.
        gate.set()
        events_a = await t_a
        assert EventType.RUN_ERROR in _types(events_a)

        # Wait until B is provably streaming on the shared worker.
        b_live = await _wait_for(b_streaming.is_set)
        assert b_live, "survivor peer never began streaming on the shared worker"

        # INVARIANT: the shared real worker survives — B is still on it. A's
        # error path must NOT have evicted/stopped it, and A's single decrement
        # leaves the refcount at exactly 1 (B still in-flight).
        entry = adapter._workers.get("shared")
        assert entry is not None, "shared worker evicted while a peer run was live"
        assert isinstance(entry["worker"], SessionWorker)
        assert entry["worker"].is_alive() is True
        assert entry["active_runs"] == 1
        assert entry["active"] is True

        # Now let B finish normally on the surviving worker.
        b_release.set()
        events_b = await t_b
        assert EventType.RUN_FINISHED in _types(events_b)
        assert EventType.RUN_ERROR not in _types(events_b)

        # (c) After both finished: refcount 0, idle, evictable, no leak.
        entry = adapter._workers["shared"]
        assert entry["active_runs"] == 0
        assert entry["active"] is False
        assert len(instances) == 1, "shared worker was duplicated"

        await adapter.shutdown()

    @pytest.mark.asyncio
    async def test_scenario_c_worker_cleanly_evictable_after_runs(
        self, make_input, monkeypatch
    ):
        # (c) explicit: after concurrent runs finish, the shared real worker is
        # refcount 0 and is actually torn down (stop() disconnects the client)
        # by clear_session — no leak, no lingering background task.
        instances = []
        releases = _install_scripted_client(
            monkeypatch, instances, release_when=lambda i: i == 0
        )

        adapter = ClaudeAgentAdapter(name="t")
        inp = make_input(
            thread_id="shared", messages=[{"id": "1", "role": "user", "content": "hi"}]
        )

        t1 = asyncio.create_task(_drive(adapter, inp))
        t2 = asyncio.create_task(_drive(adapter, inp))
        await _wait_for(
            lambda: (adapter._workers.get("shared") or {}).get("active_runs", 0) >= 2
        )
        # The worker constructs its client lazily on the background task, so wait
        # for the held release Event to exist before setting it (otherwise the
        # client would park on a stream nothing releases).
        await _wait_for(lambda: len(releases) >= 1)
        for r in releases:
            r.set()
        await asyncio.gather(t1, t2)

        entry = adapter._workers["shared"]
        worker = entry["worker"]
        assert entry["active_runs"] == 0
        assert isinstance(worker, SessionWorker)
        assert worker.is_alive() is True  # idle but still alive until evicted

        # Cleanly evict: the real worker's background task stops and the real
        # client is disconnected — proving full lifecycle teardown, not a fake.
        await adapter.clear_session("shared")
        assert "shared" not in adapter._workers
        assert worker.is_alive() is False
        assert instances[0].disconnected is True
