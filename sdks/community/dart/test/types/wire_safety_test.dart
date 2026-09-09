import 'package:ag_ui/src/types/base.dart';
import 'package:ag_ui/src/types/wire_safety.dart';
import 'package:test/test.dart';

AGUIValidationError _capture(void Function() action) {
  try {
    action();
  } on AGUIValidationError catch (error) {
    return error;
  }
  fail('Expected AGUIValidationError');
}

void main() {
  group('cipher-aware wire safety', () {
    test('preserves ordinary diagnostics for nonsensitive payloads', () {
      final json = <String, dynamic>{
        'metadata': ['bad']
      };
      final error = _capture(
        () => readCipherAwareOptionalField<Map<String, dynamic>>(
          json,
          'metadata',
        ),
      );

      expect(error.value, same(json['metadata']));
      expect(error.json, same(json));
    });

    test('recursively detects a child cipher and drops raw payloads', () {
      final json = <String, dynamic>{
        'metadata': ['nested-secret'],
        'children': [
          {'encryptedValue': 'cipher-secret'},
        ],
      };
      final error = _capture(
        () => readCipherAwareOptionalField<Map<String, dynamic>>(
          json,
          'metadata',
        ),
      );

      expect(error.value, 'List<String>');
      expect(error.json, isNull);
      expect(error.cause, isNull);
      expect(error.toString(), isNot(contains('cipher-secret')));
      expect(error.toString(), isNot(contains('nested-secret')));
    });

    test('dual-key reads retain camel-case presence precedence', () {
      expect(
        readCipherAwareOptionalEitherField<String>(
          {
            'subagentRunId': null,
            'subagent_run_id': 'snake',
          },
          'subagentRunId',
          'subagent_run_id',
        ),
        isNull,
      );
    });

    test('an enclosing cipher scrubs a nested validation error', () {
      final innerJson = <String, dynamic>{
        'metadata': ['nested-secret']
      };
      final inner = _capture(
        () => JsonDecoder.optionalField<Map<String, dynamic>>(
          innerJson,
          'metadata',
        ),
      );
      final wrapped = wrapNestedValidationError(
        enclosingJson: {
          'encryptedValue': 'parent-cipher',
          'child': innerJson,
        },
        error: inner,
        field: 'child.metadata',
      );

      expect(wrapped.field, 'child.metadata');
      expect(wrapped.value, 'List<String>');
      expect(wrapped.json, isNull);
      expect(wrapped.cause, isNull);
      expect(wrapped.toString(), isNot(contains('parent-cipher')));
      expect(wrapped.toString(), isNot(contains('nested-secret')));
    });
  });
}
