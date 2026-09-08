import 'dart:async';
import 'dart:io';

import 'package:testcontainers_core/testcontainers_core.dart';

import 'docker_process.dart';
import 'go_server_container.dart';

/// Development fault-injection gate, deliberately separate from a suite's
/// active Ryuk session. Run with `dart --enable-asserts run` from the example.
Future<void> main() async {
  var assertionsEnabled = false;
  assert(() {
    assertionsEnabled = true;
    return true;
  }());
  if (!assertionsEnabled) {
    throw StateError(
      'Run dart --enable-asserts run test/helpers/verify_container_cleanup.dart',
    );
  }
  for (final failure in ['assertion', 'readiness']) {
    final fixture = GoServerContainer();
    var observed = false;
    try {
      await fixture.build(GoImage.contract);
      if (failure == 'readiness') {
        try {
          await fixture.start(
            readinessPath: '/intentionally-missing',
            readinessTimeout: const Duration(seconds: 2),
          );
        } on TimeoutException {
          observed = true;
        }
      } else {
        await fixture.start();
        try {
          assert(false, 'intentional contract assertion failure');
        } on AssertionError {
          observed = true;
        }
      }
    } finally {
      await fixture.close();
    }
    if (!observed) {
      throw StateError('The intended $failure failure did not occur');
    }
    for (final name in [
      ...fixture.containerNames,
      'testcontainers-ryuk-$sessionId',
    ]) {
      try {
        await fixture.client
            .containerDetails(name)
            .timeout(const Duration(seconds: 10));
        throw StateError('Container survived $failure: $name');
      } on HttpException catch (error) {
        if (!error.message.contains('404')) rethrow;
      }
    }
    for (final tag in fixture.imageTags) {
      final images = await dockerCommand([
        'image',
        'ls',
        '--filter',
        'reference=$tag',
        '--format',
        '{{.ID}}',
      ]);
      if (images.isNotEmpty) throw StateError('Image survived $failure: $tag');
    }
    if (fixture.contextPath != null &&
        await Directory(fixture.contextPath!).exists()) {
      throw StateError('Build context survived $failure');
    }
    stdout.writeln(
      '$failure failure: container, image, context and Ryuk cleanup verified',
    );
  }
}
