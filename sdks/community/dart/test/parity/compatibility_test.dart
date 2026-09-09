import 'dart:convert';
import 'dart:io';

import 'package:ag_ui/ag_ui.dart';
import 'package:test/test.dart';

import 'contract_source_snapshot.dart';

// These legacy variants are intentionally exercised by the compatibility
// probe; their deprecation diagnostics are expected here.
// ignore_for_file: deprecated_member_use_from_same_package

// These switches intentionally have no default arm. They are compile probes:
// adding an EventType or a concrete BaseEvent requires an explicit migration
// here, making the approved three-event source change visible to consumers.
String _eventTypeName(EventType type) => switch (type) {
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
    };

String _baseEventName(BaseEvent event) => switch (event) {
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
    };

String? _messageId(Message message) => message.id;

Map<String, dynamic> _readJsonMap(File file) =>
    (jsonDecode(file.readAsStringSync()) as Map).cast<String, dynamic>();

Directory _packageRoot() {
  final starts = <Directory>[
    Directory.current.absolute,
    File.fromUri(Platform.script).parent.absolute,
  ];
  for (var start in starts) {
    while (true) {
      if (File('${start.path}/pubspec.yaml').existsSync()) {
        return start;
      }
      final parent = start.parent;
      if (parent.path == start.path) {
        break;
      }
      start = parent;
    }
  }
  throw StateError('Could not locate the Dart package root');
}

final _dartPackageRoot = _packageRoot();
final _repositoryRoot = _dartPackageRoot.parent.parent.parent;
final _fixture = _readJsonMap(
  File('${_dartPackageRoot.path}/test/fixtures/compatibility.json'),
);
final _parityManifest = _readJsonMap(
  File('${_dartPackageRoot.path}/test/fixtures/parity_manifest.json'),
);
late Map<String, dynamic> _goFixtures;

Map<String, dynamic> _asMap(Object? value, String description) {
  if (value is Map<Object?, Object?>) {
    return value.cast<String, dynamic>();
  }
  throw StateError('$description must be a JSON object');
}

Object? _atJsonPointer(Object? root, String pointer) {
  if (pointer.isEmpty) {
    return root;
  }
  if (!pointer.startsWith('/')) {
    throw StateError('Invalid JSON pointer: $pointer');
  }
  var current = root;
  for (final rawSegment in pointer.substring(1).split('/')) {
    final segment = rawSegment.replaceAll('~1', '/').replaceAll('~0', '~');
    if (current is Map) {
      if (!current.containsKey(segment)) {
        throw StateError('Missing JSON pointer $pointer at $segment');
      }
      current = current[segment];
    } else if (current is List) {
      final index = int.tryParse(segment);
      if (index == null || index < 0 || index >= current.length) {
        throw StateError(
          'Invalid list index $segment in JSON pointer $pointer',
        );
      }
      current = current[index];
    } else {
      throw StateError('Cannot traverse JSON pointer $pointer at $segment');
    }
  }
  return current;
}

Object? _withoutJsonPointer(Object? root, String pointer) {
  final segments = pointer
      .split('/')
      .skip(1)
      .map((segment) => segment.replaceAll('~1', '/').replaceAll('~0', '~'))
      .toList();
  if (segments.isEmpty) {
    return root;
  }
  final segment = segments.first;
  final remainder = '/${segments.skip(1).join('/')}';
  if (root is Map<Object?, Object?>) {
    final copy = <Object?, Object?>{...root};
    if (segments.length == 1) {
      copy.remove(segment);
    } else if (copy.containsKey(segment)) {
      copy[segment] = _withoutJsonPointer(copy[segment], remainder);
    }
    return copy;
  }
  if (root is List<Object?>) {
    final index = int.tryParse(segment);
    if (index == null || index < 0 || index >= root.length) {
      return root;
    }
    final copy = [...root];
    copy[index] = segments.length == 1
        ? null
        : _withoutJsonPointer(copy[index], remainder);
    return copy;
  }
  return root;
}

