import 'dart:convert';
import 'dart:io';

import 'package:crypto/crypto.dart';

/// Cached access to repository files at one immutable Git revision.
///
/// [load] reads every uncached path through one `git cat-file --batch`
/// process. Callers can then share the cached bytes, decoded text, JSON, and
/// SHA-256 digest without spawning a Git process for every assertion.
final class PinnedGitSnapshot {
  PinnedGitSnapshot(this.repositoryRoot, this.revision);

  final Directory repositoryRoot;
  final String revision;

  final Map<String, List<int>> _bytesByPath = <String, List<int>>{};
  Future<void>? _revisionCheck;

  Set<String> get loadedPaths => Set.unmodifiable(_bytesByPath.keys);

  Future<void> requireRevision() => _revisionCheck ??= _checkRevision();

  Future<void> _checkRevision() async {
    final result = await Process.run(
      'git',
      ['cat-file', '-e', '$revision^{commit}'],
      workingDirectory: repositoryRoot.path,
      stderrEncoding: utf8,
    );
    if (result.exitCode != 0) {
      throw StateError('Missing pinned Git revision $revision.');
    }
  }

  Future<void> load(Iterable<String> paths) async {
    await requireRevision();
    final requested = paths.toSet().where((path) {
      if (path.contains('\n') || path.contains('\r')) {
        throw ArgumentError.value(
          path,
          'paths',
          'Git paths cannot contain newlines',
        );
      }
      return !_bytesByPath.containsKey(path);
    }).toList()
      ..sort();
    if (requested.isEmpty) {
      return;
    }

    final process = await Process.start(
      'git',
      ['cat-file', '--batch'],
      workingDirectory: repositoryRoot.path,
    );
    final outputFuture = process.stdout.fold<List<int>>(
      <int>[],
      (output, chunk) => output..addAll(chunk),
    );
    final errorFuture = process.stderr.transform(utf8.decoder).join();
    for (final path in requested) {
      process.stdin.writeln('$revision:$path');
    }
    await process.stdin.close();

    final output = await outputFuture;
    final error = await errorFuture;
    final exitCode = await process.exitCode;
    if (exitCode != 0) {
      throw StateError('Unable to read pinned Git snapshot: ${error.trim()}');
    }
    _decodeBatch(output, requested);
  }

  void _decodeBatch(List<int> output, List<String> requested) {
    var offset = 0;
    for (final path in requested) {
      final newline = output.indexOf(0x0a, offset);
      if (newline < 0) {
        throw StateError('Truncated git cat-file header for $path.');
      }
      final header = ascii.decode(output.sublist(offset, newline));
      if (header.endsWith(' missing')) {
        throw StateError(
          'Unable to read $path at $revision: object is missing.',
        );
      }
      final parts = header.split(' ');
      if (parts.length != 3 || parts[1] != 'blob') {
        throw StateError('Unexpected git cat-file header for $path: $header');
      }
      final size = int.tryParse(parts[2]);
      if (size == null) {
        throw StateError('Invalid git cat-file size for $path: $header');
      }
      final start = newline + 1;
      final end = start + size;
      if (end >= output.length || output[end] != 0x0a) {
        throw StateError('Truncated git cat-file body for $path.');
      }
      _bytesByPath[path] = List<int>.unmodifiable(output.sublist(start, end));
      offset = end + 1;
    }
    if (offset != output.length) {
      throw StateError('Unexpected trailing data from git cat-file --batch.');
    }
  }

  Future<List<int>> bytes(String path) async {
    await load([path]);
    return _bytesByPath[path]!;
  }

  Future<String> text(String path) async => utf8.decode(await bytes(path));

  Future<Map<String, dynamic>> jsonMap(String path) async {
    final decoded = jsonDecode(await text(path));
    if (decoded case final Map<Object?, Object?> map) {
      return map.cast<String, dynamic>();
    }
    throw StateError('$path at $revision is not a JSON object.');
  }

  Future<String> sha256Hex(String path) async =>
      sha256.convert(await bytes(path)).toString();
}

