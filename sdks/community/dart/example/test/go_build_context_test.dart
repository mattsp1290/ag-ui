import 'dart:io';
import 'package:flutter_test/flutter_test.dart';
import 'helpers/go_build_context.dart';

void main() {
  late Directory root;
  late Directory source;
  late Directory target;
  setUp(() async {
    root = await Directory.systemTemp.createTemp('ag-ui-context-test-');
    source = await Directory('${root.path}/source').create();
    target = await Directory('${root.path}/target').create();
    for (final path in [
      'go.mod',
      'go.sum',
      '.dockerignore',
      'pkg/example.go',
      'internal/jsonnumber/int64.go',
      'example/server/go.mod',
      'example/server/go.sum',
      'example/server/Dockerfile',
      'example/server/Dockerfile.contract',
      'example/server/cmd/main.go',
      'example/server/internal/example.go',
    ]) {
      final file = File('${source.path}/$path');
      await file.parent.create(recursive: true);
      await file.writeAsString('fixture');
    }
  });
  tearDown(() => root.delete(recursive: true));

  test(
    'copies regular allowlisted sources and excludes unrelated files',
    () async {
      await File(
        '${source.path}/pkg/notes.txt',
      ).writeAsString('not a build input');
      await File(
        '${source.path}/pkg/.hidden.go',
      ).writeAsString('not a build input');
      final excluded = File('${source.path}/internal/parity/harness.go');
      await excluded.parent.create(recursive: true);
      await excluded.writeAsString('not a runtime dependency');
      await copyGoBuildContext(source, target);
      expect(
        await File('${target.path}/pkg/example.go').readAsString(),
        'fixture',
      );
      expect(
        await File(
          '${target.path}/internal/jsonnumber/int64.go',
        ).readAsString(),
        'fixture',
      );
      expect(Directory('${target.path}/internal/parity').existsSync(), isFalse);
      expect(File('${target.path}/pkg/notes.txt').existsSync(), isFalse);
      expect(File('${target.path}/pkg/.hidden.go').existsSync(), isFalse);
    },
  );

  test(
    'rejects a linked allowlisted root before copying outside source',
    () async {
      final outside = await Directory('${root.path}/outside').create();
      await File('${outside.path}/outside.go').writeAsString('outside');
      await Directory('${source.path}/pkg').delete(recursive: true);
      await Link('${source.path}/pkg').create(outside.path);
      await expectLater(copyGoBuildContext(source, target), throwsStateError);
      expect(File('${target.path}/pkg/outside.go').existsSync(), isFalse);
    },
  );

  test('rejects linked parents of explicit module inputs', () async {
    final outside = await Directory('${root.path}/outside').create();
    await Directory(
      '${source.path}/example/server',
    ).rename('${outside.path}/server');
    await Directory('${source.path}/example').delete();
    await Link('${source.path}/example').create(outside.path);
    await expectLater(copyGoBuildContext(source, target), throwsStateError);
    expect(File('${target.path}/example/server/go.mod').existsSync(), isFalse);
  });

  test('rejects nested links and linked source roots', () async {
    final outside = await File(
      '${root.path}/outside.go',
    ).writeAsString('outside');
    await Link('${source.path}/pkg/linked.go').create(outside.path);
    await expectLater(copyGoBuildContext(source, target), throwsStateError);
    final linkedRoot = Link('${root.path}/linked-source');
    await linkedRoot.create(source.path);
    await expectLater(
      copyGoBuildContext(Directory(linkedRoot.path), target),
      throwsStateError,
    );
  });
}
