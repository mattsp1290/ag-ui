import asyncio
from types import SimpleNamespace

from ag_ui_claude_managed_agents import SessionBinding, InMemorySessionStore
from .fake_client import FakeClient
from .test_agent import base_input, collect, new_agent


def call(id, kind="agent.tool_use", **kwargs):
    return {
        "id": id,
        "type": kind,
        "name": "lookup",
        "input": {"q": "test"},
        "evaluated_permission": "ask",
        **kwargs,
    }


def idle(ids):
    return {
        "id": "idle",
        "type": "session.status_idle",
        "stop_reason": {"type": "requires_action", "event_ids": ids},
    }


class Page:
    def __init__(self, pages):
        self.pages = pages
        self.data = pages[0]

    def has_next_page(self):
        return len(self.pages) > 1

    async def get_next_page(self):
        return Page(self.pages[1:])


class RecoveryClient(FakeClient):
    def __init__(self, pages, **kwargs):
        super().__init__(**kwargs)
        self.pages = pages
        self.beta.sessions.events.list = self.list_history
        self.beta.sessions.threads = SimpleNamespace(
            events=SimpleNamespace(stream=self.child_stream, list=self.list_history)
        )

    async def child_stream(self, thread_id, *, session_id):
        return await self._stream(thread_id, parent=session_id)

    async def list_history(self, *args, **kwargs):
        assert self.streams_opened, "history must follow stream open"
        assert not self.sent, "history must precede responses"
        return Page(self.pages)


async def test_restart_multiple_native_and_custom_requests_and_partial_resume():
    history = [
        [
            call("a"),
            call("b", "agent.mcp_tool_use"),
            call("c", "agent.custom_tool_use"),
        ],
        [idle(["a", "b", "c"])],
    ]
    store = InMemorySessionStore()
    fake = RecoveryClient(history)
    first = new_agent(
        fake,
        store,
        resolve_session=lambda _: SessionBinding("native", last_user_message_id="u1"),
    )
    events = await collect(first, base_input())
    assert [i.id for i in events[-1].outcome.interrupts] == ["a", "b"]
    assert not fake.sent and not fake.create_calls and not fake.update_calls
    second_fake = RecoveryClient(history)
    second = new_agent(second_fake, store)
    events = await collect(
        second,
        base_input(
            resume=[{"interruptId": "a", "status": "resolved", "payload": "allow"}],
            messages=[
                {"id": "r", "role": "tool", "toolCallId": "c", "content": "done"}
            ],
        ),
    )
    assert [i.id for i in events[-1].outcome.interrupts] == ["b"]
    assert second_fake.sent[0]["events"] == [
        {"type": "user.tool_confirmation", "tool_use_id": "a", "result": "allow"},
        {
            "type": "user.custom_tool_result",
            "custom_tool_use_id": "c",
            "content": [{"type": "text", "text": "done"}],
            "is_error": False,
        },
    ]


async def test_invalid_resume_is_atomic():
    fake = RecoveryClient([[call("a"), call("b"), idle(["a", "b"])]])
    agent = new_agent(fake, resolve_session=lambda _: SessionBinding("native"))
    events = await collect(
        agent,
        base_input(
            resume=[
                {"interruptId": "a", "status": "resolved", "payload": "allow"},
                {"interruptId": "b", "status": "resolved", "payload": "yes"},
            ]
        ),
    )
    assert events[-1].code == "invalid_resume"
    assert not fake.sent


async def test_resolved_history_subtracted_and_cancel_maps_to_deny():
    fake = RecoveryClient(
        [
            [call("a"), call("b"), idle(["a", "b"])],
            [
                {
                    "id": "done",
                    "type": "user.tool_confirmation",
                    "tool_use_id": "a",
                    "result": "allow",
                }
            ],
        ],
        streams=[
            [
                {
                    "id": "end",
                    "type": "session.status_idle",
                    "stop_reason": {"type": "end_turn"},
                }
            ]
        ],
    )
    agent = new_agent(fake, resolve_session=lambda _: SessionBinding("native"))
    events = await collect(
        agent, base_input(resume=[{"interruptId": "b", "status": "cancelled"}])
    )
    assert events[-1].type.value == "RUN_FINISHED"
    assert fake.sent[0]["events"] == [
        {"type": "user.tool_confirmation", "tool_use_id": "b", "result": "deny"}
    ]


