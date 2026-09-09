import 'dart:async';
import 'dart:convert';

import 'package:ag_ui/ag_ui.dart';
import 'package:test/test.dart';

Future<void> _captureUncaught(Future<void> Function() body) async {
  final uncaught = <Object>[];
  final finished = Completer<void>();
  final guarded = runZonedGuarded(
    () async {
      try {
        await body();
        // Allow errors from automatic controller closure to reach the Zone.
        await Future<void>.delayed(const Duration(milliseconds: 20));
        finished.complete();
      } on Object catch (error, stack) {
        finished.completeError(error, stack);
      }
    },
    (error, _) => uncaught.add(error),
  );
  unawaited(guarded);
  await finished.future.timeout(const Duration(seconds: 3));
  expect(uncaught, isEmpty);
}

void cancellationFailureTests() {
  group('public cancellation failures', () {
    for (final asynchronous in [false, true]) {
      for (final fromSource in [false, true]) {
        test(
            'terminal error preserves primary error: async=$asynchronous source=$fromSource',
            () async {
          await _captureUncaught(() async {
            final cleanupError = StateError('synthetic cleanup failure');
            final sourceError = StateError('synthetic source failure');
            var cancellations = 0;
            final source = StreamController<List<int>>(
              onCancel: () {
                cancellations++;
                if (asynchronous) {
                  return Future<void>.error(cleanupError);
                }
                throw cleanupError;
              },
            );
            final messages = <SseMessage>[];
            final errors = <Object>[];
            final done = Completer<void>();
            final sub = SseClient(maxLineCodeUnits: 8)
                .parseStream(source.stream)
                .listen(
                  messages.add,
                  onError: errors.add,
                  onDone: done.complete,
                );
            try {
              if (fromSource) {
                source.addError(sourceError);
              } else {
                source.add(utf8.encode(':' * 9));
              }
              source.add(utf8.encode('\n\ndata: x\n\n'));
              await done.future.timeout(const Duration(seconds: 2));
              expect(errors, hasLength(1));
              expect(
                errors.single,
                fromSource ? same(sourceError) : isA<FormatException>(),
              );
              expect(messages, isEmpty);
              expect(cancellations, 1);
            } finally {
              await sub.cancel();
              await source.close();
            }
          });
        });
      }
      test('explicit cancel returns cleanup failure: async=$asynchronous',
          () async {
        await _captureUncaught(() async {
          final cleanupError = StateError('synthetic cleanup failure');
          var cancellations = 0;
          final source = StreamController<List<int>>(
            onCancel: () {
              cancellations++;
              if (asynchronous) {
                return Future<void>.error(cleanupError);
              }
              throw cleanupError;
            },
          );
          final sub = SseClient().parseStream(source.stream).listen((_) {});
          try {
            await expectLater(sub.cancel(), throwsA(same(cleanupError)));
            expect(cancellations, 1);
          } finally {
            await source.close();
          }
        });
      });
    }
  });
}
