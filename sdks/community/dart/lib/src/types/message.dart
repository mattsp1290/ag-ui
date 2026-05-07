/// Message types for AG-UI protocol.
///
/// This library defines the message types used in agent-user conversations,
/// including user, assistant, system, tool, developer, activity, and
/// reasoning messages.
library;

import 'base.dart';
import 'tool.dart';

// `kUnsetSentinel` (from `base.dart`) is the shared sentinel for all
// `copyWith` methods in this file. The pattern lets callers distinguish
// "argument omitted" (preserve current value via `?? this.field`) from
// "argument explicitly null" (clear the field). Compared with `identical(...)`.

/// Role types for messages in the AG-UI protocol.
///
/// Mirrors the canonical TypeScript and Python `Message` discriminated
/// unions (see `sdks/typescript/packages/core/src/types.ts` and
/// `sdks/python/ag_ui/core/types.py`). The `activity` and `reasoning`
/// values exist so `MESSAGES_SNAPSHOT` payloads carrying those message
/// shapes decode in Dart with the same schema as the other SDKs.
enum MessageRole {
  developer('developer'),
  system('system'),
  assistant('assistant'),
  user('user'),
  tool('tool'),

  /// Wire spelling is `'activity'` (lowercase, single word) — canonical
  /// across the AG-UI protocol (TS `Literal["activity"]`, Python
  /// `Literal["activity"]`). The Dart symbol matches; this enum value
  /// pins the wire constant for [MessageRole.fromString] dispatch into
  /// [ActivityMessage]. Mirrors the wire-spelling-pinning style used by
  /// [ReasoningEncryptedValueSubtype.toolCall] (where the spelling
  /// difference is more consequential).
  activity('activity'),

  /// Wire spelling is `'reasoning'` (lowercase, single word) — canonical
  /// across the AG-UI protocol. The Dart symbol matches; this enum value
  /// pins the wire constant for [MessageRole.fromString] dispatch into
  /// [ReasoningMessage].
  reasoning('reasoning');

  final String value;
  const MessageRole(this.value);

  /// Parses [value] into a [MessageRole].
  ///
  /// Unlike `TextMessageRole.fromString` / `ReasoningMessageRole.fromString`
  /// (which throw `ArgumentError` and are absorbed at the event-factory
  /// level for forward-compat), this enum throws [AGUIValidationError]
  /// directly — the value is the discriminator that selects which
  /// [Message] subtype's `fromJson` to dispatch to, so an unknown role
  /// has no safe default. Mis-tagging a `MESSAGES_SNAPSHOT` payload
  /// would corrupt the snapshot rather than just lose one field.
  ///
  /// Through the public [EventDecoder] pipeline, this surfaces as
  /// `DecodingError(field: 'role')`. Direct callers of `Message.fromJson`
  /// see `AGUIValidationError` directly. See `dart-enum-parsing-safety.md`
  /// for the closed-vs-open enum rationale.
  static final Map<String, MessageRole> _byValue = {
    for (final r in MessageRole.values) r.value: r,
  };

  static MessageRole fromString(String value) {
    return _byValue[value] ??
        (throw AGUIValidationError(
          message: 'Invalid message role: $value',
          field: 'role',
          value: value,
        ));
  }
}

/// Base message class for all message types.
///
/// Messages represent the fundamental units of conversation in the AG-UI protocol.
/// Each message has a role, optional content, and may include additional metadata.
///
/// Use the [Message.fromJson] factory to deserialize messages from JSON.
///
/// Known parity gap with the canonical TS/Python SDKs: the canonical
/// `BaseMessageSchema.id` is `z.string()` (non-nullable). Dart keeps
/// `id` typed `String?` for legacy reasons but every concrete subtype
/// constructor declares it `required`, so a constructed in-memory
/// instance is null-safe by convention. A future major version may
/// tighten the type. See CHANGELOG → "Known parity gaps".
sealed class Message extends AGUIModel with TypeDiscriminator {
  final String? id;
  final MessageRole role;
  final String? content;
  final String? name;

  /// Opaque cipher payload preserved verbatim across proxy hops.
  ///
  /// Mirrors the canonical TS `BaseMessageSchema.encryptedValue:
  /// z.string().optional()` and Python `BaseMessage.encrypted_value:
  /// Optional[str]` — every concrete subtype that extends `BaseMessage`
  /// (Developer/System/Assistant/User/Tool) inherits this field. The
  /// canonical `ActivityMessage` and `ReasoningMessage` are NOT
  /// `BaseMessage` extensions; in this Dart sealed-class hierarchy they
  /// inherit the field too but their `fromJson` / `toJson` ignore it
  /// (`ActivityMessage`) or inherit it through the sealed parent without
  /// re-declaring locally (`ReasoningMessage` passes it via
  /// `super.encryptedValue` — there is no shadowing field on that subtype).
  ///
  /// Wire dual-key: factories read both `encryptedValue` (TS-canonical)
  /// and `encrypted_value` (Python-canonical) via
  /// [JsonDecoder.optionalEitherField]. `toJson` emits the camelCase
  /// spelling.
  final String? encryptedValue;

