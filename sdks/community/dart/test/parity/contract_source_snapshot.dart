import 'dart:convert';
import 'dart:io';

/// Reads repository files exactly as stored at an immutable Git revision.
Future<List<int>> readPinnedBytes(
  Directory repositoryRoot,
  String revision,
  String path,
) async {
  final result = await Process.run(
    'git',
    ['show', '$revision:$path'],
    workingDirectory: repositoryRoot.path,
    stdoutEncoding: null,
    stderrEncoding: utf8,
  );
  if (result.exitCode != 0) {
    throw StateError(
      'Unable to read $path at $revision: ${(result.stderr as String).trim()}',
    );
  }
  return (result.stdout as List<int>).toList(growable: false);
}

Future<String> readPinnedText(
  Directory repositoryRoot,
  String revision,
  String path,
) async =>
    utf8.decode(await readPinnedBytes(repositoryRoot, revision, path));

Future<void> requireRevision(
  Directory repositoryRoot,
  String revision,
) async {
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

String sha256Hex(List<int> input) {
  const roundConstants = <int>[
    0x428a2f98,
    0x71374491,
    0xb5c0fbcf,
    0xe9b5dba5,
    0x3956c25b,
    0x59f111f1,
    0x923f82a4,
    0xab1c5ed5,
    0xd807aa98,
    0x12835b01,
    0x243185be,
    0x550c7dc3,
    0x72be5d74,
    0x80deb1fe,
    0x9bdc06a7,
    0xc19bf174,
    0xe49b69c1,
    0xefbe4786,
    0x0fc19dc6,
    0x240ca1cc,
    0x2de92c6f,
    0x4a7484aa,
    0x5cb0a9dc,
    0x76f988da,
    0x983e5152,
    0xa831c66d,
    0xb00327c8,
    0xbf597fc7,
    0xc6e00bf3,
    0xd5a79147,
    0x06ca6351,
    0x14292967,
    0x27b70a85,
    0x2e1b2138,
    0x4d2c6dfc,
    0x53380d13,
    0x650a7354,
    0x766a0abb,
    0x81c2c92e,
    0x92722c85,
    0xa2bfe8a1,
    0xa81a664b,
    0xc24b8b70,
    0xc76c51a3,
    0xd192e819,
    0xd6990624,
    0xf40e3585,
    0x106aa070,
    0x19a4c116,
    0x1e376c08,
    0x2748774c,
    0x34b0bcb5,
    0x391c0cb3,
    0x4ed8aa4a,
    0x5b9cca4f,
    0x682e6ff3,
    0x748f82ee,
    0x78a5636f,
    0x84c87814,
    0x8cc70208,
    0x90befffa,
    0xa4506ceb,
    0xbef9a3f7,
    0xc67178f2,
  ];
  final bytes = List<int>.from(input);
  final bitLength = input.length * 8;
  bytes.add(0x80);
  while (bytes.length % 64 != 56) {
    bytes.add(0);
  }
  for (var shift = 56; shift >= 0; shift -= 8) {
    bytes.add((bitLength >> shift) & 0xff);
  }

  final hash = <int>[
    0x6a09e667,
    0xbb67ae85,
    0x3c6ef372,
    0xa54ff53a,
    0x510e527f,
    0x9b05688c,
    0x1f83d9ab,
    0x5be0cd19,
  ];
  final schedule = List<int>.filled(64, 0);

  for (var offset = 0; offset < bytes.length; offset += 64) {
    for (var index = 0; index < 16; index++) {
      final start = offset + index * 4;
      schedule[index] = (bytes[start] << 24) |
          (bytes[start + 1] << 16) |
          (bytes[start + 2] << 8) |
          bytes[start + 3];
    }
    for (var index = 16; index < 64; index++) {
      final s0 = _rotateRight(schedule[index - 15], 7) ^
          _rotateRight(schedule[index - 15], 18) ^
          (schedule[index - 15] >> 3);
      final s1 = _rotateRight(schedule[index - 2], 17) ^
          _rotateRight(schedule[index - 2], 19) ^
          (schedule[index - 2] >> 10);
      schedule[index] =
          (schedule[index - 16] + s0 + schedule[index - 7] + s1) & 0xffffffff;
    }

    var a = hash[0];
    var b = hash[1];
    var c = hash[2];
    var d = hash[3];
    var e = hash[4];
    var f = hash[5];
    var g = hash[6];
    var h = hash[7];
    for (var index = 0; index < 64; index++) {
      final sum1 =
          _rotateRight(e, 6) ^ _rotateRight(e, 11) ^ _rotateRight(e, 25);
      final choice = (e & f) ^ ((~e) & g);
      final temp1 =
          (h + sum1 + choice + roundConstants[index] + schedule[index]) &
              0xffffffff;
      final sum0 =
          _rotateRight(a, 2) ^ _rotateRight(a, 13) ^ _rotateRight(a, 22);
      final majority = (a & b) ^ (a & c) ^ (b & c);
      final temp2 = (sum0 + majority) & 0xffffffff;
      h = g;
      g = f;
      f = e;
      e = (d + temp1) & 0xffffffff;
      d = c;
      c = b;
      b = a;
      a = (temp1 + temp2) & 0xffffffff;
    }
    final working = [a, b, c, d, e, f, g, h];
    for (var index = 0; index < hash.length; index++) {
      hash[index] = (hash[index] + working[index]) & 0xffffffff;
    }
  }
  return hash.map((word) => word.toRadixString(16).padLeft(8, '0')).join();
}

int _rotateRight(int value, int distance) =>
    ((value >> distance) | (value << (32 - distance))) & 0xffffffff;

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
  Map<String, String> exportedSources,
) {
  final exportPattern = RegExp(
    r"^export '([^']+)'(?:\s+(?:show|hide)\s+[^;]+)?\s*;",
    multiLine: true,
  );
  final declarationPattern = RegExp(
    r'^(?:(?:abstract|base|final|interface|sealed)\s+)?(?:class|enum|mixin|typedef)\s+([A-Za-z_]\w*)',
    multiLine: true,
  );
  final names = <String>{};
  final pending = <MapEntry<String, String>>[
    MapEntry('ag_ui.dart', librarySource),
  ];
  final visited = <String>{};
  while (pending.isNotEmpty) {
    final current = pending.removeLast();
    if (!visited.add(current.key)) {
      continue;
    }
    final source = current.value;
    names.addAll(
      declarationPattern
          .allMatches(source)
          .map((declaration) => declaration.group(1)!),
    );
    for (final match in exportPattern.allMatches(source)) {
      final exportedPath = Uri.parse(current.key).resolve(match.group(1)!).path;
      final exportedSource = exportedSources[exportedPath];
      if (exportedSource != null) {
        pending.add(MapEntry(exportedPath, exportedSource));
      }
    }
  }
  return names;
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
