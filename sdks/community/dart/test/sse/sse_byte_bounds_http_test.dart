import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:ag_ui/ag_ui.dart';
import 'package:http/http.dart' as http;
import 'package:test/test.dart';

class _ControlledClient extends http.BaseClient {
  // Closed by the owning test after cancellation.
  // ignore: close_sinks
  final source = StreamController<List<int>>();
  bool closed = false;
  int requests = 0;

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    requests++;
    return http.StreamedResponse(source.stream, 200);
  }

  @override
  void close() {
    closed = true;
  }
}

void main() {
  test(
      'open loopback overflow cancels parsing and borrowed HTTP client remains usable',
      () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final transport = http.Client();
    final release = Completer<void>();
    final handlers = <Future<void>>[];
    Future<void> handle(HttpRequest request) async {
      if (request.uri.path == '/overflow') {
        request.response.headers.contentType =
            ContentType('text', 'event-stream');
        request.response.bufferOutput = false;
        request.response.write(':${'x' * 64}');
        await request.response.flush();
        await release.future;
      } else {
        request.response.write('reused');
      }
      await request.response.close();
    }

    final serving = server.listen((request) {
      handlers.add(handle(request));
    });
    StreamSubscription<SseMessage>? sub;
    final client = SseClient(httpClient: transport, maxLineCodeUnits: 32);
    try {
      final base = 'http://${server.address.address}:${server.port}';
      final response = await transport
          .send(http.Request('GET', Uri.parse('$base/overflow')));
      final error = Completer<Object>();
      sub = client
          .parseStream(response.stream)
          .listen((_) => fail('partial message'), onError: error.complete);
      expect(
        await error.future.timeout(const Duration(seconds: 3)),
        isA<FormatException>(),
      );
      expect(release.isCompleted, isFalse);
      await sub.cancel();
      expect((await transport.get(Uri.parse('$base/reuse'))).body, 'reused');
      expect(client.lastEventId, isNull);
    } finally {
      await sub?.cancel();
      release.complete();
      await client.close();
      transport.close();
      await serving.cancel();
      await server.close(force: true);
      await Future.wait(handlers);
    }
  });

  test(
      'connect passes explicit line cap and close cancels response/reconnect timer',
      () async {
    final transport = _ControlledClient();
    final cancelled = Completer<void>();
    transport.source.onCancel = cancelled.complete;
    final client = SseClient(
      httpClient: transport,
      maxLineCodeUnits: 8,
      backoffStrategy: const ConstantBackoff(Duration(milliseconds: 200)),
    );
    final messages = <SseMessage>[];
    final sub = client
        .connect(Uri.parse('http://synthetic.invalid'))
        .listen(messages.add);
    try {
      transport.source.add(utf8.encode('data: ok\n\n:${'x' * 8}'));
      await cancelled.future.timeout(const Duration(seconds: 3));
      expect(messages.single.data, 'ok');
      await client.close();
      await Future<void>.delayed(const Duration(milliseconds: 250));
      expect(transport.requests, 1);
      expect(transport.closed, isFalse);
    } finally {
      await sub.cancel();
      await client.close();
      await transport.source.close();
      transport.close();
    }
  });
}
