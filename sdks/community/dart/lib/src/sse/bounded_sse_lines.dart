import 'dart:convert';
import 'dart:typed_data';

import '../internal/sse_constants.dart';
import 'sse_transform.dart';

/// Validates both caps before an HTTP client or parser state is allocated.
/// Restrict to exact integers shared by Dart VM and JavaScript runtimes.
int effectiveSseLineLimit(int dataLimit, int? lineLimit) {
  const largestExactInteger = 9007199254740991;
  if (dataLimit <= 0 || dataLimit > largestExactInteger) {
    throw ArgumentError.value(
      dataLimit,
      'maxDataCodeUnits',
      'must be a positive exactly representable integer',
    );
  }
  if (lineLimit == null && dataLimit > largestExactInteger - 7) {
    throw ArgumentError.value(
      dataLimit,
      'maxDataCodeUnits',
      'derived line limit is not exactly representable',
    );
  }
  final effective = lineLimit ?? dataLimit + 7;
  if (effective <= 0 || effective > largestExactInteger) {
    throw ArgumentError.value(
      lineLimit,
      'maxLineCodeUnits',
      'must be a positive exactly representable integer',
    );
  }
  return effective;
}

FormatException sseLineOverflow(int limit) =>
    FormatException('SSE line exceeds $limit-code-unit limit');

/// Geometric typed storage avoids one string/list entry per source fragment.
/// Capacity never exceeds L; old storage during growth is at most L as well.
class _LineBuffer {
  _LineBuffer(this.limit);
  final int limit;
  Uint16List _units = Uint16List(0);
  int length = 0;

  void add(int unit) {
    if (length == limit) {
      throw sseLineOverflow(limit);
    }
    if (length == _units.length) {
      final proposed = _units.isEmpty ? kSseDecoderSliceBytes : length * 2;
      final next = Uint16List(proposed > limit ? limit : proposed)
        ..setRange(0, length, _units);
      _units = next;
    }
    _units[length++] = unit;
  }

  String take() {
    final line = String.fromCharCodes(_units, 0, length);
    clear();
    return line;
  }

  void clear() {
    _units = Uint16List(0);
    length = 0;
  }
}

/// Frames all fields, including ignored fields, before unbounded line assembly.
/// The UTF-8 decoder removes its initial BOM; preserve the legacy first-line
/// adjustment by removing one more leading decoded BOM, never a later one.
Stream<String> boundedSseLines(Stream<List<int>> bytes, int limit) {
  final framer = _ByteFramer(limit);
  return sseTransform(
    bytes,
    framer.add,
    finish: framer.finish,
    clear: framer.clear,
  );
}

class _ByteFramer {
  _ByteFramer(int limit) : line = _LineBuffer(limit);
  final _LineBuffer line;
  final output = StringBuffer();
  late ByteConversionSink? decoder = utf8.decoder.startChunkedConversion(
    StringConversionSink.fromStringSink(output),
  );
  bool firstCharacter = true;
  bool afterCr = false;

  void _convert(List<int>? slice) {
    try {
      if (slice == null) {
        decoder!.close();
      } else {
        decoder!.add(slice);
      }
    } on FormatException {
      // Catch only decoder errors, never arbitrary errors from the source.
      throw const FormatException('Invalid SSE UTF-8');
    }
  }

  Iterable<String> add(List<int> chunk) sync* {
    var start = 0;
    while (start < chunk.length) {
      var end = start;
      while (end < chunk.length && end - start < kSseDecoderSliceBytes) {
        final byte = chunk[end++];
        if (byte == 13 || byte == 10) {
          break;
        }
      }
      // A bounded copy, and a terminator boundary before any invalid suffix.
      _convert(chunk.sublist(start, end));
      yield* _frame();
      start = end;
    }
  }

  Iterable<String> _frame() sync* {
    final decoded = output.toString();
    output.clear();
    for (var i = 0; i < decoded.length; i++) {
      final unit = decoded.codeUnitAt(i);
      if (firstCharacter) {
        firstCharacter = false;
        if (unit == 0xFEFF) {
          continue;
        }
      }
      if (afterCr) {
        afterCr = false;
        if (unit == 10) {
          continue;
        }
      }
      if (unit == 13 || unit == 10) {
        afterCr = unit == 13;
        yield line.take();
      } else {
        line.add(unit);
      }
    }
  }

  Iterable<String> finish() sync* {
    _convert(null);
    yield* _frame();
    if (line.length != 0) {
      yield line.take();
    }
  }

  void clear() {
    line.clear();
    output.clear();
    decoder = null;
  }
}
