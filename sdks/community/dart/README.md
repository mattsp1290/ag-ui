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
  ag_ui: ^0.2.0
```

## Features

- 🎯 **Dart-native** – Idiomatic Dart APIs with full type safety and null safety
- 🔗 **HTTP connectivity** – `AgUiClient` for direct server connections with SSE streaming
- 📡 **Event streaming** – Event-type parity with the canonical Python and TypeScript SDKs (text messages, tool calls, state, activity, reasoning, lifecycle, and more) for real-time agent communication.
- 🔄 **State management** – Automatic message/state tracking with JSON Patch support
- 🛠️ **Tool interactions** – Full support for tool calls and generative UI
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

## Packages

- **`ag_ui`** – Core client library for AG-UI protocol
- **`ag_ui.client`** – HTTP client with SSE streaming support
- **`ag_ui.events`** – Event types and event handling
- **`ag_ui.types`** – Message types, tools, and data models
- **`ag_ui.encoder`** – Event encoding/decoding utilities

## Documentation

- Concepts & architecture: [`docs/concepts`](https://docs.ag-ui.com/concepts/architecture)
- Full API reference: [`docs/sdk/dart`](https://docs.ag-ui.com/sdk/dart/client/overview)

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
  }
}
```

### Error Handling

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
} on ValidationError catch (e) {
  print('Validation error: ${e.message}');
} on CancellationError {
  print('Request cancelled');
}
```

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

The `THINKING_TEXT_MESSAGE_*` event types are also deprecated in 0.2.0
in favor of the canonical `REASONING_*` events; decoding remains
supported until 1.0.0. See `CHANGELOG.md` "Deprecated" for the migration
mapping.

## Examples

See the [`example/`](example/) directory for:
- Interactive CLI for testing AG-UI servers
- Tool-based generative UI flows
- Message streaming patterns
- Complete end-to-end demonstrations

## Testing

```bash
# Run unit tests
dart test

# Run integration tests (requires server)
cd test/integration
./helpers/start_server.sh
dart test
./helpers/stop_server.sh
```

## Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

This SDK is part of the AG-UI Protocol project. See the [main repository](https://github.com/ag-ui-protocol/ag-ui) for license information.


