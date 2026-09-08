import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:testcontainers_core/testcontainers_core.dart';

import 'docker_process.dart';
import 'go_build_context.dart';

enum GoImage { contract, production }

/// Owns a suite's context, image tags and containers before any asynchronous
/// creation. Register close in tearDownAll before calling build/start.
class GoServerContainer {
  final String id = '$pid-${DateTime.now().microsecondsSinceEpoch}';
  final Directory source;
  final Future<void> Function() _deleteReaper;
  final Map<GoImage, String> _images = {};
  final Set<GoImage> _validatedImages = {};
  final Map<String, DockerContainer?> _containers = {};
  final Set<Future<void>> _pending = {};
  Directory? _context;
  DockerClient? _client;
  String? _host;
  Future<void>? _closeFuture;
  bool _closing = false;

  GoServerContainer({Directory? source, Future<void> Function()? deleteReaper})
    : source = source ?? Directory('../../go').absolute,
      _deleteReaper = deleteReaper ?? Reaper.deleteInstance;

  String? get contextPath => _context?.path;
  List<String> get imageTags => List.unmodifiable(_images.values);
  List<String> get containerNames => List.unmodifiable(_containers.keys);
  DockerClient get client => _client ?? (throw StateError('Call build first'));

  Future<void> build(GoImage kind) async {
    if (_closing) throw StateError('Container fixture is closing');
    if (_validatedImages.contains(kind)) return;
    if (_client == null) await _prepareDocker();
    if (_context == null) {
      _context = await Directory.systemTemp.createTemp('ag-ui-go-$id-');
      await copyGoBuildContext(source, _context!);
    }
    final tag = _images.putIfAbsent(
      kind,
      () => 'ag-ui-flutter-${kind.name}:$id',
    );
    final dockerfile = kind == GoImage.contract
        ? 'Dockerfile.contract'
        : 'Dockerfile';
    final iidFile = File.fromUri(_context!.uri.resolve('${kind.name}.iid'));
    await dockerCommand(
      [
        'build',
        '--iidfile',
        iidFile.path,
        '--tag',
        tag,
        '--file',
        'example/server/$dockerfile',
        '.',
      ],
      host: _host,
      workingDirectory: _context!.path,
      timeout: const Duration(minutes: 8),
    );

    // This pinned library has no image-inspect endpoint. Its public create +
    // inspect APIs prove the exact built image is visible on the same daemon
    // before executing anything. The probe is never started.
    final probe = 'ag-ui-probe-${kind.name}-$id';
    _containers[probe] = null;
    await _operation(
      client.createContainer(tag, name: probe),
      lateCleanup: () => _remove(probe),
    );
    final details = await _operation(client.containerDetails(probe));
    if (details['Image'] != (await iidFile.readAsString()).trim()) {
      throw StateError(
        'Docker CLI and Testcontainers disagree on the built image. Check DOCKER_HOST and tc.host.',
      );
    }
    final config = details['Config'] as Map<String, dynamic>;
    final user = config['User'] as String? ?? '';
    if (user.isEmpty ||
        user.split(':').first == '0' ||
        user.split(':').first == 'root') {
      throw StateError('Go image must run as a non-root user');
    }
    final entrypoint = (config['Entrypoint'] as List).cast<String>();
    final expected = kind == GoImage.production
        ? '/app/server'
        : '/app/flutter-contract.test';
    if (entrypoint.isEmpty || entrypoint.first != expected) {
      throw StateError('Unexpected Go image entrypoint: $entrypoint');
    }
    await _remove(probe);
    _validatedImages.add(kind);
  }

  Future<DockerContainer> start({
    GoImage kind = GoImage.contract,
    bool sentinelKey = false,
    String? readinessPath = '/',
    Duration readinessTimeout = const Duration(seconds: 30),
  }) async {
    if (_closing) throw StateError('Container fixture is closing');
    if (!_validatedImages.contains(kind)) {
      throw StateError('Build and validate ${kind.name} first');
    }
    final tag = _images[kind]!;
    final name = 'ag-ui-${kind.name}-$id-${_containers.length}';
    final container = DockerContainer(tag, dockerClient: client)
        .withName(name)
        .withExposedPorts([8080])
        .withEnv('AGENT_WORKSPACE', '/tmp/empty-workspace')
        .withTmpfsMount('/tmp/empty-workspace');
    if (sentinelKey) {
      container.withEnv('OPENAI_API_KEY', 'not-a-secret-contract-sentinel');
    }
    if (readinessPath != null) {
      container.waitingFor(
        HttpWaitStrategy(8080, path: readinessPath).forStatusCode(200)
          ..withStartupTimeout(readinessTimeout),
      );
    }
    _containers[name] = container;
    try {
      await _operation(
        container.start(),
        timeout: readinessTimeout + const Duration(seconds: 45),
        lateCleanup: () => _remove(name),
      );
      return container;
    } catch (_) {
      stderr.writeln('Go container startup failed. ${await logs(container)}');
      rethrow;
    }
  }

