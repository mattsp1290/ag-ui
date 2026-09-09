import 'package:test/test.dart';

import '../../tool/parity_support.dart';

void main() {
  final corpus = loadSharedCorpus();
  final interop = loadInteropManifest();

  test('artifact routes execute the manifest-pinned resolved corpus', () {
    final resolved = asMap(
      asMap(interop['corpora'], 'interop corpora')['resolved'],
      'resolved corpus',
    );
    final expectedCount = resolved['case_count'] as int;
    expect(corpus.cases, hasLength(expectedCount));
    expect(corpus.digest, resolved['sha256']);
    final mismatches = <String>[];
    for (final route in ['dart.direct', 'dart.encoder']) {
      final artifact = produceDartArtifact(
        corpus,
        route,
        useEncoder: route == 'dart.encoder',
      );
      expect(artifact.cases, hasLength(expectedCount));
      mismatches.addAll(dartProducerMismatches(corpus, artifact, interop));
      expect(
        parseArtifact(artifact.toJson(), corpus, route).cases,
        hasLength(expectedCount),
      );
    }
    expect(mismatches, isEmpty, reason: mismatches.join(', '));
  });

  test('peer import consumes serialized helper outputs without recomputing',
      () {
    final produced = produceDartArtifact(
      corpus,
      'go.produced',
      useEncoder: false,
    );
    final imported = consumeDartArtifact(corpus, produced, 'go');
    final sourceById = {for (final record in produced.cases) record.id: record};
    for (final record in imported.cases.where(
      (record) =>
          record.id.startsWith('aggregate.') || record.id.startsWith('mapper.'),
    )) {
      expect(
        record.accepted,
        sourceById[record.id]!.accepted,
        reason: record.id,
      );
      if (record.accepted) {
        expect(record.value, sourceById[record.id]!.value, reason: record.id);
      }
    }
  });

  test('consumer expectation registry is case-scoped and complete', () {
    final consumerSources = asMap(
      interop['consumer_sources'],
      'consumer sources',
    );
    final routes = asMap(interop['routes'], 'interop routes').keys.toSet();
    final consumerRoutes = consumerSources.keys.toSet();
    expect(consumerRoutes, isNotEmpty);
    expect(routes.containsAll(consumerRoutes), isTrue);
    expect(routes.containsAll(consumerSources.values), isTrue);
    final seen = <String>{};
    for (final raw in interop['consumer_expectations'] as List) {
      final expectation = asMap(raw, 'consumer expectation');
      final route = expectation['route'];
      final caseId = expectation['case_id'];
      expect(route, isIn(consumerRoutes));
      expect(caseId, isIn(corpus.caseIds));
      expect(seen.add('$route:$caseId'), isTrue);
      expect(expectation['accepted'], isA<bool>());
      expect(expectation['source_evidence'], isNotEmpty);
      expect(expectation['reason'], isNotEmpty);
      expect(expectation['review_owner'], 'PR07');
      expect(
        expectation.containsKey('expected'),
        expectation['accepted'],
      );
    }
    expect(
      seen.map((entry) => entry.substring(0, entry.lastIndexOf(':'))).toSet(),
      consumerRoutes,
    );
  });

  test('corrupt, stale, duplicate, extra, and contradictory envelopes fail',
      () {
    final valid = produceDartArtifact(
      corpus,
      'python.produced',
      useEncoder: false,
    ).toJson();
    final mutations = <Map<String, dynamic> Function()>[
      () => {...valid, 'version': 2},
      () => {...valid, 'route': 'stale.produced'},
      () => {...valid, 'corpus_sha256': 'wrong'},
      () => {...valid, 'extra': true},
      () => {
            ...valid,
            'cases': [...valid['cases'] as List]..removeLast(),
          },
      () => {
            ...valid,
            'cases': [
              ...valid['cases'] as List,
              (valid['cases'] as List).first,
            ],
          },
      () {
        final cases = (valid['cases'] as List)
            .map((value) => Map<String, dynamic>.from(value as Map))
            .toList();
        cases.first['error'] = 'contradiction';
        return {...valid, 'cases': cases};
      },
      () {
        final cases = (valid['cases'] as List)
            .map((value) => Map<String, dynamic>.from(value as Map))
            .toList();
        cases.first['accepted'] = null;
        return {...valid, 'cases': cases};
      },
      () {
        final cases = (valid['cases'] as List)
            .map((value) => Map<String, dynamic>.from(value as Map))
            .toList();
        cases.first['unsupported'] = null;
        return {...valid, 'cases': cases};
      },
      () {
        final cases = (valid['cases'] as List)
            .map((value) => Map<String, dynamic>.from(value as Map))
            .toList();
        cases.first['unsupported'] = false;
        return {...valid, 'cases': cases};
      },
      () {
        final cases = (valid['cases'] as List)
            .map((value) => Map<String, dynamic>.from(value as Map))
            .toList();
        cases.first['error'] = null;
        return {...valid, 'cases': cases};
      },
    ];
    for (final mutate in mutations) {
      expect(
        () => parseArtifact(mutate(), corpus, 'python.produced'),
        throwsFormatException,
      );
    }
  });
}