async def test_adoption_does_not_replay_history_or_execute_backend():
    fake = RecoveryClient([[call("c", "agent.custom_tool_use")]])
    agent = new_agent(fake, resolve_session=lambda _: SessionBinding("native"))
    events = await collect(agent, base_input())
    assert events[-1].type.value == "RUN_FINISHED"
    assert not fake.sent


async def test_child_followup_rejected_and_terminated_binding_retained():
    fake = RecoveryClient(
        [
            [
                {
                    "id": "idle",
                    "type": "session.thread_status_idle",
                    "stop_reason": {"type": "end_turn"},
                }
            ]
        ]
    )
    agent = new_agent(
        fake,
        resolve_session=lambda _: SessionBinding(
            "native", session_thread_id="child", last_user_message_id="u0"
        ),
    )
    events = await collect(
        agent,
        base_input(
            messages=[
                {"id": "u0", "role": "user", "content": "old"},
                {"id": "u1", "role": "user", "content": "new"},
            ]
        ),
    )
    assert events[-1].code == "unsupported_child_followup"
    assert not fake.sent
    fake = RecoveryClient([[{"id": "ended", "type": "session.status_terminated"}]])
    store = InMemorySessionStore()
    agent = new_agent(fake, store, resolve_session=lambda _: SessionBinding("native"))
    assert (await collect(agent, base_input()))[-1].code == "session_ended"
    assert (await collect(new_agent(fake, store), base_input()))[
        -1
    ].code == "session_ended"
    assert not fake.create_calls


async def test_resumed_disconnect_preserves_session():
    gate = asyncio.Event()
    fake = RecoveryClient(
        [[{"id": "running", "type": "session.status_running"}]], streams=[[gate]]
    )
    agent = new_agent(fake, resolve_session=lambda _: SessionBinding("native"))
    stream = agent.run(base_input())
    while (await anext(stream)).type.value != "CUSTOM":
        pass
    await stream.aclose()
    await asyncio.gather(*agent._tasks, return_exceptions=True)
    assert not fake.sent


async def test_disk_roundtrip_replays_missed_output_without_tool_execution(tmp_path):
    import json
    from dataclasses import asdict
    from ag_ui_claude_managed_agents import SessionRecord, BackendTool

    class DiskStore:
        def get(self, key):
            path = tmp_path / "binding.json"
            return (
                SessionRecord(**json.loads(path.read_text())) if path.exists() else None
            )

        def set(self, key, record):
            (tmp_path / "binding.json").write_text(json.dumps(asdict(record)))

        def delete(self, key):
            (tmp_path / "binding.json").unlink(missing_ok=True)

    history = [
        {
            "id": "before",
            "type": "agent.message",
            "content": [{"type": "text", "text": "already imported"}],
        },
        {
            "id": "missed",
            "type": "agent.message",
            "content": [{"type": "text", "text": "offline output"}],
        },
        call("pending", "agent.custom_tool_use"),
        idle(["pending"]),
    ]
    invoked = []
    fake = RecoveryClient([history])
    first = new_agent(
        fake,
        DiskStore(),
        resolve_session=lambda _: SessionBinding("native", last_event_id="before"),
        backend_tools=[
            BackendTool("lookup", "lookup", {}, lambda value: invoked.append(value))
        ],
    )
    events = await collect(first, base_input())
    assert [e.delta for e in events if e.type.value == "TEXT_MESSAGE_CONTENT"] == [
        "offline output"
    ]
    assert len([e for e in events if e.type.value == "RAW"]) == 3
    assert not invoked and not fake.sent
    assert [e.tool_call_id for e in events if e.type.value == "TOOL_CALL_START"] == [
        "pending"
    ]
    # A new store and adapter instance read only the serialized record.
    second_fake = RecoveryClient([history])
    second = new_agent(second_fake, DiskStore())
    events = await collect(second, base_input())
    assert not [e for e in events if e.type.value in ("TEXT_MESSAGE_CONTENT", "RAW")]
    assert not second_fake.sent and not second_fake.create_calls


async def test_end_turn_invalidates_stale_calls_and_unknown_blocker_errors():
    fake = RecoveryClient(
        [
            [
                call("stale"),
                {
                    "id": "end",
                    "type": "session.status_idle",
                    "stop_reason": {"type": "end_turn"},
                },
            ]
        ]
    )
    events = await collect(
        new_agent(fake, resolve_session=lambda _: SessionBinding("native")),
        base_input(),
    )
    assert events[-1].outcome is None
    assert not fake.sent
    fake = RecoveryClient([[idle(["missing"])]])
    events = await collect(
        new_agent(fake, resolve_session=lambda _: SessionBinding("native")),
        base_input(),
    )
    assert events[-1].code == "pending_request_unavailable"
    assert not fake.sent