  const Message({
    this.id,
    required this.role,
    this.content,
    this.name,
    this.encryptedValue,
  });

  @override
  String get type => role.value;

  /// Factory constructor to create specific message types from JSON
  factory Message.fromJson(Map<String, dynamic> json) {
    final roleStr = JsonDecoder.requireField<String>(json, 'role');
    final MessageRole role;
    try {
      role = MessageRole.fromString(roleStr);
    } on AGUIValidationError catch (e) {
      // Omit json: and cause: — the message map and cause chain may carry
      // encryptedValue from ReasoningMessage / ToolMessage subtypes.
      // Surface only the structured field path and value for log safety.
      throw AGUIValidationError(
        message: e.message,
        field: e.field,
        value: e.value,
      );
    }

    // `MessageRole.fromString` deliberately throws on unknown values rather
    // than falling back to a default — unlike `TextMessageRole.fromString`
    // and `ReasoningMessageRole.fromString`, which absorb `ArgumentError` for
    // forward-compat. The role is the *dispatch discriminator*: an unknown role
    // has no safe default subtype. Changing this to a fallback would silently
    // mis-tag a MESSAGES_SNAPSHOT message, corrupting the list instead of
    // surfacing the wire violation at the decoder boundary.
    switch (role) {
      case MessageRole.developer:
        return DeveloperMessage.fromJson(json);
      case MessageRole.system:
        return SystemMessage.fromJson(json);
      case MessageRole.assistant:
        return AssistantMessage.fromJson(json);
      case MessageRole.user:
        return UserMessage.fromJson(json);
      case MessageRole.tool:
        return ToolMessage.fromJson(json);
      case MessageRole.activity:
        return ActivityMessage.fromJson(json);
      case MessageRole.reasoning:
        return ReasoningMessage.fromJson(json);
      // No `default` clause — exhaustive switch on the [MessageRole] enum
      // (analyzer-enforced). A new MessageRole value will produce a compile
      // error here, which is the desired outcome rather than a runtime
      // fall-through.
    }
  }

  @override
  Map<String, dynamic> toJson() => {
    if (id != null) 'id': id,
    'role': role.value,
    if (content != null) 'content': content,
    if (name != null) 'name': name,
    if (encryptedValue != null) 'encryptedValue': encryptedValue,
  };
}

/// Developer message with required content.
///
/// Used for system-level or developer-facing messages in the conversation.
final class DeveloperMessage extends Message {
  @override
  final String content;

  const DeveloperMessage({
    required super.id,
    required this.content,
    super.name,
    super.encryptedValue,
  }) : super(role: MessageRole.developer);

  factory DeveloperMessage.fromJson(Map<String, dynamic> json) {
    return DeveloperMessage(
      id: JsonDecoder.requireField<String>(json, 'id'),
      content: JsonDecoder.requireField<String>(json, 'content'),
      name: JsonDecoder.optionalField<String>(json, 'name'),
      encryptedValue: JsonDecoder.optionalEitherField<String>(
        json,
        'encryptedValue',
        'encrypted_value',
      ),
    );
  }

  // `name` and `encryptedValue` are nullable on the parent — use the
  // sentinel so callers can clear either explicitly. See [kUnsetSentinel].
  @override
  DeveloperMessage copyWith({
    String? id,
    String? content,
    Object? name = kUnsetSentinel,
    Object? encryptedValue = kUnsetSentinel,
  }) {
    return DeveloperMessage(
      id: id ?? this.id,
      content: content ?? this.content,
      name: identical(name, kUnsetSentinel) ? this.name : name as String?,
      encryptedValue: identical(encryptedValue, kUnsetSentinel)
          ? this.encryptedValue
          : encryptedValue as String?,
    );
  }
}

/// System message with required content.
///
/// Represents system-level instructions or context provided to the agent.
final class SystemMessage extends Message {
  @override
  final String content;

  const SystemMessage({
    required super.id,
    required this.content,
    super.name,
    super.encryptedValue,
  }) : super(role: MessageRole.system);