Map<String, Set<String>> extractPythonModelFields(
  Iterable<String> sources,
) {
  final classes = <String, _PythonClass>{};
  final aliases = <String, String>{};
  final classPattern = RegExp(r'^class ([A-Za-z_]\w*)\(([^)]*)\):');
  final fieldPattern = RegExp(r'^    ([a-z]\w*):');
  final aliasPattern = RegExp(r'^([A-Z]\w*) = ([A-Z]\w*)\s*$');

  for (final source in sources) {
    _PythonClass? current;
    for (final line in const LineSplitter().convert(source)) {
      final classMatch = classPattern.firstMatch(line);
      if (classMatch != null) {
        current = _PythonClass(
          classMatch.group(1)!,
          classMatch
              .group(2)!
              .split(',')
              .map((base) => base.trim())
              .where((base) => base.isNotEmpty)
              .toList(),
        );
        classes[current.name] = current;
        continue;
      }
      if (line.isNotEmpty && !line.startsWith(' ')) {
        current = null;
        final aliasMatch = aliasPattern.firstMatch(line);
        if (aliasMatch != null) {
          aliases[aliasMatch.group(1)!] = aliasMatch.group(2)!;
        }
        continue;
      }
      final fieldMatch = fieldPattern.firstMatch(line);
      if (current != null && fieldMatch != null) {
        current.fields.add(_snakeToCamel(fieldMatch.group(1)!));
      }
    }
  }

  bool isModel(String name, [Set<String>? visiting]) {
    if (name == 'ConfiguredBaseModel') {
      return false;
    }
    final definition = classes[name];
    if (definition == null) {
      return false;
    }
    final seen = visiting ?? <String>{};
    if (!seen.add(name)) {
      return false;
    }
    return definition.bases.any(
      (base) =>
          base == 'ConfiguredBaseModel' ||
          base == 'BaseModel' ||
          isModel(base, seen),
    );
  }

  Set<String> fieldsFor(String name, [Set<String>? visiting]) {
    final definition = classes[name];
    if (definition == null) {
      return <String>{};
    }
    final seen = visiting ?? <String>{};
    if (!seen.add(name)) {
      return <String>{};
    }
    return {
      for (final base in definition.bases) ...fieldsFor(base, seen),
      ...definition.fields,
    };
  }

  final result = <String, Set<String>>{};
  for (final name in classes.keys.where(isModel)) {
    result[name] = fieldsFor(name);
  }
  for (final entry in aliases.entries) {
    if (result.containsKey(entry.value)) {
      result[entry.key] = {...result[entry.value]!};
    }
  }
  return result;
}

Set<String> extractPythonPublicExports(String source) {
  final allBlock = RegExp(
    r'__all__\s*=\s*\[(.*?)\]',
    dotAll: true,
  ).firstMatch(source);
  if (allBlock == null) {
    return <String>{};
  }
  return RegExp(r'["\x27]([A-Za-z_]\w*)["\x27]')
      .allMatches(allBlock.group(1)!)
      .map((match) => match.group(1)!)
      .toSet();
}

Map<String, Set<String>> extractTypeScriptSchemaFields(
  Iterable<String> sources,
) {
  final expressions = <String, String>{};
  final exported = <String>{};
  final declaration = RegExp(
    r'^(export\s+)?const\s+(\w+Schema)\s*=',
    multiLine: true,
  );
  for (final source in sources) {
    for (final match in declaration.allMatches(source)) {
      final name = match.group(2)!;
      expressions[name] = _readTypeScriptExpression(source, match.end);
      if (match.group(1) != null) {
        exported.add(name);
      }
    }
  }

  final resolved = <String, Set<String>>{};
  Set<String> resolve(String name, [Set<String>? visiting]) {
    if (resolved[name] case final fields?) {
      return fields;
    }
    final expression = expressions[name];
    if (expression == null) {
      return <String>{};
    }
    final seen = visiting ?? <String>{};
    if (!seen.add(name)) {
      return <String>{};
    }

    final baseMatch = RegExp(
      r'^\s*(\w+Schema)(?:\s*\.|\s*$)',
    ).firstMatch(expression);
    var fields = <String>{};
    if (baseMatch != null) {
      fields = {...resolve(baseMatch.group(1)!, seen)};
    }

    final omitMatch = RegExp(r'\.omit\s*\(\s*\{').firstMatch(expression);
    if (omitMatch != null) {
      final block =
          _braceBlock(expression, expression.indexOf('{', omitMatch.start));
      fields.removeAll(_topLevelTypeScriptKeys(block));
    }

    final objectMatch =
        RegExp(r'z\s*\.\s*object\s*\(\s*\{').firstMatch(expression);
    if (objectMatch != null) {
      final block =
          _braceBlock(expression, expression.indexOf('{', objectMatch.start));
      fields.addAll(_topLevelTypeScriptKeys(block));
    }
    final extendMatch = RegExp(r'\.extend\s*\(\s*\{').firstMatch(expression);
    if (extendMatch != null) {
      final block =
          _braceBlock(expression, expression.indexOf('{', extendMatch.start));
      fields.addAll(_topLevelTypeScriptKeys(block));
    }
    resolved[name] = fields;
    return fields;
  }

  return {
    for (final name in exported)
      if (resolve(name).isNotEmpty) name: {...resolve(name)},
  };
}

