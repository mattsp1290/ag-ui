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

/// Base exception for AG-UI protocol errors.
///
/// The root exception class for all AG-UI protocol-related errors.
/// `AgUiError` (lib/src/client/errors.dart) and [AGUIValidationError]
/// both extend this class — so callers can catch the entire SDK error
/// surface with `on AGUIError`. Catching `on AgUiError` covers
/// transport / decoder / runtime errors but NOT direct-factory
/// `AGUIValidationError`. See README → "Errors" for the catch-recipe.
class AGUIError implements Exception {
  /// Human-readable error message.
  final String message;

  const AGUIError(this.message);

  @override
  String toString() => 'AGUIError: $message';
}

/// Represents a validation error during JSON decoding.
///
/// Thrown by `fromJson` factories at the wire-decoding boundary. Extends
/// [AGUIError] so `on AGUIError` catches both factory-side and
/// runtime-side failures uniformly. The separate `ValidationError` in
/// `lib/src/client/errors.dart` is thrown by `Validators.requireNonEmpty`
/// inside `EventDecoder.validate`. When events are decoded through the
/// public [EventDecoder] pipeline, both classes are caught and re-thrown
/// as `DecodingError` — see `decoder.dart` for the wrapping logic. Direct
/// callers of `Event.fromJson` see this `AGUIValidationError` directly.
class AGUIValidationError extends AGUIError {
  final String? field;
  final dynamic value;
  final Map<String, dynamic>? json;

  /// Originating exception, if this validation error was raised in
  /// response to another error (e.g. a wrong-typed field caught inside a
  /// `transform` callback). Preserves structured info that would
  /// otherwise be flattened by `'$e'` interpolation.
  final Object? cause;

  const AGUIValidationError({
    required String message,
    this.field,
    this.value,
    this.json,
    this.cause,
  }) : super(message);

  @override
  String toString() {
    final buffer = StringBuffer('AGUIValidationError: $message');
    if (field != null) buffer.write(' (field: $field)');
    if (value != null) buffer.write(' (value: $value)');
    if (cause != null) buffer.write('\nCaused by: $cause');
    return buffer.toString();
  }
}

