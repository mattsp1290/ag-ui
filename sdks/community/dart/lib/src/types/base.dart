/// Base types for AG-UI protocol models.
///
/// This library provides the foundational types and utilities for the AG-UI
/// protocol implementation in Dart.
library;

import 'dart:convert';

/// Base class for all AG-UI models with JSON serialization support.
///
/// All protocol models extend this class to provide consistent JSON
/// serialization and deserialization capabilities.
abstract class AGUIModel {
  const AGUIModel();

  /// Converts this model to a JSON map.
  Map<String, dynamic> toJson();

  /// Converts this model to a JSON string.
  String toJsonString() => json.encode(toJson());

  /// Creates a copy of this model with optional field updates.
  /// Subclasses should override this with their specific type.
  AGUIModel copyWith();
}

/// Mixin for models with type discriminators.
///
/// Used by event and message types to provide a type field for
/// polymorphic deserialization.
mixin TypeDiscriminator {
  /// The type discriminator field value.
  String get type;
}

/// Represents a validation error during JSON decoding.
///
/// Thrown when JSON data does not match the expected schema for
/// AG-UI protocol models.
///
/// Note on the two-class error setup: this class is thrown by `fromJson`
/// factories (the wire-decoding boundary) and does NOT extend
/// `AgUiError`. The separate `ValidationError` in
/// `lib/src/client/errors.dart` is thrown by `Validators.requireNonEmpty`
/// inside `EventDecoder.validate`. When events are decoded through the
/// public [EventDecoder] pipeline, both classes are caught and re-thrown
/// as `DecodingError` — see `decoder.dart` for the wrapping logic. Direct
/// callers of `Event.fromJson` see this `AGUIValidationError` directly.
class AGUIValidationError implements Exception {
  final String message;
  final String? field;
  final dynamic value;
  final Map<String, dynamic>? json;

  const AGUIValidationError({
    required this.message,
    this.field,
    this.value,
    this.json,
  });

  @override
  String toString() {
    final buffer = StringBuffer('AGUIValidationError: $message');
    if (field != null) buffer.write(' (field: $field)');
    if (value != null) buffer.write(' (value: $value)');
    return buffer.toString();
  }
}

/// Base exception for AG-UI protocol errors.
///
/// The root exception class for all AG-UI protocol-related errors.
class AGUIError implements Exception {
  final String message;

  const AGUIError(this.message);

  @override
  String toString() => 'AGUIError: $message';
}

/// Utility for tolerant JSON decoding that ignores unknown fields.
///
/// Provides helper methods for safely extracting and validating fields
/// from JSON maps, with proper error handling.
class JsonDecoder {
  /// Safely extracts a required field from JSON.
  static T requireField<T>(
    Map<String, dynamic> json,
    String field, {
    T Function(dynamic)? transform,
  }) {
    if (!json.containsKey(field)) {
      throw AGUIValidationError(
        message: 'Missing required field',
        field: field,
        json: json,
      );
    }

    final value = json[field];
    if (value == null) {
      throw AGUIValidationError(
        message: 'Required field is null',
        field: field,
        value: value,
        json: json,
      );
    }

    if (transform != null) {
      try {
        return transform(value);
      } catch (e) {
        throw AGUIValidationError(
          message: 'Failed to transform field: $e',
          field: field,
          value: value,
          json: json,
        );
      }
    }

    if (value is! T) {
      throw AGUIValidationError(
        message: 'Field has incorrect type. Expected $T, got ${value.runtimeType}',
        field: field,
        value: value,
        json: json,
      );
    }

    return value;
  }

