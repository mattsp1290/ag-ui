import 'dart:convert';

import 'package:ag_ui/ag_ui.dart';
import 'package:ag_ui_example/services/ag_ui_service.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

import 'go_server_container.dart';

/// Exercise the normal executable too: the contract test binary alone cannot
/// prove its image configuration, provider construction or shutdown path.
Future<void> verifyProductionImages(GoServerContainer fixture) async {
  await fixture.build(GoImage.production);
  final missingKey = await fixture.start(
    kind: GoImage.production,
    readinessPath: null,
  );
  expect(await fixture.wait(missingKey), isNot(0));
  expect(await fixture.logs(missingKey), contains('set OPENAI_API_KEY'));
  await fixture.stop(missingKey);

  final server = await fixture.start(
    kind: GoImage.production,
    sentinelKey: true,
  );
  final uri = await fixture.baseUrl(server);
  final transport = http.Client();
  final service = AgUiService(baseUrl: uri.toString());
  try {
    final health = await transport
        .get(uri.resolve('/'))
        .timeout(const Duration(seconds: 10));
    expect(health.statusCode, 200);
    final metadata = jsonDecode(health.body) as Map<String, dynamic>;
    expect(metadata['workspace'], '/tmp/empty-workspace');
    expect(metadata['routes'], contains('/reasoning'));
    final events = await service
        .run(
          'reasoning',
          threadId: 'production-${fixture.id}',
          messages: [
            UserMessage(id: 'production-user', content: 'scripted reasoning'),
          ],
        )
        .toList()
        .timeout(const Duration(seconds: 15));
    expect(events.whereType<RunErrorEvent>(), isEmpty);
    expect(events.whereType<RunFinishedEvent>(), hasLength(1));
    expect(events.whereType<ReasoningStartEvent>(), hasLength(1));
    expect(events.whereType<ReasoningEndEvent>(), hasLength(1));
    final snapshot = events.whereType<MessagesSnapshotEvent>().single.messages;
    final reasoning = snapshot.whereType<ReasoningMessage>().single;
    final answer = snapshot.whereType<AssistantMessage>().single;
    expect(
      reasoning.id,
      events.whereType<ReasoningMessageStartEvent>().single.messageId,
    );
    expect(
      answer.id,
      events.whereType<TextMessageStartEvent>().single.messageId,
    );
    expect(await fixture.logs(server), contains('0.0.0.0:8080'));
    // Send SIGTERM through the public Docker client and observe the exit before
    // removal. No model-backed request is ever sent using the sentinel key.
    await server.dockerClient
        .stopContainer(server.name!, timeout: 10)
        .timeout(const Duration(seconds: 15));
    expect(
      await fixture.wait(server),
      0,
      reason: 'normal server must exit gracefully',
    );
  } finally {
    transport.close();
    await service.close();
    await fixture.stop(server);
  }
}
