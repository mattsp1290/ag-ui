import 'package:ag_ui/ag_ui.dart';
import 'package:test/test.dart';

void _expectCipherSafe(AGUIValidationError error, String secret) {
  expect(error.message, isNot(contains(secret)));
  expect(error.value?.toString(), isNot(contains(secret)));
  expect(error.json?.toString(), isNot(contains(secret)));
  expect(error.cause?.toString(), isNot(contains(secret)));
}

void main() {
  test('public API exposes canonical input transport and codec', () async {
    final client = AgUiClient(
      config: AgUiClientConfig(baseUrl: 'http://localhost:8000'),
    );
    const input = RunAgentInput(
      threadId: 'thread-1',
      runId: 'run-1',
      messages: [],
      tools: [],
      context: [],
      resume: [
        ResumeEntry(
          interruptId: 'interrupt-1',
          status: ResumeStatus.resolved,
        ),
      ],
    );
    final run = client.runAgentInput;

    expect(
      run,
      isA<
          Stream<BaseEvent> Function(
            String,
            RunAgentInput, {
            CancelToken? cancelToken,
          })>(),
    );
    expect(
      const Encoder().encodeCanonicalRunAgentInput(input),
      containsPair('forwardedProps', null),
    );
    await client.close();
  });

  group('Interrupt', () {
    test('round-trips every optional field through the public API', () {
      const interrupt = Interrupt(
        id: 'approval-1',
        reason: 'tool_call',
        message: 'Approve the transfer?',
        toolCallId: 'call-1',
        responseSchema: {
          'type': 'object',
          'properties': {
            'approved': {'type': 'boolean'},
          },
        },
        expiresAt: '2026-09-10T00:00:00Z',
        metadata: {'signature': 'signed'},
        subagentRunId: 'child-1',
      );

      expect(
        Interrupt.fromJson(interrupt.toJson()).toJson(),
        interrupt.toJson(),
      );
    });

    test('accepts snake_case aliases and gives camelCase presence precedence',
        () {
      final snake = Interrupt.fromJson({
        'id': 'snake',
        'reason': 'approval',
        'tool_call_id': 'snake-call',
        'response_schema': {'type': 'boolean'},
        'expires_at': 'later',
        'subagent_run_id': 'child',
      });
      expect(snake.toolCallId, 'snake-call');
      expect(snake.responseSchema, {'type': 'boolean'});
      expect(snake.expiresAt, 'later');
      expect(snake.subagentRunId, 'child');

      final decoded = Interrupt.fromJson({
        'id': 'i',
        'reason': 'approval',
        'toolCallId': null,
        'tool_call_id': 'snake-call',
        'responseSchema': null,
        'response_schema': {'type': 'boolean'},
        'expiresAt': null,
        'expires_at': 'later',
        'subagentRunId': null,
        'subagent_run_id': 'child',
      });

      expect(decoded.toolCallId, isNull);
      expect(decoded.responseSchema, isNull);
      expect(decoded.expiresAt, isNull);
      expect(decoded.subagentRunId, isNull);
    });

    test('copyWith preserves omission and clears every optional field', () {
      const original = Interrupt(
        id: 'i',
        reason: 'approval',
        message: 'message',
        toolCallId: 'call',
        responseSchema: {'type': 'boolean'},
        expiresAt: 'later',
        metadata: {'signed': true},
        subagentRunId: 'child',
      );

      expect(original.copyWith().toJson(), original.toJson());
      expect(
        original
            .copyWith(
              message: null,
              toolCallId: null,
              responseSchema: null,
              expiresAt: null,
              metadata: null,
              subagentRunId: null,
            )
            .toJson(),
        {'id': 'i', 'reason': 'approval'},
      );
    });

    test('malformed cipher-bearing input does not retain ciphertext', () {
      const secret = 'interrupt-cipher-secret';
      try {
        Interrupt.fromJson({
          'id': <String>[],
          'reason': 'approval',
          'metadata': {'encryptedValue': secret},
        });
        fail('Expected AGUIValidationError');
      } on AGUIValidationError catch (error) {
        expect(error.field, 'id');
        _expectCipherSafe(error, secret);
      }
    });
  });

  group('ResumeEntry', () {
    test('supports both statuses and arbitrary JSON payloads', () {
      const payloads = <Object?>[
        false,
        0,
        ['answer'],
        {'approved': true, 'note': null},
        null,
      ];
      for (final status in ResumeStatus.values) {
        for (final payload in payloads) {
          final entry = ResumeEntry(
            interruptId: 'i',
            status: status,
            payload: payload,
            metadata: const {'signed': true},
          );
          final decoded = ResumeEntry.fromJson(entry.toJson());
          expect(decoded.interruptId, 'i');
          expect(decoded.status, status);
          expect(decoded.payload, payload);
          expect(decoded.metadata, {'signed': true});
        }
      }
    });

    test('reads snake_case and camelCase wins by key presence', () {
      expect(
        () => ResumeEntry.fromJson({
          'interruptId': null,
          'interrupt_id': 'snake',
          'status': 'resolved',
        }),
        throwsA(
          isA<AGUIValidationError>().having(
            (error) => error.field,
            'field',
            'interruptId',
          ),
        ),
      );
      expect(
        ResumeEntry.fromJson({
          'interrupt_id': 'snake',
          'status': 'cancelled',
        }).interruptId,
        'snake',
      );
    });

    test('rejects unknown status and supports clearable copies', () {
      expect(
        () => ResumeEntry.fromJson({
          'interruptId': 'i',
          'status': 'waiting',
        }),
        throwsA(
          isA<AGUIValidationError>().having(
            (error) => error.field,
            'field',
            'status',
          ),
        ),
      );

      const entry = ResumeEntry(
        interruptId: 'i',
        status: ResumeStatus.resolved,
        payload: false,
        metadata: {'signed': true},
      );
      expect(entry.copyWith().toJson(), entry.toJson());
      expect(entry.copyWith(payload: null, metadata: null).toJson(), {
        'interruptId': 'i',
        'status': 'resolved',
      });
    });

    test('unknown cipher-bearing status is scrubbed', () {
      const secret = 'resume-status-cipher-secret';
      try {
        ResumeEntry.fromJson({
          'interruptId': 'i',
          'status': secret,
          'metadata': {'encryptedValue': 'ciphertext'},
        });
        fail('Expected AGUIValidationError');
      } on AGUIValidationError catch (error) {
        expect(error.field, 'status');
        _expectCipherSafe(error, secret);
      }
    });
  });
}
