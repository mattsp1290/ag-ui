/// Internal readers for fields carried beside opaque encrypted payloads.
library;

import 'base.dart';

bool containsEncryptedValue(Object? value) {
  if (value is Map) {
    if (value.containsKey('encryptedValue') ||
        value.containsKey('encrypted_value')) {
      return true;
    }
    return value.values.any(containsEncryptedValue);
  }
  if (value is List) {
    return value.any(containsEncryptedValue);
  }
  return false;
}

void rejectUnsupportedKeys(
  Map<String, dynamic> json,
  Set<String> allowed,
  String description, {
  String field = 'outcome',
}) {
  final extra = json.keys.where((key) => !allowed.contains(key)).toList();
  if (extra.isEmpty) {
    return;
  }
  throw sanitizeValidationError(
    enclosingJson: json,
    error: AGUIValidationError(
      message: '$description contains unsupported fields: ${extra.join(', ')}',
      field: field,
      value: json[extra.first]?.runtimeType.toString(),
      json: json,
    ),
  );
}

AGUIValidationError sanitizeValidationError({
  required Map<String, dynamic> enclosingJson,
  required AGUIValidationError error,
  String? field,
}) {
  if (!containsEncryptedValue(enclosingJson)) {
    if (field == null || field == error.field) {
      return error;
    }
    return AGUIValidationError(
      message: error.message,
      field: field,
      value: error.value,
      json: error.json,
      cause: error.json == null ? error : null,
    );
  }
  final safeValue = error.value is String &&
          !_containsExactString(enclosingJson, error.value as String)
      ? error.value
      : error.value?.runtimeType.toString();
  return AGUIValidationError(
    message: _redactPayloadStrings(error.message, enclosingJson),
    field: field ?? error.field,
    value: safeValue,
  );
}

bool _containsExactString(Object? payload, String candidate) {
  if (payload is String) {
    return payload == candidate;
  }
  if (payload is Map) {
    return payload.values
        .any((value) => _containsExactString(value, candidate));
  }
  if (payload is List) {
    return payload.any((value) => _containsExactString(value, candidate));
  }
  return false;
}

String _redactPayloadStrings(String message, Object? payload) {
  final strings = <String>{};

  void collect(Object? value) {
    if (value is String && value.isNotEmpty) {
      strings.add(value);
    } else if (value is Map) {
      value.values.forEach(collect);
    } else if (value is List) {
      value.forEach(collect);
    }
  }

  collect(payload);
  final ordered = strings.toList()
    ..sort((a, b) => b.length.compareTo(a.length));
  var sanitized = message;
  for (final value in ordered) {
    sanitized = sanitized.replaceAll(value, '<redacted>');
  }
  return sanitized;
}

T? readCipherAwareOptionalField<T>(
  Map<String, dynamic> json,
  String key,
) {
  if (!containsEncryptedValue(json)) {
    return JsonDecoder.optionalField<T>(json, key);
  }
  return _readOptionalWithoutPayload<T>(json, key);
}

T? readCipherAwareOptionalEitherField<T>(
  Map<String, dynamic> json,
  String camelKey,
  String snakeKey,
) {
  if (!containsEncryptedValue(json)) {
    return JsonDecoder.optionalEitherField<T>(json, camelKey, snakeKey);
  }
  final key = json.containsKey(camelKey) ? camelKey : snakeKey;
  return _readOptionalWithoutPayload<T>(json, key);
}

AGUIValidationError wrapNestedValidationError({
  required Map<String, dynamic> enclosingJson,
  required AGUIValidationError error,
  required String field,
}) {
  return sanitizeValidationError(
    enclosingJson: enclosingJson,
    error: error,
    field: field,
  );
}

T? _readOptionalWithoutPayload<T>(
  Map<String, dynamic> json,
  String key,
) {
  if (!json.containsKey(key) || json[key] == null) {
    return null;
  }
  final value = json[key];
  if (value is! T) {
    throw AGUIValidationError(
      message:
          'Field has incorrect type. Expected $T, got ${value.runtimeType}',
      field: key,
      value: value.runtimeType.toString(),
    );
  }
  return value;
}
