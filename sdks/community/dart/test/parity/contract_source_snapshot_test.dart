import 'dart:convert';
import 'dart:io';

import 'package:test/test.dart';

import 'contract_source_snapshot.dart';

Directory _packageRoot() {
  final current = Directory.current;
  if (File('${current.path}/pubspec.yaml').existsSync()) {
    return current;
  }
  return Directory('${current.path}/sdks/community/dart');
}

void main() {
  group('PinnedGitSnapshot', () {
    test('batch-loads immutable bytes and computes a standard SHA-256',
        () async {
      final packageRoot = _packageRoot();
      final repositoryRoot = packageRoot.parent.parent.parent;
      final manifest = (jsonDecode(
        File('${packageRoot.path}/test/fixtures/parity_manifest.json')
            .readAsStringSync(),
      ) as Map)
          .cast<String, dynamic>();
      final pins = (manifest['pins'] as Map).cast<String, dynamic>();
      final hashes = (manifest['source_files'] as Map).cast<String, dynamic>();
      final snapshot = PinnedGitSnapshot(
        repositoryRoot,
        pins['implementation_base'] as String,
      );
      const paths = <String>[
        'sdks/python/ag_ui/core/types.py',
        'sdks/typescript/packages/core/src/index.ts',
      ];

      await snapshot.load(paths);

      expect(snapshot.loadedPaths, paths.toSet());
      expect(
        await snapshot.sha256Hex(paths.first),
        hashes[paths.first],
      );
      expect(
        identical(
          await snapshot.bytes(paths.first),
          await snapshot.bytes(paths.first),
        ),
        isTrue,
        reason: 'Repeated reads must use the cached immutable byte list',
      );
      expect(await snapshot.text(paths.last), contains('export'));
    });
  });

  group('contract source extractors', () {
    test('resolves Python inheritance and aliases by field name', () {
      const source = '''
class Parent(ConfiguredBaseModel):
    parent_field: str

class Child(Parent):
    child_field: int

Alias = Child
''';

      expect(extractPythonModelFields([source]), {
        'Parent': {'parentField'},
        'Child': {'parentField', 'childField'},
        'Alias': {'parentField', 'childField'},
      });
    });

    test('resolves Zod object, omit, extend, and alias field names', () {
      const source = '''
export const ParentSchema = z.object({
  keep: z.string(),
  remove: z.string(),
  nested: z.object({
    ignored: z.string(),
  }),
});
export const ChildSchema = ParentSchema.omit({
  remove: true,
}).extend({
  added: z.number(),
});
export const AliasSchema = ChildSchema;
''';

      expect(extractTypeScriptSchemaFields([source]), {
        'ParentSchema': {'keep', 'remove', 'nested'},
        'ChildSchema': {'keep', 'nested', 'added'},
        'AliasSchema': {'keep', 'nested', 'added'},
      });
    });

    test('resolves TypeScript and Dart public barrel combinators', () {
      expect(
        extractTypeScriptPublicExports('index.ts', {
          'index.ts': '''
export * from "./types";
export { Internal as PublicAlias } from "./extra";
''',
          'types.ts': 'export type PublicType = string;',
          'extra.ts': 'export const Internal = 1;',
        }),
        {'PublicType', 'PublicAlias'},
      );

      expect(
        extractPublicDartDeclarations(
          "export 'src/types.dart' show PublicType, publicHelper;\n"
          "export 'src/extra.dart' hide HiddenType;",
          {
            'src/types.dart': '''
class PublicType {}
class HiddenByShow {}
String publicHelper() => 'ok';
''',
            'src/extra.dart': '''
class VisibleType {}
class HiddenType {}
''',
          },
        ),
        {'PublicType', 'publicHelper', 'VisibleType'},
      );
    });
  });
}
