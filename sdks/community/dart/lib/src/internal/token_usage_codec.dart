import '../types/base.dart';
import '../types/token_usage.dart';

T _atUsageIndex<T>(int index, T Function() action) {
  try {
    return action();
  } on AGUIValidationError catch (error) {
    throw AGUIValidationError(
      message: error.message,
      field: 'usage[$index].${error.field ?? 'unknown'}',
      value: error.value is num
          ? error.value
          : error.value?.runtimeType.toString(),
    );
  }
}

List<TokenUsage>? readTokenUsageList(Map<String, dynamic> json) {
  if (!json.containsKey('usage') || json['usage'] == null) {
    return null;
  }
  List<Map<String, dynamic>> raw;
  try {
    raw = JsonDecoder.requireListField<Map<String, dynamic>>(json, 'usage');
  } on AGUIValidationError catch (error) {
    throw AGUIValidationError(
      message: error.message,
      field: error.field,
      value: error.value?.runtimeType.toString(),
    );
  }
  return [
    for (var index = 0; index < raw.length; index++)
      _atUsageIndex(index, () => TokenUsage.fromJson(raw[index])),
  ];
}

void validateTokenUsageList(List<TokenUsage>? usage) {
  if (usage == null) {
    return;
  }
  for (var index = 0; index < usage.length; index++) {
    _atUsageIndex(index, usage[index].validate);
  }
}

List<Map<String, dynamic>>? encodeTokenUsageList(List<TokenUsage>? usage) {
  if (usage == null) {
    return null;
  }
  return [
    for (var index = 0; index < usage.length; index++)
      _atUsageIndex(index, usage[index].toJson),
  ];
}
