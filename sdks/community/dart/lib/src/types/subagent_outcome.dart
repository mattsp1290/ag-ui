/// Typed subagent completion outcomes for the AG-UI protocol.
library;

import 'base.dart';
import 'copy_utils.dart';
import 'wire_safety.dart';

/// A strict discriminated outcome attached to a finished subagent.
sealed class SubagentFinishedOutcome extends AGUIModel {
  const SubagentFinishedOutcome();

  factory SubagentFinishedOutcome.fromJson(Map<String, dynamic> json) {
    try {
      final type = JsonDecoder.requireField<String>(json, 'type');
      return switch (type) {
        'success' => SubagentFinishedSuccessOutcome.fromJson(json),
        'suspended' => SubagentFinishedSuspendedOutcome.fromJson(json),
        _ => throw AGUIValidationError(
            message: 'Unknown subagent finished outcome type: $type',
            field: 'type',
            value: type,
            json: json,
          ),
      };
    } on AGUIValidationError catch (error) {
      throw sanitizeValidationError(enclosingJson: json, error: error);
    }
  }

  String get type;
}

/// A subagent that completed normally.
final class SubagentFinishedSuccessOutcome extends SubagentFinishedOutcome {
  const SubagentFinishedSuccessOutcome();

  factory SubagentFinishedSuccessOutcome.fromJson(Map<String, dynamic> json) {
    try {
      rejectUnsupportedKeys(json, const {'type'}, 'Success subagent outcome');
      final type = JsonDecoder.requireField<String>(json, 'type');
      if (type != 'success') {
        throw AGUIValidationError(
          message: 'Expected success subagent outcome',
          field: 'type',
          value: type,
          json: json,
        );
      }
      return const SubagentFinishedSuccessOutcome();
    } on AGUIValidationError catch (error) {
      throw sanitizeValidationError(
        enclosingJson: json,
        error: error,
      );
    }
  }

  @override
  String get type => 'success';

  @override
  Map<String, dynamic> toJson() => const {'type': 'success'};

  @override
  SubagentFinishedSuccessOutcome copyWith() =>
      const SubagentFinishedSuccessOutcome();
}

/// A subagent paused until its run-level interrupt is resolved.
final class SubagentFinishedSuspendedOutcome extends SubagentFinishedOutcome {
  const SubagentFinishedSuspendedOutcome({this.interruptIds});

  factory SubagentFinishedSuspendedOutcome.fromJson(Map<String, dynamic> json) {
    try {
      rejectUnsupportedKeys(
        json,
        const {'type', 'interruptIds', 'interrupt_ids'},
        'Suspended subagent outcome',
      );
      final type = JsonDecoder.requireField<String>(json, 'type');
      if (type != 'suspended') {
        throw AGUIValidationError(
          message: 'Expected suspended subagent outcome',
          field: 'type',
          value: type,
          json: json,
        );
      }
      return SubagentFinishedSuspendedOutcome(
        interruptIds: JsonDecoder.optionalEitherListField<String>(
          json,
          'interruptIds',
          'interrupt_ids',
        ),
      );
    } on AGUIValidationError catch (error) {
      throw sanitizeValidationError(enclosingJson: json, error: error);
    }
  }

  /// Run-level interrupts directly owned by this subagent.
  ///
  /// Omitted or empty is valid when only a descendant owns the interrupt.
  final List<String>? interruptIds;

  @override
  String get type => 'suspended';

  @override
  Map<String, dynamic> toJson() => {
        'type': 'suspended',
        if (interruptIds != null) 'interruptIds': interruptIds,
      };

  @override
  SubagentFinishedSuspendedOutcome copyWith({
    Object? interruptIds = kUnsetSentinel,
  }) {
    return SubagentFinishedSuspendedOutcome(
      interruptIds: resolveNullableCopy<List<String>>(
        interruptIds,
        this.interruptIds,
      ),
    );
  }
}
