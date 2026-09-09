import 'dart:async';
import 'dart:convert';

import 'package:ag_ui/ag_ui.dart';
import 'package:ag_ui/src/sse/sse_parser.dart';
import 'package:test/test.dart';

const _guard = Duration(seconds: 3);

Future<List<SseMessage>> _parse(
  List<List<int>> chunks, {
  int data = 100,
  int? line,
}) {
  return SseClient(maxDataCodeUnits: data, maxLineCodeUnits: line)
      .parseStream(Stream.fromIterable(chunks))
      .toList();
}

List<Object?> _fields(SseMessage m) => [m.data, m.event, m.id, m.retry];

void boundedByteTests() {
  group('bounded public byte parsing', () {
    for (final prefix in [
      'data: ',
      ':',
      'unknown: ',
      'event: ',
      'id: ',
      'retry: ',
      'field',
    ]) {
      for (final fragmented in [false, true]) {
        test('$prefix open L+1, fragmented=$fragmented', () async {
          const limit = 32;
          var cancellations = 0;
          final source = StreamController<List<int>>(
            onCancel: () {
              cancellations++;
            },
          );
          final error = Completer<Object>();
          final messages = <SseMessage>[];
          final subscription = SseClient(maxLineCodeUnits: limit)
              .parseStream(source.stream)
              .listen(messages.add, onError: error.complete);
          try {
            final exact = utf8.encode(prefix.padRight(limit, 'x'));
            if (fragmented) {
              for (final byte in exact) {
                source.add([byte]);
              }
            } else {
              source.add(exact);
            }
            await Future<void>.delayed(const Duration(milliseconds: 20));
            expect(error.isCompleted, isFalse);
            source.add([120]);
            final failure = await error.future.timeout(_guard);
            expect(failure, isA<FormatException>());
            expect((failure as FormatException).source, isNull);
            expect(failure.offset, isNull);
            expect(source.isClosed, isFalse);
            source.add(utf8.encode('\n\ndata: later\n\n'));
            await subscription.cancel();
            expect(cancellations, 1);
            expect(messages, isEmpty);
          } finally {
            await subscription.cancel();
            await source.close();
          }
        });
      }
      test('$prefix huge open chunk', () async {
        final source = StreamController<List<int>>();
        final error = Completer<Object>();
        final sub = SseClient(maxLineCodeUnits: 32)
            .parseStream(source.stream)
            .listen((_) => fail('partial message'), onError: error.complete);
        try {
          source.add(utf8.encode(prefix.padRight(100000, 'x')));
          expect(await error.future.timeout(_guard), isA<FormatException>());
          expect(source.isClosed, isFalse);
        } finally {
          await sub.cancel();
          await source.close();
        }
      });
    }

    test('exact line terminators/EOF and derived full data/event budgets',
        () async {
      for (final ending in ['', '\n', '\r', '\r\n', '\n\n']) {
        final messages = await _parse(
          [utf8.encode('event: 12345678\ndata: 12345678$ending')],
          data: 8,
        );
        expect(messages.map(_fields), [
          ['12345678', '12345678', null, null],
        ]);
      }
      expect(
        (await _parse([utf8.encode('data: 1234')], line: 10)).single.data,
        '1234',
      );
      for (final field in ['data', 'event']) {
        await expectLater(
          _parse([utf8.encode('$field: 123456789\n\n')], data: 8, line: 40),
          throwsFormatException,
        );
      }
      expect(
        (await _parse([utf8.encode('data:\ndata: abc\ndata: def')], data: 8))
            .single
            .data,
        '\nabc\ndef',
      );
      await expectLater(
        _parse([utf8.encode('data:\ndata: abcd\ndata: def')], data: 8),
        throwsFormatException,
      );
    });

    test('valid enormous transport chunk is consumed as many small messages',
        () async {
      final messages = await _parse(
        [utf8.encode(List.filled(10000, 'data: x\n\n').join())],
        data: 1,
      );
      expect(messages.length, 10000);
      expect(messages.every((m) => m.data == 'x'), isTrue);
    });

    test('complete frames survive malformed/oversize suffix at every split',
        () async {
      for (final suffix in [
        [255],
        utf8.encode(':${'x' * 40}'),
        [...utf8.encode(':${'x' * 40}'), 255],
      ]) {
        final bytes = [...utf8.encode('data: ok\r\n\r\n'), ...suffix];
        for (var split = 0; split <= bytes.length; split++) {
          final messages = <SseMessage>[];
          final errors = <Object>[];
          final done = Completer<void>();
          SseClient(maxLineCodeUnits: 32)
              .parseStream(
                Stream.fromIterable([
                  bytes.sublist(0, split),
                  bytes.sublist(split),
                  utf8.encode('\n\ndata: later\n\n'),
                ]),
              )
              .listen(messages.add, onError: errors.add, onDone: done.complete);
          await done.future.timeout(_guard);
          expect(
            messages.map(_fields),
            [
              ['ok', null, null, null],
            ],
            reason: 'split $split',
          );
          expect(errors, hasLength(1));
          expect(errors.single, isA<FormatException>());
          expect((errors.single as FormatException).source, isNull);
        }
      }
    });

    test(
        'truncated UTF-8 never EOF-flushes partial data; upstream errors retain identity',
        () async {
      await expectLater(
        _parse([
          [...utf8.encode('data: partial\n:'), 0xE2, 0x82],
        ]),
        throwsFormatException,
      );
      const upstream = FormatException('upstream', 'upstream source');
      await expectLater(
        SseClient().parseStream(Stream.error(upstream)),
        emitsError(same(upstream)),
      );
    });

    test(
        'valid fixtures match explicit messages and old decoding chain at every split',
        () async {
      final fixtures = <String, List<List<Object?>>>{
        '\uFEFFid: a\r\nevent: old\revent: new\nretry: 42\ndata:\ndata: 😀é\r\n\r\ndata: tail':
            [
          ['\n😀é', 'new', 'a', const Duration(milliseconds: 42)],
          ['tail', null, 'a', null],
        ],
        ':comment\r\runknown: x\nid: first\nid: second\nid: bad\x00id\nretry: invalid\ndata\n\ndata:  space\n\uFEFFdata: ignored\n\n':
            [
          ['', null, 'second', null],
          [' space', null, 'second', null],
        ],
        '\ndata: \uFEFFx\r\n': [
          ['\uFEFFx', null, null, null],
        ],
      };
      for (final entry in fixtures.entries) {
        final bytes = utf8.encode(entry.key);
        for (var split = 0; split <= bytes.length; split++) {
          final chunks = [
            bytes.sublist(0, split),
            <int>[],
            bytes.sublist(split),
          ];
          final actual = (await _parse(chunks)).map(_fields).toList();
          expect(actual, entry.value);
          var first = true;
          final oldLines = utf8.decoder
              .bind(Stream.fromIterable(chunks))
              .transform(const LineSplitter())
              .map((line) {
            if (first) {
              first = false;
              if (line.startsWith('\uFEFF')) {
                return line.substring(1);
              }
            }
            return line;
          });
          expect(
            actual,
            (await SseParser().parseLines(oldLines).toList()).map(_fields),
          );
        }
      }
    });

    test('one/two initial BOMs excluded, third counts; surrogate units count',
        () async {
      for (var boms = 1; boms <= 3; boms++) {
        final bytes = utf8.encode('${'\uFEFF' * boms}data: x');
        for (var split = 0; split <= bytes.length; split++) {
          final chunks = [bytes.sublist(0, split), bytes.sublist(split)];
          if (boms < 3) {
            expect((await _parse(chunks, line: 7)).single.data, 'x');
          } else {
            await expectLater(_parse(chunks, line: 7), throwsFormatException);
            expect(await _parse(chunks, line: 8), isEmpty);
          }
        }
      }
      expect(
        (await _parse([utf8.encode('data: 😀')], line: 8)).single.data,
        '😀',
      );
      await expectLater(
        _parse([utf8.encode('data: 😀')], line: 7),
        throwsFormatException,
      );
      expect(
        await SseParser().parseLines(Stream.value('\uFEFFdata: x')).toList(),
        isEmpty,
      );
    });

    test(
        'parser reuse clears partial metadata on failure and cancel but preserves ID',
        () async {
      final parser = SseParser(maxLineCodeUnits: 20);
      await expectLater(
        parser.parseBytes(
          Stream.value(
            utf8.encode(
              'id: keep\nevent: stale\nretry: 42\ndata: partial\n${'x' * 21}',
            ),
          ),
        ),
        emitsError(isA<FormatException>()),
      );
      expect(
          (await parser
                  .parseBytes(Stream.value(utf8.encode('data: next')))
                  .toList())
              .map(_fields),
          [
            ['next', null, 'keep', null],
          ]);
      final source = StreamController<List<int>>();
      final sub = parser.parseBytes(source.stream).listen((_) {});
      source.add(utf8.encode('event: stale\nretry: 42\ndata: partial\n'));
      await Future<void>.delayed(const Duration(milliseconds: 20));
      await sub.cancel();
      await source.close();
      expect(
          (await parser.parseLines(Stream.value('data: next')).toList())
              .map(_fields),
          [
            ['next', null, 'keep', null],
          ]);
      parser.reset();
      expect(parser.lastEventId, isNull);
      await expectLater(
        parser.parseLines(Stream.value(':' * 21)),
        emitsError(isA<FormatException>()),
      );
    });

    for (final prefix in [
      'data: ',
      ':',
      'unknown: ',
      'event: ',
      'id: ',
      'retry: ',
    ]) {
      test('content-free $prefix diagnostics', () async {
        const canary = 'SYNTHETIC_SSE_CANARY';
        final prints = <String>[];
        Object? failure;
        await runZoned(
          () async {
            try {
              await _parse([utf8.encode('$prefix${canary * 5}')], line: 32);
            } on Object catch (error) {
              failure = error;
            }
          },
          zoneSpecification: ZoneSpecification(
            print: (_, __, ___, line) => prints.add(line),
          ),
        );
        expect(failure, isA<FormatException>());
        expect((failure! as FormatException).source, isNull);
        expect(failure.toString(), isNot(contains(canary)));
        expect(prints.join(), isNot(contains(canary)));
      });
    }
  });
  _lifecycleTests();
}

