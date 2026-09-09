import 'dart:convert';
import 'dart:io';

import 'package:ag_ui/ag_ui.dart';
import 'package:crypto/crypto.dart';

Directory packageRoot() {
  var directory = Directory.current.absolute;
  while (!File('${directory.path}/pubspec.yaml').existsSync()) {
    if (directory.parent.path == directory.path) {
      throw StateError('Could not locate Dart package root');
    }
    directory = directory.parent;
  }
  return directory;
}

Directory repositoryRoot() => packageRoot().parent.parent.parent;

Map<String, dynamic> asMap(Object? value, String description) {
  if (value is Map) {
    return value.cast<String, dynamic>();
  }
  throw FormatException('$description must be an object');
}

final class ParityCorpus {
  ParityCorpus(this.bytes)
      : digest = sha256.convert(bytes).toString(),
        document = asMap(jsonDecode(utf8.decode(bytes)), 'corpus') {
    if (document['version'] != 1) {
      throw const FormatException('Unsupported corpus version');
    }
    final rawCases = document['cases'];
    if (rawCases is! List || rawCases.isEmpty) {
      throw const FormatException('Corpus must contain cases');
    }
    cases = rawCases.map((value) => asMap(value, 'case')).toList();
    final ids = <String>{};
    for (final testCase in cases) {
      final id = testCase['id'];
      if (id is! String || id.isEmpty || !ids.add(id)) {
        throw const FormatException('Case IDs must be unique and nonempty');
      }
      if (testCase['valid'] is! bool || testCase['input'] is! Map) {
        throw FormatException('Case $id lacks explicit validity/object input');
      }
    }
  }

  final List<int> bytes;
  final String digest;
  final Map<String, dynamic> document;
  late final List<Map<String, dynamic>> cases;

  Set<String> get caseIds =>
      cases.map((testCase) => testCase['id'] as String).toSet();
}

final class ArtifactRecord {
  const ArtifactRecord({
    required this.id,
    required this.accepted,
    this.value,
    this.hasValue = false,
    this.error,
    this.unsupported = false,
  });

  final String id;
  final bool accepted;
  final Object? value;
  final bool hasValue;
  final String? error;
  final bool unsupported;

  Map<String, dynamic> toJson() => {
        'id': id,
        'accepted': accepted,
        if (hasValue) 'value': value,
        if (error != null) 'error': error,
        if (unsupported) 'unsupported': true,
      };
}

final class ArtifactDocument {
  const ArtifactDocument({
    required this.route,
    required this.corpusDigest,
    required this.cases,
  });

  final String route;
  final String corpusDigest;
  final List<ArtifactRecord> cases;

  Map<String, dynamic> toJson() => {
        'version': 1,
        'route': route,
        'corpus_sha256': corpusDigest,
        'cases': cases.map((record) => record.toJson()).toList(),
      };
}

Object? _jsonRoundTrip(Object? value) => jsonDecode(jsonEncode(value));

bool deepEqualJson(Object? left, Object? right) {
  if (left is Map && right is Map) {
    return left.length == right.length &&
        left.keys.every(
          (key) =>
              right.containsKey(key) && deepEqualJson(left[key], right[key]),
        );
  }
  if (left is List && right is List) {
    return left.length == right.length &&
        List.generate(left.length, (index) => index)
            .every((index) => deepEqualJson(left[index], right[index]));
  }
  return left == right;
}

Object? transformDartCase(
  Map<String, dynamic> testCase,
  Object? input, {
  required bool useEncoder,
  bool fromPeer = false,
}) {
  final kind = testCase['kind'];
  if (kind == 'aggregate') {
    if (fromPeer) {
      if (input is! List) {
        throw const FormatException('usage list required');
      }
      return input
          .map((value) => TokenUsage.fromJson(asMap(value, 'usage')).toJson())
          .toList();
    }
    final map = asMap(input, 'aggregate request');
    final entries = map['entries'];
    if (entries is! List) {
      throw const FormatException('entries list required');
    }
    return aggregateTokenUsage(
      entries.map((value) => TokenUsage.fromJson(asMap(value, 'usage'))),
    ).map((usage) => usage.toJson()).toList();
  }
  if (kind == 'mapper') {
    if (fromPeer) {
      return input == null
          ? null
          : TokenUsage.fromJson(asMap(input, 'usage')).toJson();
    }
    final map = asMap(input, 'mapper request');
    return tokenUsageFromLangChainMetadata(
      map['metadata'],
      provider: map['provider'] as String?,
      model: map['model'] as String?,
    )?.toJson();
  }

  final map = asMap(input, '${testCase['id']} input');
  final Object? value;
  switch (kind) {
    case 'event':
      final event = BaseEvent.fromJson(map);
      if (useEncoder) {
        value = const EventDecoder()
            .decodeSSE(EventEncoder().encodeSSE(event))
            .toJson();
      } else {
        value = event.toJson();
      }
    case 'message':
      value = Message.fromJson(map).toJson();
    case 'content':
      value = InputContent.fromJson(map).toJson();
    case 'request':
      value = RunAgentInput.fromJson(map).toJson();
    case 'usage':
      value = TokenUsage.fromJson(map).toJson();
    case 'capabilities':
      value = AgentCapabilities.fromJson(map).toJson();
    default:
      throw UnsupportedError('Unsupported corpus kind: $kind');
  }
  return useEncoder ? _jsonRoundTrip(value) : value;
}

