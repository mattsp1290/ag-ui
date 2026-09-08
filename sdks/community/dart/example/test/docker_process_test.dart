import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import 'helpers/docker_process.dart';
import 'helpers/go_build_context.dart';
import 'helpers/go_server_container.dart';

void main() {
  test('missing Docker CLI reports actionable setup guidance', () async {
    await expectLater(
      dockerCommand(const [
        'version',
      ], executable: '/definitely/missing/docker'),
      throwsA(
        isA<StateError>()
            .having(
              (error) => error.message,
              'message',
              contains('Docker CLI is unavailable'),
            )
            .having(
              (error) => error.message,
              'message',
              contains('Install Docker'),
            ),
      ),
    );
  });

  test('TLS Docker configuration is rejected before daemon access', () async {
    final flutterRoot = Platform.environment['FLUTTER_ROOT']!;
    final result = await Process.run(
      '$flutterRoot/bin/dart',
      const [
        'run',
        'test/helpers/docker_process.dart',
        '--validate-environment',
      ],
      environment: {...Platform.environment, 'DOCKER_TLS_VERIFY': '1'},
    ).timeout(const Duration(seconds: 30));

    expect(result.exitCode, isNot(0));
    expect(
      '${result.stdout}\n${result.stderr}',
      contains('TLS-enabled Docker endpoints are unsupported'),
    );
  });

  test(
    'failed image builds remain retryable and cannot be started',
    () async {
      final source = await Directory.systemTemp.createTemp(
        'ag-ui-bad-docker-source-',
      );
      addTearDown(() async {
        if (await source.exists()) await source.delete(recursive: true);
      });
      await copyGoBuildContext(Directory('../../go').absolute, source);
      await File.fromUri(
        source.uri.resolve('example/server/Dockerfile.contract'),
      ).writeAsString('THIS IS NOT A DOCKERFILE\n');

      final fixture = GoServerContainer(source: source);
      addTearDown(fixture.close);
      for (var attempt = 0; attempt < 2; attempt++) {
        await expectLater(fixture.build(GoImage.contract), throwsStateError);
      }
      await expectLater(
        fixture.start(kind: GoImage.contract),
        throwsA(
          isA<StateError>().having(
            (error) => error.message,
            'message',
            'Build and validate contract first',
          ),
        ),
      );
      expect(fixture.imageTags, hasLength(1));
    },
    tags: 'requires-go-server',
    timeout: const Timeout(Duration(minutes: 10)),
  );

  test(
    'a Reaper error still removes owned images and build context',
    () async {
      const expectedError =
          'Could not delete the Testcontainers reaper: '
          'Bad state: injected reaper error';
      final fixture = GoServerContainer(
        deleteReaper: () => Future.error(StateError('injected reaper error')),
      );
      addTearDown(() async {
        try {
          await fixture.close();
        } on StateError catch (error) {
          expect(error.message, expectedError);
        }
      });
      await fixture.build(GoImage.contract);
      final context = Directory(fixture.contextPath!);
      final tags = fixture.imageTags;

      await expectLater(
        fixture.close(),
        throwsA(
          isA<StateError>().having(
            (error) => error.message,
            'message',
            expectedError,
          ),
        ),
      );

      expect(context.existsSync(), isFalse);
      for (final tag in tags) {
        await expectLater(
          dockerCommand(['image', 'inspect', tag]),
          throwsStateError,
        );
      }
    },
    tags: 'requires-go-server',
    timeout: const Timeout(Duration(minutes: 10)),
  );
}
