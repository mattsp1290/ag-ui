# ag-ui-dart

Dart SDK for the **Agent-User Interaction (AG-UI) Protocol**.

`ag-ui-dart` provides Dart developers with strongly-typed client implementations for connecting to AG-UI compatible agent servers. Built with modern Dart patterns for robust validation, reactive programming, and seamless server-sent event streaming.

## Installation

```bash
dart pub add ag_ui
```

Or add to your `pubspec.yaml`:

```yaml
dependencies:
  ag_ui: ^0.3.0
```

## Features

- 🎯 **Dart-native** – Idiomatic Dart APIs with full type safety and null safety
- 🔗 **HTTP connectivity** – `AgUiClient` for direct server connections with SSE streaming
- 📡 **Event streaming** – 36 canonical event models covering messages, tools, state, activity, reasoning, lifecycle, and subagents, with the remaining compatibility boundaries documented below
- 🔄 **State management** – Automatic message/state tracking with JSON Patch support
- 🛠️ **Tool interactions** – Typed tool calls plus generative UI patterns in the Flutter example
- ⚡ **High performance** – Efficient event decoding with backpressure handling

## Quick example

```dart
import 'package:ag_ui/ag_ui.dart';

// Initialize client
final client = AgUiClient(
  config: AgUiClientConfig(
    baseUrl: 'https://api.example.com',
    defaultHeaders: {'Authorization': 'Bearer token'},
  ),
);

// Create and send message
final input = SimpleRunAgentInput(
  messages: [
    UserMessage(
      id: 'msg_123',
      content: 'Hello from Dart!',
    ),
  ],
);

// Stream response events
await for (final event in client.runAgent('agentic_chat', input)) {
  if (event is TextMessageContentEvent) {
    print('Assistant: ${event.delta}');
  }
}
```

## Package

Import the public SDK surface from `package:ag_ui/ag_ui.dart`. It exports the
client, event and message models, protocol codecs, metadata helpers, run
outcomes, usage helpers, and capability value models.

## Documentation