  /// Safely extracts an optional field from JSON.
  static T? optionalField<T>(
    Map<String, dynamic> json,
    String field, {
    T Function(dynamic)? transform,
  }) {
    if (!json.containsKey(field) || json[field] == null) {
      return null;
    }

    final value = json[field];
    
    if (transform != null) {
      try {
        return transform(value);
      } catch (e) {
        throw AGUIValidationError(
          message: 'Failed to transform field: $e',
          field: field,
          value: value,
          json: json,
        );
      }
    }

    if (value is! T) {
      throw AGUIValidationError(
        message: 'Field has incorrect type. Expected $T, got ${value.runtimeType}',
        field: field,
        value: value,
        json: json,
      );
    }

    return value;
  }

  /// Reads a required field that may arrive under either of two keys.
  ///
  /// Servers in this protocol use camelCase (TypeScript) or snake_case
  /// (Python) field names interchangeably. This helper tries [camelKey]
  /// first (canonical), then [snakeKey], and throws an
  /// [AGUIValidationError] naming BOTH keys if neither is present —
  /// avoiding the misleading "missing message_id" error when the caller
  /// actually sent `messageId`.
  static T requireEitherField<T>(
    Map<String, dynamic> json,
    String camelKey,
    String snakeKey,
  ) {
    final v = optionalField<T>(json, camelKey) ??
        optionalField<T>(json, snakeKey);
    if (v == null) {
      throw AGUIValidationError(
        message: 'Missing required field "$camelKey" (or "$snakeKey")',
        field: camelKey,
        json: json,
      );
    }
    return v;
  }

  /// Reads an optional field that may arrive under either of two keys.
  ///
  /// Returns the camelCase value if present, otherwise the snake_case
  /// value, otherwise null.
  static T? optionalEitherField<T>(
    Map<String, dynamic> json,
    String camelKey,
    String snakeKey,
  ) {
    return optionalField<T>(json, camelKey) ??
        optionalField<T>(json, snakeKey);
  }

  /// Safely extracts a list field from JSON.
  ///
  /// Use this when the elements have a concrete element type that the SDK
  /// strongly types (`requireListField<Map<String, dynamic>>` for nested
  /// records, etc.) — the inner `cast<T>()` step provides the type safety.
  /// For loosely-typed payloads where the elements are intentionally
  /// `dynamic` (e.g. JSON Patch operations in `STATE_DELTA` / `ACTIVITY_DELTA`)
  /// prefer `requireField<List<dynamic>>` to avoid an unnecessary cast.
  static List<T> requireListField<T>(
    Map<String, dynamic> json,
    String field, {
    T Function(dynamic)? itemTransform,
  }) {
    final list = requireField<List<dynamic>>(json, field);
    
    if (itemTransform != null) {
      return list.map((item) {
        try {
          return itemTransform(item);
        } catch (e) {
          throw AGUIValidationError(
            message: 'Failed to transform list item: $e',
            field: field,
            value: item,
            json: json,
          );
        }
      }).toList();
    }

    return list.cast<T>();
  }

  /// Safely extracts an optional list field from JSON.
  static List<T>? optionalListField<T>(
    Map<String, dynamic> json,
    String field, {
    T Function(dynamic)? itemTransform,
  }) {
    final list = optionalField<List<dynamic>>(json, field);
    if (list == null) return null;
    
    if (itemTransform != null) {
      return list.map((item) {
        try {
          return itemTransform(item);
        } catch (e) {
          throw AGUIValidationError(
            message: 'Failed to transform list item: $e',
            field: field,
            value: item,
            json: json,
          );
        }
      }).toList();
    }

    return list.cast<T>();
  }
}

/// Converts snake_case to camelCase
String snakeToCamel(String snake) {
  final parts = snake.split('_');
  if (parts.isEmpty) return snake;
  
  return parts.first + 
    parts.skip(1).map((part) => 
      part.isEmpty ? '' : part[0].toUpperCase() + part.substring(1)
    ).join();
}

/// Converts camelCase to snake_case
String camelToSnake(String camel) {
  return camel.replaceAllMapped(
    RegExp(r'[A-Z]'),
    (match) => '_${match.group(0)!.toLowerCase()}',
  ).replaceFirst(RegExp(r'^_'), '');
}