"""Restart controls use saved SDK identities, never inferred history callbacks."""
from copy import deepcopy
from types import SimpleNamespace
import pytest
from ag_ui_claude_sdk.recovery import RecoveryRun, SessionBinding

PENDING = {'id': 'call-original', 'name': 'Bash', 'input': {'command': 'echo test'}}

def request(behavior=None, call_id='call-original'):
    return SimpleNamespace(resume=[] if behavior is None else [SimpleNamespace(
        interrupt_id=call_id, status='resolved', payload={'behavior': behavior})])

def event(reason=None, pending=None, error=False):
    return SimpleNamespace(stop_reason=reason, deferred_tool_use=pending, is_error=error)

def hook_input(**updates):
    return {'hook_event_name': 'PreToolUse', 'tool_use_id': PENDING['id'],
            'tool_name': PENDING['name'], 'tool_input': PENDING['input'], **updates}

@pytest.mark.asyncio
@pytest.mark.parametrize('behavior', ['allow', 'deny'])
async def test_restart_replays_original_call_and_clears_only_after_completion(behavior):
    first = RecoveryRun(None, request())
    first.observe('native-uuid', 'original-environment', event('tool_deferred', deepcopy(PENDING)))
    saved = deepcopy(first.binding)
    second = RecoveryRun(saved, request(behavior))
    output = await second.hook(hook_input(), PENDING['id'], {})
    assert output['hookSpecificOutput']['permissionDecision'] == behavior
    assert saved.pending['id'] == PENDING['id']
    second.observe('native-uuid', 'original-environment', event())
    assert saved.pending is None
    assert saved.session_id == 'native-uuid'
    assert await second.hook(hook_input(), PENDING['id'], {}) == {}
    with pytest.raises(ValueError, match='No saved SDK deferral'):
        RecoveryRun(deepcopy(saved), request(behavior))

@pytest.mark.asyncio
async def test_unanswered_deferral_remains_actionable_after_restart():
    run = RecoveryRun(SessionBinding('native', 'profile', deepcopy(PENDING)), request())
    assert run.awaiting_decision
    assert run.interrupts[0]['id'] == PENDING['id']
    assert (await run.hook(hook_input(), PENDING['id'], {}))['hookSpecificOutput']['permissionDecision'] == 'defer'

@pytest.mark.asyncio
async def test_mismatch_and_missing_tool_do_not_consume_approval():
    run = RecoveryRun(SessionBinding('native', 'profile', deepcopy(PENDING)), request('allow'))
    output = await run.hook(hook_input(tool_input={'command': 'different'}), PENDING['id'], {})
    assert output['hookSpecificOutput']['permissionDecision'] == 'deny'
    with pytest.raises(ValueError, match='does not match'):
        run.observe('native', 'profile', event())
    run = RecoveryRun(SessionBinding('native', 'profile', deepcopy(PENDING)), request('allow'))
    with pytest.raises(ValueError, match='unavailable'):
        run.observe('native', 'profile', event('tool_deferred_unavailable', PENDING, True))
    assert run.binding.pending == PENDING
    with pytest.raises(ValueError, match='UUID'):
        run.observe('replacement', 'profile')


def test_callback_history_and_parallel_approvals_are_not_recoverable():
    with pytest.raises(ValueError, match='No saved SDK deferral'):
        RecoveryRun(SessionBinding('native', 'profile'), request('allow'))
    multiple = request('allow')
    multiple.resume *= 2
    with pytest.raises(ValueError, match='one tool call'):
        RecoveryRun(SessionBinding('native', 'profile', PENDING), multiple)

@pytest.mark.asyncio
@pytest.mark.parametrize('behavior', ['allow', 'deny'])
async def test_adapter_restart_restores_profile_original_uuid_and_decision(monkeypatch, behavior):
    from ag_ui.core import RunAgentInput
    from claude_agent_sdk import ResultMessage, SystemMessage
    from ag_ui_claude_sdk import ClaudeAgentAdapter
    import ag_ui_claude_sdk.adapter as module
    import claude_agent_sdk
    class Store:
        value = None
        async def load(self, thread):
            return deepcopy(self.value)
        async def save(self, thread, binding):
            self.value = deepcopy(binding)
    store = Store()
    options_seen = []
    decisions = []
    class Worker:
        def __init__(self, thread, options):
            self.options = options
            options_seen.append(options)
        async def start(self): pass
        async def stop(self): pass
        def is_alive(self): return True
        async def query(self, prompt, session_id):
            yield SystemMessage(subtype='init', data={'session_id': 'native-uuid'})
            if self.options.resume:
                assert prompt is None
                response = await self.options.hooks['PreToolUse'][-1].hooks[0](hook_input(), PENDING['id'], {})
                decisions.append(response['hookSpecificOutput']['permissionDecision'])
            yield ResultMessage(subtype='success', duration_ms=1, duration_api_ms=1,
                                is_error=False, num_turns=1, session_id='native-uuid',
                                stop_reason=None if self.options.resume else 'tool_deferred',
                                deferred_tool_use=None if self.options.resume else deepcopy(PENDING))
    monkeypatch.setattr(module, 'SessionWorker', Worker)
    monkeypatch.setattr(claude_agent_sdk, 'get_session_info', lambda *a, **k: SimpleNamespace(session_id='native-uuid'))
    profiles = []
    async def resolve(profile):
        profiles.append(profile)
        return {'cwd': '/original-project', 'permission_mode': 'default'}
    def adapter():
        return ClaudeAgentAdapter('test', session_binding_store=store, resolve_profile=resolve, execution_profile='original-profile')
    def run_input(resume=None):
        return RunAgentInput(thread_id='public-thread', run_id='run', messages=[], tools=[], context=[], state={}, forwarded_props={}, resume=resume)
    first = adapter()
    events = [e async for e in first.run(run_input())]
    assert events[-1].outcome.type == 'interrupt'
    assert events[-1].outcome.interrupts[0].id == PENDING['id']
    await first.shutdown()
    second = adapter()
    events = [e async for e in second.run(run_input([{'interruptId': PENDING['id'], 'status': 'resolved', 'payload': {'behavior': behavior}}]))]
    assert events[-1].type == 'RUN_FINISHED'
    assert options_seen[1].resume == 'native-uuid'
    assert options_seen[1].fork_session is False
    assert options_seen[1].cwd == '/original-project'
    assert decisions == [behavior]
    assert store.value.pending is None
    assert profiles == ['original-profile', 'original-profile']
    await second.shutdown()

@pytest.mark.asyncio
async def test_completed_tool_stays_completed_when_model_later_fails():
    run = RecoveryRun(SessionBinding('native', 'profile', deepcopy(PENDING)), request('allow'))
    await run.hook(hook_input(), PENDING['id'], {})
    assert run.observe_tool_result(PENDING['id'])
    run.observe('native', 'profile', event(error=True))
    assert run.binding.pending is None
