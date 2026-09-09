// Public dependency probe: copy unchanged into an external consumer's bin/.
import 'dart:async';
import 'dart:convert';

import 'package:ag_ui/ag_ui.dart';

void _check(bool condition, String reason) {
  if (!condition) {
    throw StateError(reason);
  }
}

Future<void> _openOverflow(String prefix, {required bool fragmented}) async {
  const limit = 32;
  var cancellations = 0;
  var messages = 0;
  final source = StreamController<List<int>>(
    onCancel: () {
      cancellations++;
    },
  );
  final failure = Completer<Object>();
  final sub =
      SseClient(maxLineCodeUnits: limit).parseStream(source.stream).listen(
    (_) {
      messages++;
    },
    onError: failure.complete,
  );
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
    _check(!failure.isCompleted, 'exact line failed before overflow');
    source.add([120]);
    final error = await failure.future.timeout(const Duration(seconds: 3));
    _check(!source.isClosed, 'source must remain open until failure');
    _check(
      error is FormatException && error.source == null && error.offset == null,
      'expected content-free FormatException',
    );
    _check(messages == 0, 'partial message escaped');
    await sub.cancel();
    _check(cancellations == 1, 'upstream cancellation must occur exactly once');
  } finally {
    await sub.cancel();
    await source.close();
  }
}

Future<void> main() async {
  for (final prefix in ['data: ', ':', 'unknown: ']) {
    for (final fragmented in [false, true]) {
      await _openOverflow(prefix, fragmented: fragmented);
    }
  }
  final client = SseClient(maxDataCodeUnits: 8);
  _check(
    client.maxLineCodeUnits == 15,
    'line limit must derive from data limit',
  );
  final exact = await client
      .parseStream(Stream.value(utf8.encode('event: 12345678\ndata: 12345678')))
      .toList();
  _check(
    exact.length == 1 &&
        exact.single.data == '12345678' &&
        exact.single.event == '12345678',
    'exact value/EOF boundary failed',
  );
  var aggregateRejected = false;
  try {
    await client
        .parseStream(Stream.value(utf8.encode('data: 1234\ndata: 5678\n')))
        .drain<void>();
  } on FormatException catch (error) {
    aggregateRejected = error.source == null;
  }
  _check(aggregateRejected, 'aggregate D+1 must fail');
  await client.close();
  // CLI success marker.
  // ignore: avoid_print
  print(
    'PASS: public open-stream bounds, exact limits, aggregation, cancellation',
  );
}