Object? _projectExpected(String rowId, Object? expected) {
  final policy =
      (_parityManifest['evidence_policy'] as Map).cast<String, dynamic>();
  final projections =
      (policy['typed_field_projections'] as Map?)?.cast<String, dynamic>();
  final paths = (projections?[rowId] as List?)?.cast<String>() ?? const [];
  var projected = expected;
  for (final path in paths) {
    projected = _withoutJsonPointer(projected, path);
  }
  return projected;
}

Map<String, dynamic> _caseById(String caseId) {
  for (final rawCase in (_goFixtures['cases'] as List)) {
    final candidate = _asMap(rawCase, 'Parity case');
    if (candidate['id'] == caseId) {
      return candidate;
    }
  }
  throw StateError('Canonical parity case not found: $caseId');
}

Iterable<Map<String, dynamic>> _implementedRows() sync* {
  for (final rawModel in (_parityManifest['models'] as List)) {
    final model = _asMap(rawModel, 'Manifest model');
    for (final rawField in (model['fields'] as List)) {
      final field = _asMap(rawField, 'Manifest field');
      if (field['dart_status'] == 'implemented') {
        yield {
          ...field,
          'model': model['symbol'],
          'kind': model['kind'],
        };
      }
    }
  }
}

Map<String, dynamic> _canonicalEvidence() {
  final rows = _implementedRows().toList();
  final rowsById = {
    for (final row in rows) row['id'] as String: row,
  };
  final aliases = (_parityManifest['canonical_evidence_aliases'] as Map)
      .cast<String, dynamic>();
  final evidence = <String, dynamic>{};

  for (final row in rows) {
    final rowId = row['id'] as String;
    var source = row;
    final visited = <String>{rowId};
    while (source['canonical_case'] == null) {
      final target = aliases[source['id']];
      if (target is! String || !visited.add(target)) {
        throw StateError('Invalid canonical evidence alias graph at $rowId');
      }
      source = rowsById[target]!;
    }
    final canonicalCase = _asMap(source['canonical_case'], rowId);
    final kind = source['kind'] as String;
    final adapter = switch (kind) {
      'event' => 'event',
      'message' => 'message',
      'content' => 'content',
      'type' => 'type',
      _ => throw StateError('No evidence adapter for $kind ($rowId)'),
    };
    evidence['canonical.$rowId'] = <String, dynamic>{
      'testName': 'canonical parity rows survive decode -> encode',
      'fields': [rowId],
      'adapter': adapter,
      'model': source['model'],
      'field': source['name'],
      'caseId': canonicalCase['case_id'],
      'document': canonicalCase['document'],
      'path': canonicalCase['path'],
    };
  }
  return evidence;
}

Map<String, dynamic> _decodeEvidence(
  Map<String, dynamic> evidence,
  Map<String, dynamic> input,
) {
  final adapter = evidence['adapter'] as String;
  final model = evidence['model'] as String;
  switch (adapter) {
    case 'event':
      return BaseEvent.fromJson(input).toJson();
    case 'message':
      return Message.fromJson(input).toJson();
    case 'content':
      return InputContent.fromJson(input).toJson();
    case 'type':
      switch (model) {
        case 'Context':
          return Context.fromJson(input).toJson();
        case 'FunctionCall':
          return FunctionCall.fromJson(input).toJson();
        case 'RunAgentInput':
          return RunAgentInput.fromJson(input).toJson();
        case 'Tool':
          return Tool.fromJson(input).toJson();
        case 'ToolCall':
          return ToolCall.fromJson(input).toJson();
        default:
          throw StateError('No type adapter registered for $model');
      }
    default:
      throw StateError('No evidence adapter registered for $adapter');
  }
}

String _modelRelativePath(Map<String, dynamic> evidence) {
  final path = evidence['path'] as String;
  if (evidence['adapter'] == 'content' && path.startsWith('/content/')) {
    final segments = path.substring(1).split('/');
    return '/${segments.skip(2).join('/')}';
  }
  switch (evidence['model']) {
    case 'Context':
      return path.replaceFirst('/context/0', '');
    case 'FunctionCall':
      return path.replaceFirst('/toolCalls/0/function', '');
    case 'RunAgentInput':
      return path.replaceFirst('/input', '');
    case 'Tool':
      return path.replaceFirst('/tools/items/0', '');
    case 'ToolCall':
      return path.replaceFirst('/toolCalls/0', '');
    default:
      return path;
  }
}