ArtifactDocument produceDartArtifact(
  ParityCorpus corpus,
  String route, {
  required bool useEncoder,
}) {
  final records = <ArtifactRecord>[];
  for (final testCase in corpus.cases) {
    try {
      final value = transformDartCase(
        testCase,
        testCase['input'],
        useEncoder: useEncoder,
      );
      records.add(
        ArtifactRecord(
          id: testCase['id'] as String,
          accepted: true,
          value: value,
          hasValue: true,
        ),
      );
    } on Object catch (error) {
      records.add(
        ArtifactRecord(
          id: testCase['id'] as String,
          accepted: false,
          error: error.runtimeType.toString(),
        ),
      );
    }
  }
  return ArtifactDocument(
    route: route,
    corpusDigest: corpus.digest,
    cases: records,
  );
}

ArtifactDocument parseArtifact(
  Object? raw,
  ParityCorpus corpus,
  String expectedRoute,
) {
  final document = asMap(raw, 'artifact');
  const topKeys = {'version', 'route', 'corpus_sha256', 'cases'};
  if (document.keys.toSet().difference(topKeys).isNotEmpty ||
      document['version'] != 1 ||
      document['route'] != expectedRoute ||
      document['corpus_sha256'] != corpus.digest ||
      document['cases'] is! List) {
    throw const FormatException('Artifact envelope metadata is invalid');
  }
  final records = <ArtifactRecord>[];
  final ids = <String>{};
  final corpusById = {
    for (final testCase in corpus.cases) testCase['id'] as String: testCase,
  };
  for (final rawRecord in document['cases'] as List) {
    final record = asMap(rawRecord, 'artifact record');
    const keys = {'id', 'accepted', 'value', 'error', 'unsupported'};
    final id = record['id'];
    final accepted = record['accepted'];
    final hasUnsupported = record.containsKey('unsupported');
    final unsupportedValue = record['unsupported'];
    if (record.keys.toSet().difference(keys).isNotEmpty ||
        id is! String ||
        !ids.add(id) ||
        accepted is! bool ||
        (hasUnsupported && unsupportedValue is! bool)) {
      throw const FormatException('Artifact record metadata is invalid');
    }
    final unsupported = hasUnsupported && unsupportedValue as bool;
    final hasValue = record.containsKey('value');
    final error = record['error'];
    if ((accepted && (!hasValue || error != null || unsupported)) ||
        (accepted &&
            record['value'] == null &&
            corpusById[id]!['kind'] != 'mapper') ||
        (!accepted && (hasValue || error is! String || error.isEmpty))) {
      throw FormatException('Artifact record $id has contradictory status');
    }
    records.add(
      ArtifactRecord(
        id: id,
        accepted: accepted,
        value: record['value'],
        hasValue: hasValue,
        error: error as String?,
        unsupported: unsupported,
      ),
    );
  }
  if (!ids.containsAll(corpus.caseIds) || !corpus.caseIds.containsAll(ids)) {
    throw const FormatException('Artifact case IDs do not match corpus');
  }
  return ArtifactDocument(
    route: expectedRoute,
    corpusDigest: corpus.digest,
    cases: records,
  );
}