  Future<Uri> baseUrl(DockerContainer container) async => Uri(
    scheme: 'http',
    host: await _operation(container.containerHostIp()),
    port: await _operation(container.exposedPort(8080)),
  );

  Future<String> logs(DockerContainer container) async {
    try {
      final (out, err) = await _operation(container.logs());
      final text = utf8.decode([...out, ...err], allowMalformed: true);
      return text.length > 12000 ? text.substring(text.length - 12000) : text;
    } catch (_) {
      return 'Server logs unavailable; container name: ${container.name}';
    }
  }

  Future<int> wait(DockerContainer container) =>
      _operation(container.wait(), timeout: const Duration(seconds: 20));

  Future<void> stop(DockerContainer container) => _remove(container.name!);

  Future<void> _prepareDocker() async {
    if (testcontainersConfig.ryukDisabled) {
      throw StateError(
        'This suite requires Ryuk. Unset TESTCONTAINERS_RYUK_DISABLED / ryuk.disabled.',
      );
    }
    if (testcontainersConfig.hubImageNamePrefix.isNotEmpty) {
      throw StateError(
        'Unset TESTCONTAINERS_HUB_IMAGE_NAME_PREFIX for locally built Go images.',
      );
    }
    validateDockerEnvironment(Platform.environment);
    final configured =
        dockerHost(); // tc.host takes precedence over DOCKER_HOST.
    final actualSocket = 'unix://${dockerSocket()}';
    final libraryHost = configured != null && !configured.startsWith('unix:')
        ? configured
        : actualSocket;
    final uri = Uri.parse(libraryHost);
    if (uri.scheme != 'unix' && uri.scheme != 'tcp' && uri.scheme != 'http') {
      throw StateError(
        'This Testcontainers revision supports Unix sockets or plain TCP only. Select a local Docker endpoint.',
      );
    }
    if (configured != null &&
        configured.startsWith('unix:') &&
        await _canonical(configured) != await _canonical(actualSocket)) {
      throw StateError(
        'tc.host and the Testcontainers socket disagree. Set DOCKER_HOST to the same Unix endpoint as tc.host.',
      );
    }
    final cliHost = Platform.environment['DOCKER_HOST'];
    late final String contextName;
    late final String contextHost;
    try {
      contextName = await dockerCommand(['context', 'show']);
      contextHost = await dockerCommand([
        'context',
        'inspect',
        contextName,
        '--format',
        '{{.Endpoints.docker.Host}}',
      ]);
    } catch (error) {
      throw StateError(
        'Docker CLI context is unavailable. Install/start Docker and verify '
        '`docker context show` and `docker context inspect` succeed. $error',
      );
    }
    final effectiveCli = Platform.environment.containsKey('DOCKER_CONTEXT')
        ? contextHost
        : cliHost ?? contextHost;
    if (await _canonical(effectiveCli) != await _canonical(libraryHost)) {
      throw StateError(
        'Docker CLI context and Testcontainers use different daemons. Align DOCKER_HOST, DOCKER_CONTEXT and ~/.testcontainers.properties tc.host.',
      );
    }
    _host = libraryHost;
    try {
      await dockerCommand([
        'version',
        '--format',
        '{{.Server.Version}}',
      ], host: _host);
    } catch (_) {
      throw StateError(
        'Docker Engine is unavailable. Start Docker Desktop/Engine and verify docker version before running requires-go-server tests.',
      );
    }
    _client = DockerClient();
    await dockerCommand(
      ['pull', testcontainersConfig.ryukImage],
      host: _host,
      timeout: const Duration(minutes: 2),
    );
  }

  static Future<String> _canonical(String endpoint) async {
    final uri = Uri.parse(endpoint);
    if (uri.scheme != 'unix') return endpoint.replaceFirst(RegExp(r'/$'), '');
    try {
      return 'unix://${await File(uri.path).resolveSymbolicLinks()}';
    } on FileSystemException {
      return endpoint;
    }
  }

  Future<T> _operation<T>(
    Future<T> future, {
    Duration timeout = const Duration(seconds: 20),
    Future<void> Function()? lateCleanup,
  }) async {
    // A timeout is not cancellation. Keep every daemon operation registered
    // until it settles, including its error, before declaring cleanup complete.
    Future<void> cleanLate() async {
      if (!_closing || lateCleanup == null) return;
      try {
        await lateCleanup();
      } catch (error) {
        stderr.writeln('Late Docker operation cleanup failed: $error');
      }
    }

    late Future<void> settled;
    settled = future.then<void>(
      (_) => cleanLate(),
      onError: (Object error, StackTrace stackTrace) => cleanLate(),
    );
    _pending.add(settled);
    unawaited(settled.whenComplete(() => _pending.remove(settled)));
    return future.timeout(timeout);
  }