- Concepts & architecture: [`docs/concepts`](https://docs.ag-ui.com/concepts/architecture)
- Full API reference: [`docs/sdk/dart`](https://docs.ag-ui.com/sdk/dart/client/overview)

## Protocol parity scope

The parity suite pins the Dart implementation base
`aaa75b54d572be8cd1d51c72e951273c5b893ed0` and canonical upstream inspection
revision `0fa1bebd9772de79347f0caf79744535e94ec37c`. The resolved shared corpus has
95 cases across 16 cross-language artifact routes. These include Dart direct
and encoder routes plus six directed Dart-to/from-peer consumer routes. It
verifies model JSON, canonical HTTP input encoding, SSE event encoding and
decoding, and cross-language exchange with the Go, Python, and TypeScript SDKs. See the
[`parity manifest`](test/fixtures/parity_manifest.json),
[`inventory test`](test/parity/inventory_test.dart),
[`route test`](test/parity/codec_routes_test.dart), and
[`parity gate`](../../../scripts/dart-sdk-parity.sh).

This evidence covers the shared wire contract. Language-specific agent,
middleware, reactive-stream, and UI runtimes remain separate APIs.

### Metadata and subagent attribution

Canonical metadata is represented by `Metadata`; `agUiMetadataKey` is the
reserved AG-UI key. `mergeMetadata(existing, incoming)` performs a shallow
merge in which incoming entries win, including explicit null, false, zero, or
empty values. It does not recursively merge nested maps.

All seven concrete message roles can carry optional `subagentRunId`. The 24
optional event carriers are text start/content/end/chunk; tool-call
start/args/end/chunk/result; state snapshot/delta; activity snapshot/delta;
raw/custom; step start/finish; and reasoning start, message
start/content/end/chunk, end, and encrypted-value events.

`RUN_*`, `MESSAGES_SNAPSHOT`, and deprecated `THINKING_*` event classes do not
gain optional attribution. The three subagent lifecycle events instead use a
required `subagentRunId` as the identity of the child run. A
`MessagesSnapshotEvent` carries attribution on each contained message.

### Interrupts, resume, and subagents

Use `runAgentInput` for the canonical request shape and typed resume entries:

<!-- documentation-test:canonical-resume:start -->
```dart
final resumedEvents = client.runAgentInput(
  'human_in_the_loop',
  const RunAgentInput(
    threadId: 'thread-1',
    runId: 'run-2',
    parentRunId: 'run-1',
    messages: [],
    tools: [],
    context: [],
    resume: [
      ResumeEntry(
        interruptId: 'approval-1',
        status: ResumeStatus.resolved,
        payload: {'approved': true},
      ),
    ],
  ),
);
```
<!-- documentation-test:canonical-resume:end -->

```dart
await for (final event in resumedEvents) {
  if (event case RunFinishedEvent(
    outcome: final RunFinishedInterruptOutcome outcome,
  )) {
    for (final interrupt in outcome.interrupts) {
      print('Interrupted: ${interrupt.id}');
    }
  } else if (event is SubagentStartedEvent) {
    print('Child started: ${event.subagentRunId}');
  } else if (event is SubagentFinishedEvent) {
    print('Child result: ${event.result}');
  }
}
```

The legacy convenience path can carry the same typed resume entry while
retaining its historical request defaults:

<!-- documentation-test:legacy-resume:start -->
```dart
final legacyResumedEvents = client.runAgent(
  'human_in_the_loop',
  const SimpleRunAgentInput(
    threadId: 'thread-1',
    runId: 'run-2',
    parentRunId: 'run-1',
    resume: [
      ResumeEntry(
        interruptId: 'approval-1',
        status: ResumeStatus.resolved,
        payload: {'approved': true},
      ),
    ],
  ),
);
```
<!-- documentation-test:legacy-resume:end -->

A suspended subagent outcome may have no interrupt IDs when a descendant owns
the active interrupt. `runAgent(String, SimpleRunAgentInput)` remains the
convenience API with its legacy empty-container defaults. `RunAgentInput.toJson`
omits a null `forwardedProps`; `runAgentInput` uses `Encoder` to include the
required `forwardedProps: null` key in the canonical HTTP request.

### Usage and capabilities

`RunFinishedEvent` and `RunErrorEvent` can carry `TokenUsage`. Use
`aggregateTokenUsage` to combine entries by the first-seen `(provider, model)`
pair. Token counts are validated through `maxTokenCount` (`2^53 - 1`), the
largest exact shared integer range across the supported SDKs.

`AgentCapabilities.fromJson` parses capability declarations for identity,
transport, tools, output, state, multi-agent operation, reasoning, multimodal
input, execution, and human-in-the-loop behavior. These are value models; the
Dart client does not provide capability discovery or negotiation transport.

### Exhaustive-switch migration

The next release containing this Unreleased surface adds three enum values and
matching sealed event subtypes. Code that previously ended its exhaustive
switch at `reasoningEncryptedValue` must add these cases:

```diff
   EventType.reasoningEncryptedValue => 'ReasoningEncryptedValueEvent',
+  EventType.subagentStarted => 'SubagentStartedEvent',
+  EventType.subagentFinished => 'SubagentFinishedEvent',
+  EventType.subagentError => 'SubagentErrorEvent',
```

The same migration applies to sealed `BaseEvent` switches. This complete
post-migration probe is compiled directly from this README by
`test/parity/documentation_test.dart`:

<!-- documentation-test:exhaustive-switch:start -->
```dart
String _documentedEventTypeName(EventType type) => switch (type) {
  EventType.textMessageStart => 'TextMessageStartEvent',
  EventType.textMessageContent => 'TextMessageContentEvent',
  EventType.textMessageEnd => 'TextMessageEndEvent',
  EventType.textMessageChunk => 'TextMessageChunkEvent',
  EventType.thinkingTextMessageStart => 'ThinkingTextMessageStartEvent',
  EventType.thinkingTextMessageContent => 'ThinkingTextMessageContentEvent',
  EventType.thinkingTextMessageEnd => 'ThinkingTextMessageEndEvent',
  EventType.toolCallStart => 'ToolCallStartEvent',
  EventType.toolCallArgs => 'ToolCallArgsEvent',
  EventType.toolCallEnd => 'ToolCallEndEvent',
  EventType.toolCallChunk => 'ToolCallChunkEvent',
  EventType.toolCallResult => 'ToolCallResultEvent',
  EventType.thinkingStart => 'ThinkingStartEvent',
  EventType.thinkingContent => 'ThinkingContentEvent',
  EventType.thinkingEnd => 'ThinkingEndEvent',
  EventType.stateSnapshot => 'StateSnapshotEvent',
  EventType.stateDelta => 'StateDeltaEvent',
  EventType.messagesSnapshot => 'MessagesSnapshotEvent',
  EventType.activitySnapshot => 'ActivitySnapshotEvent',
  EventType.activityDelta => 'ActivityDeltaEvent',
  EventType.raw => 'RawEvent',
  EventType.custom => 'CustomEvent',
  EventType.runStarted => 'RunStartedEvent',
  EventType.runFinished => 'RunFinishedEvent',
  EventType.runError => 'RunErrorEvent',
  EventType.stepStarted => 'StepStartedEvent',
  EventType.stepFinished => 'StepFinishedEvent',
  EventType.reasoningStart => 'ReasoningStartEvent',
  EventType.reasoningMessageStart => 'ReasoningMessageStartEvent',
  EventType.reasoningMessageContent => 'ReasoningMessageContentEvent',
  EventType.reasoningMessageEnd => 'ReasoningMessageEndEvent',
  EventType.reasoningMessageChunk => 'ReasoningMessageChunkEvent',
  EventType.reasoningEnd => 'ReasoningEndEvent',
  EventType.reasoningEncryptedValue => 'ReasoningEncryptedValueEvent',
  EventType.subagentStarted => 'SubagentStartedEvent',
  EventType.subagentFinished => 'SubagentFinishedEvent',
  EventType.subagentError => 'SubagentErrorEvent',
};

String _documentedEventName(BaseEvent event) => switch (event) {
  TextMessageStartEvent() => 'TextMessageStartEvent',
  TextMessageContentEvent() => 'TextMessageContentEvent',
  TextMessageEndEvent() => 'TextMessageEndEvent',
  TextMessageChunkEvent() => 'TextMessageChunkEvent',
  ThinkingStartEvent() => 'ThinkingStartEvent',
  ThinkingContentEvent() => 'ThinkingContentEvent',
  ThinkingEndEvent() => 'ThinkingEndEvent',
  ThinkingTextMessageStartEvent() => 'ThinkingTextMessageStartEvent',
  ThinkingTextMessageContentEvent() => 'ThinkingTextMessageContentEvent',
  ThinkingTextMessageEndEvent() => 'ThinkingTextMessageEndEvent',
  ToolCallStartEvent() => 'ToolCallStartEvent',
  ToolCallArgsEvent() => 'ToolCallArgsEvent',
  ToolCallEndEvent() => 'ToolCallEndEvent',
  ToolCallChunkEvent() => 'ToolCallChunkEvent',
  ToolCallResultEvent() => 'ToolCallResultEvent',
  StateSnapshotEvent() => 'StateSnapshotEvent',
  StateDeltaEvent() => 'StateDeltaEvent',
  MessagesSnapshotEvent() => 'MessagesSnapshotEvent',
  ActivitySnapshotEvent() => 'ActivitySnapshotEvent',
  ActivityDeltaEvent() => 'ActivityDeltaEvent',
  RawEvent() => 'RawEvent',
  CustomEvent() => 'CustomEvent',
  RunStartedEvent() => 'RunStartedEvent',
  RunFinishedEvent() => 'RunFinishedEvent',
  RunErrorEvent() => 'RunErrorEvent',
  StepStartedEvent() => 'StepStartedEvent',
  StepFinishedEvent() => 'StepFinishedEvent',
  ReasoningStartEvent() => 'ReasoningStartEvent',
  ReasoningMessageStartEvent() => 'ReasoningMessageStartEvent',
  ReasoningMessageContentEvent() => 'ReasoningMessageContentEvent',
  ReasoningMessageEndEvent() => 'ReasoningMessageEndEvent',
  ReasoningMessageChunkEvent() => 'ReasoningMessageChunkEvent',
  ReasoningEndEvent() => 'ReasoningEndEvent',
  ReasoningEncryptedValueEvent() => 'ReasoningEncryptedValueEvent',
  SubagentStartedEvent() => 'SubagentStartedEvent',
  SubagentFinishedEvent() => 'SubagentFinishedEvent',
  SubagentErrorEvent() => 'SubagentErrorEvent',
};
```
<!-- documentation-test:exhaustive-switch:end -->

### Deliberate compatibility boundaries

- Protobuf event encoding and WebSocket transport are not implemented.
- Capability models do not add an HTTP discovery endpoint.
- `Tool.metadata` is declared in TypeScript and Go; Python accepts it through
  permissive extra fields. Go collapses absent and empty optional metadata
  maps. Go and Python accept an outer null metadata value, while TypeScript
  rejects it. All peers retain null values stored under metadata keys.
- Python's `MetadataMixin` has no direct Dart type; Dart exposes metadata on
  the concrete public models and through the shared helpers instead.
- `THINKING_CONTENT` remains a deprecated Dart-only event for compatibility.
- `Message.id` remains nullable on the base API, although concrete decoders
  and constructors require IDs where the protocol does.
- Dart and Python aggregate usage by structural `(provider, model)` tuples.
  TypeScript joins labels with a space, so distinct tuples whose concatenated
  labels match can collide. Go uses tuple grouping but may collapse empty
  labels to absent values. Shared token counts stop at `2^53 - 1` even though
  Go can represent larger `int64` values.
- `ExecutionCapabilities.maxIterations` and `maxExecutionTime` use the shared
  integral signed-`int64` domain; TypeScript-only fractional limits are
  excluded.
- `tokenUsageFromLangChainMetadata` accepts nonnegative safe integers and
  ignores invalid counts. TypeScript preserves some negative, fractional, or
  unsafe metadata values. The TypeScript-only AI SDK usage mapper is outside
  this package.
- Go legacy encrypted-content fields, request aliases, and other extensions
  remain outside shared unknown-key round-trip guarantees.
- `SimpleRunAgentInput.state` and `forwardedProps` retain legacy dynamic types;
  their map-or-null checks are debug-only assertions. Use `RunAgentInput` for
  canonical typed request construction.
- Route normalization also covers omitted `ActivitySnapshotEvent.replace`
  defaulting to true, cipher-event `rawEvent` scrubbing, semantic JSON Patch
  validation differences, and omitted null subagent results.
- TypeScript agent, middleware, and reactive runtime facilities are outside
  the Dart wire-model parity scope.

## Core Usage

### Initialize Client

```dart
import 'package:ag_ui/ag_ui.dart';

final client = AgUiClient(
  config: AgUiClientConfig(
    baseUrl: 'https://api.example.com',
    defaultHeaders: {'Authorization': 'Bearer token'},
    requestTimeout: Duration(seconds: 30),
  ),
);
```

### Stream Agent Responses

```dart
final input = SimpleRunAgentInput(
  messages: [
    UserMessage(
      id: 'msg_${DateTime.now().millisecondsSinceEpoch}',
      content: 'Explain quantum computing',
    ),
  ],
);

await for (final event in client.runAgent('agentic_chat', input)) {
  switch (event.eventType) {
    case EventType.textMessageContent:
      final text = (event as TextMessageContentEvent).delta;
      print(text); // Stream tokens
      break;
    case EventType.runFinished:
      print('Complete');
      break;
    default:
      break; // This example handles only content and completion.
  }
}
```

### Activity & Reasoning Events

```dart
import 'dart:io'; // for `stderr` in the example below

await for (final event in client.runAgent('agentic_chat', input)) {
  if (event is ActivitySnapshotEvent) {
    // `content` is `Object?` — the Python reference server may emit a
    // primitive or `null`. Guard before treating it as a structured record.
    final content = event.content;
    if (content is Map<String, dynamic>) {
      // `event.replace == true`  → discard prior content for this messageId.
      // `event.replace == false` → merge/extend on top of existing content.
      print(
        'Activity (${event.activityType}, replace=${event.replace}): $content',
      );
    } else {
      // Wire-protocol surprise: log and skip rather than crash.
      stderr.writeln(
        'ActivitySnapshotEvent.content is ${content.runtimeType}, '
        'expected Map<String, dynamic>',
      );
    }
  } else if (event is ActivityDeltaEvent) {
    print('Activity patch (${event.activityType}): ${event.patch}');
  } else if (event is ReasoningMessageContentEvent) {
    print('Reasoning: ${event.delta}');
  } else if (event is ReasoningEncryptedValueEvent) {
    // Opaque cipher payload — pass through to the next agent rather than
    // attempting to decode locally.
  }
}
```

### Multimodal Input

A `UserMessage` accepts either plain text or an ordered list of typed parts
(text, image, audio, video, document). Use `UserMessage.multimodal` for parts:

```dart
// A base64-encoded payload for an inline data part.
const base64Pdf = 'JVBERi0xLjQKJ...';

final input = SimpleRunAgentInput(
  messages: [
    UserMessage.multimodal(
      id: 'msg_${DateTime.now().millisecondsSinceEpoch}',
      parts: [
        TextInputContent('What is in this image?'),
        ImageInputContent(
          // UrlSource.mimeType is optional; DataSource requires it.
          source: UrlSource(
            value: 'https://example.com/photo.png',
            mimeType: 'image/png',
          ),
        ),
        DocumentInputContent(
          source: DataSource(value: base64Pdf, mimeType: 'application/pdf'),
        ),
      ],
    ),
  ],
);
```

The `content` getter returns the text for text-only messages and `null` for
multimodal ones; read `messageContent` for the typed union.

The default `UserMessage({content})` constructor is not `const` because it
wraps the string in `TextContent` at runtime. Use `UserMessage.fromContent` to
keep a compile-time constant — this is also the migration path if you
previously used `const UserMessage(content: '...')`:

```dart
// Before (no longer const):
// UserMessage(id: 'u-1', content: 'Hello')

// After — const-friendly:
const msg = UserMessage.fromContent(
  id: 'u-1',
  messageContent: TextContent('Hello'),
);
```

### Tool-Based Interactions

```dart
List<ToolCall> toolCalls = [];

// Collect tool calls from first run
await for (final event in client.runToolBasedGenerativeUi(input)) {
  if (event is MessagesSnapshotEvent) {
    for (final msg in event.messages) {
      if (msg is AssistantMessage && msg.toolCalls != null) {
        toolCalls.addAll(msg.toolCalls!);
      }
    }
  }
}

// Process tool calls and send results
final toolResults = toolCalls.map((call) => ToolMessage(
  id: 'tool_${DateTime.now().millisecondsSinceEpoch}',
  toolCallId: call.id,
  content: processToolCall(call),
)).toList();

final followUp = SimpleRunAgentInput(
  threadId: input.threadId,
  messages: [...input.messages, ...toolResults],
);

// Get final response
await for (final event in client.runToolBasedGenerativeUi(followUp)) {
  // Handle response
}
```

### State Management

```dart
Map<String, dynamic> state = {};
List<Message> messages = [];

await for (final event in client.runSharedState(input)) {
  switch (event.eventType) {
    case EventType.stateSnapshot:
      state = (event as StateSnapshotEvent).snapshot;
      break;
    case EventType.stateDelta:
      // Apply JSON Patch (RFC 6902) operations
      applyJsonPatch(state, (event as StateDeltaEvent).delta);
      break;
    case EventType.messagesSnapshot:
      messages = (event as MessagesSnapshotEvent).messages;
      break;
    default:
      break; // This example handles only state and message snapshots.
  }
}
```

### Error Handling

The Dart SDK errors form a single hierarchy under [`AGUIError`](https://pub.dev/documentation/ag_ui/latest/ag_ui/AGUIError-class.html). Catch that base if you want one handler for everything; catch the specific subclasses below for targeted recovery. Through [`EventDecoder`](https://pub.dev/documentation/ag_ui/latest/ag_ui/EventDecoder-class.html) the wire-decode side throws [`DecodingError`]; the client-side request/transport layer throws [`TransportError`] and [`ValidationError`]; cancellation surfaces as [`CancellationError`].

```dart
final cancelToken = CancelToken();

try {
  await for (final event in client.runAgent('agent', input, cancelToken: cancelToken)) {
    // Process events
    if (shouldCancel(event)) {
      cancelToken.cancel();
      break;
    }
  }
} on TransportError catch (e) {
  print('Connection error: ${e.message}');
} on DecodingError catch (e) {
  print('Decode error: ${e.message}');
} on ValidationError catch (e) {
  print('Validation error: ${e.message}');
} on CancellationError {
  print('Request cancelled');
} on AGUIError catch (e) {
  // Catch-all for any AG-UI-originated error (covers
  // AGUIValidationError thrown directly from a `Type.fromJson` call
  // when the event isn't routed through the EventDecoder pipeline).
  print('AG-UI error: $e');
}
```

> **Cancellation note:** `CancelToken.cancel()` stops event delivery to your
> stream, but does **not** abort the underlying HTTP socket. You may inject an
> `http.Client` when constructing `AgUiClient`; disposing the `AgUiClient`
> closes that shared client and all of its connections.

### Proxy notes: wire-spelling normalization

The Dart SDK accepts both **camelCase** (TypeScript-canonical, e.g. `threadId`,
`runId`, `parentRunId`, `encryptedValue`, `rawEvent`) and **snake_case**
(Python-canonical, e.g. `thread_id`, `run_id`, `parent_run_id`,
`encrypted_value`, `raw_event`) on every `fromJson` factory, but always
emits **camelCase** on `toJson` — there is no opt-in to snake_case wire
output.

If you use the Dart SDK as a proxy between a snake_case-emitting Python
server and a strictly snake_case-only consumer, you must convert keys
back at the boundary. The TypeScript and Python canonical SDKs both
tolerate the camelCase form on input, so this is rarely an issue in
practice — but a strict snake_case consumer is technically protocol-valid
and will see a normalized payload from a Dart middle-tier.

Within a single `BaseEvent.rawEvent` round-trip the spelling is
preserved by the helper that reads both keys (`rawEvent` /
`raw_event`); the camelCase emit on the Dart side is the only
normalization point.

## Complete Example

```dart
import 'dart:io';
import 'package:ag_ui/ag_ui.dart';

void main() async {
  // Initialize client from environment
  final client = AgUiClient(
    config: AgUiClientConfig(
      baseUrl: Platform.environment['AGUI_BASE_URL'] ?? 'http://localhost:8000',
      defaultHeaders: Platform.environment['AGUI_API_KEY'] != null
          ? {'Authorization': 'Bearer ${Platform.environment['AGUI_API_KEY']}'}
          : null,
    ),
  );

  // Interactive chat loop
  stdout.write('You: ');
  final userInput = stdin.readLineSync() ?? '';

  final input = SimpleRunAgentInput(
    messages: [
      UserMessage(
        id: 'msg_${DateTime.now().millisecondsSinceEpoch}',
        content: userInput,
      ),
    ],
  );

  stdout.write('Assistant: ');
  await for (final event in client.runAgent('agentic_chat', input)) {
    if (event is TextMessageContentEvent) {
      stdout.write(event.delta);
    } else if (event is ToolCallStartEvent) {
      print('\nCalling tool: ${event.toolCallName}');
    } else if (event.eventType == EventType.runFinished) {
      print('\nDone!');
      break;
    }
  }

  client.dispose();
}
```

## Migrating from 0.1.0

0.2.0 introduces one source-breaking change for callers that construct
events directly:

- **`ToolCallResultEvent.role` is now `ToolCallResultRole?` instead of
  `String?`.** Update direct constructions:

  ```dart
  // Before (0.1.0)
  ToolCallResultEvent(
    messageId: '...',
    toolCallId: '...',
    content: '...',
    role: 'tool',
  );

  // After (0.2.0)
  ToolCallResultEvent(
    messageId: '...',
    toolCallId: '...',
    content: '...',
    role: ToolCallResultRole.tool,
  );
  ```

  Wire decoding is unaffected: an unknown `role` string on the wire is
  absorbed via `ToolCallResultRole.fromString` and falls back to
  `ToolCallResultRole.tool` for forward compatibility. See
  [`CHANGELOG.md`](CHANGELOG.md) "Breaking Changes" for the full
  rationale.

- **`TimeoutError` was renamed to `AGUITimeoutError`** to avoid
  shadowing `dart:async.TimeoutError` (raised by `Future.timeout(...)` /
  `Stream.timeout(...)`). The bare name is preserved as a deprecated
  typedef alias and will be removed in 1.0.0:

  ```dart
  // Before (0.1.0)
  } on TimeoutError catch (e) { /* ... */ }

  // After (0.2.0)
  } on AGUITimeoutError catch (e) { /* ... */ }
  ```

  If you import both `package:ag_ui/ag_ui.dart` and `dart:async`, prefer
  the new name to avoid a symbol collision and to ensure raw
  `dart:async.TimeoutError` instances (very common from any
  `.timeout(...)` call) are not silently absorbed by an `on TimeoutError`
  arm targeting the SDK type.

  Note for the inverse case: if you previously meant
  `dart:async.TimeoutError` and were accidentally catching SDK instances
  (because `package:ag_ui/ag_ui.dart`'s `TimeoutError` won the unqualified
  name resolution), the rename surfaces the prior collision. After you
  migrate to `AGUITimeoutError`, the bare `TimeoutError` arm now
  unambiguously refers to `dart:async.TimeoutError` — runtime behavior
  changes accordingly.

The `THINKING_TEXT_MESSAGE_*` event types are also deprecated in 0.2.0
in favor of the canonical `REASONING_*` events; decoding remains
supported until 1.0.0. See `CHANGELOG.md` "Deprecated" for the migration
mapping.

## Errors

The SDK exposes a small error hierarchy that is intentionally split by origin:

- `AGUIError` — the SDK-wide root. Catching `on AGUIError` covers every
  error the SDK can raise: runtime, transport, decoding, AND direct-factory
  validation. Use this when you want a single catch-all.
- `AgUiError` — extends `AGUIError`. Covers runtime / transport / decoding:
  `TransportError`, `AGUITimeoutError`, `CancellationError`, `DecodingError`,
  and the client-side `ValidationError`. Catch this when you want to scope
  to "the SDK encountered a runtime problem" but explicitly do NOT want to
  catch direct-factory validation errors. (`TimeoutError` is preserved as
  a deprecated alias for `AGUITimeoutError`; prefer the new name to avoid
  shadowing `dart:async.TimeoutError`.)
- `AGUIValidationError` — extends `AGUIError` (NOT `AgUiError`). Thrown by
  `*.fromJson` factory constructors at the wire-decoding boundary. When
  events flow through `EventDecoder`, this is wrapped as `DecodingError`,
  so consumers using the decoder pipeline never see this directly. Direct
  factory callers (`TextMessageStartEvent.fromJson(...)`) do.
- `EncoderError` and its subtypes (`DecodeError`, `EncodeError`,
  encoder-side `ValidationError`) extend `AGUIError`. The `EventDecoder`
  pipeline rethrows these unchanged so callers can pattern-match by type.

Recommended catch recipe in production code that uses `EventDecoder`:

```dart
try {
  for (final event in stream) { handle(event); }
} on DecodingError catch (e) {
  // Wire-format problem — log e.field, e.expectedType, e.actualValue.
} on TransportError catch (e) {
  // HTTP / SSE transport failure.
} on AgUiError catch (e) {
  // Anything else from the runtime/transport family.
} on AGUIError catch (e) {
  // Catch-all (would also catch direct-factory AGUIValidationError if you
  // ever bypass the decoder).
}
```

## Examples

The [Flutter dojo](example/) pairs this SDK with the local Go example server. Its eleven destinations demonstrate streaming messages, client tools, local approvals, cards, shared/predictive state, media, and scripted reasoning. The example README includes the two-terminal quickstart and a credential-free Docker contract suite.

## Testing

```bash
# From sdks/community/dart:
dart pub get --no-example
AGUI_SKIP_DOJO=1 dart test
dart test test/parity/documentation_test.dart

# From the repository root:
bash scripts/dart-sdk-parity.sh
```

The Dojo smoke and resilience tests under `test/integration` use
`AGUI_DOJO_BASE_URL` (or `AGUI_BASE_URL`) and skip their live cases when the
server is unavailable. See [`TEST_GUIDE.md`](TEST_GUIDE.md) for browser and
cross-language commands.

## Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## Cipher-data preservation

Some AG-UI events (`ReasoningEncryptedValueEvent`, `ReasoningMessage`, `ToolMessage`) carry
opaque cipher payloads that must be forwarded verbatim between agents. This SDK implements
defense-in-depth around those payloads:

**Success paths** — ordinary decoded events can preserve the wire-format map in
`BaseEvent.rawEvent`. Cipher-bearing paths may intentionally clear it:
`ReasoningEncryptedValueEvent` always does so, and `RunStartedEvent` or
`MessagesSnapshotEvent` can do so when nested input or messages contain cipher
data. A proxy that requires exact forwarding must retain the original map or
bytes before decoding.

**Error paths** — when a factory (`fromJson`) fails to decode an event, the thrown
`AGUIValidationError` intentionally omits the raw JSON map (`json:` field) for any event
that may carry cipher data. This prevents raw cipher bytes from leaking through
reflection-based log shippers or error serializers that walk the exception cause chain.

**Parsed values** — `toJson()` emits the typed event fields and is not an
exact-byte forwarding mechanism. Never render or log encrypted payloads or
arbitrary metadata unless the application explicitly owns that disclosure.

## License

This SDK is part of the AG-UI Protocol project. See the [main repository](https://github.com/ag-ui-protocol/ag-ui) for license information.

## Bounded SSE byte parsing

`SseClient.parseStream` incrementally decodes strict UTF-8 and bounds every
unfinished line, including comments, unknown/colonless fields, IDs and retry
fields. `connect` uses the same parser limits and retains its existing reconnect
policy. `parseStream` errors are terminal for that call: a content-free
`FormatException` cancels the source and discards partial data/event/retry state.
Completed messages before a malformed or oversized suffix remain delivered.
The caller's HTTP client remains usable, and concurrent `parseStream` calls
have independent state and do not change `SseClient.lastEventId`.

```dart
final sse = SseClient(
  maxDataCodeUnits: 1024 * 1024,
  maxLineCodeUnits: 1024 * 1024 + 7, // optional; this is the derived default
);
final messages = sse.parseStream(responseBytes);
```

Both limits count **UTF-16 code units**, not UTF-8 bytes: a supplementary
character counts as two. The data/event cap D defaults to `8 * 1024 * 1024`.
Data aggregation counts the newline separators between `data:` fields,
including empty fields; an event value is replaced on each `event:` line and
is separately limited to D. The line cap L defaults to the actual D **plus 7**,
preserving full D-unit `data: ` and `event: ` values. `AgUiClient` derives L
from its existing `EventStreamAdapter.maxDataCodeUnits` configuration as well.

L counts the entire line, including field name, colon and all spaces, and
excludes CR/LF terminators. For compatibility, the standard decoder removes
one initial UTF-8 BOM and the first-line adjustment removes one more leading
U+FEFF. A third initial BOM remains and counts; later BOMs are never stripped.
The internal `SseParser.parseLines` also checks L, but does not strip BOMs or
control allocations of caller-supplied complete strings.

Overflow fails as soon as a decoded unit would exceed L, without waiting for a
terminator, EOF, or another source chunk. IDs longer than 1024 units but within
L still get dropped with a content-free log and preserve the prior ID. IDs
exceeding L fail like every other line. Malformed/truncated UTF-8 fails without
replacement characters or partial EOF flush. If malformed UTF-8 and overflow
coexist in one decoder slice, encoding failure can take precedence. Normal EOF
still flushes a final unterminated line/message; CR, LF and fragmented CRLF
retain their dispatch behavior.

Existing call shapes remain valid. Oversized ignored lines and nonpositive
limits that were previously accepted are now rejected. Migrate legitimate
larger inputs by setting larger positive finite caps; there is no unlimited
mode or feature flag. Both caps must be integers in `1..9007199254740991`, the
exact range shared by VM and JavaScript; omitted L also requires D + 7 to fit.
Invalid configurations throw `ArgumentError` synchronously.

### Memory and backpressure

Let C = 1024 decoder-input bytes and I = 1024 sticky-ID units. The framer uses
geometrically growing `Uint16List` storage with capacity at most L, avoiding a
string or list entry for each one-byte source fragment. A decoder slice ends at
the earliest CR, LF or C-byte boundary. The strict decoder retains at most three
incomplete UTF-8 bytes. Logical state is at most L unfinished line units, D
aggregate data units, D event units, I ID units, one bounded decoded slice
(at most C + 2 units), and constant counters/BOM/CR flags.

A conservative bound including simultaneous transient payloads is
**6L + 8D + 2I + 8(C + 3) UTF-16 units, plus C + 3 bytes**:

- 6L covers typed storage and its old allocation during growth, the completed
  line string, field/value substrings and the optional leading-space copy.
- 8D + 2I covers both message buffers, buffer-to-string copies, and the current
  message during dispatch/handoff (ID references normally share storage).
- 8(C + 3) plus C + 3 bytes covers bounded decoder output, StringBuffer/join
  copies, the current byte-slice copy and UTF-8 carry.

This is an O(L + D + I + C) payload bound, not an exact heap-byte or RSS promise.
StringBuffer capacity, object headers, iterator objects and allocator overhead
are runtime-dependent and separate from logical payload units. Buffer fragment
counts are bounded by their contents; line framing does not create one heap
object per code unit. The parser retains no completed-message history.
Producer-owned current chunks (which may remain referenced until consumed),
producer/transport queues, caller `parseLines` strings and messages retained by
consumers are excluded. On cancellation/error, pending iterators, line buffers
and decoder state are released. Each transform keeps at most one lazy input
iterator and propagates pause/resume/cancel; it never expands a chunk into an
eager list or queues an unbounded number of output events.

See [TEST_GUIDE.md](TEST_GUIDE.md#bounded-byte-parser-conformance) for public API,
VM/Chrome lifecycle, diagnostic-capture and external immutable-pin probes.