void _lifecycleTests() {
  group('public byte subscription lifecycle', () {
    for (final partial in [
      <int>[],
      utf8.encode('data: partial'),
      [...utf8.encode('data: '), 0xF0, 0x9F],
      utf8.encode('data: x\r'),
    ]) {
      test('cancel while paused with partial $partial', () async {
        final paused = Completer<void>();
        var cancellations = 0;
        final source = StreamController<List<int>>(
          onPause: () {
            if (!paused.isCompleted) {
              paused.complete();
            }
          },
          onCancel: () {
            cancellations++;
          },
        );
        final sub = SseClient().parseStream(source.stream).listen((_) {});
        try {
          if (partial.isNotEmpty) {
            source.add(partial);
          }
          await Future<void>.delayed(const Duration(milliseconds: 10));
          sub.pause();
          await paused.future.timeout(_guard);
          await sub.cancel().timeout(_guard);
          expect(cancellations, 1);
        } finally {
          await sub.cancel();
          await source.close();
        }
      });
    }
    for (final split in [8, 11]) {
      test('pause/resume mid UTF-8 or CRLF at $split', () async {
        var pauses = 0;
        var resumes = 0;
        var cancels = 0;
        final source = StreamController<List<int>>(
          onPause: () {
            pauses++;
          },
          onResume: () {
            resumes++;
          },
          onCancel: () {
            cancels++;
          },
        );
        final messages = <SseMessage>[];
        final done = Completer<void>();
        final sub = SseClient()
            .parseStream(source.stream)
            .listen(messages.add, onDone: done.complete);
        try {
          final bytes = utf8.encode('data: 😀\r\n\r\n');
          source.add(bytes.sublist(0, split));
          await Future<void>.delayed(const Duration(milliseconds: 10));
          sub.pause();
          await Future<void>.delayed(const Duration(milliseconds: 10));
          final beforeResume = resumes;
          source.add(bytes.sublist(split));
          expect(messages, isEmpty);
          sub.resume();
          await Future<void>.delayed(const Duration(milliseconds: 20));
          await source.close();
          await done.future.timeout(_guard);
          expect(pauses, greaterThan(0));
          expect(resumes, greaterThan(beforeResume));
          expect(messages.map(_fields), [
            ['😀', null, null, null],
          ]);
          expect(cancels, 1);
        } finally {
          await sub.cancel();
          await source.close();
        }
      });
    }
    test('pause after first frame defers invalid suffix in the same huge chunk',
        () async {
      var cancellations = 0;
      final source = StreamController<List<int>>(
        onCancel: () {
          cancellations++;
        },
      );
      final first = Completer<void>();
      final errors = <Object>[];
      var messages = 0;
      late StreamSubscription<SseMessage> sub;
      sub = SseClient(maxLineCodeUnits: 32).parseStream(source.stream).listen(
        (_) {
          messages++;
          sub.pause();
          first.complete();
        },
        onError: errors.add,
      );
      try {
        source.add(
          [...utf8.encode('data: ok\n\n${'data: later\n\n' * 10000}'), 255],
        );
        await first.future.timeout(_guard);
        await Future<void>.delayed(const Duration(milliseconds: 20));
        expect(messages, 1);
        expect(errors, isEmpty);
        await sub.cancel().timeout(_guard);
        expect(cancellations, 1);
        expect(errors, isEmpty);
      } finally {
        await sub.cancel();
        await source.close();
      }
    });

    test('overlapping parseStream calls isolate failure, ID and later reuse',
        () async {
      final client = SseClient(maxLineCodeUnits: 20);
      final a = StreamController<List<int>>();
      final b = StreamController<List<int>>();
      final error = Completer<Object>();
      final sub = client
          .parseStream(a.stream)
          .listen((_) => fail('partial A'), onError: error.complete);
      final result = client.parseStream(b.stream).toList();
      try {
        a.add(utf8.encode('id: A\ndata: partial\n'));
        b.add(utf8.encode('id: B\ndata: oth'));
        a.add(utf8.encode(':' * 21));
        await error.future.timeout(_guard);
        b.add(utf8.encode('er\n\n'));
        await b.close();
        expect((await result).map(_fields), [
          ['other', null, 'B', null],
        ]);
        expect(
            (await client
                    .parseStream(Stream.value(utf8.encode('data: fresh')))
                    .toList())
                .map(_fields),
            [
              ['fresh', null, null, null],
            ]);
        expect(client.lastEventId, isNull);
      } finally {
        await sub.cancel();
        await a.close();
        await b.close();
        await client.close();
      }
    });
  });
}
