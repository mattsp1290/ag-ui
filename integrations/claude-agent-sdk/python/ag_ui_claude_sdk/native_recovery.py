"""Opt-in native child recovery; no event database dependency.

The transport shim is deliberately pinned to the verified SDK/CLI release.
"""
import asyncio
import copy
import json
import os
import shlex
import signal
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Awaitable, Callable
from claude_agent_sdk import ClaudeSDKClient, HookMatcher, ResultMessage, SystemMessage, UserMessage, ToolResultBlock
from claude_agent_sdk._internal.transport.subprocess_cli import SubprocessCLITransport
from .native_history import NativeHistory

MESSAGE = 'Continue the original task using the recorded tool results. Do not repeat completed tools.'


@dataclass
class NativeChildRecoveryConfig:
    directory: str
    tools: list[str]
    execute: Callable[[dict, str], Awaitable[dict]]
    acquire: Callable[[str], Awaitable[Callable[[], Awaitable[None]]]]
    timeout_seconds: float = 10


class IsolatedTransport(SubprocessCLITransport):
    """Own a process group, and kill it before SDK EOF/close can unblock hooks."""
    def __init__(self, options):
        async def empty():
            if False:
                yield {}
        super().__init__(empty(), options)
        self._kill_sent = False
        self.finished_normally = False
        self._wrapper = tempfile.TemporaryDirectory(prefix='agui-claude-worker-')
        path = Path(self._wrapper.name) / 'claude'
        code = 'import os,sys;os.setsid();os.execv(sys.argv[1],sys.argv[1:])'
        path.write_text('#!/bin/sh\nexec ' + shlex.quote(sys.executable) + ' -c ' + shlex.quote(code) + ' ' + shlex.quote(self._cli_path or self._find_cli()) + ' "$@"\n')
        path.chmod(0o700)
        self._cli_path = str(path)

    def kill_now(self):
        if self._process and self._process.returncode is None and not self._kill_sent:
            try:
                os.killpg(self._process.pid, signal.SIGKILL)
                self._kill_sent = True
            except ProcessLookupError:
                self._process.kill()
                self._kill_sent = True

    async def close(self):
        if not self.finished_normally:
            self.kill_now()
        if self._process and not self.finished_normally:
            await self._process.wait()
        try:
            await super().close()
        finally:
            self._wrapper.cleanup()


