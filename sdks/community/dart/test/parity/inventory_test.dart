import 'dart:convert';
import 'dart:io';

import 'package:test/test.dart';

Map<String, dynamic> _asMap(Object? value) {
  if (value case final Map<Object?, Object?> map) {
    return map.cast<String, dynamic>();
  }
  throw StateError('Expected a JSON object, got ${value.runtimeType}.');
}

List<dynamic> _asList(Object? value) {
  if (value case final List<dynamic> list) {
    return list;
  }
  throw StateError('Expected a JSON array, got ${value.runtimeType}.');
}

Directory _packageRoot() {
  final current = Directory.current;
  if (File('${current.path}/pubspec.yaml').existsSync()) {
    return current;
  }

  final nested = Directory('${current.path}/sdks/community/dart');
  if (File('${nested.path}/pubspec.yaml').existsSync()) {
    return nested;
  }

  throw StateError(
    'Run the inventory test from the repository or Dart SDK root.',
  );
}

Object? _lookup(Map<String, dynamic> root, String dottedPath) {
  Object? current = root;
  for (final segment in dottedPath.split('.')) {
    current = _asMap(current)[segment];
  }
  return current;
}

Iterable<Map<String, dynamic>> _fieldRows(Map<String, dynamic> manifest) sync* {
  for (final modelValue in _asList(manifest['models'])) {
    final model = _asMap(modelValue);
    for (final fieldValue in _asList(model['fields'])) {
      yield _asMap(fieldValue);
    }
  }
}

