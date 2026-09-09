import 'dart:convert';
import 'dart:io';

import 'package:ag_ui/ag_ui.dart';
import 'package:test/test.dart';

import '../../tool/parity_support.dart';

void main() {
  final fixture = asMap(
    jsonDecode(
      File('${packageRoot().path}/test/fixtures/parity_cases.json')
          .readAsStringSync(),
    ),
    'supplemental corpus',
  );

  test('supplemental result shapes survive factories and SSE codecs', () {
    expect(fixture['version'], 1);
    final cases = fixture['cases'] as List;
    expect(cases, hasLength(7));
    for (final rawCase in cases) {
      final testCase = asMap(rawCase, 'supplemental case');
      final input = asMap(testCase['input'], '${testCase['id']} input');
      final expected = testCase['expected'];
      final direct = BaseEvent.fromJson(input);
      expect(direct, isA<SubagentFinishedEvent>());
      expect(direct.toJson(), expected, reason: testCase['id'] as String);

      final encoded = EventEncoder().encodeSSE(direct);
      final decoded = const EventDecoder().decodeSSE(encoded);
      expect(decoded.toJson(), expected, reason: testCase['id'] as String);
    }
  });

  test('direct factory and encoder retain typed shared terminal fields', () {
    final event = RunFinishedEvent(
      threadId: 'thread',
      runId: 'run',
      outcome: RunFinishedInterruptOutcome(
        interrupts: const [
          Interrupt(
            id: 'approval',
            reason: 'approve',
            subagentRunId: 'child',
            metadata: {'nested': null},
          ),
        ],
      ),
      usage: [TokenUsage(provider: '', inputTokens: 0, outputTokens: 2)],
      metadata: const {'trace': false},
    );
    final expected = event.toJson();
    final decoded = const EventDecoder().decodeSSE(
      EventEncoder().encodeSSE(event),
    );

    expect(decoded.toJson(), expected);
    final finished = decoded as RunFinishedEvent;
    expect(finished.outcome, isA<RunFinishedInterruptOutcome>());
    expect(finished.usage?.single.inputTokens, 0);
    expect(finished.metadata, {'trace': false});
  });

  test('public model routes retain capabilities and canonical resume input',
      () {
    final capabilities = AgentCapabilities.fromJson({
      'identity': {
        'metadata': {'nullable': null},
      },
      'tools': {
        'supported': false,
        'items': <Map<String, dynamic>>[],
      },
      'humanInTheLoop': {'interrupts': true},
      'custom': {
        'zero': 0,
        'values': [false, null],
      },
    });
    expect(
      AgentCapabilities.fromJson(capabilities.toJson()).toJson(),
      capabilities.toJson(),
    );

    final input = RunAgentInput.fromJson({
      'threadId': 'thread',
      'runId': 'run',
      'state': {
        'nullable': null,
        'items': [false, 0],
      },
      'messages': <Map<String, dynamic>>[],
      'tools': <Map<String, dynamic>>[],
      'context': <Map<String, dynamic>>[],
      'forwardedProps': null,
      'resume': [
        {
          'interruptId': 'approval',
          'status': 'resolved',
          'payload': {'approved': false, 'editedArgs': <String, dynamic>{}},
          'metadata': {'nullable': null},
        },
      ],
    });
    final output = input.toJson();
    expect(output.containsKey('forwardedProps'), isFalse);
    expect(
      (output['resume'] as List).single,
      containsPair('interruptId', 'approval'),
    );
  });
}
