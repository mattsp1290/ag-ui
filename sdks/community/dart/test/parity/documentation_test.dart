import 'dart:io';

import 'package:ag_ui/ag_ui.dart';
import 'package:test/test.dart';

String _markedCode(String source, String marker) {
  final startMarker = '<!-- documentation-test:$marker:start -->';
  final endMarker = '<!-- documentation-test:$marker:end -->';
  final start = source.indexOf(startMarker);
  final end = source.indexOf(endMarker);
  expect(start, isNonNegative, reason: 'missing $startMarker');
  expect(end, greaterThan(start), reason: 'missing $endMarker');
  final marked = source.substring(start + startMarker.length, end).trim();
  return marked
      .replaceFirst(RegExp(r'^```dart\s*'), '')
      .replaceFirst(RegExp(r'\s*```$'), '')
      .trim();
}

void main() {
  test('public guides share the exact canonical and legacy resume examples',
      () {
    final readme = File('README.md').readAsStringSync();
    final exampleReadme = File('example/README.md').readAsStringSync();
    final canonicalResume = _markedCode(readme, 'canonical-resume');
    final legacyResume = _markedCode(readme, 'legacy-resume');
    expect(
      _markedCode(exampleReadme, 'canonical-resume'),
      canonicalResume,
    );
    expect(_markedCode(exampleReadme, 'legacy-resume'), legacyResume);
    for (final fragment in [
      'client.runAgentInput',
      'client.runAgent(',
      'RunFinishedInterruptOutcome',
      'SubagentStartedEvent',
      'aggregateTokenUsage',
      'AgentCapabilities.fromJson',
      'EventType.subagentStarted',
      'SubagentErrorEvent()',
      'forwardedProps',
    ]) {
      expect(readme, contains(fragment), reason: fragment);
    }
  });

  test('README parity examples compile exactly as published', () async {
    final readme = File('README.md').readAsStringSync();
    final switchSource = _markedCode(readme, 'exhaustive-switch');
    final canonicalResume = _markedCode(readme, 'canonical-resume');
    final legacyResume = _markedCode(readme, 'legacy-resume');
    for (final mapping in [
      "EventType.subagentStarted => 'SubagentStartedEvent'",
      "EventType.subagentFinished => 'SubagentFinishedEvent'",
      "EventType.subagentError => 'SubagentErrorEvent'",
      "SubagentStartedEvent() => 'SubagentStartedEvent'",
      "SubagentFinishedEvent() => 'SubagentFinishedEvent'",
      "SubagentErrorEvent() => 'SubagentErrorEvent'",
    ]) {
      expect(switchSource, contains(mapping), reason: mapping);
    }
    final directory = Directory(
      'test/.documentation-snippet-$pid-${DateTime.now().microsecondsSinceEpoch}',
    );
    await directory.create();
    final source = File('${directory.path}/exhaustive_switch.dart');
    try {
      await source.writeAsString('''
// ignore_for_file: deprecated_member_use_from_same_package, unused_element
import 'package:ag_ui/ag_ui.dart';

final client = AgUiClient(
  config: AgUiClientConfig(baseUrl: 'https://example.invalid'),
);

Future<void> compileResumeExamples() async {
$canonicalResume
$legacyResume
  await resumedEvents.drain<void>();
  await legacyResumedEvents.drain<void>();
}

$switchSource

void main() {}
''');
      final result = await Process.run(
        Platform.resolvedExecutable,
        ['analyze', source.path],
      );
      expect(
        result.exitCode,
        0,
        reason: '${result.stdout}\n${result.stderr}',
      );
    } finally {
      if (directory.existsSync()) {
        await directory.delete(recursive: true);
      }
    }
  });

  test('metadata examples use incoming-wins shallow merge semantics', () {
    final existing = <String, dynamic>{
      agUiMetadataKey: {'old': true},
      'trace': 'old',
      'nested': {'keep': true},
    };
    final incoming = <String, dynamic>{
      agUiMetadataKey: {'new': true},
      'trace': 'new',
      'nested': {'replacement': true},
      'child': false,
      'zero': 0,
      'nullValue': null,
      'empty': <String, dynamic>{},
    };
    final merged = mergeMetadata(existing, incoming);
    expect(merged, {
      agUiMetadataKey: {'new': true},
      'trace': 'new',
      'nested': {'replacement': true},
      'child': false,
      'zero': 0,
      'nullValue': null,
      'empty': <String, dynamic>{},
    });
    expect(existing['trace'], 'old');
    expect(incoming['trace'], 'new');
    expect(mergeMetadata(existing, null), same(existing));
    expect(mergeMetadata(null, incoming), isNot(same(incoming)));
  });

  test('canonical interrupt resume input uses the public HTTP codec', () {
    const input = RunAgentInput(
      threadId: 'thread-1',
      runId: 'run-2',
      parentRunId: 'run-1',
      messages: [],
      tools: [],
      context: [],
      resume: [
        ResumeEntry(
          interruptId: 'approval-1',
          status: ResumeStatus.resolved,
          payload: {'approved': true},
          metadata: {'source': 'user'},
        ),
      ],
    );
    expect(input.toJson().containsKey('forwardedProps'), isFalse);
    final wire = const Encoder().encodeCanonicalRunAgentInput(input);
    expect(wire, containsPair('forwardedProps', null));
    expect((wire['resume'] as List).single, containsPair('status', 'resolved'));

    final legacy = const SimpleRunAgentInput(
      threadId: 'thread-1',
      runId: 'run-2',
      parentRunId: 'run-1',
      resume: [
        ResumeEntry(
          interruptId: 'approval-1',
          status: ResumeStatus.resolved,
          payload: {'approved': true},
        ),
      ],
    ).toJson();
    expect(legacy, containsPair('state', <String, dynamic>{}));
    expect(legacy, containsPair('messages', <dynamic>[]));
    expect(legacy, containsPair('tools', <dynamic>[]));
    expect(legacy, containsPair('context', <dynamic>[]));
    expect(legacy, containsPair('forwardedProps', <String, dynamic>{}));
    expect(
      (legacy['resume'] as List).single,
      containsPair('status', 'resolved'),
    );

    final outcome = RunFinishedOutcome.fromJson({
      'type': 'interrupt',
      'interrupts': [
        {'id': 'approval-1', 'reason': 'Approve the action?'},
      ],
    });
    expect(outcome, isA<RunFinishedInterruptOutcome>());
    expect(
      RunFinishedOutcome.fromJson(
        const RunFinishedSuccessOutcome().toJson(),
      ),
      isA<RunFinishedSuccessOutcome>(),
    );
  });

  test('subagent, usage, and capability examples use public exports', () {
    const subagent = SubagentFinishedEvent(
      subagentRunId: 'researcher',
      result: {'answer': 42},
      outcome: SubagentFinishedSuspendedOutcome(
        interruptIds: ['approval-1'],
      ),
    );
    expect(subagent.result, {'answer': 42});
    expect(subagent.outcome, isA<SubagentFinishedSuspendedOutcome>());

    final usage = aggregateTokenUsage([
      TokenUsage(provider: 'example', model: 'model', inputTokens: 2),
      TokenUsage(provider: 'example', model: 'model', outputTokens: 3),
    ]);
    expect(usage.single.toJson(), {
      'provider': 'example',
      'model': 'model',
      'inputTokens': 2,
      'outputTokens': 3,
    });
    expect(maxTokenCount, 9007199254740991);
    expect(
      tokenUsageFromLangChainMetadata({
        'input_tokens': 2,
        'output_tokens': 3,
        'total_tokens': 5,
        'output_token_details': {'reasoning': 1},
        'input_token_details': {'cache_read': 1},
      })?.toJson(),
      {
        'inputTokens': 2,
        'outputTokens': 3,
        'totalTokens': 5,
        'reasoningTokens': 1,
        'cachedInputTokens': 1,
      },
    );

    final capabilities = AgentCapabilities.fromJson({
      'transport': {'streaming': true},
      'multiAgent': {
        'supported': true,
        'subAgents': [
          {'name': 'researcher'},
        ],
      },
      'humanInTheLoop': {'interrupts': true},
    });
    expect(capabilities.transport?.streaming, isTrue);
    expect(capabilities.multiAgent?.subAgents?.single.name, 'researcher');
    expect(capabilities.humanInTheLoop?.interrupts, isTrue);
  });
}
