import 'package:ag_ui/ag_ui.dart';
import 'package:test/test.dart';

Matcher _fieldError(String field) => isA<AGUIValidationError>()
    .having((error) => error.field, 'field', field)
    .having((error) => error.json, 'json', isNull)
    .having((error) => error.cause, 'cause', isNull);

Map<String, dynamic> _completeCapabilities() => {
      'identity': {
        'name': 'Fixture Agent',
        'type': 'test',
        'description': 'synthetic',
        'version': '1.0.0',
        'provider': 'example',
        'documentationUrl': 'https://example.invalid/docs',
        'metadata': {
          'nested': {'nullable': null},
          'enabled': false,
        },
      },
      'transport': {
        'streaming': true,
        'websocket': false,
        'httpBinary': true,
        'pushNotifications': false,
        'resumable': true,
      },
      'tools': {
        'supported': true,
        'items': [
          {
            'name': 'lookup',
            'description': 'synthetic',
            'parameters': {'type': 'object', 'required': <dynamic>[]},
            'metadata': {'nested': null},
          },
        ],
        'parallelCalls': false,
        'clientProvided': true,
      },
      'output': {
        'structuredOutput': false,
        'supportedMimeTypes': ['text/plain', 'application/json'],
      },
      'state': {
        'snapshots': true,
        'deltas': true,
        'memory': false,
        'persistentState': false,
      },
      'multiAgent': {
        'supported': true,
        'delegation': true,
        'handoffs': false,
        'subAgents': [
          {'name': 'researcher', 'description': 'synthetic'},
        ],
      },
      'reasoning': {'supported': true, 'streaming': false, 'encrypted': true},
      'multimodal': {
        'input': {
          'image': true,
          'audio': false,
          'video': true,
          'pdf': false,
          'file': true,
        },
        'output': {'image': false, 'audio': true},
      },
      'execution': {
        'codeExecution': false,
        'sandboxed': true,
        'maxIterations': 1.0,
        'maxExecutionTime': 1000.0,
      },
      'humanInTheLoop': {
        'supported': true,
        'approvals': false,
        'interventions': true,
        'feedback': false,
        'interrupts': true,
        'approveWithEdits': false,
      },
      'custom': {
        'flag': false,
        'limit': 0,
        'empty': <dynamic>[],
        'nested': {'nullable': null},
      },
    };

