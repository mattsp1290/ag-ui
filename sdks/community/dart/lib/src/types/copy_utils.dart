/// Internal helpers for three-state nullable `copyWith` parameters.
library;

import 'base.dart';

T? resolveNullableCopy<T>(Object? candidate, T? current) =>
    identical(candidate, kUnsetSentinel) ? current : candidate as T?;