  Future<bool> _drainPending(Duration timeout) async {
    final deadline = DateTime.now().add(timeout);
    while (_pending.isNotEmpty) {
      final remaining = deadline.difference(DateTime.now());
      if (remaining <= Duration.zero) return false;
      try {
        await Future.wait(List<Future<void>>.of(_pending)).timeout(remaining);
      } on TimeoutException {
        return false;
      }
      // Completion callbacks remove settled work in a later microtask and a
      // late start may register its name-based cleanup here.
      await Future<void>.delayed(Duration.zero);
    }
    return true;
  }

  Future<bool> _exists(String name) async {
    try {
      await _operation(client.containerDetails(name));
      return true;
    } on HttpException catch (error) {
      if (error.message.contains('404')) return false;
      rethrow;
    }
  }

  Future<void> _remove(String name) async {
    if (!await _exists(name)) return;
    try {
      await _operation(client.stopContainer(name, timeout: 10));
    } on HttpException catch (error) {
      if (!error.message.contains('404')) rethrow;
    }
    final container = _containers[name];
    if (container != null) {
      await _operation(container.stop(force: true));
    }
    // start may have failed after create, before DockerContainer records its ID.
    if (await _exists(name)) {
      await _operation(
        client.removeContainer(name, force: true, removeVolumes: true),
      );
    }
    if (await _exists(name)) {
      throw StateError('Could not remove test container $name');
    }
  }

  Future<void> close() => _closeFuture ??= _close();

  Future<void> _close() async {
    _closing = true;
    final failures = <String>[];
    // Wait for late startup completion before final removal; do not race create.
    final operationsDrained = await _drainPending(const Duration(seconds: 45));
    if (_client != null) {
      if (operationsDrained) {
        for (final name in _containers.keys) {
          try {
            await _remove(name);
          } catch (error) {
            failures.add('Could not verify removal of $name: $error');
          }
        }
        // Preserve Ryuk as the fallback if any owned container could not be
        // removed. Once those removals are verified, attempt both singleton
        // and name-based Reaper cleanup even if the first attempt fails.
        if (failures.isEmpty) {
          try {
            await _operation(
              _deleteReaper(),
              timeout: const Duration(seconds: 30),
            );
          } catch (error) {
            failures.add('Could not delete the Testcontainers reaper: $error');
          }
          try {
            await _remove('testcontainers-ryuk-$sessionId');
          } catch (error) {
            failures.add('Could not verify Ryuk removal: $error');
          }
        }
      } else {
        failures.add(
          'Docker operations remain pending (${_pending.length}); container '
          'cleanup is unverifiable. Late cleanup remains registered for: '
          '${_containers.keys.join(', ')}. Ryuk was retained as fallback.',
        );
      }
      for (final tag in _images.values) {
        try {
          await dockerCommand(['image', 'rm', '--force', tag], host: _host);
        } catch (error) {
          if (!error.toString().contains('No such image')) {
            failures.add('Could not verify image removal: $tag');
          }
        }
      }
    }
    if (_context != null) {
      try {
        await _context!.delete(recursive: true);
      } catch (error) {
        failures.add(
          'Could not remove temporary build context ${_context!.path}: $error',
        );
      }
    }
    if (failures.isNotEmpty) throw StateError(failures.join('\n'));
  }
}

Future<void> main() async {
  final fixture = GoServerContainer();
  final done = Completer<void>();
  void finish() {
    if (!done.isCompleted) done.complete();
  }

  final interrupt = ProcessSignal.sigint.watch().listen((_) => finish());
  final terminate = ProcessSignal.sigterm.watch().listen((_) => finish());
  Timer? timer;
  try {
    await fixture.build(GoImage.contract);
    final server = await fixture.start();
    final deadline = DateTime.now().add(const Duration(minutes: 18));
    stdout.writeln(
      'AG-UI deterministic fixture: ${await fixture.baseUrl(server)}',
    );
    stdout.writeln(
      'Stops at ${deadline.toIso8601String()}; restart for another mapped port. Ctrl-C cleans up.',
    );
    stdout.writeln(
      'Prompts: fixture:plain, fixture:calculate, fixture:time, fixture:approval, fixture:card, fixture:shared-state, fixture:predictive, fixture:predictive-failure. Use the matching page. Chat also accepts fixture:delayed / fixture:interrupted; fixture:provider-failure exercises image, vision and document errors.',
    );
    timer = Timer(const Duration(minutes: 18), finish);
    await done.future;
  } finally {
    timer?.cancel();
    await interrupt.cancel();
    await terminate.cancel();
    await fixture.close();
  }
}