void main() {
  group('AgentCapabilities', () {
    test('public model round-trips every shared field', () {
      final input = _completeCapabilities();
      final encoded = AgentCapabilities.fromJson(input).toJson();

      expect(encoded, input);
      expect((encoded['execution'] as Map)['maxIterations'], 1);
      expect((encoded['identity'] as Map)['metadata'], {
        'nested': {'nullable': null},
        'enabled': false,
      });
      expect(
        ((encoded['tools'] as Map)['items'] as List).single,
        containsPair('metadata', {'nested': null}),
      );
    });

    test(
      'empty and explicit false, zero, lists, maps, and objects survive',
      () {
        expect(const AgentCapabilities().toJson(), isEmpty);
        final value = AgentCapabilities.fromJson({
          'identity': <String, dynamic>{},
          'transport': {'streaming': false},
          'tools': {'supported': false, 'items': <dynamic>[]},
          'output': {'supportedMimeTypes': <dynamic>[]},
          'multiAgent': {'subAgents': <dynamic>[]},
          'execution': {'maxIterations': 0, 'maxExecutionTime': 0},
          'custom': <String, dynamic>{},
        });
        expect(value.toJson(), {
          'identity': <String, dynamic>{},
          'transport': {'streaming': false},
          'tools': {'supported': false, 'items': <dynamic>[]},
          'output': {'supportedMimeTypes': <dynamic>[]},
          'multiAgent': {'subAgents': <dynamic>[]},
          'execution': {'maxIterations': 0, 'maxExecutionTime': 0},
          'custom': <String, dynamic>{},
        });
      },
    );

    test('reads every snake_case alias and emits camelCase', () {
      final value = AgentCapabilities.fromJson({
        'identity': {'documentation_url': 'docs'},
        'transport': {'http_binary': false, 'push_notifications': true},
        'tools': {'parallel_calls': false, 'client_provided': true},
        'output': {
          'structured_output': true,
          'supported_mime_types': ['text/plain'],
        },
        'state': {'persistent_state': false},
        'multi_agent': {
          'sub_agents': [
            {'name': 'worker'},
          ],
        },
        'execution': {
          'code_execution': true,
          'max_iterations': 2.0,
          'max_execution_time': 3,
        },
        'human_in_the_loop': {'approve_with_edits': false},
      });

      expect(value.toJson(), {
        'identity': {'documentationUrl': 'docs'},
        'transport': {'httpBinary': false, 'pushNotifications': true},
        'tools': {'parallelCalls': false, 'clientProvided': true},
        'output': {
          'structuredOutput': true,
          'supportedMimeTypes': ['text/plain'],
        },
        'state': {'persistentState': false},
        'multiAgent': {
          'subAgents': [
            {'name': 'worker'},
          ],
        },
        'execution': {
          'codeExecution': true,
          'maxIterations': 2,
          'maxExecutionTime': 3,
        },
        'humanInTheLoop': {'approveWithEdits': false},
      });
    });

    test('camelCase presence wins over snake_case, including null', () {
      final value = AgentCapabilities.fromJson({
        'multiAgent': null,
        'multi_agent': {'supported': true},
        'transport': {'httpBinary': null, 'http_binary': true},
      });
      expect(value.multiAgent, isNull);
      expect(value.transport?.httpBinary, isNull);
    });

    test('nested wrong types identify complete canonical paths', () {
      final invalid = <Map<String, dynamic>, String>{
        {
          'transport': {'httpBinary': 'yes'},
        }: 'transport.httpBinary',
        {
          'multimodal': {
            'input': {'video': 1},
          },
        }: 'multimodal.input.video',
        {
          'multiAgent': {
            'subAgents': [
              {'name': false},
            ],
          },
        }: 'multiAgent.subAgents[0].name',
        {
          'tools': {
            'items': [
              {'name': 'x', 'description': false},
            ],
          },
        }: 'tools.items[0].description',
        {
          'output': {
            'supportedMimeTypes': ['text/plain', false],
          },
        }: 'output.supportedMimeTypes[1]',
        {'custom': <dynamic>[]}: 'custom',
      };
      for (final entry in invalid.entries) {
        expect(
          () => AgentCapabilities.fromJson(entry.key),
          throwsA(_fieldError(entry.value)),
          reason: entry.key.toString(),
        );
      }
    });

    test('required sub-agent name is enforced', () {
      expect(
        () => AgentCapabilities.fromJson({
          'multiAgent': {
            'subAgents': [<String, dynamic>{}],
          },
        }),
        throwsA(_fieldError('multiAgent.subAgents[0].name')),
      );
    });

    test('copy keeps omitted values and clears nullable fields', () {
      final value = AgentCapabilities.fromJson(_completeCapabilities());
      expect(value.copyWith().toJson(), value.toJson());
      expect(value.copyWith(identity: null).identity, isNull);
      expect(value.identity!.copyWith(metadata: null).metadata, isNull);
      expect(value.transport!.copyWith(httpBinary: null).httpBinary, isNull);
      expect(value.tools!.copyWith(items: null).items, isNull);
      expect(
        value.output!.copyWith(supportedMimeTypes: null).supportedMimeTypes,
        isNull,
      );
      expect(
        value.state!.copyWith(persistentState: null).persistentState,
        isNull,
      );
      expect(value.multiAgent!.copyWith(subAgents: null).subAgents, isNull);
      expect(value.reasoning!.copyWith(encrypted: null).encrypted, isNull);
      expect(value.multimodal!.copyWith(input: null).input, isNull);
      expect(
        value.execution!.copyWith(maxIterations: null).maxIterations,
        isNull,
      );
      expect(
        value.humanInTheLoop!.copyWith(approveWithEdits: null).approveWithEdits,
        isNull,
      );
      expect(value.copyWith(custom: null).custom, isNull);
    });

    test('execution limits use the manifest shared integral int64 domain', () {
      for (final value in <num>[-1, 0, 1.0]) {
        expect(
          ExecutionCapabilities.fromJson({
            'maxIterations': value,
          }).maxIterations,
          value.toInt(),
        );
      }

      for (final value in <Object?>[
        1.5,
        double.nan,
        double.infinity,
        '1',
        true,
        9.223372036854776e18,
        -9.223372036854778e18,
      ]) {
        expect(
          () => ExecutionCapabilities.fromJson({'maxExecutionTime': value}),
          throwsA(_fieldError('maxExecutionTime')),
          reason: value.toString(),
        );
      }
      expect(
        () => ExecutionCapabilities(maxIterations: 1.5),
        throwsA(_fieldError('maxIterations')),
      );
      expect(
        () => ExecutionCapabilities(maxIterations: 1)
            .copyWith(maxIterations: '1'),
        throwsA(_fieldError('maxIterations')),
      );
    });

    test(
      'execution limits accept signed int64 endpoints on the VM',
      () {
        final minimum = int.parse('-9223372036854775808');
        final maximum = int.parse('9223372036854775807');
        for (final value in <int>[minimum, maximum]) {
          expect(
            ExecutionCapabilities.fromJson({
              'maxIterations': value,
            }).maxIterations,
            value,
          );
        }
      },
      testOn: 'vm',
    );

    test(
      'unknown fields are dropped without changing arbitrary JSON values',
      () {
        final custom = {
          'nullable': null,
          'nested': [false, 0, <String, dynamic>{}],
        };
        final value = AgentCapabilities.fromJson({
          'custom': custom,
          'unknown': {'drop': true},
        });
        expect(value.toJson(), {'custom': custom});
      },
    );

    test('maps reject non-string keys with structured field paths', () {
      final invalid = <Map<String, dynamic>, String>{
        {
          'identity': {
            'metadata': <Object?, Object?>{1: 'value'},
          },
        }: 'identity.metadata',
        {
          'custom': <Object?, Object?>{1: 'value'},
        }: 'custom',
        {
          'custom': {
            'nested': <Object?, Object?>{1: 'value'},
          },
        }: 'custom.nested',
        {
          'identity': {
            'metadata': {
              'items': [
                <Object?, Object?>{1: 'value'},
              ],
            },
          },
        }: 'identity.metadata.items[0]',
        {
          'tools': {
            'items': [
              <Object?, Object?>{1: 'value'},
            ],
          },
        }: 'tools.items[0]',
        {
          'tools': {
            'items': [
              {
                'name': 'tool',
                'description': 'tool',
                'parameters': <String, dynamic>{},
                'metadata': {
                  'nested': <Object?, Object?>{1: 'value'},
                },
              },
            ],
          },
        }: 'tools.items[0].metadata.nested',
      };
      for (final entry in invalid.entries) {
        expect(
          () => AgentCapabilities.fromJson(entry.key),
          throwsA(_fieldError(entry.value)),
        );
      }
    });

    test('direct arbitrary maps are revalidated during encoding', () {
      const capabilities = AgentCapabilities(
        custom: {
          'nested': <Object?, Object?>{1: 'value'},
        },
      );
      expect(
        capabilities.toJson,
        throwsA(_fieldError('custom.nested')),
      );
    });
  });
}