Set<String> extractTypeScriptExports(Iterable<String> sources) {
  final pattern = RegExp(
    r'^export\s+(?:declare\s+)?(?:const|enum|function|class|type|interface)\s+([A-Za-z_]\w*)',
    multiLine: true,
  );
  return {
    for (final source in sources)
      for (final match in pattern.allMatches(source)) match.group(1)!,
  };
}

/// Resolves the symbols exported by a TypeScript package entry point.
///
/// Both `export *` and named `export { ... } from` declarations are followed,
/// so a declaration in an internal module does not count as public unless the
/// package entry point exposes it.
Set<String> extractTypeScriptPublicExports(
  String entryPath,
  Map<String, String> sources,
) {
  final memo = <String, Set<String>>{};
  final visiting = <String>{};

  Set<String> resolve(String path) {
    if (memo[path] case final exports?) {
      return exports;
    }
    if (!visiting.add(path)) {
      throw StateError('Cyclic TypeScript export graph at $path.');
    }
    final source = sources[path];
    if (source == null) {
      throw StateError('Missing TypeScript export source $path.');
    }
    final exports = extractTypeScriptExports([source]);
    final starPattern = RegExp(
      r'''^export\s+\*\s+from\s+["']([^"']+)["']\s*;''',
      multiLine: true,
    );
    for (final match in starPattern.allMatches(source)) {
      exports.addAll(resolve(_resolveTypeScriptPath(path, match.group(1)!)));
    }
    final namedPattern = RegExp(
      r'''export(?:\s+type)?\s*\{(.*?)\}\s*from\s*["']([^"']+)["']\s*;''',
      dotAll: true,
    );
    for (final match in namedPattern.allMatches(source)) {
      final targetPath = _resolveTypeScriptPath(path, match.group(2)!);
      final targetExports = resolve(targetPath);
      final block = match.group(1)!.replaceAll(RegExp(r'//[^\n]*'), '');
      for (var name in block.split(',')) {
        name = name.trim().replaceFirst(RegExp(r'^type\s+'), '');
        if (name.isEmpty) {
          continue;
        }
        final parts = name.split(RegExp(r'\s+as\s+'));
        final original = parts.first;
        final exported = parts.last;
        if (!targetExports.contains(original)) {
          throw StateError(
            '$path re-exports missing $original from $targetPath.',
          );
        }
        exports.add(exported);
      }
    }
    visiting.remove(path);
    memo[path] = Set.unmodifiable(exports);
    return memo[path]!;
  }

  return resolve(entryPath);
}

String _resolveTypeScriptPath(String fromPath, String reference) {
  var path = Uri.parse(fromPath).resolve(reference).path;
  if (!path.endsWith('.ts')) {
    path = '$path.ts';
  }
  return path;
}

Set<String> extractGoDeclarations(Iterable<String> sources) {
  final pattern = RegExp(
    r'^(?:type|func|const|var)\s+([A-Z]\w*)',
    multiLine: true,
  );
  return {
    for (final source in sources)
      for (final match in pattern.allMatches(source)) match.group(1)!,
  };
}

Set<String> extractPublicDartDeclarations(
  String librarySource,
  Map<String, String> exportedSources, {
  Set<String>? rootExports,
}) {
  final sources = <String, String>{
    'ag_ui.dart': librarySource,
    ...exportedSources,
  };
  final memo = <String, Set<String>>{};
  final visiting = <String>{};

  Set<String> resolve(String path) {
    if (memo[path] case final declarations?) {
      return declarations;
    }
    if (!visiting.add(path)) {
      throw StateError('Cyclic Dart export graph at $path.');
    }
    final source = sources[path];
    if (source == null) {
      throw StateError('Missing Dart export source $path.');
    }
    final declarations = _extractDartDeclarations(source);
    final exportPattern = RegExp(
      r"^export '([^']+)'(?:\s+(show|hide)\s+([^;]+))?\s*;",
      multiLine: true,
    );
    for (final match in exportPattern.allMatches(source)) {
      final exportedPath = Uri.parse(path).resolve(match.group(1)!).path;
      if (path == 'ag_ui.dart' &&
          rootExports != null &&
          !rootExports.contains(exportedPath)) {
        continue;
      }
      final exported = {...resolve(exportedPath)};
      final combinator = match.group(2);
      final names = match
          .group(3)
          ?.split(',')
          .map((name) => name.trim())
          .where((name) => name.isNotEmpty)
          .toSet();
      if (combinator == 'show') {
        exported.retainAll(names!);
      } else if (combinator == 'hide') {
        exported.removeAll(names!);
      }
      declarations.addAll(exported);
    }
    visiting.remove(path);
    memo[path] = Set.unmodifiable(declarations);
    return memo[path]!;
  }

  return resolve('ag_ui.dart');
}

