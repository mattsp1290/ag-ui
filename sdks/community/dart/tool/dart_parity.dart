import 'dart:convert';
import 'dart:io';

import 'parity_support.dart';

Map<String, dynamic> _readObject(String path) =>
    asMap(jsonDecode(File(path).readAsStringSync()), path);

Never _usage() => throw ArgumentError(
      'usage: dart_parity.dart resolve <frozen> <supplemental> <destination>\n'
      '   or: dart_parity.dart <produce|consume|verify> <corpus> <output>',
    );

void _assertSame(Object? actual, Object? expected, String description) {
  if (!deepEqualJson(actual, expected)) {
    throw StateError('$description does not match a fresh replay');
  }
}

void main(List<String> args) {
  if (args.length == 4 && args.first == 'resolve') {
    final destination = File(args[3]);
    if (!destination.parent.existsSync()) {
      throw StateError('destination parent directory does not exist');
    }
    destination.writeAsBytesSync(
      composeParityCorpusBytes(File(args[1]), File(args[2])),
      flush: true,
    );
    return;
  }
  if (args.length != 3) {
    _usage();
  }

  final phase = args[0];
  final corpus = ParityCorpus(File(args[1]).readAsBytesSync());
  final output = Directory(args[2]);
  if (!output.existsSync()) {
    throw StateError('output directory does not exist');
  }

  switch (phase) {
    case 'produce':
      writeArtifact(
        output,
        produceDartArtifact(corpus, 'dart.direct', useEncoder: false),
      );
      writeArtifact(
        output,
        produceDartArtifact(corpus, 'dart.encoder', useEncoder: true),
      );
    case 'consume':
      for (final source in ['go', 'python', 'typescript']) {
        final sourceRoute = source == 'go' ? 'go.direct' : '$source.produced';
        final peer = parseArtifact(
          _readObject('${output.path}/$sourceRoute.json'),
          corpus,
          sourceRoute,
        );
        writeArtifact(output, consumeDartArtifact(corpus, peer, source));
      }
    case 'verify':
      final interop = loadInteropManifest();
      for (final route in ['dart.direct', 'dart.encoder']) {
        final stored = parseArtifact(
          _readObject('${output.path}/$route.json'),
          corpus,
          route,
        );
        final replayed = produceDartArtifact(
          corpus,
          route,
          useEncoder: route == 'dart.encoder',
        );
        final mismatches = dartProducerMismatches(corpus, replayed, interop);
        if (mismatches.isNotEmpty) {
          throw StateError(
            'Dart producer mismatches: ${mismatches.join(', ')}',
          );
        }
        _assertSame(stored.toJson(), replayed.toJson(), route);
      }
      for (final source in ['go', 'python', 'typescript']) {
        final sourceRoute = source == 'go' ? 'go.direct' : '$source.produced';
        final peer = parseArtifact(
          _readObject('${output.path}/$sourceRoute.json'),
          corpus,
          sourceRoute,
        );
        final route = 'dart.from-$source';
        final stored = parseArtifact(
          _readObject('${output.path}/$route.json'),
          corpus,
          route,
        );
        final replayed = consumeDartArtifact(corpus, peer, source);
        _assertSame(stored.toJson(), replayed.toJson(), route);
      }
    default:
      _usage();
  }
}