void main() {
  late Directory packageRoot;
  late Directory repositoryRoot;
  late Map<String, dynamic> manifest;

  setUpAll(() {
    packageRoot = _packageRoot();
    repositoryRoot = packageRoot.parent.parent.parent;
    final source =
        File('${packageRoot.path}/test/fixtures/parity_manifest.json');
    manifest = _asMap(jsonDecode(source.readAsStringSync()));
  });

  group('Dart parity inventory', () {
    test('pins the reviewed stack and every authoritative source file', () {
      final pins = _asMap(manifest['pins']);
      expect(
        pins['implementation_base'],
        'aaa75b54d572be8cd1d51c72e951273c5b893ed0',
      );
      expect(
        pins['parent_branch'],
        'plan-pr/1189b5aa4d7c75e5/17-parity-docs',
      );
      expect(pins['parent_pr'], 35);
      expect(
        pins['upstream_inspection'],
        '0fa1bebd9772de79347f0caf79744535e94ec37c',
      );

      final sources = _asMap(manifest['source_files']);
      expect(sources.length, greaterThanOrEqualTo(80));
      final digestPattern = RegExp(r'^[0-9a-f]{64}$');
      for (final entry in sources.entries) {
        expect(entry.value, isA<String>());
        expect(entry.value, matches(digestPattern), reason: entry.key);
        expect(
          File('${repositoryRoot.path}/${entry.key}').existsSync(),
          isTrue,
          reason: 'Missing authoritative source ${entry.key}',
        );
      }

      expect(
        sources.keys,
        containsAll(<String>[
          'sdks/typescript/packages/core/src/events.ts',
          'sdks/typescript/packages/core/src/types.ts',
          'sdks/typescript/packages/core/src/metadata.ts',
          'sdks/typescript/packages/core/src/capabilities.ts',
          'sdks/typescript/packages/core/src/token-usage.ts',
          'sdks/python/ag_ui/core/events.py',
          'sdks/python/ag_ui/core/types.py',
          'sdks/python/ag_ui/core/capabilities.py',
          'sdks/python/ag_ui/core/token_usage.py',
          'sdks/community/dart/lib/src/events/events.dart',
          'sdks/community/dart/lib/src/types/message.dart',
        ]),
      );
    });

    test('accounts for every peer model field exactly once', () {
      final peerShapes = _asMap(manifest['peer_shapes']);
      final python = _asMap(peerShapes['python']);
      final typescript = _asMap(peerShapes['typescript']);

      expect(python.length, 84);
      expect(typescript.length, 83);
      expect(
        python.values
            .map((value) => _asMap(value).length)
            .fold<int>(0, (sum, count) => sum + count),
        452,
      );
      expect(
        typescript.values
            .map((value) => _asMap(value).length)
            .fold<int>(0, (sum, count) => sum + count),
        452,
      );

      final expectedIds = <String>{};
      for (final entry in python.entries) {
        for (final field in _asMap(entry.value).keys) {
          expectedIds.add('${entry.key}.$field');
        }
      }
      for (final entry in typescript.entries) {
        final symbol = entry.key.replaceFirst(RegExp(r'Schema$'), '');
        for (final field in _asMap(entry.value).keys) {
          expectedIds.add('$symbol.$field');
        }
      }

      final rows = _fieldRows(manifest).toList();
      final actualIds = rows.map((row) => row['id'] as String).toSet();
      expect(rows.length, actualIds.length, reason: 'Duplicate field row ID');
      expect(actualIds, unorderedEquals(expectedIds));
      expect(actualIds.length, 453);
    });

    test('assigns every field a disposition, owner, shape, and test', () {
      final tests = _asMap(manifest['test_catalog']);
      final ownerPattern = RegExp(r'^PR0[1-6]$');

      for (final row in _fieldRows(manifest)) {
        final id = row['id'] as String;
        final owner = row['owner'];
        final status = row['dart_status'];
        final testId = row['test_id'];

        expect(owner, matches(ownerPattern), reason: id);
        expect(status, anyOf('implemented', 'pending'), reason: id);
        expect(row['disposition'], isNotEmpty, reason: id);
        expect(tests.containsKey(testId), isTrue, reason: id);
        expect(
          row['python_shape'] != null || row['typescript_shape'] != null,
          isTrue,
          reason: '$id has no authoritative peer shape',
        );

        for (final key in ['python_shape', 'typescript_shape']) {
          final reference = row[key];
          if (reference != null) {
            expect(
              _lookup(manifest, reference as String),
              isA<Map<Object?, Object?>>(),
            );
          }
        }

        final testRecord = _asMap(tests[testId]);
        if (status == 'implemented') {
          expect(owner, 'PR01', reason: id);
          expect(testRecord['status'], 'executable', reason: id);
        } else {
          expect(owner, isNot('PR01'), reason: id);
          expect(testRecord['status'], 'planned', reason: id);
          expect(testRecord['owner'], owner, reason: id);
        }
      }
    });

    test('implemented rows resolve to executable test declarations', () {
      final tests = _asMap(manifest['test_catalog']);
      final implementedTestIds = _fieldRows(manifest)
          .where((row) => row['dart_status'] == 'implemented')
          .map((row) => row['test_id'] as String)
          .toSet();

      expect(implementedTestIds, isNotEmpty);
      for (final testId in implementedTestIds) {
        final record = _asMap(tests[testId]);
        expect(record['status'], 'executable', reason: testId);
        final testFile = File('${packageRoot.path}/${record['path']}');
        expect(testFile.existsSync(), isTrue, reason: testId);
        expect(
          testFile.readAsStringSync(),
          contains(record['name_fragment']),
          reason: '$testId does not resolve to the named executable test',
        );
      }
    });

    test('records all shared exports and helper ownership', () {
      final exports = _asList(manifest['shared_exports']).map(_asMap).toList();
      final symbols =
          exports.map((entry) => entry['symbol'] as String).toList();
      expect(symbols.length, symbols.toSet().length);
      expect(
        symbols,
        containsAll(<String>[
          'AGUIEvent',
          'EventType',
          'Message',
          'Role',
          'State',
          'InputContent',
          'InputContentPart',
          'InputContentSource',
          'Metadata',
          'AGUI_METADATA_KEY',
          'mergeMetadata',
          'Interrupt',
          'ResumeEntry',
          'RunFinishedOutcome',
          'ResumeStatus',
          'SubagentFinishedOutcome',
          'ReasoningEncryptedValueSubtype',
          'TokenUsage',
          'AgentCapabilities',
          'aggregateTokenUsage',
          'tokenUsageFromLangChainMetadata',
        ]),
      );

      for (final entry in exports) {
        final symbol = entry['symbol'] as String;
        final kind = entry['kind'] as String;
        final dartSymbol = entry['dart'] as String;
        expect(kind, isNotEmpty, reason: symbol);
        expect(dartSymbol, isNotEmpty, reason: symbol);
        final owners = _asList(entry['owners']).cast<String>();
        expect(owners, isNotEmpty, reason: symbol);
        expect(
          owners.every(RegExp(r'^PR0[1-6]$').hasMatch),
          isTrue,
          reason: symbol,
        );
      }
    });

    test('records every discriminator and the Dart-only legacy event', () {
      final discriminators = _asMap(manifest['discriminators']);
      final events =
          _asList(discriminators['canonical_event_types']).map(_asMap).toList();
      expect(events.length, 36);
      expect(
        events.map((event) => event['value']),
        containsAll(<String>[
          'TEXT_MESSAGE_START',
          'TOOL_CALL_RESULT',
          'RUN_FINISHED',
          'REASONING_ENCRYPTED_VALUE',
          'SUBAGENT_STARTED',
          'SUBAGENT_FINISHED',
          'SUBAGENT_ERROR',
        ]),
      );
      expect(
        events
            .where(
              (event) => (event['value'] as String).startsWith('SUBAGENT_'),
            )
            .every(
              (event) =>
                  event['owner'] == 'PR04' && event['dart_status'] == 'pending',
            ),
        isTrue,
      );

      final roles = _asList(discriminators['message_roles'])
          .map(_asMap)
          .map((role) => role['value'])
          .toList();
      expect(
        roles,
        unorderedEquals(<String>[
          'developer',
          'system',
          'assistant',
          'user',
          'tool',
          'activity',
          'reasoning',
        ]),
      );

      final dartOnly =
          _asList(discriminators['dart_only_event_types']).map(_asMap).single;
      expect(dartOnly['value'], 'THINKING_CONTENT');
      expect(dartOnly['dart_status'], 'implemented-legacy');
    });

    test('assigns all later verification and documentation work', () {
      final packages = _asMap(manifest['work_packages']);
      expect(
        packages.keys,
        unorderedEquals(List<String>.generate(9, (index) => 'PR0${index + 1}')),
      );

      final tests = _asMap(manifest['test_catalog']);
      final surfaces = _asList(manifest['verification_surfaces']).map(_asMap);
      final surfaceOwners = <String>{};
      for (final surface in surfaces) {
        final owner = surface['owner'] as String;
        surfaceOwners.add(owner);
        expect(owner, matches(RegExp(r'^PR0[7-9]$')));
        expect(surface['status'], 'pending');
        final testRecord = _asMap(tests[surface['test_id']]);
        expect(testRecord['status'], 'planned');
        expect(testRecord['owner'], owner);
      }
      expect(surfaceOwners, unorderedEquals(['PR07', 'PR08', 'PR09']));
    });

    test('keeps compatibility differences and exclusions explicit', () {
      final differences =
          _asList(manifest['explicit_differences']).map(_asMap).toList();
      expect(differences.length, greaterThanOrEqualTo(15));
      for (final difference in differences) {
        expect(difference['scope'], isNotEmpty);
        expect(difference['difference'], isNotEmpty);
        expect(difference['disposition'], isNotEmpty);
        expect(difference['documentation_owner'], 'PR09');
      }

      expect(
        differences.map((entry) => entry['scope']),
        containsAll(<String>[
          'tokenUsageFromAiSdkUsage',
          'protobuf and WebSockets',
          'capability HTTP discovery',
          'Dart THINKING_CONTENT',
          'SimpleRunAgentInput defaults',
          'RunAgentInput null serialization',
        ]),
      );
    });
  });
}