class NativeChildRecovery:
    def __init__(self, config):
        if not config.tools or not 0 < config.timeout_seconds < 50:
            raise ValueError('Native child recovery requires tools and a deadline below 50 seconds')
        self.config = config
        self.outcome = None
        self.session_id = None
        self.transport = None
        self.stopped = False
        self.pausing = False
        self.bridge = False
        self.completed = False
        self.gate_held = False
        self.resume_issued = False
        self.failure = None
        self.bridge_results = {}

    def stop(self):
        self.stopped = True
        if self.transport:
            self.transport.kill_now()

    def history(self):
        return NativeHistory(self.config.directory, self.session_id, self.config.tools)

    def interrupt(self, checkpoint):
        self.outcome = {'type': 'interrupt', 'interrupts': [dict(id=c['id'], toolCallId=c['id'], reason='tool_approval',
                         metadata=dict(source='claude-sdk-native-child', sessionId=self.session_id, childId=checkpoint['childId'], parentToolUseId=checkpoint['parent']['id'],toolName=c['name'], input=c['input'])) for c in checkpoint['pending']]}

    async def answer_checkpoint(self, input_data, checkpoint):
        results = {}
        for call in checkpoint['calls']:
            decisions = [r for r in (input_data.resume or []) if r.interrupt_id == call['id']]
            messages = [m for m in input_data.messages if m.role == 'tool' and m.tool_call_id == call['id']]
            if len(decisions) > 1 or len(messages) > 1 or (decisions and messages):
                raise ValueError('Duplicate or conflicting tool answers')
            response = decisions[0] if decisions else None
            message = messages[0] if messages else None
            if response is None and message is None:
                continue
            submitted = None
            if message is not None:
                if call['name'] not in self.frontend_names:
                    raise ValueError('Client tool results may only answer configured frontend tools')
                marker = (message.metadata or {}).get('ag-ui:tool-result', {})
                error = message.error or (marker.get('error') if isinstance(marker, dict) else None)
                submitted = dict(content=message.content if isinstance(message.content, str) else json.dumps(message.content), is_error=bool(error))
            if response is not None:
                if call['name'] in self.frontend_names:
                    raise ValueError('Frontend calls require their actual tool result')
                if response.status != 'cancelled' and (response.payload or {}).get('behavior') not in ('allow', 'deny'):
                    raise ValueError('Child answer requires behavior allow or deny')
                if (response.payload or {}).get('updatedInput'):
                    raise ValueError('Native recovery answers cannot change the original tool input')
            saved = self.history().result(checkpoint, call['id'])
            if saved is not None:
                if submitted is not None and submitted != saved:
                    raise ValueError('Retransmitted frontend result differs from canonical history')
                continue
            if not any(c['id'] == call['id'] for c in checkpoint['pending']):
                raise ValueError('Tool is not awaiting an answer')
            result = submitted
            if result is None:
                result = await self.config.execute(copy.deepcopy(call), f"{self.session_id}:{checkpoint['childId']}:{call['id']}") if response.status != 'cancelled' and response.payload.get('behavior') == 'allow' else dict(content='Tool execution denied by user', is_error=True)
            if not isinstance(result.get('content'), str) or ('is_error' in result and not isinstance(result['is_error'], bool)):
                raise ValueError('Recovery executor must return actual string content and optional is_error')
            self.history().answer(checkpoint, {call['id']: result})
            results[call['id']] = result
        if results:
            yield UserMessage(content=[ToolResultBlock(tool_use_id=key, content=r['content'], is_error=bool(r.get('is_error'))) for key,r in results.items()], parent_tool_use_id=checkpoint['parent']['id'] or None)

    async def run(self, input_data, options, prompt):
        if os.name != 'posix' or options.session_store or options.fork_session or options.resume_session_at or options.continue_conversation:
            raise ValueError('Native child recovery requires exclusive local POSIX sessions without custom store, fork or rewind')
        self.frontend_names = {f'mcp__ag_ui__{t.name}' for t in input_data.tools}
        self.config = copy.copy(self.config)
        self.config.tools = list(set(self.config.tools) | self.frontend_names)
        self.session_id = options.resume or options.session_id
        if not self.session_id:
            raise ValueError('Native child recovery requires an explicit session_id for new sessions')
        release = await self.config.acquire(self.session_id)
        try:
            root = self.history().root() if options.resume else None
            checkpoint = self.history().inspect() if options.resume else None
            known = (root['calls'] if root else []) + (checkpoint['calls'] if checkpoint else [])
            ids = [r.interrupt_id for r in (input_data.resume or [])]
            if len(set(ids)) != len(ids) or any(key not in {c['id'] for c in known} for key in ids):
                raise ValueError('Answer does not match the original tool call or is duplicated')
            for saved in [root, checkpoint]:
                if saved:
                    async for message in self.answer_checkpoint(input_data, saved):
                        yield message
            root_pending = self.history().root() if root else None
            if root_pending and root_pending['pending']:
                child_pending = self.history().inspect()
                self.interrupt({**root_pending, 'pending': root_pending['pending'] + (child_pending['pending'] if child_pending else [])})
                return
            if checkpoint:
                checkpoint = self.history().inspect()
                if checkpoint['pending']:
                    self.interrupt(checkpoint)
                    return
                if checkpoint['final'] is None:
                    self.bridge = True
                    async for _ in self.execute(options, f"Resume child {checkpoint['childId']} with SendMessage, message exactly: {MESSAGE}", checkpoint):
                        pass
                    self.history().settle_bridge(checkpoint, self.bridge_results)
                    if self.pausing:
                        next_checkpoint = self.history().inspect()
                        if next_checkpoint and next_checkpoint['pending']:
                            self.interrupt(next_checkpoint)
                            return
                    if not self.completed or not self.gate_held:
                        raise self.failure or RuntimeError('Child recovery did not complete while the parent was held')
                    self.bridge = False
                completed = self.history().inspect()
                self.history().repair(checkpoint)
                yield UserMessage(content=[ToolResultBlock(tool_use_id=checkpoint['parent']['id'], content=completed['final'], is_error=False)])
                prompt = 'Continue from the completed child result recorded in your history.'
            async for message in self.execute(options, prompt):
                yield message
            if self.pausing:
                root_pending = self.history().root()
                child_pending = self.history().inspect()
                checkpoint = {**(child_pending or root_pending), 'pending': root_pending['pending'] + (child_pending['pending'] if child_pending else [])}
                if not checkpoint or not checkpoint['pending']:
                    raise RuntimeError('Child approval was not durably recorded')
                self.interrupt(checkpoint)
        finally:
            self.stop()
            await release()

    async def execute(self, options, prompt, checkpoint=None):
        self.stopped = False
        opts = copy.copy(options)
        opts.session_id = None if options.resume else options.session_id
        opts.extra_args = {**(options.extra_args or {}), 'await-initialize': None}

        async def pre(data, tool_id, context):
            if data['tool_name'] == 'Agent' and (data.get('agent_id') or data.get('tool_input', {}).get('run_in_background') is not False):
                return {'hookSpecificOutput': {'hookEventName': 'PreToolUse', 'permissionDecision': 'deny', 'permissionDecisionReason': 'Native recovery requires one direct foreground child. Set run_in_background to false.'}}
            if self.bridge and not data.get('agent_id'):
                args = data.get('tool_input', {})
                if not self.resume_issued and data['tool_name'] == 'SendMessage' and args.get('to') == checkpoint['childId'] and args.get('message') == MESSAGE:
                    self.resume_issued = True
                    return {}
                self.bridge_results[data['tool_use_id']] = dict(content='Parent tool was not executed: held during child recovery', is_error=True)
                return {'hookSpecificOutput': {'hookEventName': 'PreToolUse', 'permissionDecision': 'deny', 'permissionDecisionReason': 'Parent is held during child recovery'}}
            if data['tool_name'] in self.config.tools:
                self.pausing = True
                deadline = asyncio.get_running_loop().time() + 4
                while True:
                    try:
                        saved = self.history().inspect() if data.get('agent_id') else self.history().root()
                        if saved and any(c['id'] == data['tool_use_id'] for c in saved['pending']):
                            break
                    except (ValueError, FileNotFoundError) as error:
                        if asyncio.get_running_loop().time() >= deadline:
                            self.failure = error
                            break
                    if asyncio.get_running_loop().time() >= deadline:
                        self.failure = RuntimeError('Native approval was not flushed before recovery deadline')
                        break
                    await asyncio.sleep(.025)
                self.stop()
                await asyncio.Event().wait()
            return {}

        async def post(data, tool_id, context):
            if self.bridge and not data.get('agent_id') and data['tool_name'] == 'SendMessage':
                self.bridge_results[data['tool_use_id']] = dict(content=json.dumps(data['tool_response']), is_error=False)
                self.gate_held = True
                if self.completed:
                    self.stop()
                await asyncio.Event().wait()
            return {}

        opts.hooks = {**(options.hooks or {}), 'PreToolUse': [*(options.hooks or {}).get('PreToolUse', []), HookMatcher(hooks=[pre], timeout=60)],
                      'PostToolUse': [*(options.hooks or {}).get('PostToolUse', []), HookMatcher(hooks=[post], timeout=60)]}
        self.transport = IsolatedTransport(opts)
        def timeout():
            self.failure = RuntimeError('Child recovery timed out; parent remains unresolved')
            self.stop()
        timer = asyncio.get_running_loop().call_later(self.config.timeout_seconds, timeout) if self.bridge else None
        expected_stop = False
        try:
            async with ClaudeSDKClient(options=opts, transport=self.transport) as client:
                await client.query(prompt, session_id=self.session_id)
                try:
                    async for message in client.receive_messages():
                        if self.bridge and isinstance(message, SystemMessage) and message.subtype == 'task_notification' and message.data.get('task_id') == checkpoint['childId'] and message.data.get('status') == 'completed':
                            while not self.stopped and not self.history().completed(checkpoint):
                                await asyncio.sleep(.025)
                            self.completed = not self.stopped
                            if self.gate_held:
                                self.stop()
                        if isinstance(message, SystemMessage) and message.subtype == 'init' and message.data.get('claude_code_version') != '2.1.306':
                            raise RuntimeError('Native child recovery requires CLI 2.1.306')
                        if not self.bridge:
                            yield message
                        if not self.bridge and isinstance(message, ResultMessage):
                            self.transport.finished_normally = True
                            break
                finally:
                    expected_stop = self.stopped
                    if not self.transport.finished_normally:
                        self.stop()
        except Exception:
            if not expected_stop:
                raise
        finally:
            self.stop()
            if timer:
                timer.cancel()
        if self.failure:
            raise self.failure
