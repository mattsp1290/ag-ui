import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:ag_ui/ag_ui.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:ag_ui_example/models/endpoint_config.dart';
import 'package:ag_ui_example/models/chat_message.dart';
import 'package:ag_ui_example/pages/chat_page.dart';
import 'package:ag_ui_example/services/ag_ui_service.dart';

class _CapturingService extends AgUiService {
  final List<Stream<BaseEvent>> responses;
  final List<(String, List<InputContent>)> requests = [];
  _CapturingService(this.responses);

  @override
  Stream<BaseEvent> sendMultimodalMessage(
    String endpoint,
    List<InputContent> parts,
  ) {
    requests.add((endpoint, List<InputContent>.unmodifiable(parts)));
    return responses.removeAt(0);
  }
}

class _RunService extends AgUiService {
  final Stream<BaseEvent> response;

  _RunService(this.response);

  @override
  Stream<BaseEvent> run(
    String endpoint, {
    required String threadId,
    required List<Message> messages,
    List<Tool> tools = const [],
    dynamic state,
    Map<String, String> extraQuery = const {},
  }) => response;
}

Stream<BaseEvent> _success(String id, String text) => Stream.fromIterable([
  TextMessageStartEvent(messageId: id),
  TextMessageContentEvent(messageId: id, delta: text),
  TextMessageEndEvent(messageId: id),
  const RunFinishedEvent(threadId: 'thread', runId: 'run'),
]);

