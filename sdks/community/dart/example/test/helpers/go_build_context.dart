import 'dart:io';

/// Copies only the two Go modules and their source/fixtures, preserving ../../.
/// Rejects links instead of letting a build traverse outside the source tree.
Future<void> copyGoBuildContext(Directory source, Directory target) async {
  if (await FileSystemEntity.type(source.path, followLinks: false) !=
      FileSystemEntityType.directory) {
    throw StateError('Go source must be a regular directory: ${source.path}');
  }
  const files = [
    'go.mod',
    'go.sum',
    '.dockerignore',
    'example/server/go.mod',
    'example/server/go.sum',
    'example/server/Dockerfile',
    'example/server/Dockerfile.contract',
  ];
  for (final relative in files) {
    await _copyFile(source, target, relative);
  }
  for (final relative in [
    'pkg',
    'example/server/cmd',
    'example/server/internal',
  ]) {
    await _requireInput(source, relative, FileSystemEntityType.directory);
    final directory = Directory.fromUri(source.uri.resolve('$relative/'));
    await for (final entry in directory.list(
      recursive: true,
      followLinks: false,
    )) {
      if (entry is Link) {
        throw StateError(
          'Go build context cannot contain symlinks: ${entry.path}',
        );
      }
      if (entry is! File) continue;
      final path = entry.uri.path.substring(source.uri.path.length);
      final segments = path.split('/');
      if (segments.any((part) => part.startsWith('.'))) continue;
      if (!path.endsWith('.go') && !segments.contains('testdata')) continue;
      // Embedded inputs need an explicit allowlist addition, never a broad copy.
      if (path.endsWith('.go') &&
          (await entry.readAsString()).contains('//go:embed')) {
        throw StateError('Review embedded build inputs before copying $path');
      }
      await _copyFile(source, target, path);
    }
  }
}

Future<void> _copyFile(
  Directory source,
  Directory target,
  String relative,
) async {
  final input = File.fromUri(source.uri.resolve(relative));
  await _requireInput(source, relative, FileSystemEntityType.file);
  final output = File.fromUri(target.uri.resolve(relative));
  await output.parent.create(recursive: true);
  await input.copy(output.path);
}

// Check every component before traversing an allowlisted root or copying a file;
// checking only the leaf would follow a linked parent outside the source tree.
Future<void> _requireInput(
  Directory source,
  String relative,
  FileSystemEntityType leafType,
) async {
  final segments = relative.split('/');
  for (var i = 0; i < segments.length; i++) {
    final path = source.uri
        .resolve(segments.take(i + 1).join('/'))
        .toFilePath();
    final expected = i == segments.length - 1
        ? leafType
        : FileSystemEntityType.directory;
    if (await FileSystemEntity.type(path, followLinks: false) != expected) {
      throw StateError(
        'Build input must be a regular $expected without links: $relative',
      );
    }
  }
}
