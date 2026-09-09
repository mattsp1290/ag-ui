/// Numeric token usage models and provider adapters.
library;

import 'base.dart';
import 'copy_utils.dart';

/// Largest integer that every AG-UI binding can represent exactly.
const int maxTokenCount = 9007199254740991;

const _countFields = <String>[
  'inputTokens',
  'outputTokens',
  'totalTokens',
  'reasoningTokens',
  'cachedInputTokens',
];

int? _parseOptionalCount(
  Object? value,
  String field, {
  bool ignoreInvalid = false,
}) {
  if (value == null) {
    return null;
  }
  if (value is! num ||
      !value.isFinite ||
      value < 0 ||
      value > maxTokenCount ||
      value.truncateToDouble() != value) {
    if (ignoreInvalid) {
      return null;
    }
    throw AGUIValidationError(
      message:
          'Token count must be a finite nonnegative whole number at most $maxTokenCount',
      field: field,
      value: value is num ? value : value.runtimeType.toString(),
    );
  }
  return value.toInt();
}

Object? _eitherValue(
  Map<String, dynamic> json,
  String camelKey,
  String snakeKey,
) =>
    json.containsKey(camelKey) ? json[camelKey] : json[snakeKey];

String? _optionalLabel(Map<String, dynamic> json, String field) {
  final value = json[field];
  if (value == null) {
    return null;
  }
  if (value is! String) {
    throw AGUIValidationError(
      message: 'Token usage label must be a string',
      field: field,
      value: value.runtimeType.toString(),
    );
  }
  return value;
}

/// Numeric-only token usage for one provider/model pair.
final class TokenUsage extends AGUIModel {
  factory TokenUsage({
    String? provider,
    String? model,
    num? inputTokens,
    num? outputTokens,
    num? totalTokens,
    num? reasoningTokens,
    num? cachedInputTokens,
  }) {
    return TokenUsage._(
      provider: provider,
      model: model,
      inputTokens: _parseOptionalCount(inputTokens, 'inputTokens'),
      outputTokens: _parseOptionalCount(outputTokens, 'outputTokens'),
      totalTokens: _parseOptionalCount(totalTokens, 'totalTokens'),
      reasoningTokens: _parseOptionalCount(reasoningTokens, 'reasoningTokens'),
      cachedInputTokens: _parseOptionalCount(
        cachedInputTokens,
        'cachedInputTokens',
      ),
    );
  }

  const TokenUsage._({
    this.provider,
    this.model,
    this.inputTokens,
    this.outputTokens,
    this.totalTokens,
    this.reasoningTokens,
    this.cachedInputTokens,
  });

  factory TokenUsage.fromJson(Map<String, dynamic> json) {
    return TokenUsage._(
      provider: _optionalLabel(json, 'provider'),
      model: _optionalLabel(json, 'model'),
      inputTokens: _parseOptionalCount(
        _eitherValue(json, 'inputTokens', 'input_tokens'),
        'inputTokens',
      ),
      outputTokens: _parseOptionalCount(
        _eitherValue(json, 'outputTokens', 'output_tokens'),
        'outputTokens',
      ),
      totalTokens: _parseOptionalCount(
        _eitherValue(json, 'totalTokens', 'total_tokens'),
        'totalTokens',
      ),
      reasoningTokens: _parseOptionalCount(
        _eitherValue(json, 'reasoningTokens', 'reasoning_tokens'),
        'reasoningTokens',
      ),
      cachedInputTokens: _parseOptionalCount(
        _eitherValue(json, 'cachedInputTokens', 'cached_input_tokens'),
        'cachedInputTokens',
      ),
    );
  }

  final String? provider;
  final String? model;
  final int? inputTokens;
  final int? outputTokens;
  final int? totalTokens;
  final int? reasoningTokens;
  final int? cachedInputTokens;

  /// Rechecks all count invariants before this value reaches an encoder.
  void validate() {
    _parseOptionalCount(inputTokens, 'inputTokens');
    _parseOptionalCount(outputTokens, 'outputTokens');
    _parseOptionalCount(totalTokens, 'totalTokens');
    _parseOptionalCount(reasoningTokens, 'reasoningTokens');
    _parseOptionalCount(cachedInputTokens, 'cachedInputTokens');
  }

  @override
  Map<String, dynamic> toJson() {
    validate();
    return {
      if (provider != null) 'provider': provider,
      if (model != null) 'model': model,
      if (inputTokens != null) 'inputTokens': inputTokens,
      if (outputTokens != null) 'outputTokens': outputTokens,
      if (totalTokens != null) 'totalTokens': totalTokens,
      if (reasoningTokens != null) 'reasoningTokens': reasoningTokens,
      if (cachedInputTokens != null) 'cachedInputTokens': cachedInputTokens,
    };
  }