/// Utility for tolerant JSON decoding that ignores unknown fields.
///
/// Provides helper methods for safely extracting and validating fields
/// from JSON maps, with proper error handling.
///
/// camelCase/snake_case parity is handled by [requireEitherField] and
/// [optionalEitherField] for keys whose two spellings differ —
/// e.g. `messageId` / `message_id`, `toolCallId` / `tool_call_id`,
/// `parentRunId` / `parent_run_id`. Single-word keys whose camelCase and
/// snake_case spellings are identical (`delta`, `name`, `title`,
/// `replace`, `content`, `value`, `event`, `source`, `code`, `subtype`,
/// `messages`, `patch`, `snapshot`, `role`, `result`, `input`,
/// `timestamp`, `details`, `error`, `state`) are read with the bare
/// [requireField] / [optionalField] helpers — they don't need
/// `*EitherField` because there's no second spelling to fall back to.
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
  ///
  /// Note on short-circuit behavior: if [camelKey] is present but holds
  /// a wrong-typed value, [optionalField] throws and the [snakeKey]
  /// fallback is NOT attempted. This is intentional — a payload that
  /// carries both keys with conflicting types is itself a protocol
  /// violation, and surfacing the type error at [camelKey] is more
  /// useful than silently rescuing via the snake_case alias. The same
  /// rule applies to [optionalEitherField].
  ///
  /// Note on falsy non-null values: the `??` chain only fires on `null`,
  /// so a falsy non-null value at [camelKey] (`false`, `0`, `""`, an
  /// empty list/map) is preserved and the [snakeKey] fallback is not
  /// consulted. This matters for any future `T` other than `String` —
  /// e.g. `requireEitherField<bool>(json, 'replace', 'replace_all')`
  /// returns `false` when `camelKey` carries `false`, not `null`,
  /// keeping the canonical-key value in the camelCase preference order.
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

  /// Reads an optional integer field, accepting either `int` or `num`
  /// on the wire.
  ///
  /// JS/TS producers serialize all numbers through a single Number type,
  /// so a server emitting `Date.now() / 1000` (or any fractional value)
  /// arrives in Dart as `double`. `optionalField<int>` rejects that with
  /// `AGUIValidationError` even when the value is integer-shaped. This
  /// helper accepts any `num` and coerces via `.toInt()`, fixing the
  /// cross-runtime decode for `timestamp`-shaped fields.
  static int? optionalIntField(
    Map<String, dynamic> json,
    String field,
  ) {
    if (!json.containsKey(field) || json[field] == null) return null;
    final value = json[field];
    if (value is int) return value;
    if (value is num) return value.toInt();
    throw AGUIValidationError(
      message:
          'Field has incorrect type. Expected int or num, got ${value.runtimeType}',
      field: field,
      value: value,
      json: json,
    );
  }

  /// Safely extracts a list field from JSON.
  ///
  /// Use this when the elements have a concrete element type that the SDK
  /// strongly types (`requireListField<Map<String, dynamic>>` for nested
  /// records, etc.) — the inner per-element type check provides the type
  /// safety. Wrong-typed elements raise [AGUIValidationError] eagerly with
  /// `field: '$field[$i]'` so the decoder pipeline can preserve the
  /// originating index instead of flattening to a generic `field: 'json'`.
  /// For loosely-typed payloads where the elements are intentionally
  /// `dynamic` (e.g. JSON Patch operations in `STATE_DELTA` /
  /// `ACTIVITY_DELTA`) prefer `requireField<List<dynamic>>` to avoid an
  /// unnecessary check.
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
            message: 'Failed to transform list item',
            field: field,
            value: item,
            json: json,
            cause: e,
          );
        }
      }).toList();
    }

    return _eagerCast<T>(list, field, json);
  }

  /// Safely extracts an optional list field from JSON.
  ///
  /// Mirrors [requireListField]'s eager element-type validation when no
  /// transform is supplied, so a malformed list element raises
  /// [AGUIValidationError] with the originating index instead of leaking
  /// a `TypeError` to the decoder catch-all.
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
            message: 'Failed to transform list item',
            field: field,
            value: item,
            json: json,
            cause: e,
          );
        }
      }).toList();
    }

    return _eagerCast<T>(list, field, json);
  }

  /// Reads an optional list field that may arrive under either of two
  /// keys, with the same eager element-type validation as
  /// [optionalListField] / [requireListField].
  ///
  /// Composes the dual-key resolution rule from [optionalEitherField]
  /// (camelCase wins when present, even when the list is empty; snake_case
  /// is consulted ONLY when camelCase is absent) with the index-aware
  /// element-type errors from [_eagerCast]. Use this when a list-shaped
  /// field has both camelCase and snake_case wire spellings AND the
  /// elements have a concrete type the SDK strongly types.
  ///
  /// The behavior matches [optionalListField] when [itemTransform] is
  /// supplied: the transform is wrapped in a per-element try/catch
  /// producing an [AGUIValidationError] (without index info, for
  /// transform-side failures). Without [itemTransform], element type
  /// mismatches are reported with `field: '$camelKey[$i]'`.
  static List<T>? optionalEitherListField<T>(
    Map<String, dynamic> json,
    String camelKey,
    String snakeKey, {
    T Function(dynamic)? itemTransform,
  }) {
    final list = optionalEitherField<List<dynamic>>(json, camelKey, snakeKey);
    if (list == null) return null;

    if (itemTransform != null) {
      return list.map((item) {
        try {
          return itemTransform(item);
        } catch (e) {
          throw AGUIValidationError(
            message: 'Failed to transform list item',
            field: camelKey,
            value: item,
            json: json,
            cause: e,
          );
        }
      }).toList();
    }

    return _eagerCast<T>(list, camelKey, json);
  }

  /// Eagerly validates element types in a list and returns a typed copy.
  ///
  /// Replaces `list.cast<T>()`'s lazy view (which raises a raw `TypeError`
  /// at access time, swallowed by the decoder catch-all and flattened to
  /// `field: 'json'`) with a fail-fast loop that names the bad index.
  static List<T> _eagerCast<T>(
    List<dynamic> list,
    String field,
    Map<String, dynamic> json,
  ) {
    final out = <T>[];
    for (var i = 0; i < list.length; i++) {
      final item = list[i];
      if (item is! T) {
        throw AGUIValidationError(
          message:
              'List item has incorrect type. Expected $T, got ${item.runtimeType}',
          field: '$field[$i]',
          value: item,
          json: json,
        );
      }
      out.add(item);
    }
    return out;
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