ArtifactDocument consumeDartArtifact(
  ParityCorpus corpus,
  ArtifactDocument peer,
  String source,
) {
  final byId = {for (final record in peer.cases) record.id: record};
  final records = <ArtifactRecord>[];
  for (final testCase in corpus.cases) {
    final sourceRecord = byId[testCase['id']]!;
    if (!sourceRecord.accepted) {
      records.add(
        ArtifactRecord(
          id: sourceRecord.id,
          accepted: false,
          error: 'not round-tripped: ${sourceRecord.error}',
          unsupported: sourceRecord.unsupported,
        ),
      );
      continue;
    }
    try {
      final value = transformDartCase(
        testCase,
        sourceRecord.value,
        useEncoder: false,
        fromPeer: true,
      );
      records.add(
        ArtifactRecord(
          id: sourceRecord.id,
          accepted: true,
          value: value,
          hasValue: true,
        ),
      );
    } on Object catch (error) {
      records.add(
        ArtifactRecord(
          id: sourceRecord.id,
          accepted: false,
          error: error.runtimeType.toString(),
        ),
      );
    }
  }
  return ArtifactDocument(
    route: 'dart.from-$source',
    corpusDigest: corpus.digest,
    cases: records,
  );
}

void writeArtifact(Directory output, ArtifactDocument document) {
  if (!output.existsSync()) {
    throw StateError('Caller-owned artifact directory does not exist');
  }
  File('${output.path}/${document.route}.json').writeAsStringSync(
    jsonEncode(document.toJson()),
    flush: true,
  );
}

List<int> composeParityCorpusBytes(File frozenFile, File supplementalFile) {
  final frozen = asMap(
    jsonDecode(frozenFile.readAsStringSync()),
    'frozen corpus',
  );
  final supplemental = asMap(
    jsonDecode(supplementalFile.readAsStringSync()),
    'supplemental corpus',
  );
  if (frozen['version'] != 1 || supplemental['version'] != 1) {
    throw const FormatException('Parity corpora must use version 1');
  }
  final frozenCases = frozen['cases'];
  final supplementalCases = supplemental['cases'];
  if (frozenCases is! List || supplementalCases is! List) {
    throw const FormatException('Parity corpora must contain case lists');
  }
  final ids = <String>{};
  for (final rawCase in [...frozenCases, ...supplementalCases]) {
    final testCase = asMap(rawCase, 'parity case');
    final id = testCase['id'];
    if (id is! String || id.isEmpty || !ids.add(id)) {
      throw const FormatException(
        'Parity case IDs must be unique and nonempty',
      );
    }
  }
  return utf8.encode(
    '${jsonEncode({
          'version': 1,
          'cases': [...frozenCases, ...supplementalCases],
        })}\n',
  );
}

ParityCorpus loadSharedCorpus() {
  final configured = Platform.environment['AG_UI_DART_PARITY_CORPUS'];
  if (configured != null) {
    return ParityCorpus(File(configured).readAsBytesSync());
  }
  final root = repositoryRoot().path;
  return ParityCorpus(
    composeParityCorpusBytes(
      File('$root/sdks/community/go/testdata/parity/fixtures.json'),
      File('${packageRoot().path}/test/fixtures/parity_cases.json'),
    ),
  );
}

Map<String, dynamic> loadInteropManifest() => asMap(
      asMap(
        jsonDecode(
          File('${packageRoot().path}/test/fixtures/parity_manifest.json')
              .readAsStringSync(),
        ),
        'parity manifest',
      )['dart_interop'],
      'Dart interop manifest',
    );

({bool accepted, Object? expected}) expectedForDartRoute(
  Map<String, dynamic> testCase,
  String route,
  Map<String, dynamic> interop,
) {
  for (final raw in interop['route_expectations'] as List) {
    final expectation = asMap(raw, 'route expectation');
    if (expectation['case_id'] == testCase['id'] &&
        (expectation['routes'] as List).contains(route)) {
      return (
        accepted: expectation['accepted'] as bool,
        expected: expectation['expected'],
      );
    }
  }
  return (
    accepted: testCase['valid'] as bool,
    expected: testCase['expected'],
  );
}

List<String> dartProducerMismatches(
  ParityCorpus corpus,
  ArtifactDocument artifact,
  Map<String, dynamic> interop,
) {
  final mismatches = <String>[];
  for (var index = 0; index < corpus.cases.length; index++) {
    final testCase = corpus.cases[index];
    final record = artifact.cases[index];
    final expected = expectedForDartRoute(testCase, artifact.route, interop);
    if (record.id != testCase['id'] || record.accepted != expected.accepted) {
      mismatches
          .add('${artifact.route}:${record.id}:accepted=${record.accepted}');
    } else if (record.accepted &&
        !deepEqualJson(record.value, expected.expected)) {
      mismatches.add('${artifact.route}:${record.id}:value');
    }
  }
  return mismatches;
}