void main() {
  testWidgets('picker cancellation and removal leave no selected file', (
    tester,
  ) async {
    final endpoint = EndpointConfig.availableEndpoints.firstWhere(
      (item) => item.path == 'vision',
    );
    final state = MultimodalChatPageState(
      endpoint: endpoint,
      service: AgUiService(),
      filePicker: (_) async => null,
    );
    addTearDown(state.dispose);
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: Text('host'))),
    );
    final context = tester.element(find.byType(Scaffold));
    await state.pickFile(['png'], context);
    expect(state.hasFile, isFalse);

    final selected = MultimodalChatPageState(
      endpoint: endpoint,
      service: AgUiService(),
      filePicker: (_) async => PlatformFile(
        name: 'tiny.png',
        size: 2,
        bytes: Uint8List.fromList([1, 2]),
      ),
    );
    addTearDown(selected.dispose);
    await selected.pickFile(['png'], context);
    expect(selected.hasFile, isTrue);
    selected.clearPicked();
    expect(selected.hasFile, isFalse);
  });

  testWidgets('picker enforces the five MiB limit and recovers picker errors', (
    tester,
  ) async {
    final endpoint = EndpointConfig.availableEndpoints.firstWhere(
      (item) => item.path == 'document',
    );
    final tooLarge = MultimodalChatPageState(
      endpoint: endpoint,
      service: AgUiService(),
      filePicker: (_) async => PlatformFile(
        name: 'large.pdf',
        size: 5 * 1024 * 1024 + 1,
        bytes: Uint8List(5 * 1024 * 1024 + 1),
      ),
    );
    addTearDown(tooLarge.dispose);
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: Text('host'))),
    );
    final context = tester.element(find.byType(Scaffold));
    await tooLarge.pickFile(['pdf'], context);
    await tester.pump();
    expect(tooLarge.hasFile, isFalse);
    expect(find.textContaining('under 5 MB'), findsOneWidget);

    final failed = MultimodalChatPageState(
      endpoint: endpoint,
      service: AgUiService(),
      filePicker: (_) async => throw StateError('picker unavailable'),
    );
    addTearDown(failed.dispose);
    await failed.pickFile(['pdf'], context);
    expect(failed.messages.single.content, contains('Could not pick file'));
  });

  testWidgets('disposing while picker is pending ignores its completion', (
    tester,
  ) async {
    final pending = Completer<PlatformFile?>();
    final state = MultimodalChatPageState(
      endpoint: EndpointConfig.availableEndpoints.firstWhere(
        (item) => item.path == 'vision',
      ),
      service: AgUiService(),
      filePicker: (_) => pending.future,
    );
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: Text('host'))),
    );
    final pick = state.pickFile(['png'], tester.element(find.byType(Scaffold)));
    state.dispose();
    pending.complete(
      PlatformFile(name: 'late.png', size: 1, bytes: Uint8List.fromList([1])),
    );
    await pick;
    expect(state.hasFile, isFalse);
  });

  testWidgets('only the newest overlapping picker may update selection', (
    tester,
  ) async {
    final picks = <Completer<PlatformFile?>>[
      Completer<PlatformFile?>(),
      Completer<PlatformFile?>(),
    ];
    var index = 0;
    final state = MultimodalChatPageState(
      endpoint: EndpointConfig.availableEndpoints.firstWhere(
        (item) => item.path == 'vision',
      ),
      service: AgUiService(),
      filePicker: (_) => picks[index++].future,
    );
    addTearDown(state.dispose);
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: Text('host'))),
    );
    final context = tester.element(find.byType(Scaffold));

    final older = state.pickFile(['png'], context);
    final newer = state.pickFile(['png'], context);
    picks[1].complete(
      PlatformFile(name: 'newer.png', size: 1, bytes: Uint8List.fromList([2])),
    );
    await newer;
    picks[0].complete(
      PlatformFile(name: 'older.png', size: 1, bytes: Uint8List.fromList([1])),
    );
    await older;

    expect(state.pickedFileName, 'newer.png');
    expect(state.pickedBytes, [2]);
  });

  testWidgets('clear invalidates a pending picker and its stale error', (
    tester,
  ) async {
    final result = Completer<PlatformFile?>();
    final error = Completer<PlatformFile?>();
    var call = 0;
    final state = MultimodalChatPageState(
      endpoint: EndpointConfig.availableEndpoints.firstWhere(
        (item) => item.path == 'vision',
      ),
      service: AgUiService(),
      filePicker: (_) => call++ == 0 ? result.future : error.future,
    );
    addTearDown(state.dispose);
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: Text('host'))),
    );
    final context = tester.element(find.byType(Scaffold));

    final pendingResult = state.pickFile(['png'], context);
    state.clearPicked();
    result.complete(
      PlatformFile(name: 'late.png', size: 1, bytes: Uint8List.fromList([1])),
    );
    await pendingResult;
    expect(state.hasFile, isFalse);

    final messageCount = state.messages.length;
    final pendingError = state.pickFile(['png'], context);
    state.clearPicked();
    error.completeError(StateError('stale picker failure'));
    await pendingError;
    expect(state.messages, hasLength(messageCount));
  });

  for (final invalidValue in <Object?>[
    null,
    'https://example.com/image.png',
    'data:text/plain;base64,SGVsbG8=',
    'data:image/png;base64,',
    'data:image/png;base64,***',
    <String, Object?>{'url': 42},
    'unexpected payload',
  ]) {
    testWidgets('invalid generated image payload $invalidValue is visible', (
      tester,
    ) async {
      final state = ChatPageState(
        endpoint: EndpointConfig.availableEndpoints.first,
        service: _RunService(
          Stream.fromIterable([
            CustomEvent(name: 'image_generated', value: invalidValue),
            const RunFinishedEvent(threadId: 'thread', runId: 'run'),
          ]),
        ),
      );
      addTearDown(state.dispose);

      state.sendMessage('generate an image');
      await tester.pump();
      await tester.pump();

      expect(
        state.messages.where(
          (message) => message.content == 'The image response was invalid.',
        ),
        hasLength(1),
      );
      expect(
        state.messages.where(
          (message) => message.type == ChatMessageType.image,
        ),
        isEmpty,
      );
    });
  }

  for (final testCase in [
    ('vision', 'tiny.png', 'png', 'image/png', ImageInputContent),
    ('audio', 'tiny.wav', 'wav', 'audio/wav', AudioInputContent),
    ('document', 'tiny.pdf', 'pdf', 'application/pdf', DocumentInputContent),
  ]) {
    testWidgets('${testCase.$1} sends typed inline data and renders success', (
      tester,
    ) async {
      final endpoint = EndpointConfig.availableEndpoints.firstWhere(
        (item) => item.path == testCase.$1,
      );
      final service = _CapturingService([
        _success('answer-${testCase.$1}', 'visible reply'),
      ]);
      final state = MultimodalChatPageState(
        endpoint: endpoint,
        service: service,
        filePicker: (_) async => PlatformFile(
          name: testCase.$2,
          size: 3,
          bytes: Uint8List.fromList([1, 2, 3]),
        ),
      );
      addTearDown(state.dispose);
      await tester.pumpWidget(
        const MaterialApp(home: Scaffold(body: Text('host'))),
      );
      await state.pickFile(
        endpoint.allowedExtensions,
        tester.element(find.byType(Scaffold)),
      );
      state.sendMultimodal('What is this?');
      await tester.pump();
      await tester.pump();

      final request = service.requests.single;
      expect(request.$1, testCase.$1);
      expect(request.$2.first.runtimeType, testCase.$5);
      final media = request.$2.first;
      final InputContentSource source = switch (media) {
        ImageInputContent() => media.source,
        AudioInputContent() => media.source,
        DocumentInputContent() => media.source,
        _ => throw StateError('unexpected media part'),
      };
      expect(source, isA<DataSource>());
      expect((source as DataSource).mimeType, testCase.$4);
      expect(base64Decode(source.value), [1, 2, 3]);
      if (testCase.$1 == 'audio') {
        expect(request.$2.whereType<TextInputContent>(), isEmpty);
      } else {
        expect(
          request.$2.whereType<TextInputContent>().single.text,
          'What is this?',
        );
      }
      expect(
        state.messages.any((message) => message.content == 'visible reply'),
        isTrue,
      );
    });
  }

  testWidgets(
    'a new multimodal run does not apply deltas to prior state rows',
    (tester) async {
      final endpoint = EndpointConfig.availableEndpoints.firstWhere(
        (item) => item.path == 'vision',
      );
      Future<PlatformFile> picker(List<String> _) async => PlatformFile(
        name: 'tiny.png',
        size: 1,
        bytes: Uint8List.fromList([1]),
      );
      final service = _CapturingService([
        Stream.fromIterable(const [
          StateSnapshotEvent(
            snapshot: {
              'steps': [
                {'description': 'Old step', 'status': 'pending'},
              ],
            },
          ),
          RunFinishedEvent(threadId: 'thread', runId: 'first'),
        ]),
        Stream.fromIterable(const [
          StateDeltaEvent(
            delta: [
              {
                'op': 'replace',
                'path': '/steps/0/status',
                'value': 'completed',
              },
            ],
          ),
          RunFinishedEvent(threadId: 'thread', runId: 'second'),
        ]),
      ]);
      final state = MultimodalChatPageState(
        endpoint: endpoint,
        service: service,
        filePicker: picker,
      );
      addTearDown(state.dispose);
      await tester.pumpWidget(
        const MaterialApp(home: Scaffold(body: Text('host'))),
      );
      final context = tester.element(find.byType(Scaffold));

      await state.pickFile(['png'], context);
      state.sendMultimodal('first');
      await tester.pump();
      await tester.pump();
      final stateRow = state.messages.singleWhere(
        (message) => message.content.contains('Old step'),
      );

      await state.pickFile(['png'], context);
      state.sendMultimodal('second');
      await tester.pump();
      await tester.pump();

      expect(
        state.messages
            .singleWhere((message) => message.id == stateRow.id)
            .content,
        stateRow.content,
      );
    },
  );

  testWidgets(
    'media RUN_ERROR ignores trailing events and next interruption recovers',
    (tester) async {
      final endpoint = EndpointConfig.availableEndpoints.firstWhere(
        (item) => item.path == 'vision',
      );
      Future<PlatformFile> picker(List<String> _) async => PlatformFile(
        name: 'tiny.png',
        size: 1,
        bytes: Uint8List.fromList([1]),
      );
      final service = _CapturingService([
        Stream.fromIterable(const [
          RunErrorEvent(message: 'media failed'),
          TextMessageStartEvent(messageId: 'late'),
          TextMessageContentEvent(messageId: 'late', delta: 'ignored'),
        ]),
        const Stream<BaseEvent>.empty(),
      ]);
      final state = MultimodalChatPageState(
        endpoint: endpoint,
        service: service,
        filePicker: picker,
      );
      addTearDown(state.dispose);
      await tester.pumpWidget(
        const MaterialApp(home: Scaffold(body: Text('host'))),
      );
      final context = tester.element(find.byType(Scaffold));
      await state.pickFile(['png'], context);
      state.sendMultimodal('first');
      await tester.pump();
      await tester.pump();
      expect(
        state.messages.any(
          (message) => message.content.contains('media failed'),
        ),
        isTrue,
      );
      expect(state.messages.any((message) => message.id == 'late'), isFalse);

      await state.pickFile(['png'], context);
      state.sendMultimodal('second');
      await tester.pump();
      await tester.pump();
      expect(state.messages.last.content, contains('before the run finished'));
      expect(state.isLoading, isFalse);
    },
  );
}