async def test_live_overlap_deduplicated_and_cursor_persisted():
    old = {
        "id": "old",
        "type": "agent.message",
        "content": [{"type": "text", "text": "old"}],
    }
    new = {
        "id": "new",
        "type": "agent.message",
        "content": [{"type": "text", "text": "new"}],
    }
    fake = RecoveryClient(
        [[old, {"id": "running", "type": "session.status_running"}]],
        streams=[
            [
                old,
                new,
                {
                    "id": "done",
                    "type": "session.status_idle",
                    "stop_reason": {"type": "end_turn"},
                },
            ]
        ],
    )
    store = InMemorySessionStore()
    agent = new_agent(fake, store, resolve_session=lambda _: SessionBinding("native"))
    events = await collect(agent, base_input())
    assert [e.delta for e in events if e.type.value == "TEXT_MESSAGE_CONTENT"] == [
        "new"
    ]
    assert store.get(agent._session_key("thread_1")).last_event_id == "done"


async def test_resume_without_binding_does_not_create_session():
    fake = FakeClient()
    events = await collect(
        new_agent(fake),
        base_input(
            resume=[
                {"interruptId": "unknown", "status": "resolved", "payload": "allow"}
            ]
        ),
    )
    assert events[-1].code == "invalid_resume"
    assert not fake.create_calls and not fake.sent


async def test_live_child_termination_retains_ended_binding():
    fake = RecoveryClient(
        [[{"id": "running", "type": "session.thread_status_running"}]],
        streams=[[{"id": "terminated", "type": "session.thread_status_terminated"}]],
    )
    store = InMemorySessionStore()
    agent = new_agent(
        fake,
        store,
        resolve_session=lambda _: SessionBinding("native", session_thread_id="child"),
    )
    events = await collect(agent, base_input())
    assert events[-1].code == "session_ended"
    assert store.get(agent._session_key("thread_1")).ended
    assert not fake.sent


async def test_owned_session_without_cursor_replays_history():
    from ag_ui_claude_managed_agents import SessionRecord

    fake = RecoveryClient(
        [
            [
                {
                    "id": "offline",
                    "type": "agent.message",
                    "content": [{"type": "text", "text": "finished offline"}],
                },
                {
                    "id": "end",
                    "type": "session.status_idle",
                    "stop_reason": {"type": "end_turn"},
                },
            ]
        ]
    )
    store = InMemorySessionStore()
    agent = new_agent(fake, store)
    store.set(
        agent._session_key("thread_1"),
        SessionRecord("native", last_user_message_id="u1"),
    )
    events = await collect(agent, base_input())
    assert [e.delta for e in events if e.type.value == "TEXT_MESSAGE_CONTENT"] == [
        "finished offline"
    ]
    assert not fake.sent


async def test_recovered_live_unknown_tool_is_parked_without_rejection():
    fake = RecoveryClient(
        [[{"id": "running", "type": "session.status_running"}]],
        streams=[[call("new-call", "agent.custom_tool_use")]],
    )
    store = InMemorySessionStore()
    agent = new_agent(fake, store, resolve_session=lambda _: SessionBinding("native"))
    events = await collect(agent, base_input())
    assert events[-1].type.value == "RUN_FINISHED"
    assert store.get(agent._session_key("thread_1")).pending_client_tool_use_ids == [
        "new-call"
    ]
    assert not fake.sent


async def test_recovered_live_approval_ignores_automatic_policy():
    fake = RecoveryClient(
        [[{"id": "running", "type": "session.status_running"}]], streams=[[call("ask")]]
    )
    agent = new_agent(
        fake,
        resolve_session=lambda _: SessionBinding("native"),
        tool_confirmation="allow",
    )
    events = await collect(agent, base_input())
    interrupt = events[-1].outcome.interrupts[0]
    assert interrupt.id == "ask"
    assert interrupt.response_schema == {"type": "string", "enum": ["allow", "deny"]}
    assert interrupt.metadata["sessionId"] == "native"
    assert interrupt.metadata["toolName"] == "lookup"
    assert not fake.sent


