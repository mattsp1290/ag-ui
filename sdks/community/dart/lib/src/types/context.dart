/// Context and run types for AG-UI protocol.
library;

import 'base.dart';
import 'message.dart';
import 'tool.dart';

// Sentinel used by copyWith to distinguish "argument omitted" from
// "argument explicitly null" on nullable fields. Mirrors the same
// pattern in lib/src/types/message.dart and lib/src/events/events.dart.
class _Unset {
  const _Unset();
}

const _Unset _unsetContext = _Unset();

/// Additional context for the agent
class Context extends AGUIModel {
  final String description;
  final String value;

  const Context({
    required this.description,
    required this.value,
  });

  factory Context.fromJson(Map<String, dynamic> json) {
    return Context(
      description: JsonDecoder.requireField<String>(json, 'description'),
      value: JsonDecoder.requireField<String>(json, 'value'),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    'description': description,
    'value': value,
  };

  @override
  Context copyWith({
    String? description,
    String? value,
  }) {
    return Context(
      description: description ?? this.description,
      value: value ?? this.value,
    );
  }
}

/// Input for running an agent.
///
/// The optional [parentRunId] mirrors the canonical TS/Python
/// `RunAgentInput.parentRunId` / `parent_run_id` field; it links the
/// run to a parent run in nested-run scenarios.
class RunAgentInput extends AGUIModel {
  final String threadId;
  final String runId;
  final String? parentRunId;
  final dynamic state;
  final List<Message> messages;
  final List<Tool> tools;
  final List<Context> context;
  final dynamic forwardedProps;

  const RunAgentInput({
    required this.threadId,
    required this.runId,
    this.parentRunId,
    this.state,
    required this.messages,
    required this.tools,
    required this.context,
    this.forwardedProps,
  });

  factory RunAgentInput.fromJson(Map<String, dynamic> json) {
    return RunAgentInput(
      threadId: JsonDecoder.requireEitherField<String>(
        json,
        'threadId',
        'thread_id',
      ),
      runId: JsonDecoder.requireEitherField<String>(
        json,
        'runId',
        'run_id',
      ),
      parentRunId: JsonDecoder.optionalEitherField<String>(
        json,
        'parentRunId',
        'parent_run_id',
      ),
      state: json['state'],
      messages: JsonDecoder.requireListField<Map<String, dynamic>>(
        json,
        'messages',
      ).map((item) => Message.fromJson(item)).toList(),
      tools: JsonDecoder.requireListField<Map<String, dynamic>>(
        json,
        'tools',
      ).map((item) => Tool.fromJson(item)).toList(),
      context: JsonDecoder.requireListField<Map<String, dynamic>>(
        json,
        'context',
      ).map((item) => Context.fromJson(item)).toList(),
      // `forwardedProps` is intentionally `dynamic` (any JSON shape),
      // so the inline KEY-presence chain is preferred over
      // `optionalEitherField<T>` (which requires a concrete `T`). Behavior
      // matches the helper: `camelKey` wins when the key is present (even
      // when its value is explicitly `null`); `snake_case` is consulted
      // ONLY when camelCase is entirely absent.
      forwardedProps: json.containsKey('forwardedProps')
          ? json['forwardedProps']
          : json['forwarded_props'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    'threadId': threadId,
    'runId': runId,
    if (parentRunId != null) 'parentRunId': parentRunId,
    if (state != null) 'state': state,
    'messages': messages.map((m) => m.toJson()).toList(),
    'tools': tools.map((t) => t.toJson()).toList(),
    'context': context.map((c) => c.toJson()).toList(),
    if (forwardedProps != null) 'forwardedProps': forwardedProps,
  };

  // `parentRunId`, `state`, and `forwardedProps` are nullable —
  // sentinel lets callers clear them explicitly via `copyWith(field: null)`.
  // Mirrors the message-class sentinel in lib/src/types/message.dart.
  @override
  RunAgentInput copyWith({
    String? threadId,
    String? runId,
    Object? parentRunId = _unsetContext,
    Object? state = _unsetContext,
    List<Message>? messages,
    List<Tool>? tools,
    List<Context>? context,
    Object? forwardedProps = _unsetContext,
  }) {
    return RunAgentInput(
      threadId: threadId ?? this.threadId,
      runId: runId ?? this.runId,
      parentRunId: identical(parentRunId, _unsetContext)
          ? this.parentRunId
          : parentRunId as String?,
      state: identical(state, _unsetContext) ? this.state : state,
      messages: messages ?? this.messages,
      tools: tools ?? this.tools,
      context: context ?? this.context,
      forwardedProps: identical(forwardedProps, _unsetContext)
          ? this.forwardedProps
          : forwardedProps,
    );
  }
}

/// Represents a run in the AG-UI protocol
class Run extends AGUIModel {
  final String threadId;
  final String runId;
  final dynamic result;

  const Run({
    required this.threadId,
    required this.runId,
    this.result,
  });

  factory Run.fromJson(Map<String, dynamic> json) {
    return Run(
      threadId: JsonDecoder.requireEitherField<String>(
        json,
        'threadId',
        'thread_id',
      ),
      runId: JsonDecoder.requireEitherField<String>(
        json,
        'runId',
        'run_id',
      ),
      result: json['result'],
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    'threadId': threadId,
    'runId': runId,
    if (result != null) 'result': result,
  };

  // `result` is nullable — sentinel for explicit-clear semantics.
  @override
  Run copyWith({
    String? threadId,
    String? runId,
    Object? result = _unsetContext,
  }) {
    return Run(
      threadId: threadId ?? this.threadId,
      runId: runId ?? this.runId,
      result: identical(result, _unsetContext) ? this.result : result,
    );
  }
}

/// Type alias for state (can be any type)
typedef State = dynamic;