  factory SystemMessage.fromJson(Map<String, dynamic> json) {
    return SystemMessage(
      id: JsonDecoder.requireField<String>(json, 'id'),
      content: JsonDecoder.requireField<String>(json, 'content'),
      name: JsonDecoder.optionalField<String>(json, 'name'),
      encryptedValue: JsonDecoder.optionalEitherField<String>(
        json,
        'encryptedValue',
        'encrypted_value',
      ),
    );
  }

  // `name` and `encryptedValue` are nullable on the parent — sentinel
  // for explicit-clear semantics.
  @override
  SystemMessage copyWith({
    String? id,
    String? content,
    Object? name = kUnsetSentinel,
    Object? encryptedValue = kUnsetSentinel,
  }) {
    return SystemMessage(
      id: id ?? this.id,
      content: content ?? this.content,
      name: identical(name, kUnsetSentinel) ? this.name : name as String?,
      encryptedValue: identical(encryptedValue, kUnsetSentinel)
          ? this.encryptedValue
          : encryptedValue as String?,
    );
  }
}

/// Assistant message with optional content and tool calls.
///
/// Represents responses from the AI assistant, which may include
/// text content and/or tool call requests.
final class AssistantMessage extends Message {
  final List<ToolCall>? toolCalls;

  const AssistantMessage({
    required super.id,
    super.content,
    super.name,
    this.toolCalls,
    super.encryptedValue,
  }) : super(role: MessageRole.assistant);

  factory AssistantMessage.fromJson(Map<String, dynamic> json) {
    // KEY-level dual-key resolution with eager element-type validation.
    // Documented precedence rule (see [JsonDecoder.requireEitherField]
    // dartdoc): if camelCase `toolCalls` is present, it wins even when the
    // list is empty; snake_case `tool_calls` is consulted ONLY when
    // camelCase is absent. The pre-fix `??`-on-value chain incorrectly
    // surfaced `tool_calls` whenever camelCase resolved to null OR an
    // empty list — silently dropping snake_case data on payloads that
    // (incorrectly) carry both keys. The regression test
    // `message_test.dart:401-446` ("AssistantMessage.fromJson dual-key
    // precedence") pins this contract.
    //
    // Element-type validation: `optionalEitherListField` reports
    // `field: 'toolCalls[$i]'` on a malformed nested element rather than
    // letting a raw `TypeError` leak from the `as Map<String, dynamic>`
    // cast — same convention as `MessagesSnapshotEvent.fromJson`.
    final rawToolCalls =
        JsonDecoder.optionalEitherListField<Map<String, dynamic>>(
      json,
      'toolCalls',
      'tool_calls',
    );
    return AssistantMessage(
      id: JsonDecoder.requireField<String>(json, 'id'),
      content: JsonDecoder.optionalField<String>(json, 'content'),
      name: JsonDecoder.optionalField<String>(json, 'name'),
      toolCalls: rawToolCalls == null ? null : () {
        final result = <ToolCall>[];
        for (var i = 0; i < rawToolCalls.length; i++) {
          try {
            result.add(ToolCall.fromJson(rawToolCalls[i]));
          } catch (e) {
            if (e is AGUIValidationError) {
              // Omit `json:` and `cause:` — ToolCall.fromJson can set e.json
              // to a payload with sensitive `arguments`; the cause chain
              // exposes it to reflection-based log shippers.
              throw AGUIValidationError(
                message: e.message,
                field: 'toolCalls[$i].${e.field ?? 'unknown'}',
                value: e.value,
              );
            }
            throw AGUIValidationError(
              message: 'Failed to decode tool call at index $i: $e',
              field: 'toolCalls[$i]',
              cause: e,
            );
          }
        }
        return result;
      }(),
      encryptedValue: JsonDecoder.optionalEitherField<String>(
        json,
        'encryptedValue',
        'encrypted_value',
      ),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    // Emit `toolCalls` whenever the in-memory field is non-null, even
    // when empty, so the round-trip `fromJson(m.toJson()) == m` is
    // symmetric. The previous `&& toolCalls!.isNotEmpty` guard dropped
    // the key on empty lists, which decoded back to `null` instead of
    // `[]` and made tests that depend on field-by-field equality
    // surprising.
    if (toolCalls != null)
      'toolCalls': toolCalls!.map((tc) => tc.toJson()).toList(),
  };

  // See [kUnsetSentinel] for the sentinel rationale. `content`,
  // `name`, `toolCalls`, and `encryptedValue` are all nullable on
  // `AssistantMessage`, so callers may legitimately want to clear any
  // of them via `copyWith`.
  @override
  AssistantMessage copyWith({
    String? id,
    Object? content = kUnsetSentinel,
    Object? name = kUnsetSentinel,
    Object? toolCalls = kUnsetSentinel,
    Object? encryptedValue = kUnsetSentinel,
  }) {
    return AssistantMessage(
      id: id ?? this.id,
      content: identical(content, kUnsetSentinel)
          ? this.content
          : content as String?,
      name: identical(name, kUnsetSentinel) ? this.name : name as String?,
      toolCalls: identical(toolCalls, kUnsetSentinel)
          ? this.toolCalls
          : toolCalls as List<ToolCall>?,
      encryptedValue: identical(encryptedValue, kUnsetSentinel)
          ? this.encryptedValue
          : encryptedValue as String?,
    );
  }
}

/// User message with required content.
///
/// Represents input from the user in the conversation.
///
/// Known parity gap with the canonical TS/Python schemas: TS uses
/// `content: z.union([z.string(), z.array(InputContentSchema)])` and
/// Python uses `content: Union[str, List[InputContent]]` for full
/// multimodal support. This Dart SDK currently only supports the string
/// variant — a multimodal payload from a TS or Python server raises
/// `AGUIValidationError(field: 'content')` because the factory's
/// `requireField<String>` rejects the list type. Tracked for a future
/// release; see CHANGELOG → "Known parity gaps".
final class UserMessage extends Message {
  @override
  final String content;