  @override
  TokenUsage copyWith({
    Object? provider = kUnsetSentinel,
    Object? model = kUnsetSentinel,
    Object? inputTokens = kUnsetSentinel,
    Object? outputTokens = kUnsetSentinel,
    Object? totalTokens = kUnsetSentinel,
    Object? reasoningTokens = kUnsetSentinel,
    Object? cachedInputTokens = kUnsetSentinel,
  }) {
    return TokenUsage(
      provider: resolveNullableCopy<String>(provider, this.provider),
      model: resolveNullableCopy<String>(model, this.model),
      inputTokens: _parseOptionalCount(
        identical(inputTokens, kUnsetSentinel) ? this.inputTokens : inputTokens,
        'inputTokens',
      ),
      outputTokens: _parseOptionalCount(
        identical(outputTokens, kUnsetSentinel)
            ? this.outputTokens
            : outputTokens,
        'outputTokens',
      ),
      totalTokens: _parseOptionalCount(
        identical(totalTokens, kUnsetSentinel) ? this.totalTokens : totalTokens,
        'totalTokens',
      ),
      reasoningTokens: _parseOptionalCount(
        identical(reasoningTokens, kUnsetSentinel)
            ? this.reasoningTokens
            : reasoningTokens,
        'reasoningTokens',
      ),
      cachedInputTokens: _parseOptionalCount(
        identical(cachedInputTokens, kUnsetSentinel)
            ? this.cachedInputTokens
            : cachedInputTokens,
        'cachedInputTokens',
      ),
    );
  }
}

/// Maps the numeric portion of LangChain usage metadata into [TokenUsage].
TokenUsage? tokenUsageFromLangChainMetadata(
  Object? metadata, {
  String? provider,
  String? model,
}) {
  if (metadata is! Map) {
    return null;
  }
  Object? property(Object? value, String key) =>
      value is Map ? value[key] : null;

  final inputDetails = property(metadata, 'input_token_details');
  final outputDetails = property(metadata, 'output_token_details');
  final counts = <int?>[
    _parseOptionalCount(
      property(metadata, 'input_tokens'),
      'inputTokens',
      ignoreInvalid: true,
    ),
    _parseOptionalCount(
      property(metadata, 'output_tokens'),
      'outputTokens',
      ignoreInvalid: true,
    ),
    _parseOptionalCount(
      property(metadata, 'total_tokens'),
      'totalTokens',
      ignoreInvalid: true,
    ),
    _parseOptionalCount(
      property(outputDetails, 'reasoning'),
      'reasoningTokens',
      ignoreInvalid: true,
    ),
    _parseOptionalCount(
      property(inputDetails, 'cache_read'),
      'cachedInputTokens',
      ignoreInvalid: true,
    ),
  ];
  if (counts.every((count) => count == null)) {
    return null;
  }
  return TokenUsage(
    provider: provider,
    model: model,
    inputTokens: counts[0],
    outputTokens: counts[1],
    totalTokens: counts[2],
    reasoningTokens: counts[3],
    cachedInputTokens: counts[4],
  );
}

/// Sums usage by structural `(provider, model)` pairs in first-seen order.
List<TokenUsage> aggregateTokenUsage(Iterable<TokenUsage> entries) {
  final groups = <(String?, String?), List<int?>>{};
  var entryIndex = 0;
  for (final entry in entries) {
    entry.validate();
    final key = (entry.provider, entry.model);
    final totals = groups.putIfAbsent(key, () => List<int?>.filled(5, null));
    final counts = <int?>[
      entry.inputTokens,
      entry.outputTokens,
      entry.totalTokens,
      entry.reasoningTokens,
      entry.cachedInputTokens,
    ];
    for (var fieldIndex = 0; fieldIndex < counts.length; fieldIndex++) {
      final value = counts[fieldIndex];
      if (value == null) {
        continue;
      }
      final current = totals[fieldIndex];
      if (current != null && value > maxTokenCount - current) {
        throw AGUIValidationError(
          message: 'Aggregated token count exceeds $maxTokenCount',
          field: 'usage[$entryIndex].${_countFields[fieldIndex]}',
          value: value,
        );
      }
      totals[fieldIndex] = (current ?? 0) + value;
    }
    entryIndex++;
  }

  return [
    for (final group in groups.entries)
      TokenUsage(
        provider: group.key.$1,
        model: group.key.$2,
        inputTokens: group.value[0],
        outputTokens: group.value[1],
        totalTokens: group.value[2],
        reasoningTokens: group.value[3],
        cachedInputTokens: group.value[4],
      ),
  ];
}
