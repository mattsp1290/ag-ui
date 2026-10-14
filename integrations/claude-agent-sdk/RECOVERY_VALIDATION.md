# PNI-659 recovery evidence — 2026-10-14

Status: draft implementation; not released, not full product validation.
Companion content implementation: [AG-UI #3058](https://github.com/ag-ui-protocol/ag-ui/pull/3058).
Import/routing/projection companion: [Intelligence #1879](https://github.com/CopilotKit/Intelligence/pull/1879).

## Baseline and tests

- TypeScript Agent SDK 0.3.296 / bundled CLI 2.1.296; AG-UI core/client 1.0.0 in the isolated validation environment. Typecheck and 54 focused tests pass.
- Python Agent SDK 0.2.166 / bundled CLI 2.1.299; AG-UI protocol 1.0.0, Python 3.14.4. 149 focused tests pass.
- Tests use actual published SDK packages, not editable SDK overrides. The adapters are this branch's source plus the explicit PNI-658 companion.
- Nx was attempted first; the isolated dependency layout could not provide Nx's pnpm graph metadata. Focused Vitest/pytest/typecheck commands were used. No full workspace or product-stack pass is claimed.

## Actual SDK subprocess controls

The checked-in [TypeScript probe](typescript/scripts/probe-durable-resume.mjs)
and [Python probe](python/scripts/probe_durable_resume.py) use a localhost fake
Anthropic Messages endpoint and generated synthetic files. There are no external
model requests or personal histories. Each control creates one SDK session,
saves a native `Read` deferral, destroys the adapter, constructs another adapter
with the same binding store/profile, and answers the original call.

Allow and deny independently preserve the SDK UUID and tool-use ID. Allow saves
the actual synthetic file text. Deny saves the real native error result and does
not execute Read. The emitted AG-UI tool result agrees with native storage;
denials use the adapter's explicit JSON error envelope and retain `is_error` in
`rawEvent`. Each case uses exactly two model requests: initial tool selection and
one resumed completion, without a synthetic empty-user follow-up.

The controls use local JSONL storage. They demonstrate adapter reconstruction
and separate SDK subprocesses; the binding-store test double remains in the test
host. A production database store, host restart, distributed locking, custom
SessionStore backend, remote storage, credentials, browser/Intelligence replay,
and the six-row product matrix are not certified by this evidence.

## Startup findings and remaining blocker

Without `--await-initialize`, automatic native deferred replay can occur before
SDK hooks register. A requested deny can be skipped. The adapter therefore sets
that barrier for original-session resume. It requires the tested bundled CLI
(or another verified compatible binary); the flag was added in CLI 2.1.261.
Sending an empty string adds a user/model turn; TypeScript instead keeps an
empty input stream open through the result, and Python receives the automatic
replay without calling `client.query` again.

The [MCP probe](typescript/scripts/probe-mcp-deferral-limit.mjs) reproduces a
separate limitation: deferred in-process SDK MCP tools are checked before SDK MCP
registration, returning `tool_deferred_unavailable` before `system:init` announces
the same tool connected. Reproduced on TypeScript 0.3.296 and 0.3.299, including a
fresh MCP server instance, `alwaysLoad`, and the initialization barrier. The
adapter rejects known pending in-process SDK MCP calls before execution and
retains the binding. Consequently the frontend durable-result bridge is still
blocked; this ticket and #3058 are not ready to ship as frontend recovery.
Python's equivalent in-process MCP path is conservatively guarded but has not
been independently exercised against a live SDK subprocess in this pass.

The SDK's durable deferral supports one tool call at a time. Parallel-call
approval recovery is not implemented or promised. Historical callbacks without
a saved `hook_deferred_tool` marker remain unrecoverable.

## Follow-up

Resolve in-process SDK MCP startup before enabling frontend durable recovery;
then rerun actual SDK proxy invocation/result persistence controls for each port.
Validate Intelligence's consumer projection and browser compatibility separately.
Release and final six-row validation remain with the authorized release/user
workflow. No Linear Done or matrix update is justified by this draft.