Map<String, dynamic> _adapterInput(
  Map<String, dynamic> evidence,
  Map<String, dynamic> rootInput,
) {
  final model = evidence['model'] as String;
  final path = evidence['path'] as String;
  switch (model) {
    case 'Context':
      return _asMap(_atJsonPointer(rootInput, '/context/0'), model);
    case 'FunctionCall':
      return _asMap(
        _atJsonPointer(rootInput, '/toolCalls/0/function'),
        model,
      );
    case 'RunAgentInput':
      return _asMap(_atJsonPointer(rootInput, '/input'), model);
    case 'Tool':
      return _asMap(_atJsonPointer(rootInput, '/tools/items/0'), model);
    case 'ToolCall':
      return _asMap(_atJsonPointer(rootInput, '/toolCalls/0'), model);
  }
  if (evidence['adapter'] == 'content' && path.startsWith('/content/')) {
    final segments = path.substring(1).split('/');
    return _asMap(
      _atJsonPointer(rootInput, '/content/${segments[1]}'),
      model,
    );
  }
  return rootInput;
}

void main() {
  setUpAll(() async {
    final pins = (_parityManifest['pins'] as Map).cast<String, dynamic>();
    final peerContract =
        (_parityManifest['peer_contract'] as Map).cast<String, dynamic>();
    final implementationBase = pins['implementation_base'] as String;
    final casesSource = peerContract['cases_source'] as String;
    final snapshot = PinnedGitSnapshot(_repositoryRoot, implementationBase);
    _goFixtures = await snapshot.jsonMap(casesSource);
  });

  group('compatibility baseline', () {
    test('fixture records the pinned base and all current variants', () {
      expect(_fixture['version'], 1);
      expect(
        _fixture['baseCommit'],
        'aaa75b54d572be8cd1d51c72e951273c5b893ed0',
      );
      expect((_fixture['eventTypes'] as List).length, 34);
      expect((_fixture['messageRoles'] as List).length, 7);
      expect((_fixture['cases'] as List).length, 14);
    });

    test('public package import and version remain available', () {
      expect(agUiVersion, '0.3.0');
      expect(initAgUI, returnsNormally);
    });

    test('const construction and constructor tear-offs remain valid', () {
      const message = UserMessage.fromContent(
        id: 'u1',
        messageContent: TextContent('hello'),
      );
      expect(message.content, 'hello');

      const UserMessage Function({required String id, required String content})
          userConstructor = UserMessage.new;
      final constructed = userConstructor(id: 'u2', content: 'tear-off');
      expect(constructed.content, 'tear-off');
      expect(message.copyWith().content, 'hello');
    });

    test('nullable Message.id typing and sentinel clearing are preserved', () {
      const message = UserMessage.fromContent(
        id: 'u1',
        messageContent: TextContent('hello'),
        name: 'named',
        encryptedValue: 'cipher',
      );
      final id = _messageId(message);
      expect(id, 'u1');
      expect(message.copyWith().name, 'named');
      expect(message.copyWith(name: null).name, isNull);
      expect(message.copyWith(encryptedValue: null).encryptedValue, isNull);
    });

    test('roles, defaults, and unknown fields follow the base contract', () {
      expect(
        MessageRole.values.map((role) => role.value),
        (_fixture['messageRoles'] as List).cast<String>(),
      );
      expect(
        const TextMessageStartEvent(messageId: 'm').role,
        TextMessageRole.assistant,
      );
      expect(
        const ReasoningMessageStartEvent(messageId: 'm').role,
        ReasoningMessageRole.reasoning,
      );

      final event = TextMessageStartEvent.fromJson({
        'type': 'TEXT_MESSAGE_START',
        'messageId': 'm',
        'futureField': {'kept': true},
      });
      expect(event.role, TextMessageRole.assistant);
      expect(event.toJson().containsKey('futureField'), isFalse);
    });

    test('arbitrary JSON and explicit nulls retain established behavior', () {
      final state = {
        'nested': [
          1,
          true,
          null,
          {'value': 'x'},
        ],
      };
      final snapshot = StateSnapshotEvent(snapshot: state);
      expect(StateSnapshotEvent.fromJson(snapshot.toJson()).snapshot, state);

      const raw = RawEvent(event: null);
      expect(raw.toJson().containsKey('event'), isTrue);
      expect(raw.toJson()['event'], isNull);

      final finished = RunFinishedEvent.fromJson({
        'type': 'RUN_FINISHED',
        'threadId': 't',
        'runId': 'r',
        'result': null,
      });
      expect(finished.result, isNull);
      expect(finished.toJson().containsKey('result'), isFalse);
    });

    test('cipher-bearing snapshots scrub rawEvent before re-emission', () {
      final cipher = {
        'type': 'MESSAGES_SNAPSHOT',
        'messages': [
          {
            'id': 'm',
            'role': 'assistant',
            'content': 'visible',
            'encryptedValue': 'secret',
          },
        ],
        'rawEvent': {'encryptedValue': 'secret'},
      };
      final snapshot = MessagesSnapshotEvent.fromJson(cipher);
      expect(snapshot.rawEvent, isNull);
      expect(snapshot.toJson().containsKey('rawEvent'), isFalse);
      expect(
        (snapshot.messages.single as AssistantMessage).encryptedValue,
        'secret',
      );

      final started = RunStartedEvent.fromJson({
        'type': 'RUN_STARTED',
        'threadId': 't',
        'runId': 'r',
        'input': {
          'threadId': 't',
          'runId': 'r',
          'messages': [
            {
              'id': 'm',
              'role': 'assistant',
              'content': 'visible',
              'encryptedValue': 'secret',
            },
          ],
          'tools': <Map<String, dynamic>>[],
          'context': <Map<String, dynamic>>[],
        },
        'rawEvent': {'encryptedValue': 'secret'},
      });
      expect(started.rawEvent, isNull);

      final nestedToolCipher = {
        'id': 'm',
        'role': 'assistant',
        'content': 'visible',
        'toolCalls': [
          {
            'id': 'call',
            'type': 'function',
            'function': {'name': 'search', 'arguments': '{}'},
            'encryptedValue': 'nested-secret',
          },
        ],
      };
      final nestedSnapshot = MessagesSnapshotEvent.fromJson({
        'type': 'MESSAGES_SNAPSHOT',
        'messages': [nestedToolCipher],
        'rawEvent': {'proxy': 'snapshot'},
      });
      expect(nestedSnapshot.rawEvent, isNull);
      expect(nestedSnapshot.toJson().containsKey('rawEvent'), isFalse);
      expect(
        nestedSnapshot.copyWith(rawEvent: {'proxy': 'reattached'}).rawEvent,
        isNull,
      );

      final nestedStarted = RunStartedEvent.fromJson({
        'type': 'RUN_STARTED',
        'threadId': 't',
        'runId': 'r',
        'input': {
          'threadId': 't',
          'runId': 'r',
          'messages': [nestedToolCipher],
          'tools': <Map<String, dynamic>>[],
          'context': <Map<String, dynamic>>[],
        },
        'rawEvent': {'proxy': 'run'},
      });
      expect(nestedStarted.rawEvent, isNull);
      expect(nestedStarted.toJson().containsKey('rawEvent'), isFalse);
      expect(
        nestedStarted.copyWith(rawEvent: {'proxy': 'reattached'}).rawEvent,
        isNull,
      );
    });

    test('Tool.metadata null is absent and explicitly clearable', () {
      final fromNull = Tool.fromJson({
        'name': 'search',
        'description': 'Search',
        'metadata': null,
      });
      expect(fromNull.metadata, isNull);
      expect(fromNull.toJson().containsKey('metadata'), isFalse);

      final withMetadata = fromNull.copyWith(metadata: {'source': 'test'});
      expect(withMetadata.metadata, {'source': 'test'});
      expect(withMetadata.copyWith().metadata, {'source': 'test'});
      expect(withMetadata.copyWith(metadata: null).metadata, isNull);
    });

    test('SimpleRunAgentInput keeps defaults and map assertions contract', () {
      const input = SimpleRunAgentInput();
      expect(input.toJson(), {
        'state': <String, dynamic>{},
        'messages': <Map<String, dynamic>>[],
        'tools': <Map<String, dynamic>>[],
        'context': <Map<String, dynamic>>[],
        'forwardedProps': <String, dynamic>{},
      });

      const populated = SimpleRunAgentInput(
        threadId: 't',
        runId: 'r',
        parentRunId: 'p',
        state: {'count': 1},
        forwardedProps: {'trace': true},
        config: {'model': 'test'},
        metadata: {'source': 'compatibility'},
      );
      expect(populated.toJson()['state'], {'count': 1});
      expect(populated.toJson()['forwardedProps'], {'trace': true});
      expect(populated.toJson()['config'], {'model': 'test'});
      expect(populated.toJson()['metadata'], {'source': 'compatibility'});

      expect(
        () => const SimpleRunAgentInput(state: <dynamic>[]).toJson(),
        throwsA(isA<AssertionError>()),
      );
      expect(
        () => const SimpleRunAgentInput(forwardedProps: <dynamic>[]).toJson(),
        throwsA(isA<AssertionError>()),
      );
    });

    test('RunAgentInput preserves JSON, aliases, defaults, and clearing', () {
      const input = RunAgentInput(
        threadId: 't',
        runId: 'r',
        state: {'count': 1},
        messages: <Message>[],
        tools: <Tool>[],
        context: <Context>[],
        forwardedProps: {'trace': true},
      );
      expect(
        RunAgentInput.fromJson({
          'thread_id': 't',
          'run_id': 'r',
          'state': {'count': 1},
          'messages': <Map<String, dynamic>>[],
          'tools': <Map<String, dynamic>>[],
          'context': <Map<String, dynamic>>[],
          'forwarded_props': {'trace': true},
          'unknown': 'ignored',
        }).toJson(),
        input.toJson(),
      );
      expect(input.copyWith(parentRunId: null).parentRunId, isNull);
      expect(input.copyWith(state: null).state, isNull);
      expect(input.copyWith(forwardedProps: null).forwardedProps, isNull);
      expect(input.copyWith().forwardedProps, {'trace': true});
    });

    test('canonical evidence executes every implemented manifest field', () {
      final registry = _canonicalEvidence();
      final implementedIds =
          _implementedRows().map((row) => row['id'] as String).toSet();
      final evidencePolicy =
          (_parityManifest['evidence_policy'] as Map).cast<String, dynamic>();
      final omissionRows = (evidencePolicy['intentional_omissions'] as Map)
          .cast<String, dynamic>();
      final expectedKeys = implementedIds.map((id) => 'canonical.$id').toSet();

      expect(registry.keys.toSet(), expectedKeys);

      final covered = <String>{};
      for (final entry in registry.entries) {
        final record = _asMap(entry.value, entry.key);
        final fields = (record['fields'] as List).cast<String>();
        expect(fields, hasLength(1), reason: entry.key);
        final rowId = fields.single;
        expect(implementedIds, contains(rowId), reason: entry.key);
        expect(entry.key, 'canonical.$rowId');
        covered.add(rowId);

        final source = record;
        final caseId = source['caseId'] as String?;
        expect(caseId, isNotNull, reason: rowId);
        final parityCase = _caseById(caseId!);
        final rootInput = _asMap(parityCase['input'], '$caseId input');
        final adapterInput = _adapterInput(source, rootInput);
        final encoded = _decodeEvidence(source, adapterInput);
        final encodedPath = _modelRelativePath(source);
        final expectedDocument = _asMap(
          parityCase[source['document']],
          '$caseId ${source['document']}',
        );
        final expectedValue = _atJsonPointer(
          expectedDocument,
          source['path'] as String,
        );

        if (omissionRows.containsKey(rowId)) {
          expect(
            () => _atJsonPointer(encoded, encodedPath),
            throwsA(isA<StateError>()),
            reason: rowId,
          );
        } else {
          expect(
            _atJsonPointer(encoded, encodedPath),
            _projectExpected(rowId, expectedValue),
            reason: rowId,
          );
        }
      }
      expect(covered, implementedIds);
    });

    test('exhaustive switches cover every current event type and subtype', () {
      final expectedTypes = (_fixture['eventTypes'] as List)
          .map((item) => (item as Map<String, dynamic>)['wire'] as String)
          .toList();
      final expectedDartNames = (_fixture['eventTypes'] as List)
          .map((item) => (item as Map<String, dynamic>)['dart'] as String)
          .toList();
      expect(
        EventType.values.map((type) => type.value).toList(),
        expectedTypes,
      );
      expect(
        EventType.values.map(_eventTypeName).toList(),
        expectedDartNames,
      );

      final events = <BaseEvent>[
        const TextMessageStartEvent(messageId: 'm'),
        const TextMessageContentEvent(messageId: 'm', delta: ''),
        const TextMessageEndEvent(messageId: 'm'),
        const TextMessageChunkEvent(),
        const ThinkingStartEvent(),
        const ThinkingContentEvent(delta: ''),
        const ThinkingEndEvent(),
        const ThinkingTextMessageStartEvent(),
        const ThinkingTextMessageContentEvent(delta: ''),
        const ThinkingTextMessageEndEvent(),
        const ToolCallStartEvent(toolCallId: 'c', toolCallName: 'search'),
        const ToolCallArgsEvent(toolCallId: 'c', delta: ''),
        const ToolCallEndEvent(toolCallId: 'c'),
        const ToolCallChunkEvent(),
        const ToolCallResultEvent(
          messageId: 'm',
          toolCallId: 'c',
          content: '',
        ),
        const StateSnapshotEvent(snapshot: <String, dynamic>{}),
        const StateDeltaEvent(delta: <Map<String, dynamic>>[]),
        MessagesSnapshotEvent(messages: <Message>[]),
        const ActivitySnapshotEvent(
          messageId: 'm',
          activityType: 'task',
          content: <String, dynamic>{},
        ),
        const ActivityDeltaEvent(
          messageId: 'm',
          activityType: 'task',
          patch: <Map<String, dynamic>>[],
        ),
        const RawEvent(event: null),
        const CustomEvent(name: 'custom', value: null),
        RunStartedEvent(threadId: 't', runId: 'r'),
        const RunFinishedEvent(threadId: 't', runId: 'r'),
        const RunErrorEvent(message: 'error'),
        const StepStartedEvent(stepName: 'step'),
        const StepFinishedEvent(stepName: 'step'),
        const ReasoningStartEvent(messageId: 'm'),
        const ReasoningMessageStartEvent(messageId: 'm'),
        const ReasoningMessageContentEvent(messageId: 'm', delta: ''),
        const ReasoningMessageEndEvent(messageId: 'm'),
        const ReasoningMessageChunkEvent(),
        const ReasoningEndEvent(messageId: 'm'),
        const ReasoningEncryptedValueEvent(
          subtype: ReasoningEncryptedValueSubtype.toolCall,
          entityId: 'm',
          encryptedValue: 'secret',
        ),
      ];
      const expectedBaseEventNames = [
        'TextMessageStartEvent',
        'TextMessageContentEvent',
        'TextMessageEndEvent',
        'TextMessageChunkEvent',
        'ThinkingStartEvent',
        'ThinkingContentEvent',
        'ThinkingEndEvent',
        'ThinkingTextMessageStartEvent',
        'ThinkingTextMessageContentEvent',
        'ThinkingTextMessageEndEvent',
        'ToolCallStartEvent',
        'ToolCallArgsEvent',
        'ToolCallEndEvent',
        'ToolCallChunkEvent',
        'ToolCallResultEvent',
        'StateSnapshotEvent',
        'StateDeltaEvent',
        'MessagesSnapshotEvent',
        'ActivitySnapshotEvent',
        'ActivityDeltaEvent',
        'RawEvent',
        'CustomEvent',
        'RunStartedEvent',
        'RunFinishedEvent',
        'RunErrorEvent',
        'StepStartedEvent',
        'StepFinishedEvent',
        'ReasoningStartEvent',
        'ReasoningMessageStartEvent',
        'ReasoningMessageContentEvent',
        'ReasoningMessageEndEvent',
        'ReasoningMessageChunkEvent',
        'ReasoningEndEvent',
        'ReasoningEncryptedValueEvent',
      ];
      expect(events.map(_baseEventName).toList(), expectedBaseEventNames);
      expect(EventType.values.map(_eventTypeName).length, 34);
    });
  });
}
