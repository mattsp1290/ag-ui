import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:ag_ui_example/models/chat_message.dart';
import 'package:ag_ui_example/widgets/chat_message_widget.dart';

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
}
