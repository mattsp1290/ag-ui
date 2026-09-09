import 'package:ag_ui/ag_ui.dart';
import 'package:ag_ui/src/sse/sse_parser.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:test/test.dart';

void main() {
  group('SseClient Basic Tests', () {
    test('finite exact caps validate synchronously in both constructors', () {
      for (final invalid in [0, -1, 9007199254740992]) {
        expect(() => SseClient(maxDataCodeUnits: invalid), throwsArgumentError);
        expect(() => SseClient(maxLineCodeUnits: invalid), throwsArgumentError);
        expect(() => SseParser(maxDataCodeUnits: invalid), throwsArgumentError);
        expect(() => SseParser(maxLineCodeUnits: invalid), throwsArgumentError);
      }
      const largest = 9007199254740991;
      for (final data in [largest - 6, largest]) {
        expect(() => SseClient(maxDataCodeUnits: data), throwsArgumentError);
        expect(() => SseParser(maxDataCodeUnits: data), throwsArgumentError);
      }
      expect(
        SseClient(maxDataCodeUnits: largest - 7).maxLineCodeUnits,
        largest,
      );
      expect(
        SseParser(maxDataCodeUnits: largest, maxLineCodeUnits: 1)
            .maxLineCodeUnits,
        1,
      );
      expect(SseClient(maxDataCodeUnits: 20).maxLineCodeUnits, 27);
      expect(SseParser(maxDataCodeUnits: 20).maxLineCodeUnits, 27);
      expect(
        SseClient(maxDataCodeUnits: 20, maxLineCodeUnits: 2).maxLineCodeUnits,
        2,
      );
    });
    test('constructor initializes with default parameters', () {
      final client = SseClient();
      expect(client.isConnected, isFalse);
      expect(client.lastEventId, isNull);
    });

    test('constructor accepts custom parameters', () {
      final customHttpClient = MockClient((request) async {
        return http.Response('', 200);
      });
      final customTimeout = Duration(seconds: 30);
      final customBackoff = ExponentialBackoff();

      final client = SseClient(
        httpClient: customHttpClient,
        idleTimeout: customTimeout,
        backoffStrategy: customBackoff,
      );

      expect(client.isConnected, isFalse);
    });

    test('close is idempotent', () async {
      final client = SseClient();

      // Multiple closes should not throw
      await client.close();
      await client.close();
      await client.close();

      expect(client.isConnected, isFalse);
    });

    test('isConnected returns false when not connected', () {
      final client = SseClient();
      expect(client.isConnected, isFalse);
    });

    test('lastEventId is initially null', () {
      final client = SseClient();
      expect(client.lastEventId, isNull);
    });

    test('different backoff strategies can be used', () {
      // Test with ExponentialBackoff
      var client = SseClient(
        backoffStrategy: ExponentialBackoff(
          initialDelay: Duration(milliseconds: 100),
          maxDelay: Duration(seconds: 10),
        ),
      );
      expect(client.isConnected, isFalse);

      // Test with ConstantBackoff
      client = SseClient(
        backoffStrategy: ConstantBackoff(Duration(seconds: 1)),
      );
      expect(client.isConnected, isFalse);

      // Test with LegacyBackoffStrategy
      client = SseClient(
        backoffStrategy: LegacyBackoffStrategy(),
      );
      expect(client.isConnected, isFalse);
    });
  });
}