  const UserMessage({
    required super.id,
    required this.content,
    super.name,
    super.encryptedValue,
  }) : super(role: MessageRole.user);

  factory UserMessage.fromJson(Map<String, dynamic> json) {
    return UserMessage(
      id: JsonDecoder.requireField<String>(json, 'id'),
      content: JsonDecoder.requireField<String>(json, 'content'),
      name: JsonDecoder.optionalField<String>(json, 'name'),
      encryptedValue: JsonDecoder.optionalEitherField<String>(
        json,
        'encryptedValue',
        'encrypted_value',
      ),
    );
  }

  // `name` and `encryptedValue` are nullable on the parent — sentinel
  // for explicit-clear semantics.
  @override
  UserMessage copyWith({
    String? id,
    String? content,
    Object? name = kUnsetSentinel,
    Object? encryptedValue = kUnsetSentinel,
  }) {
    return UserMessage(
      id: id ?? this.id,
      content: content ?? this.content,
      name: identical(name, kUnsetSentinel) ? this.name : name as String?,
      encryptedValue: identical(encryptedValue, kUnsetSentinel)
          ? this.encryptedValue
          : encryptedValue as String?,
    );
  }
}

/// Tool message with tool call result.
///
/// Contains the result of a tool execution, linked to a specific tool call
/// via the [toolCallId] field. The optional [encryptedValue] mirrors the
/// canonical TypeScript `ToolMessageSchema` and Python `ToolMessage` and
/// carries an opaque cipher payload that a Dart proxy must forward
/// verbatim to a downstream agent.
final class ToolMessage extends Message {
  @override
  final String content;
  final String toolCallId;
  final String? error;

  const ToolMessage({
    required super.id,
    required this.content,
    required this.toolCallId,
    this.error,
    super.encryptedValue,
  }) : super(role: MessageRole.tool);

