import 'dart:convert';
import 'dart:io';

import 'package:test/test.dart';

import 'contract_source_snapshot.dart';

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

Map<String, Set<String>> _shapeFieldSets(Object? value) => {
      for (final entry in _asMap(value).entries)
        entry.key: _asMap(entry.value).keys.toSet(),
    };

bool _containsJsonPointer(Object? document, String pointer) {
  if (pointer.isEmpty) {
    return true;
  }
  var current = document;
  for (final encodedSegment in pointer.split('/').skip(1)) {
    final segment = encodedSegment.replaceAll('~1', '/').replaceAll('~0', '~');
    if (current case final Map<Object?, Object?> map) {
      if (!map.containsKey(segment)) {
        return false;
      }
      current = map[segment];
    } else if (current case final List<Object?> list) {
      final index = int.tryParse(segment);
      if (index == null || index < 0 || index >= list.length) {
        return false;
      }
      current = list[index];
    } else {
      return false;
    }
  }
  return true;
}

void main() {
  late Directory packageRoot;
  late Directory repositoryRoot;
  late Map<String, dynamic> manifest;
  late PinnedGitSnapshot snapshot;

  setUpAll(() {
    packageRoot = _packageRoot();
    repositoryRoot = packageRoot.parent.parent.parent;
    final source =
        File('${packageRoot.path}/test/fixtures/parity_manifest.json');
    manifest = _asMap(jsonDecode(source.readAsStringSync()));
    snapshot = PinnedGitSnapshot(
      repositoryRoot,
      _asMap(manifest['pins'])['implementation_base'] as String,
    );
  });

  group('Dart parity inventory', () {
    test('pins the reviewed stack and every authoritative source file',
        () async {
      final pins = _asMap(manifest['pins']);
      final implementationBase = pins['implementation_base'] as String;
      final upstreamInspection = pins['upstream_inspection'] as String;
      expect(
        implementationBase,
        'aaa75b54d572be8cd1d51c72e951273c5b893ed0',
      );
      expect(
        pins['parent_branch'],
        'plan-pr/1189b5aa4d7c75e5/17-parity-docs',
      );
      expect(pins['parent_pr'], 35);
      expect(
        upstreamInspection,
        '0fa1bebd9772de79347f0caf79744535e94ec37c',
      );
      await snapshot.requireRevision();

      final sources = _asMap(manifest['source_files']);
      expect(sources.length, 88);
      final peerContract = _asMap(manifest['peer_contract']);
      final peerSources = [
        for (final prefix in ['cases', 'compatibility', 'manifest', 'sse'])
          peerContract['${prefix}_source'] as String,
      ];
      await snapshot.load([...sources.keys, ...peerSources]);
      final digestPattern = RegExp(r'^[0-9a-f]{64}$');
      for (final entry in sources.entries) {
        expect(entry.value, isA<String>());
        expect(entry.value, matches(digestPattern), reason: entry.key);
        expect(
          await snapshot.sha256Hex(entry.key),
          entry.value,
          reason: 'Digest mismatch for ${entry.key} at $implementationBase',
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

      for (final prefix in ['cases', 'compatibility', 'manifest', 'sse']) {
        final source = peerContract['${prefix}_source'] as String;
        final expectedDigest = peerContract['${prefix}_sha256'] as String;
        expect(
          await snapshot.sha256Hex(source),
          expectedDigest,
          reason: 'Digest mismatch for pinned peer corpus $source',
        );
      }

      final fixtures = _asMap(
        jsonDecode(
          await snapshot.text(peerContract['cases_source'] as String),
        ),
      );
      final cases = _asList(fixtures['cases']).map(_asMap).toList();
      expect(cases.length, peerContract['expected_case_count']);
      final actualKindCounts = <String, int>{};
      for (final parityCase in cases) {
        final kind = parityCase['kind'] as String;
        actualKindCounts[kind] = (actualKindCounts[kind] ?? 0) + 1;
      }
      expect(actualKindCounts, _asMap(peerContract['case_kind_counts']));

      final goManifest = _asMap(
        jsonDecode(
          await snapshot.text(peerContract['manifest_source'] as String),
        ),
      );
      expect(
        _asList(goManifest['case_ids']),
        cases.map((parityCase) => parityCase['id']).toList(),
        reason: 'Go manifest case IDs must exactly match the pinned corpus',
      );
    });

    test('derives every peer model field from immutable source', () async {
      final extractionPolicy = _asMap(manifest['source_extraction_policy']);
      expect(
        extractionPolicy['field_scope'],
        contains('does not reconstruct requiredness'),
      );
      expect(
        _asMap(manifest['shape_policy'])['descriptor_verification'],
        'recorded-review-snapshot',
      );
      final peerShapes = _asMap(manifest['peer_shapes']);
      final python = _asMap(peerShapes['python']);
      final typescript = _asMap(peerShapes['typescript']);

      final pythonPaths = <String>[
        'sdks/python/ag_ui/core/types.py',
        'sdks/python/ag_ui/core/events.py',
        'sdks/python/ag_ui/core/capabilities.py',
      ];
      final pythonSources = <String>[];
      for (final path in pythonPaths) {
        pythonSources.add(await snapshot.text(path));
      }
      final extractedPython = extractPythonModelFields(pythonSources);
      expect(_shapeFieldSets(python), extractedPython);

      final typescriptPaths = <String>[
        'sdks/typescript/packages/core/src/types.ts',
        'sdks/typescript/packages/core/src/events.ts',
        'sdks/typescript/packages/core/src/capabilities.ts',
        'sdks/typescript/packages/core/src/metadata.ts',
      ];
      final typescriptSources = <String>[];
      for (final path in typescriptPaths) {
        typescriptSources.add(await snapshot.text(path));
      }
      final extractedTypeScript =
          extractTypeScriptSchemaFields(typescriptSources);
      final expectedTypeScript = _shapeFieldSets(typescript);
      expect(
        extractedTypeScript.keys,
        unorderedEquals(expectedTypeScript.keys),
        reason:
            'missing ${expectedTypeScript.keys.toSet().difference(extractedTypeScript.keys.toSet())}; '
            'unexpected ${extractedTypeScript.keys.toSet().difference(expectedTypeScript.keys.toSet())}',
      );
      for (final entry in expectedTypeScript.entries) {
        expect(
          extractedTypeScript[entry.key],
          entry.value,
          reason: entry.key,
        );
      }

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

    test('derives shared exports and implemented Dart symbols from source',
        () async {
      final exports = _asList(manifest['shared_exports']).map(_asMap).toList();

      final pythonInit =
          await snapshot.text('sdks/python/ag_ui/core/__init__.py');
      final pythonExports = extractPythonPublicExports(pythonInit);

      final typescriptPaths = <String>[
        'sdks/typescript/packages/core/src/index.ts',
        'sdks/typescript/packages/core/src/types.ts',
        'sdks/typescript/packages/core/src/events.ts',
        'sdks/typescript/packages/core/src/capabilities.ts',
        'sdks/typescript/packages/core/src/metadata.ts',
        'sdks/typescript/packages/core/src/token-usage.ts',
      ];
      final typescriptSources = <String, String>{};
      for (final path in typescriptPaths) {
        typescriptSources[path.split('/').last] = await snapshot.text(path);
      }
      final typescriptExports =
          extractTypeScriptPublicExports('index.ts', typescriptSources);

      final sourceFiles = _asMap(manifest['source_files']);
      final goSources = <String>[];
      for (final path in sourceFiles.keys.where(
        (path) =>
            path.startsWith('sdks/community/go/pkg/core/') &&
            path.endsWith('.go'),
      )) {
        goSources.add(await snapshot.text(path));
      }
      final goDeclarations = extractGoDeclarations(goSources);

      final dartLibrary =
          File('${packageRoot.path}/lib/ag_ui.dart').readAsStringSync();
      final dartSources = <String, String>{};
      for (final entity in Directory('${packageRoot.path}/lib')
          .listSync(recursive: true)
          .whereType<File>()
          .where((file) => file.path.endsWith('.dart'))) {
        final libraryPath = entity.path.replaceFirst(
          '${packageRoot.path}/lib/',
          '',
        );
        dartSources[libraryPath] = entity.readAsStringSync();
      }
      final dartDeclarations = extractPublicDartDeclarations(
        dartLibrary,
        dartSources,
        rootExports: {
          'src/types/types.dart',
          'src/events/events.dart',
        },
      );

      for (final entry in exports) {
        final symbol = entry['symbol'] as String;
        expect(
          entry['status'],
          anyOf('implemented', 'partial', 'excluded'),
          reason: symbol,
        );
        final python = entry['python'];
        final typescript = entry['typescript'];
        final go = entry['go'];
        if (python != null) {
          expect(pythonExports, contains(python), reason: symbol);
        }
        if (typescript != null) {
          expect(typescriptExports, contains(typescript), reason: symbol);
        }
        if (go != null && go != 'any') {
          expect(goDeclarations, contains(go), reason: symbol);
        }
        if (entry['status'] != 'excluded') {
          expect(
            dartDeclarations,
            contains(entry['dart']),
            reason: '$symbol maps to a missing public Dart declaration',
          );
        }
      }

      final coveredPythonModels =
          exports.map((entry) => entry['python']).whereType<String>().toSet();
      final mappedTypeScriptExports = exports
          .map((entry) => entry['typescript'])
          .whereType<String>()
          .toList();
      final coveredTypeScriptSchemas = mappedTypeScriptExports.toSet();
      expect(coveredPythonModels, pythonExports);
      expect(
        coveredTypeScriptSchemas.length,
        mappedTypeScriptExports.length,
        reason: 'Each logical shared export needs a unique TypeScript symbol',
      );
      expect(
        _asMap(_asMap(manifest['peer_shapes'])['python']).keys,
        everyElement(isIn(coveredPythonModels)),
      );
      expect(
        _asMap(_asMap(manifest['peer_shapes'])['typescript']).keys,
        everyElement(isIn(coveredTypeScriptSchemas)),
      );
    });

    test('assigns every field a disposition, owner, shape, and test', () {
      final tests = _asMap(manifest['test_catalog']);
      final workPackages = _asMap(manifest['work_packages']);
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
        final workPackage = _asMap(workPackages[owner]);
        if (status == 'implemented') {
          expect(
            workPackage['state'],
            anyOf('implemented-by-this-slice', 'implemented-by-prior-slice'),
            reason: id,
          );
          expect(testRecord['status'], 'executable', reason: id);
        } else {
          expect(workPackage['state'], 'pending', reason: id);
          expect(testRecord['status'], 'planned', reason: id);
          expect(testRecord['owner'], owner, reason: id);
        }
      }
    });

    test(
      'binds every field to exact canonical evidence',
      () async {
        final policy = _asMap(manifest['evidence_policy']);
        expect(policy['schema_version'], 1);
        expect(policy['registry_key_pattern'], 'canonical.<row.id>');
        final rows = _fieldRows(manifest).toList();
        final rowsById = {
          for (final row in rows) row['id'] as String: row,
        };
        final implementedIds = rows
            .where((row) => row['dart_status'] == 'implemented')
            .map((row) => row['id'] as String)
            .toSet();
        expect(implementedIds, isNotEmpty);
        expect(
          rows
              .where(
                (row) =>
                    row['owner'] == 'PR02' &&
                    row['dart_status'] == 'implemented',
              )
              .length,
          76,
        );
        final evidenceKeys =
            implementedIds.map((id) => 'canonical.$id').toSet();
        expect(evidenceKeys.length, implementedIds.length);

        final canonicalAliases = _asMap(manifest['canonical_evidence_aliases']);
        expect(
          canonicalAliases.keys,
          everyElement(isIn(rowsById.keys)),
        );
        expect(
          canonicalAliases.values,
          everyElement(isIn(rowsById.keys)),
        );

        final peerContract = _asMap(manifest['peer_contract']);
        final corpus = <String, Map<String, dynamic>>{};
        final corpusDocument = _asMap(
          jsonDecode(
            await snapshot.text(peerContract['cases_source'] as String),
          ),
        );
        for (final value in _asList(corpusDocument['cases'])) {
          final parityCase = _asMap(value);
          final caseId = parityCase['id'] as String;
          expect(corpus.containsKey(caseId), isFalse, reason: caseId);
          corpus[caseId] = parityCase;
        }

        for (final row in rows) {
          final rowId = row['id'] as String;
          var sourceRow = row;
          final visited = <String>{rowId};
          while (sourceRow['canonical_case'] == null) {
            final target = canonicalAliases[sourceRow['id']];
            expect(target, isA<String>(), reason: rowId);
            expect(visited.add(target as String), isTrue, reason: rowId);
            sourceRow = rowsById[target]!;
          }
          final canonicalCase = _asMap(sourceRow['canonical_case']);
          final caseId = canonicalCase['case_id'];
          final documentName = canonicalCase['document'];
          final path = canonicalCase['path'];
          expect(documentName, anyOf('input', 'expected'), reason: rowId);
          expect(path, matches(RegExp('^/')), reason: rowId);
          final parityCase = corpus[caseId];
          expect(parityCase, isNotNull, reason: '$rowId: $caseId');
          expect(
            _containsJsonPointer(parityCase![documentName], path as String),
            isTrue,
            reason: '$rowId: $caseId $documentName$path',
          );
        }
      },
    );

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

      final role = exports.singleWhere((entry) => entry['symbol'] == 'Role');
      expect(role['python'], 'Role');
      expect(role['typescript'], 'Role');
      expect(role['go'], 'Role');
      expect(role['dart'], 'MessageRole');
      expect(role['status'], 'implemented');

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
                  event['owner'] == 'PR04' &&
                  event['dart_status'] == 'implemented',
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

    test('records completed verification and documentation work', () {
      expect(
        manifest['inventory_status'],
        'resolved-with-explicit-differences',
      );
      final packages = _asMap(manifest['work_packages']);
      expect(
        packages.keys,
        unorderedEquals(List<String>.generate(9, (index) => 'PR0${index + 1}')),
      );
      expect(
        packages.values.map(_asMap).map((entry) => entry['state']),
        everyElement(startsWith('implemented-by-')),
      );

      final tests = _asMap(manifest['test_catalog']);
      final surfaces = _asList(manifest['verification_surfaces']).map(_asMap);
      final surfaceOwners = <String>{};
      for (final surface in surfaces) {
        final owner = surface['owner'] as String;
        surfaceOwners.add(owner);
        expect(owner, matches(RegExp(r'^PR0[7-9]$')));
        final testRecord = _asMap(tests[surface['test_id']]);
        expect(surface['status'], 'verified');
        expect(testRecord['status'], 'executable');
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
          'Python MetadataMixin',
          'tokenUsageFromAiSdkUsage',
          'protobuf and WebSockets',
          'capability HTTP discovery',
          'Dart THINKING_CONTENT',
          'Message.id API type',
          'SimpleRunAgentInput defaults',
          'RunAgentInput null serialization',
        ]),
      );
    });
  });
}
