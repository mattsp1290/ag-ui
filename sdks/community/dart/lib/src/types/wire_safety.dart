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
  final sensitive = containsEncryptedValue(enclosingJson);
  return AGUIValidationError(
    message: error.message,
    field: field,
    value: sensitive ? error.value?.runtimeType.toString() : error.value,
    cause: sensitive ? null : (error.json == null ? error : null),
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
