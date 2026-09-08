import 'dart:async';
import 'dart:convert';
import 'dart:io';

void validateDockerEnvironment(Map<String, String> environment) {
  final tlsVariables = [
    'DOCKER_TLS',
    'DOCKER_TLS_VERIFY',
  ].where((name) => (environment[name] ?? '').trim().isNotEmpty);
  if (tlsVariables.isNotEmpty) {
    throw StateError(
      'TLS-enabled Docker endpoints are unsupported by this pinned '
      'Testcontainers client. Unset ${tlsVariables.join(' and ')} and use '
      'a local Unix socket or plain TCP endpoint.',
    );
  }
}

/// Runs only Docker image/configuration commands. Container lifecycle belongs
/// to testcontainers_core. Retains a bounded tail and always reaps the process.
Future<String> dockerCommand(
  List<String> arguments, {
  String executable = 'docker',
  String? host,
  String? workingDirectory,
  Duration timeout = const Duration(seconds: 30),
}) async {
  late final Process process;
  try {
    process = await Process.start(executable, [
      if (host != null) ...['--host', host],
      ...arguments,
    ], workingDirectory: workingDirectory);
  } on ProcessException catch (error) {
    throw StateError(
      'Docker CLI is unavailable. Install Docker and ensure the docker '
      'executable is on PATH. Failed to start "$executable": $error',
    );
  }
  var output = '';
  void collect(String chunk) {
    output += chunk;
    if (output.length > 32000) {
      output = output.substring(output.length - 32000);
    }
  }

  final stdoutDone = process.stdout.transform(utf8.decoder).forEach(collect);
  final stderrDone = process.stderr.transform(utf8.decoder).forEach(collect);
  try {
    final code = await process.exitCode.timeout(timeout);
    await Future.wait([stdoutDone, stderrDone]);
    if (code != 0) {
      throw StateError('Docker ${arguments.first} failed ($code):\n$output');
    }
    return output.trim();
  } on TimeoutException {
    process.kill(ProcessSignal.sigterm);
    try {
      await process.exitCode.timeout(const Duration(seconds: 3));
    } on TimeoutException {
      process.kill(ProcessSignal.sigkill);
      await process.exitCode;
    }
    await Future.wait([stdoutDone, stderrDone]);
    throw TimeoutException(
      'Docker ${arguments.first} exceeded $timeout; process stopped.\n$output',
    );
  }
}

void main(List<String> arguments) {
  if (arguments.contains('--validate-environment')) {
    validateDockerEnvironment(Platform.environment);
  }
}
