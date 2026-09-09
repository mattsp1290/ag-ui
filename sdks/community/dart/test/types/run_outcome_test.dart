import 'dart:convert';

import 'package:ag_ui/ag_ui.dart';
import 'package:test/test.dart';

void main() {
  group('RunFinishedOutcome', () {
    test('success emits exactly its discriminator', () {
      const outcome = RunFinishedSuccessOutcome();
      expect(outcome.toJson(), {'type': 'success'});
      expect(
        RunFinishedOutcome.fromJson(outcome.toJson()),
        isA<RunFinishedSuccessOutcome>(),
      );
      expect(outcome.copyWith().toJson(), {'type': 'success'});
    });

    test('interrupt requires and round-trips a nonempty interrupt list', () {
      final outcome = RunFinishedInterruptOutcome(
        interrupts: const [
          Interrupt(id: 'i1', reason: 'approval'),
          Interrupt(id: 'i2', reason: 'input', subagentRunId: 'child'),
        ],
      );
      expect(
        RunFinishedOutcome.fromJson(outcome.toJson()).toJson(),
        outcome.toJson(),
      );
      expect(outcome.copyWith().toJson(), outcome.toJson());
    });

    test('rejects empty interrupts at construction and decoding', () {
      expect(
        () => RunFinishedInterruptOutcome(interrupts: const []),
        throwsA(
          isA<AGUIValidationError>().having(
            (error) => error.field,
            'field',
            'interrupts',
          ),
        ),
      );
      expect(
        () => RunFinishedOutcome.fromJson({
          'type': 'interrupt',
          'interrupts': <Map<String, dynamic>>[],
        }),
        throwsA(isA<AGUIValidationError>()),
      );
    });

    test('strict variants reject unknown types, wrong types, and extra keys',
        () {
      for (final json in <Map<String, dynamic>>[
        <String, dynamic>{},
        {'type': 'future'},
        {'type': 1},
        {'type': 'success', 'interrupts': <Object>[]},
        {'type': 'success', 'extra': true},
        {'type': 'interrupt', 'interrupts': 'wrong'},
        {'type': 'interrupt'},
        {
          'type': 'interrupt',
          'interrupts': [
            {'id': 1, 'reason': 'approval'},
          ],
        },
      ]) {
        expect(
          () => RunFinishedOutcome.fromJson(json),
          throwsA(isA<AGUIValidationError>()),
          reason: json.toString(),
        );
      }
    });

    test('cipher-bearing extra key does not leak through error fields', () {
      const secret = 'outcome-cipher-secret';
      try {
        RunFinishedOutcome.fromJson({
          'type': 'success',
          secret: true,
          'metadata': {'encryptedValue': secret},
        });
        fail('Expected AGUIValidationError');
      } on AGUIValidationError catch (error) {
        expect(error.field, 'outcome');
        expect(error.message, isNot(contains(secret)));
        expect(error.value?.toString(), isNot(contains(secret)));
        expect(error.json, isNull);
        expect(error.cause, isNull);
        expect(error.toString(), isNot(contains(secret)));
      }
    });

    test('concrete outcome factories scrub cipher-bearing failures', () {
      const secret = 'concrete-outcome-secret';
      for (final decode in <void Function()>[
        () => RunFinishedSuccessOutcome.fromJson({
              'type': 'success',
              secret: true,
              'metadata': {'encryptedValue': secret},
            }),
        () => RunFinishedInterruptOutcome.fromJson({
              'type': 'interrupt',
              'interrupts': secret,
              'metadata': {'encryptedValue': secret},
            }),
      ]) {
        try {
          decode();
          fail('Expected AGUIValidationError');
        } on AGUIValidationError catch (error) {
          expect(error.message, isNot(contains(secret)));
          expect(error.value?.toString(), isNot(contains(secret)));
          expect(error.json, isNull);
          expect(error.cause, isNull);
          expect(error.toString(), isNot(contains(secret)));
        }
      }
    });
  });

  group('RunFinishedEvent outcome', () {
    test('legacy absent/null outcome stays absent and result is independent',
        () {
      for (final json in <Map<String, dynamic>>[
        {'type': 'RUN_FINISHED', 'threadId': 't', 'runId': 'r'},
        {
          'type': 'RUN_FINISHED',
          'threadId': 't',
          'runId': 'r',
          'outcome': null,
        },
      ]) {
        final decoded = RunFinishedEvent.fromJson(json);
        expect(decoded.outcome, isNull);
        expect(decoded.toJson().containsKey('outcome'), isFalse);
      }

      const event = RunFinishedEvent(
        threadId: 't',
        runId: 'r',
        result: {'legacy': true},
        outcome: RunFinishedSuccessOutcome(),
      );
      expect(event.toJson()['result'], {'legacy': true});
      expect(event.toJson()['outcome'], {'type': 'success'});
      expect(event.copyWith().outcome, isA<RunFinishedSuccessOutcome>());
      expect(event.copyWith(outcome: null).outcome, isNull);
      expect(event.copyWith(result: null).result, isNull);
    });

    test('decodes and re-encodes through JSON and SSE public codecs', () {
      final input = {
        'type': 'RUN_FINISHED',
        'threadId': 't',
        'runId': 'r',
        'outcome': {
          'type': 'interrupt',
          'interrupts': [
            {
              'id': 'i',
              'reason': 'approval',
              'responseSchema': {'type': 'boolean'},
            },
          ],
        },
      };
      const decoder = EventDecoder();
      final event = decoder.decode(jsonEncode(input)) as RunFinishedEvent;
      expect(event.outcome, isA<RunFinishedInterruptOutcome>());

      final sse = EventEncoder().encodeSSE(event);
      expect(decoder.decodeSSE(sse).toJson(), event.toJson());
    });

    test('public decoder reports indexed malformed interrupt paths', () {
      expect(
        () => const EventDecoder().decodeJson({
          'type': 'RUN_FINISHED',
          'threadId': 't',
          'runId': 'r',
          'outcome': {
            'type': 'interrupt',
            'interrupts': [
              {'id': 'i', 'reason': 1},
            ],
          },
        }),
        throwsA(
          isA<DecodingError>().having(
            (error) => error.field,
            'field',
            'outcome.interrupts[0].reason',
          ),
        ),
      );
    });

    test('validate rejects empty nested interrupt identifiers with its index',
        () {
      final event = RunFinishedEvent(
        threadId: 't',
        runId: 'r',
        outcome: RunFinishedInterruptOutcome(
          interrupts: const [Interrupt(id: '', reason: 'approval')],
        ),
      );
      expect(
        () => const EventDecoder().validate(event),
        throwsA(
          isA<ValidationError>().having(
            (error) => error.field,
            'field',
            'outcome.interrupts[0].id',
          ),
        ),
      );
    });

    test('malformed cipher-bearing outcome errors suppress raw payloads', () {
      const secret = 'run-outcome-cipher-secret';
      try {
        const EventDecoder().decodeJson({
          'type': 'RUN_FINISHED',
          'threadId': 't',
          'runId': 'r',
          'result': {'encryptedValue': secret},
          'outcome': {
            'type': 'interrupt',
            'interrupts': [
              {'id': <String>[], 'reason': 'approval'},
            ],
          },
        });
        fail('Expected DecodingError');
      } on DecodingError catch (error) {
        expect(error.actualValue, isNull);
        expect(error.toString(), isNot(contains(secret)));
        expect(error.cause?.toString(), isNot(contains(secret)));
      }
    });
  });
}