Set<String> _extractDartDeclarations(String source) {
  final typePattern = RegExp(
    r'^(?:(?:abstract|base|final|interface|sealed)\s+)?(?:class|enum|mixin|typedef|extension)\s+([A-Za-z_]\w*)',
    multiLine: true,
  );
  final functionPattern = RegExp(
    r'^(?:[A-Za-z_]\w*(?:<[^;=]+?>)?[?]?\s+)+([a-zA-Z]\w*)\s*\(',
    multiLine: true,
  );
  final variablePattern = RegExp(
    r'^(?:const|final)\s+(?:[A-Za-z_]\w*(?:<[^;=]+?>)?[?]?\s+)?([a-zA-Z]\w*)\s*(?:=|;)',
    multiLine: true,
  );
  return {
    for (final match in typePattern.allMatches(source))
      if (!match.group(1)!.startsWith('_')) match.group(1)!,
    for (final match in functionPattern.allMatches(source))
      if (!match.group(1)!.startsWith('_')) match.group(1)!,
    for (final match in variablePattern.allMatches(source))
      if (!match.group(1)!.startsWith('_')) match.group(1)!,
  };
}

String _readTypeScriptExpression(String source, int start) {
  var parentheses = 0;
  var braces = 0;
  var brackets = 0;
  String? quote;
  var escaped = false;
  var lineComment = false;
  var blockComment = false;
  for (var index = start; index < source.length; index++) {
    final char = source[index];
    final next = index + 1 < source.length ? source[index + 1] : '';
    if (lineComment) {
      if (char == '\n') {
        lineComment = false;
      }
      continue;
    }
    if (blockComment) {
      if (char == '*' && next == '/') {
        blockComment = false;
        index++;
      }
      continue;
    }
    if (quote != null) {
      if (escaped) {
        escaped = false;
      } else if (char == r'\') {
        escaped = true;
      } else if (char == quote) {
        quote = null;
      }
      continue;
    }
    if (char == '/' && next == '/') {
      lineComment = true;
      index++;
      continue;
    }
    if (char == '/' && next == '*') {
      blockComment = true;
      index++;
      continue;
    }
    if (char == '"' || char == "'" || char == '`') {
      quote = char;
      continue;
    }
    if (char == '(') {
      parentheses++;
    } else if (char == ')') {
      parentheses--;
    } else if (char == '{') {
      braces++;
    } else if (char == '}') {
      braces--;
    } else if (char == '[') {
      brackets++;
    } else if (char == ']') {
      brackets--;
    } else if (char == ';' &&
        parentheses == 0 &&
        braces == 0 &&
        brackets == 0) {
      return source.substring(start, index);
    }
  }
  throw StateError('Unterminated TypeScript const expression.');
}

String _braceBlock(String source, int openBrace) {
  var depth = 0;
  String? quote;
  var escaped = false;
  var lineComment = false;
  var blockComment = false;
  for (var index = openBrace; index < source.length; index++) {
    final char = source[index];
    final next = index + 1 < source.length ? source[index + 1] : '';
    if (lineComment) {
      if (char == '\n') {
        lineComment = false;
      }
      continue;
    }
    if (blockComment) {
      if (char == '*' && next == '/') {
        blockComment = false;
        index++;
      }
      continue;
    }
    if (quote != null) {
      if (escaped) {
        escaped = false;
      } else if (char == r'\') {
        escaped = true;
      } else if (char == quote) {
        quote = null;
      }
      continue;
    }
    if (char == '/' && next == '/') {
      lineComment = true;
      index++;
    } else if (char == '/' && next == '*') {
      blockComment = true;
      index++;
    } else if (char == '"' || char == "'" || char == '`') {
      quote = char;
    } else if (char == '{') {
      depth++;
    } else if (char == '}') {
      depth--;
      if (depth == 0) {
        return source.substring(openBrace + 1, index);
      }
    }
  }
  throw StateError('Unterminated TypeScript object literal.');
}

Set<String> _topLevelTypeScriptKeys(String block) {
  final matches = RegExp(
    r'^([ \t]+)([A-Za-z_]\w*)\s*:',
    multiLine: true,
  ).allMatches(block).toList();
  if (matches.isEmpty) {
    return <String>{};
  }
  final minimumIndent = matches
      .map((match) => match.group(1)!.length)
      .reduce((left, right) => left < right ? left : right);
  return {
    for (final match in matches)
      if (match.group(1)!.length == minimumIndent) match.group(2)!,
  };
}

String _snakeToCamel(String value) => value.replaceAllMapped(
      RegExp('_([a-z])'),
      (match) => match.group(1)!.toUpperCase(),
    );

final class _PythonClass {
  _PythonClass(this.name, this.bases);

  final String name;
  final List<String> bases;
  final Set<String> fields = <String>{};
}