  factory ToolMessage.fromJson(Map<String, dynamic> json) {
    return ToolMessage(
      id: JsonDecoder.requireField<String>(json, 'id'),
      content: JsonDecoder.requireField<String>(json, 'content'),
      toolCallId: JsonDecoder.requireEitherField<String>(
        json,
        'toolCallId',
        'tool_call_id',
      ),
      error: JsonDecoder.optionalField<String>(json, 'error'),
      encryptedValue: JsonDecoder.optionalEitherField<String>(
        json,
        'encryptedValue',
        'encrypted_value',
      ),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        ...super.toJson(),
        'toolCallId': toolCallId,
        if (error != null) 'error': error,
      };

  // `error` and `encryptedValue` are nullable — use the sentinel so a
  // caller can explicitly clear either via `copyWith(error: null)` /
  // `copyWith(encryptedValue: null)`. Mirrors the event-class sentinel
  // discipline.
  @override
  ToolMessage copyWith({
    String? id,
    String? content,
    String? toolCallId,
    Object? error = kUnsetSentinel,
    Object? encryptedValue = kUnsetSentinel,
  }) {
    return ToolMessage(
      id: id ?? this.id,
      content: content ?? this.content,
      toolCallId: toolCallId ?? this.toolCallId,
      error: identical(error, kUnsetSentinel) ? this.error : error as String?,
      encryptedValue: identical(encryptedValue, kUnsetSentinel)
          ? this.encryptedValue
          : encryptedValue as String?,
    );
  }
}

/// Activity message embedded in a `MESSAGES_SNAPSHOT` payload.
///
/// Mirrors the canonical TypeScript `ActivityMessageSchema`
/// (`sdks/typescript/packages/core/src/types.ts`) and the Python
/// `ActivityMessage` model (`sdks/python/ag_ui/core/types.py`). The wire
/// shape is `{id, role: 'activity', activityType, content}` where
/// `content` is a JSON object (`z.record(z.any())` / `Dict[str, Any]`).
///
/// The Dart in-memory accessor for the wire `content` field is named
/// [activityContent] to avoid shadowing the parent [Message.content]
/// (which is `String?`). The wire key remains `content` in [toJson] /
/// [fromJson] for protocol parity.
///
/// **`encryptedValue` note.** `ActivityMessage` inherits [encryptedValue]
/// from [Message] but intentionally does not expose it in the constructor,
/// [fromJson], or [toJson]. In the canonical protocol `ActivityMessage` is
/// NOT a `BaseMessage` extension (unlike Developer/System/Assistant/User/Tool
/// messages), so cipher-payload forwarding does not apply here. If the wire
/// payload contains `encryptedValue` / `encrypted_value`, [fromJson] strips
/// it silently (matching TS zod-default strip behavior). In-memory instances
/// constructed via [copyWith] on a parent [Message] may inherit the field,
/// but [toJson] never emits it.
final class ActivityMessage extends Message {
  final String activityType;
  final Map<String, dynamic> activityContent;

  const ActivityMessage({
    required super.id,
    required this.activityType,
    required this.activityContent,
  }) : super(role: MessageRole.activity);

  factory ActivityMessage.fromJson(Map<String, dynamic> json) {
    // `ActivityMessage` is NOT a `BaseMessage` extension in the canonical
    // protocol — cipher-payload forwarding does not apply. Strip any inbound
    // `encryptedValue` / `encrypted_value` silently, matching TS zod-default
    // strip behavior. A hard-fail here would make Dart the only SDK that tears
    // down the stream when a proxy emits the field (TS strips, Python preserves).
    return ActivityMessage(
      id: JsonDecoder.requireField<String>(json, 'id'),
      activityType: JsonDecoder.requireEitherField<String>(
        json,
        'activityType',
        'activity_type',
      ),
      activityContent:
          JsonDecoder.requireField<Map<String, dynamic>>(json, 'content'),
    );
  }

  @override
  Map<String, dynamic> toJson() => {
        // Explicitly skip super.toJson() — the inherited Message.content field
        // must not appear in the wire output (activityContent is the `content`
        // key here). Using ...super.toJson() would rely on map-spread
        // overwrite order to mask any future super.content emission.
        if (id != null) 'id': id,
        'role': role.value,
        'activityType': activityType,
        'content': activityContent,
      };

  @override
  ActivityMessage copyWith({
    String? id,
    String? activityType,
    Map<String, dynamic>? activityContent,
  }) {
    return ActivityMessage(
      id: id ?? this.id,
      activityType: activityType ?? this.activityType,
      activityContent: activityContent ?? this.activityContent,
    );
  }
}

/// Reasoning message embedded in a `MESSAGES_SNAPSHOT` payload.
///
/// Mirrors the canonical TypeScript `ReasoningMessageSchema` and the
/// Python `ReasoningMessage` model. The wire shape is
/// `{id, role: 'reasoning', content, encryptedValue?}` with `content` as
/// a string and `encryptedValue` as an optional opaque cipher payload.
final class ReasoningMessage extends Message {
  @override
  final String content;

  const ReasoningMessage({
    required super.id,
    required this.content,
    super.encryptedValue,
  }) : super(role: MessageRole.reasoning);

  factory ReasoningMessage.fromJson(Map<String, dynamic> json) {
    return ReasoningMessage(
      id: JsonDecoder.requireField<String>(json, 'id'),
      content: JsonDecoder.requireField<String>(json, 'content'),
      encryptedValue: JsonDecoder.optionalEitherField<String>(
        json,
        'encryptedValue',
        'encrypted_value',
      ),
    );
  }

  // `encryptedValue` is nullable on the parent — sentinel lets callers
  // clear it.
  @override
  ReasoningMessage copyWith({
    String? id,
    String? content,
    Object? encryptedValue = kUnsetSentinel,
  }) {
    return ReasoningMessage(
      id: id ?? this.id,
      content: content ?? this.content,
      encryptedValue: identical(encryptedValue, kUnsetSentinel)
          ? this.encryptedValue
          : encryptedValue as String?,
    );
  }
}