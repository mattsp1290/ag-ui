# Live conversation content contract

This describes the adapter implementation, not released product validation. TypeScript,
Python and .NET need independent release and application validation.

## Input and supported media

`RUN_STARTED.input` retains the original AG-UI messages, message IDs, content parts,
filenames in part metadata, source MIME types, state and run context. Recorder consumers
must retain that input even if the run subsequently reports an error.

User messages and frontend tool results accept text, image and document parts. Image
MIME types are PNG, JPEG, GIF and WebP; documents accept PDF and plain text. Inline
sources map to native base64 blocks, URL sources remain URLs, and `file` sources map to
native file IDs when `provider` is absent or `anthropic`. The application must supply a
handle accessible to the configured Anthropic workspace. The adapter does not fetch
URLs, upload files, resolve foreign provider handles, or promise model comprehension.
Document `metadata.filename` becomes the native document title. Image names and other
app metadata remain in the original AG-UI input because the native image block has no
filename field.

Audio, video, foreign provider handles and unsupported media types produce an explicit
`RUN_ERROR` with code `unsupported_content`; the original input remains recorded.
A mixed message is not partially sent. File storage and model understanding are separate
capabilities. This adapter does not mount arbitrary files into the managed sandbox.

## Native output

Canonical `agent.*` and echoed `user.*` events exposed by the installed SDK are emitted
as `RAW` events with `source: "claude-managed-agents"`. They retain native identities,
content, errors, child-thread fields and other source fields. RAW is evidence, not an
additional chat message; consumers should not project it as a duplicate transcript item.
Unknown event types can only be forwarded if the installed SDK exposes them.

Native built-in and MCP results use the native result event ID as `messageId` and the
original tool-use event ID as `toolCallId`. Plain text is untruncated and untrimmed.
Mixed results use AG-UI content parts: images/documents carry their original bytes, URLs
or native file IDs; each part retains its source block at
`metadata.claudeManagedAgents`. Native-only blocks, including search results, use a
text representation of the complete JSON block plus this metadata, rather than a
truncated preview or placeholder. RAW retains the complete event envelope as well.

## App state and rich UI events

The input state is emitted as `STATE_SNAPSHOT`. Backend tools may return a result envelope:

- TypeScript: `BackendToolResult` with `content: string` and optional `events`.
- Python: `BackendToolResult(content="...", events=[...])`.
- .NET: `ManagedAgentsBackendTool.ResultHandler` returns `ManagedAgentsBackendToolResult`.
  When set, it takes precedence over the existing string-returning `Handler`.

Existing string-returning handlers are unchanged. The event list accepts state snapshots,
state deltas, activity snapshots/deltas, custom events and raw events. Lifecycle and
message/tool events are rejected as an invalid handler result. Only `content` is sent to
Claude as the tool result. App events are emitted in order after that send succeeds;
a failed handler or failed delivery does not emit its app events.

Use `STATE_SNAPSHOT` for full replacement and `STATE_DELTA` for explicit RFC 6902 patches.
The adapter does not infer a merge from an arbitrary tool JSON object. The app or
Intelligence must retain the state and provide the next run's state. Likewise, middleware
UI artifacts only exist in native history if they were actually part of a native saved
payload. This API does not add native state/UI writeback for historical import.

## Companions and validation boundary

PNI-654 owns native extraction/import and Intelligence replay; PNI-656 owns original-session
recovery and approvals. This change covers the live content boundary, with fake native
streams through each adapter and independent language regression tests. It does not prove
cloud execution, credential access, file download availability, browser rendering or the
six-row product checklist. PNI-660 documentation waits for release and final validation.

Native contracts: [session events](https://platform.claude.com/docs/en/managed-agents/events-and-streaming)
and [session files](https://platform.claude.com/docs/en/managed-agents/files).

## Intelligence compatibility blocker

Intelligence baseline [`7ec248a8f`](https://github.com/CopilotKit/Intelligence/blob/7ec248a8f/pnpm-lock.yaml) uses `@ag-ui/client` 0.0.57, whose `ToolCallResultEventSchema` rejects `ContentPart[]`. Its live recorder and reconstruction path require a companion upgrade to consume this adapter's AG-UI 1.0 multipart results. Companion ownership is pending a user decision; this branch is not product-review-ready until that compatibility work is established and validated.
