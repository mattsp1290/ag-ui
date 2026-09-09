/// Typed run completion outcomes for the AG-UI protocol.
library;

import 'base.dart';
import 'interrupt.dart';
import 'wire_safety.dart';

AGUIValidationError _nestedInterruptError(
  AGUIValidationError error,
  int index,
) {
  return AGUIValidationError(
    message: error.message,
    field: 'interrupts[$index].${error.field ?? 'unknown'}',
    value: error.value,
    json: error.json,
    cause: error.json == null ? error : null,
  );
}

/// A strict discriminated outcome attached to a finished run.
sealed class RunFinishedOutcome extends AGUIModel {
  const RunFinishedOutcome();

  factory RunFinishedOutcome.fromJson(Map<String, dynamic> json) {
    try {
      final type = JsonDecoder.requireField<String>(json, 'type');
      return switch (type) {
        'success' => RunFinishedSuccessOutcome.fromJson(json),
        'interrupt' => RunFinishedInterruptOutcome.fromJson(json),
        _ => throw AGUIValidationError(
            message: 'Unknown run finished outcome type: $type',
            field: 'type',
            value: type,
            json: json,
          ),
      };
    } on AGUIValidationError catch (error) {
      throw sanitizeValidationError(
        enclosingJson: json,
        error: error,
      );
    }
  }

  String get type;
}

/// A run that completed normally.
final class RunFinishedSuccessOutcome extends RunFinishedOutcome {
  const RunFinishedSuccessOutcome();

  factory RunFinishedSuccessOutcome.fromJson(Map<String, dynamic> json) {
    try {
      rejectUnsupportedKeys(json, const {'type'}, 'Success outcome');
      final type = JsonDecoder.requireField<String>(json, 'type');
      if (type != 'success') {
        throw AGUIValidationError(
          message: 'Expected success outcome',
          field: 'type',
          value: type,
          json: json,
        );
      }
      return const RunFinishedSuccessOutcome();
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
  RunFinishedSuccessOutcome copyWith() => const RunFinishedSuccessOutcome();
}

/// A run paused on one or more interrupts.
final class RunFinishedInterruptOutcome extends RunFinishedOutcome {
  factory RunFinishedInterruptOutcome({required List<Interrupt> interrupts}) {
    if (interrupts.isEmpty) {
      throw const AGUIValidationError(
        message: "Outcome 'interrupt' requires at least one interrupt",
        field: 'interrupts',
      );
    }
    return RunFinishedInterruptOutcome._(List.unmodifiable(interrupts));
  }

  const RunFinishedInterruptOutcome._(this.interrupts);

  factory RunFinishedInterruptOutcome.fromJson(Map<String, dynamic> json) {
    try {
      rejectUnsupportedKeys(
        json,
        const {'type', 'interrupts'},
        'Interrupt outcome',
      );
      final type = JsonDecoder.requireField<String>(json, 'type');
      if (type != 'interrupt') {
        throw AGUIValidationError(
          message: 'Expected interrupt outcome',
          field: 'type',
          value: type,
          json: json,
        );
      }
      final raw = JsonDecoder.requireListField<Map<String, dynamic>>(
        json,
        'interrupts',
      );
      final interrupts = <Interrupt>[];
      for (var index = 0; index < raw.length; index++) {
        try {
          interrupts.add(Interrupt.fromJson(raw[index]));
        } on AGUIValidationError catch (error) {
          throw _nestedInterruptError(error, index);
        }
      }
      return RunFinishedInterruptOutcome(interrupts: interrupts);
    } on AGUIValidationError catch (error) {
      throw sanitizeValidationError(
        enclosingJson: json,
        error: error,
      );
    }
  }

  final List<Interrupt> interrupts;

  @override
  String get type => 'interrupt';

  @override
  Map<String, dynamic> toJson() {
    if (interrupts.isEmpty) {
      throw const AGUIValidationError(
        message: "Outcome 'interrupt' requires at least one interrupt",
        field: 'interrupts',
      );
    }
    return {
      'type': type,
      'interrupts': interrupts.map((interrupt) => interrupt.toJson()).toList(),
    };
  }

  @override
  RunFinishedInterruptOutcome copyWith({List<Interrupt>? interrupts}) {
    return RunFinishedInterruptOutcome(
      interrupts: interrupts ?? this.interrupts,
    );
  }
}
