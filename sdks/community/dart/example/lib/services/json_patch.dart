/// Apply the add/replace/remove subset emitted by the paired Go routes in place.
/// Callers that need an atomic batch apply this to a cloned document and commit
/// the returned root only after every operation succeeds.
dynamic applyJsonPatch(dynamic root, List<Map<String, dynamic>> ops) {
  for (final op in ops) {
    final kind = op['op'];
    final path = op['path'];
    if (kind != 'add' && kind != 'replace' && kind != 'remove') {
      throw FormatException('Unsupported patch operation: $kind');
    }
    if (path is! String || (path.isNotEmpty && !path.startsWith('/'))) {
      throw const FormatException('Patch path must be a JSON pointer');
    }
    if (kind != 'remove' && !op.containsKey('value')) {
      throw const FormatException('Patch operation requires a value');
    }
    if (path.isEmpty) {
      root = kind == 'remove' ? null : op['value'];
      continue;
    }
    final segments = path.substring(1).split('/').map(_unescape).toList();
    var parent = root;
    for (final segment in segments.take(segments.length - 1)) {
      if (parent is Map && parent.containsKey(segment)) {
        parent = parent[segment];
      } else if (parent is List) {
        parent = parent[_index(segment, parent.length)];
      } else {
        throw FormatException('Patch parent does not exist: $path');
      }
    }
    final key = segments.last;
    if (parent is Map) {
      if (kind != 'add' && !parent.containsKey(key)) {
        throw FormatException('Patch target does not exist: $path');
      }
      if (kind == 'remove') {
        parent.remove(key);
      } else {
        parent[key] = op['value'];
      }
    } else if (parent is List) {
      if (kind == 'add') {
        final index = key == '-'
            ? parent.length
            : _index(key, parent.length, insertion: true);
        parent.insert(index, op['value']);
      } else {
        final index = _index(key, parent.length);
        if (kind == 'remove') {
          parent.removeAt(index);
        } else {
          parent[index] = op['value'];
        }
      }
    } else {
      throw FormatException(
        'Patch target parent is not an object or array: $path',
      );
    }
  }
  return root;
}

String _unescape(String token) {
  if (RegExp(r'~(?:[^01]|$)').hasMatch(token)) {
    throw const FormatException('Invalid JSON pointer escape');
  }
  return token.replaceAll('~1', '/').replaceAll('~0', '~');
}

int _index(String token, int length, {bool insertion = false}) {
  if (!RegExp(r'^(0|[1-9][0-9]*)$').hasMatch(token)) {
    throw FormatException('Invalid array index: $token');
  }
  final index = int.tryParse(token);
  if (index == null || index > length || (!insertion && index == length)) {
    throw FormatException('Array index out of range: $token');
  }
  return index;
}