async def test_recovered_unknown_live_action_does_not_interrupt():
    fake = RecoveryClient(
        [[{"id": "running", "type": "session.status_running"}]],
        streams=[[idle(["unknown"])]],
    )
    events = await collect(
        new_agent(fake, resolve_session=lambda _: SessionBinding("native")),
        base_input(),
    )
    assert events[-1].code == "unsupported_action"
    assert not fake.sent


async def test_completed_offline_custom_lifecycle_is_replayed_without_execution():
    history = [
        {"id": "cursor", "type": "session.status_running"},
        call("custom", "agent.custom_tool_use"),
        {
            "id": "answer",
            "type": "user.custom_tool_result",
            "custom_tool_use_id": "custom",
            "content": [{"type": "text", "text": "offline result"}],
        },
        {
            "id": "end",
            "type": "session.status_idle",
            "stop_reason": {"type": "end_turn"},
        },
    ]
    fake = RecoveryClient([history])
    agent = new_agent(
        fake, resolve_session=lambda _: SessionBinding("native", last_event_id="cursor")
    )
    events = await collect(agent, base_input())
    assert [e.type.value for e in events if e.type.value.startswith("TOOL_CALL")] == [
        "TOOL_CALL_START",
        "TOOL_CALL_ARGS",
        "TOOL_CALL_END",
        "TOOL_CALL_RESULT",
    ]
    result = next(e for e in events if e.type.value == "TOOL_CALL_RESULT")
    assert result.tool_call_id == "custom" and "offline result" in result.content
    assert not fake.sent


async def test_missing_cursor_is_explicit_and_not_advanced():
    fake = RecoveryClient(
        [
            [
                {
                    "id": "end",
                    "type": "session.status_idle",
                    "stop_reason": {"type": "end_turn"},
                }
            ]
        ]
    )
    store = InMemorySessionStore()
    agent = new_agent(
        fake,
        store,
        resolve_session=lambda _: SessionBinding("native", last_event_id="missing"),
    )
    events = await collect(agent, base_input())
    assert events[-1].code == "history_cursor_unavailable"
    assert store.get(agent._session_key("thread_1")).last_event_id == "missing"
    assert not fake.sent


async def test_failed_event_translation_does_not_advance_cursor():
    import pytest
    from ag_ui_claude_managed_agents.turn import _consume
    from .fake_client import FakeStream

    observed = []

    def fail_emit(event):
        if event.type.value == "TEXT_MESSAGE_CONTENT":
            raise RuntimeError("consumer failed")

    with pytest.raises(RuntimeError, match="consumer failed"):
        await _consume(
            client=FakeClient(),
            session_id="native",
            stream=FakeStream(
                [
                    {
                        "id": "message",
                        "type": "agent.message",
                        "content": [{"type": "text", "text": "hello"}],
                    }
                ]
            ),
            client_tools={},
            backend_tools={},
            tool_confirmation=None,
            emit=fail_emit,
            on_event=lambda event: observed.append(event),
        )
    assert observed == []


async def test_custom_pending_and_replayed_names_match_frontend_tools():
    tools = [{"name": "show chart", "description": "show", "parameters": {}}]
    pending = {
        "id": "chart",
        "type": "agent.custom_tool_use",
        "name": "show_chart",
        "input": {},
    }
    for cursor in (None, "before"):
        fake = RecoveryClient(
            [
                [
                    {"id": "before", "type": "session.status_running"},
                    pending,
                    idle(["chart"]),
                ]
            ]
        )
        agent = new_agent(
            fake,
            resolve_session=lambda _: SessionBinding("native", last_event_id=cursor),
        )
        events = await collect(agent, base_input(tools=tools))
        assert [
            e.tool_call_name for e in events if e.type.value == "TOOL_CALL_START"
        ] == ["show chart"]
        assert not fake.sent


async def test_pending_calls_in_assistant_history_are_not_emitted_twice():
    fake = RecoveryClient(
        [[call("existing", "agent.custom_tool_use"), idle(["existing"])]]
    )
    agent = new_agent(fake, resolve_session=lambda _: SessionBinding("native"))
    events = await collect(
        agent,
        base_input(
            messages=[
                {"id": "u1", "role": "user", "content": "hello"},
                {
                    "id": "a1",
                    "role": "assistant",
                    "toolCalls": [
                        {
                            "id": "existing",
                            "type": "function",
                            "function": {"name": "lookup", "arguments": "{}"},
                        }
                    ],
                },
            ]
        ),
    )
    assert not [e for e in events if e.type.value.startswith("TOOL_CALL")]
    assert events[-1].type.value == "RUN_FINISHED"
    assert not fake.sent
