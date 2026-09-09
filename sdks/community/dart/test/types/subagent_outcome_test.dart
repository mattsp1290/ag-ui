import 'package:ag_ui/ag_ui.dart';
import 'package:test/test.dart';

void _expectCipherSafe(AGUIValidationError error, String secret) {
  expect(error.message, isNot(contains(secret)));
  expect(error.value?.toString(), isNot(contains(secret)));
  expect(error.json?.toString(), isNot(contains(secret)));
  expect(error.cause?.toString(), isNot(contains(secret)));
}

void main() {
  group('SubagentFinishedOutcome', () {
    test('success emits exactly its discriminator', () {
      const outcome = SubagentFinishedSuccessOutcome();

      expect(outcome.toJson(), {'type': 'success'});
      expect(
        SubagentFinishedOutcome.fromJson(outcome.toJson()),
        isA<SubagentFinishedSuccessOutcome>(),
      );
      expect(outcome.copyWith().toJson(), {'type': 'success'});
    });

    test('suspended preserves omitted, empty, and populated interrupt IDs', () {
      for (final outcome in <SubagentFinishedSuspendedOutcome>[
        const SubagentFinishedSuspendedOutcome(),
        const SubagentFinishedSuspendedOutcome(interruptIds: []),
        const SubagentFinishedSuspendedOutcome(
          interruptIds: ['interrupt-1', 'interrupt-2'],
        ),
      ]) {
        expect(
          SubagentFinishedOutcome.fromJson(outcome.toJson()).toJson(),
          outcome.toJson(),
        );
      }
      expect(
        const SubagentFinishedSuspendedOutcome().toJson().containsKey(
              'interruptIds',
            ),
        isFalse,
      );
      expect(
        const SubagentFinishedSuspendedOutcome(interruptIds: []).toJson(),
        {'type': 'suspended', 'interruptIds': <String>[]},
      );
    });

    test('reads snake_case and gives camelCase key presence precedence', () {
      expect(
        SubagentFinishedSuspendedOutcome.fromJson({
          'type': 'suspended',
          'interrupt_ids': ['snake'],
        }).interruptIds,
        ['snake'],
      );
      expect(
        SubagentFinishedSuspendedOutcome.fromJson({
          'type': 'suspended',
          'interruptIds': <String>[],
          'interrupt_ids': ['snake'],
        }).interruptIds,
        isEmpty,
      );
      expect(
        SubagentFinishedSuspendedOutcome.fromJson({
          'type': 'suspended',
          'interruptIds': null,
          'interrupt_ids': ['snake'],
        }).interruptIds,
        isNull,
      );
    });

    test('copy can replace and clear interrupt IDs', () {
      const original = SubagentFinishedSuspendedOutcome(
        interruptIds: ['interrupt-1'],
      );

      expect(original.copyWith().interruptIds, ['interrupt-1']);
      expect(
        original.copyWith(interruptIds: const <String>[]).interruptIds,
        isEmpty,
      );
      expect(original.copyWith(interruptIds: null).interruptIds, isNull);
    });

    test('strict decode rejects unknown, wrong, and extra variant keys', () {
      for (final json in <Map<String, dynamic>>[
        {'type': 'unknown'},
        {'type': 1},
        {'type': 'success', 'interruptIds': <String>[]},
        {'type': 'suspended', 'extra': true},
        {
          'type': 'suspended',
          'interruptIds': ['ok', 1],
        },
      ]) {
        expect(
          () => SubagentFinishedOutcome.fromJson(json),
          throwsA(isA<AGUIValidationError>()),
          reason: json.toString(),
        );
      }
    });

    test('malformed cipher-bearing outcomes do not leak cipher text', () {
      const secret = 'cipher-secret-subagent-outcome';
      for (final decode in <void Function()>[
        () => SubagentFinishedOutcome.fromJson({
              'type': 'suspended',
              'interruptIds': [secret, 1],
              'encryptedValue': secret,
            }),
        () => SubagentFinishedSuccessOutcome.fromJson({
              'type': secret,
              'encryptedValue': secret,
            }),
      ]) {
        try {
          decode();
          fail('Expected validation error');
        } on AGUIValidationError catch (error) {
          _expectCipherSafe(error, secret);
        }
      }
    });
  });
}
