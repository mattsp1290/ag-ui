import 'dart:convert';

import 'package:ag_ui/ag_ui.dart';
import 'package:test/test.dart';

Matcher _usageError(String field) => isA<AGUIValidationError>()
    .having((error) => error.field, 'field', field)
    .having((error) => error.json, 'json', isNull)
    .having((error) => error.cause, 'cause', isNull);

void main() {
  group('TokenUsage', () {
    test('public model preserves all fields, zero, and the safe maximum', () {
      final usage = TokenUsage(
        provider: '',
        model: 'model',
        inputTokens: 0,
        outputTokens: maxTokenCount,
        totalTokens: maxTokenCount.toDouble(),
        reasoningTokens: 0.0,
        cachedInputTokens: 1,
      );

      expect(usage.toJson(), {
        'provider': '',
        'model': 'model',
        'inputTokens': 0,
        'outputTokens': maxTokenCount,
        'totalTokens': maxTokenCount,
        'reasoningTokens': 0,
        'cachedInputTokens': 1,
      });
      expect(TokenUsage.fromJson(usage.toJson()).toJson(), usage.toJson());
    });

    test('omits absent optionals and drops unknown input keys', () {
      final usage = TokenUsage.fromJson({
        'inputTokens': 1,
        'prompt': 'must-not-copy',
        'messages': ['must-not-copy'],
        'threadId': 'must-not-copy',
        'extension': {'must': 'not-copy'},
      });

      expect(usage.toJson(), {'inputTokens': 1});
    });

    test('reads snake_case with camelCase key presence precedence', () {
      expect(
        TokenUsage.fromJson({
          'input_tokens': 1.0,
          'output_tokens': 2,
          'total_tokens': 3,
          'reasoning_tokens': 4,
          'cached_input_tokens': 5,
        }).toJson(),
        {
          'inputTokens': 1,
          'outputTokens': 2,
          'totalTokens': 3,
          'reasoningTokens': 4,
          'cachedInputTokens': 5,
        },
      );
      expect(
        TokenUsage.fromJson({
          'inputTokens': null,
          'input_tokens': 9,
        }).inputTokens,
        isNull,
      );
    });

    test('rejects every invalid scalar with its exact field', () {
      final invalid = <Object?>[
        '1',
        true,
        -1,
        1.5,
        double.nan,
        double.infinity,
        maxTokenCount + 1,
      ];
      for (final field in <String>[
        'inputTokens',
        'outputTokens',
        'totalTokens',
        'reasoningTokens',
        'cachedInputTokens',
      ]) {
        for (final value in invalid) {
          expect(
            () => TokenUsage.fromJson({field: value}),
            throwsA(_usageError(field)),
            reason: '$field=$value',
          );
        }
      }
      expect(
        () => TokenUsage.fromJson({'provider': 1}),
        throwsA(_usageError('provider')),
      );
    });

    test('constructor and copy enforce counts and copy can clear values', () {
      expect(
        () => TokenUsage(inputTokens: -1),
        throwsA(_usageError('inputTokens')),
      );
      final usage = TokenUsage(
        provider: 'provider',
        model: 'model',
        inputTokens: 1,
      );
      expect(usage.copyWith().toJson(), usage.toJson());
      expect(
        usage.copyWith(provider: null, inputTokens: null).toJson(),
        {'model': 'model'},
      );
      expect(
        () => usage.copyWith(inputTokens: maxTokenCount + 1),
        throwsA(_usageError('inputTokens')),
      );
      expect(
        () => usage.copyWith(inputTokens: '1'),
        throwsA(_usageError('inputTokens')),
      );
    });
  });

  group('tokenUsageFromLangChainMetadata', () {
    test('maps only the five documented count locations and labels', () {
      final usage = tokenUsageFromLangChainMetadata(
        {
          'input_tokens': 10,
          'output_tokens': 5.0,
          'total_tokens': 15,
          'input_token_details': {'cache_read': 4},
          'output_token_details': {'reasoning': 3},
          'prompt': 'must-not-copy',
          'completion': 'must-not-copy',
          'other': 99,
        },
        provider: 'provider',
        model: 'model',
      );

      expect(usage?.toJson(), {
        'provider': 'provider',
        'model': 'model',
        'inputTokens': 10,
        'outputTokens': 5,
        'totalTokens': 15,
        'reasoningTokens': 3,
        'cachedInputTokens': 4,
      });
    });

    test('ignores malformed counts and returns null for labels only', () {
      for (final metadata in <Object?>[
        null,
        false,
        'usage',
        <Object?>[],
        <String, Object?>{},
        {'prompt': 'must-not-copy'},
        {
          'input_tokens': '10',
          'output_tokens': -1,
          'total_tokens': 1.5,
          'input_token_details': {'cache_read': true},
          'output_token_details': {'reasoning': maxTokenCount + 1},
        },
      ]) {
        expect(
          tokenUsageFromLangChainMetadata(
            metadata,
            provider: 'provider',
            model: 'model',
          ),
          isNull,
          reason: metadata.toString(),
        );
      }
    });

    test('preserves zero and explicitly supplied empty labels', () {
      expect(
        tokenUsageFromLangChainMetadata(
          {'input_tokens': 0},
          provider: '',
          model: '',
        )?.toJson(),
        {'provider': '', 'model': '', 'inputTokens': 0},
      );
    });
  });

  group('aggregateTokenUsage', () {
    test('preserves order, missing counts, zero, and source objects', () {
      final sources = [
        TokenUsage(provider: 'p', model: 'm1', inputTokens: 0),
        TokenUsage(provider: 'p', model: 'm2', outputTokens: 2),
        TokenUsage(
          provider: 'p',
          model: 'm1',
          inputTokens: 3,
          totalTokens: 4,
        ),
        TokenUsage(provider: 'p', model: 'm2', outputTokens: 0),
      ];
      final before = sources.map((entry) => entry.toJson()).toList();

      expect(
        aggregateTokenUsage(sources).map((entry) => entry.toJson()).toList(),
        [
          {
            'provider': 'p',
            'model': 'm1',
            'inputTokens': 3,
            'totalTokens': 4,
          },
          {'provider': 'p', 'model': 'm2', 'outputTokens': 2},
        ],
      );
      expect(sources.map((entry) => entry.toJson()).toList(), before);
      expect(aggregateTokenUsage(const []), isEmpty);
    });

    test('uses structural tuple keys and keeps absent distinct from empty', () {
      final aggregated = aggregateTokenUsage([
        TokenUsage(provider: 'a b', model: 'c', inputTokens: 1),
        TokenUsage(provider: 'a', model: 'b c', inputTokens: 2),
        TokenUsage(provider: '', model: '', inputTokens: 3),
        TokenUsage(inputTokens: 4),
      ]).map((entry) => entry.toJson()).toList();

      expect(aggregated, [
        {'provider': 'a b', 'model': 'c', 'inputTokens': 1},
        {'provider': 'a', 'model': 'b c', 'inputTokens': 2},
        {'provider': '', 'model': '', 'inputTokens': 3},
        {'inputTokens': 4},
      ]);
    });

    test('rejects addition before the safe-integer limit overflows', () {
      expect(
        () => aggregateTokenUsage([
          TokenUsage(provider: 'p', model: 'm', inputTokens: maxTokenCount),
          TokenUsage(provider: 'p', model: 'm', inputTokens: 1),
        ]),
        throwsA(_usageError('usage[1].inputTokens')),
      );
    });
  });

  group('terminal events', () {
    test('finished and error preserve empty and populated usage arrays', () {
      for (final event in <BaseEvent>[
        const RunFinishedEvent(threadId: 't', runId: 'r', usage: []),
        RunFinishedEvent(
          threadId: 't',
          runId: 'r',
          usage: [TokenUsage(inputTokens: 0)],
        ),
        const RunErrorEvent(message: 'failed', usage: []),
        RunErrorEvent(
          message: 'failed',
          usage: [TokenUsage(outputTokens: 2)],
        ),
      ]) {
        expect(BaseEvent.fromJson(event.toJson()).toJson(), event.toJson());
      }
    });

    test('absent/null usage omits output and copy can clear it', () {
      for (final json in <Map<String, dynamic>>[
        {'type': 'RUN_FINISHED', 'threadId': 't', 'runId': 'r'},
        {
          'type': 'RUN_FINISHED',
          'threadId': 't',
          'runId': 'r',
          'usage': null,
        },
        {'type': 'RUN_ERROR', 'message': 'failed'},
        {'type': 'RUN_ERROR', 'message': 'failed', 'usage': null},
      ]) {
        final event = BaseEvent.fromJson(json);
        expect(event.toJson().containsKey('usage'), isFalse);
      }
      final finished = RunFinishedEvent(
        threadId: 't',
        runId: 'r',
        usage: [TokenUsage(totalTokens: 1)],
      );
      final error = RunErrorEvent(
        message: 'failed',
        usage: [TokenUsage(totalTokens: 1)],
      );
      expect(finished.copyWith().usage, hasLength(1));
      expect(finished.copyWith(usage: null).usage, isNull);
      expect(error.copyWith().usage, hasLength(1));
      expect(error.copyWith(usage: null).usage, isNull);
    });

    test('public SSE retains usage on successful and failed root runs', () {
      final events = <BaseEvent>[
        RunFinishedEvent(
          threadId: 't',
          runId: 'r',
          usage: [TokenUsage(inputTokens: 1)],
        ),
        RunErrorEvent(
          message: 'failed',
          usage: [TokenUsage(outputTokens: 2)],
        ),
      ];
      final encoder = EventEncoder();
      const decoder = EventDecoder();
      for (final event in events) {
        final decoded = decoder.decodeSSE(encoder.encodeSSE(event));
        expect(decoded.toJson(), event.toJson());
        expect(jsonEncode(decoded.toJson()), contains('Tokens'));
      }
    });

    test('malformed list entries report index without retaining payload', () {
      const secret = 'content-must-not-leak';
      for (final json in <Map<String, dynamic>>[
        {
          'type': 'RUN_FINISHED',
          'threadId': 't',
          'runId': 'r',
          'usage': [
            {'inputTokens': secret},
          ],
          'rawEvent': {'encryptedValue': secret},
        },
        {
          'type': 'RUN_ERROR',
          'message': 'failed',
          'usage': [
            {'outputTokens': -1},
          ],
        },
      ]) {
        try {
          BaseEvent.fromJson(json);
          fail('Expected validation error');
        } on AGUIValidationError catch (error) {
          expect(error.field, startsWith('usage[0].'));
          expect(error.json, isNull);
          expect(error.cause, isNull);
          expect(error.toString(), isNot(contains(secret)));
        }
      }
    });

    test('malformed usage shapes report exact fields without payload', () {
      const secret = 'content-must-not-leak';
      for (final testCase in <(Object?, String)>[
        (secret, 'usage'),
        ({'encryptedValue': secret}, 'usage'),
        ([secret], 'usage[0]'),
      ]) {
        try {
          BaseEvent.fromJson({
            'type': 'RUN_FINISHED',
            'threadId': 't',
            'runId': 'r',
            'usage': testCase.$1,
          });
          fail('Expected validation error');
        } on AGUIValidationError catch (error) {
          expect(error.field, testCase.$2);
          expect(error.json, isNull);
          expect(error.cause, isNull);
          expect(error.toString(), isNot(contains(secret)));
        }
      }
    });
  });
}
