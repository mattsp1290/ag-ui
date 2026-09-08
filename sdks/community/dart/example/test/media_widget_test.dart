import 'dart:async';
import 'dart:typed_data';

import 'package:ag_ui/ag_ui.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:ag_ui_example/models/chat_message.dart';
import 'package:ag_ui_example/models/endpoint_config.dart';
import 'package:ag_ui_example/pages/multimodal_chat_page.dart';
import 'package:ag_ui_example/services/ag_ui_service.dart';
import 'package:ag_ui_example/widgets/chat_message_widget.dart';

class _PageService extends AgUiService {
  final StreamController<BaseEvent> response = StreamController<BaseEvent>();
  final requests = <(String, List<InputContent>)>[];
  int closeCalls = 0;

  @override
  Stream<BaseEvent> sendMultimodalMessage(
    String endpoint,
    List<InputContent> parts,
  ) {
    requests.add((endpoint, List<InputContent>.of(parts)));
    return response.stream;
  }

  @override
  Future<void> close() async {
    closeCalls++;
  }
}

void main() {
  final now = DateTime(2026, 1, 1);

  Widget host(ChatMessage message) {
    return MaterialApp(
      home: Scaffold(
        body: SingleChildScrollView(child: ChatMessageWidget(message: message)),
      ),
    );
  }

  testWidgets(
    'malformed generated image displays a fallback without throwing',
    (tester) async {
      await tester.pumpWidget(
        host(
          ChatMessage(
            id: 'generated',
            type: ChatMessageType.image,
            content: 'data:image/png;base64,this-is-not-base64',
            timestamp: now,
          ),
        ),
      );

      expect(tester.takeException(), isNull);
      expect(find.text('Image could not be loaded'), findsOneWidget);
    },
  );

  for (final testCase in const [
    ('empty image data URL', 'data:image/png;base64,'),
    ('non-image data URL', 'data:text/plain;base64,AQIDBA=='),
    ('non-data URL', 'https://example.test/image.png'),
  ]) {
    testWidgets('${testCase.$1} displays a fallback', (tester) async {
      await tester.pumpWidget(
        host(
          ChatMessage(
            id: testCase.$1,
            type: ChatMessageType.image,
            content: testCase.$2,
            timestamp: now,
          ),
        ),
      );

      expect(tester.takeException(), isNull);
      expect(find.text('Image could not be loaded'), findsOneWidget);
    });
  }

  testWidgets('generated image codec failure displays a fallback', (
    tester,
  ) async {
    await tester.pumpWidget(
      host(
        ChatMessage(
          id: 'codec-invalid',
          type: ChatMessageType.image,
          content: 'data:image/png;base64,AQIDBA==',
          timestamp: now,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('Image could not be loaded'), findsOneWidget);
  });

  testWidgets('invalid selected image bytes display a visible fallback', (
    tester,
  ) async {
    await tester.pumpWidget(
      host(
        ChatMessage(
          id: 'attachment',
          type: ChatMessageType.imageAttachment,
          content: 'photo.png',
          imageBytes: Uint8List.fromList(<int>[1, 2, 3, 4]),
          timestamp: now,
        ),
      ),
    );
    await tester.pump();

    expect(tester.takeException(), isNull);
    expect(find.text('Preview could not be loaded'), findsOneWidget);
  });

  testWidgets('attachment names remain bounded on a narrow viewport', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(220, 240));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.pumpWidget(
      host(
        ChatMessage(
          id: 'document',
          type: ChatMessageType.documentAttachment,
          content: 'very-long-document-name-that-must-not-overflow.pdf',
          fileName: 'very-long-document-name-that-must-not-overflow.pdf',
          timestamp: now,
        ),
      ),
    );

    expect(tester.takeException(), isNull);
    expect(find.textContaining('very-long-document-name'), findsOneWidget);
  });

  testWidgets('injected picker and service follow page ownership', (
    tester,
  ) async {
    final endpoint = EndpointConfig.availableEndpoints.singleWhere(
      (item) => item.path == 'vision',
    );
    final service = _PageService();
    Future<PlatformFile> picker(List<String> extensions) async {
      expect(extensions, endpoint.allowedExtensions);
      return PlatformFile(
        name: 'picked.png',
        size: 4,
        bytes: Uint8List.fromList([1, 2, 3, 4]),
      );
    }

    await tester.pumpWidget(
      MaterialApp(
        home: MultimodalChatPage(
          endpoint: endpoint,
          service: service,
          filePicker: picker,
        ),
      ),
    );

    await tester.tap(find.byTooltip('Attach file'));
    await tester.pumpAndSettle();
    expect(find.text('picked.png'), findsOneWidget);
    expect(
      tester
          .widget<IconButton>(find.widgetWithIcon(IconButton, Icons.close))
          .onPressed,
      isNotNull,
    );

    await tester.tap(find.byTooltip('Remove'));
    await tester.pump();
    expect(find.text('picked.png'), findsNothing);
    expect(
      tester
          .widget<IconButton>(find.widgetWithIcon(IconButton, Icons.send))
          .onPressed,
      isNull,
    );

    await tester.tap(find.byTooltip('Attach file'));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Send'));
    await tester.pump();
    expect(service.requests, hasLength(1));
    expect(service.requests.single.$1, 'vision');
    expect(
      tester
          .widget<IconButton>(
            find.widgetWithIcon(IconButton, Icons.attach_file),
          )
          .onPressed,
      isNull,
    );
    expect(
      tester
          .widget<IconButton>(find.widgetWithIcon(IconButton, Icons.send))
          .onPressed,
      isNull,
    );

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
    expect(service.closeCalls, 1);
    await service.response.close();
  });
}
