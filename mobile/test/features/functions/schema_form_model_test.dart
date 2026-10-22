import 'package:croupier_mobile/features/functions/schema_form_model.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('classify 映射矩阵（§2.6 每形态一例）', () {
    test('string 无 enum → text', () {
      expect(SchemaFormSpec.classify({'type': 'string'}), FieldKind.text);
    });

    test('string + enum ≤4 → segmented，>4 → dropdown', () {
      expect(
        SchemaFormSpec.classify({
          'type': 'string',
          'enum': ['a', 'b'],
        }),
        FieldKind.enumSegmented,
      );
      expect(
        SchemaFormSpec.classify({
          'type': 'string',
          'enum': ['a', 'b', 'c', 'd'],
        }),
        FieldKind.enumSegmented,
      );
      expect(
        SchemaFormSpec.classify({
          'type': 'string',
          'enum': ['a', 'b', 'c', 'd', 'e'],
        }),
        FieldKind.enumDropdown,
      );
    });

    test('number / integer → 数字控件', () {
      expect(SchemaFormSpec.classify({'type': 'number'}), FieldKind.number);
      expect(SchemaFormSpec.classify({'type': 'integer'}), FieldKind.integer);
    });

    test('boolean → switch', () {
      expect(SchemaFormSpec.classify({'type': 'boolean'}), FieldKind.boolean);
    });

    test('标量数组 → tagInput，对象数组 → 只读', () {
      expect(
        SchemaFormSpec.classify({
          'type': 'array',
          'items': {'type': 'string'},
        }),
        FieldKind.scalarArray,
      );
      expect(
        SchemaFormSpec.classify({
          'type': 'array',
          'items': {'type': 'integer'},
        }),
        FieldKind.scalarArray,
      );
      expect(
        SchemaFormSpec.classify({
          'type': 'array',
          'items': {'type': 'object'},
        }),
        FieldKind.readOnlyJson,
      );
    });

    test('简单 object → 递归一层，含嵌套结构 → 只读', () {
      expect(
        SchemaFormSpec.classify({
          'type': 'object',
          'properties': {
            'a': {'type': 'string'},
            'b': {'type': 'boolean'},
          },
        }),
        FieldKind.simpleObject,
      );
      // 嵌套 object 即复杂。
      expect(
        SchemaFormSpec.classify({
          'type': 'object',
          'properties': {
            'a': {
              'type': 'object',
              'properties': {
                'x': {'type': 'string'},
              },
            },
          },
        }),
        FieldKind.readOnlyJson,
      );
      // object 内数组同样复杂。
      expect(
        SchemaFormSpec.classify({
          'type': 'object',
          'properties': {
            'a': {
              'type': 'array',
              'items': {'type': 'string'},
            },
          },
        }),
        FieldKind.readOnlyJson,
      );
    });

    test('无 type（anyOf/缺声明）→ 只读兜底', () {
      expect(
        SchemaFormSpec.classify({
          'anyOf': [
            {'type': 'string'},
          ],
        }),
        FieldKind.readOnlyJson,
      );
    });
  });

  group('fromSchema 分类与只读拆分', () {
    test('无 schema / 无 properties → rawEditor', () {
      expect(SchemaFormSpec.fromSchema(null).rawEditor, isTrue);
      expect(SchemaFormSpec.fromSchema({}).rawEditor, isTrue);
      expect(SchemaFormSpec.fromSchema({'type': 'object'}).rawEditor, isTrue);
    });

    test('顶层简单参数进 fields，复杂参数进 readOnly', () {
      final spec = SchemaFormSpec.fromSchema({
        'type': 'object',
        'properties': {
          'playerId': {'type': 'integer'},
          'reason': {'type': 'string'},
          'nested': {
            'type': 'object',
            'properties': {
              'deep': {
                'type': 'object',
                'properties': {
                  'x': {'type': 'string'},
                },
              },
            },
          },
        },
        'required': ['playerId'],
      });
      expect(spec.rawEditor, isFalse);
      expect(spec.fields.map((f) => f.name).toList(), ['playerId', 'reason']);
      expect(spec.readOnly.map((f) => f.name).toList(), ['nested']);
      final playerId = spec.fields.first;
      expect(playerId.required, isTrue);
      expect(spec.fields[1].required, isFalse);
    });
  });

  group('defaults 初值', () {
    test('显式 default 优先；boolean false；数组 []；object {}', () {
      final spec = SchemaFormSpec.fromSchema({
        'type': 'object',
        'properties': {
          'name': {'type': 'string', 'default': 'abc'},
          'flag': {'type': 'boolean'},
          'tags': {
            'type': 'array',
            'items': {'type': 'string'},
          },
          'pos': {
            'type': 'object',
            'properties': {
              'x': {'type': 'integer'},
              'on': {'type': 'boolean'},
            },
          },
        },
      });
      final d = spec.defaults();
      expect(d['name'], 'abc');
      expect(d['flag'], false);
      expect(d['tags'], isEmpty);
      expect(d['pos'], {'x': '', 'on': false});
    });
  });

  group('coerceScalar 值归一', () {
    test('integer/number 解析失败保原串，成功转数值', () {
      expect(coerceScalar(FieldKind.integer, '42'), 42);
      expect(coerceScalar(FieldKind.integer, '4.2'), '4.2');
      expect(coerceScalar(FieldKind.number, '3.14'), 3.14);
      expect(coerceScalar(FieldKind.text, 'x'), 'x');
    });
  });

  group('validateField 校验', () {
    SchemaField field(Map<String, Object?> prop, {bool required = false}) =>
        SchemaField(
          name: 'f',
          kind: SchemaFormSpec.classify(prop),
          propertySchema: prop,
          required: required,
        );

    test('必填：空串/空列表/ null 拦截', () {
      expect(
        validateField(field({'type': 'string'}, required: true), ''),
        '必填',
      );
      expect(
        validateField(
          field({
            'type': 'array',
            'items': {'type': 'string'},
          }, required: true),
          <Object?>[],
        ),
        '必填',
      );
      expect(
        validateField(field({'type': 'string'}, required: true), 'x'),
        isNull,
      );
    });

    test('长度边界', () {
      expect(
        validateField(field({'type': 'string', 'minLength': 3}), 'ab'),
        '至少 3 个字符',
      );
      expect(
        validateField(field({'type': 'string', 'maxLength': 2}), 'abc'),
        '最多 2 个字符',
      );
    });

    test('数值边界', () {
      expect(
        validateField(field({'type': 'integer', 'minimum': 1}), 0),
        '不得小于 1',
      );
      expect(
        validateField(field({'type': 'number', 'maximum': 10}), 11),
        '不得大于 10',
      );
    });

    test('pattern 命中/不命中，坏正则降级忽略', () {
      expect(
        validateField(field({'type': 'string', 'pattern': r'^\d+$'}), 'abc'),
        '格式不匹配',
      );
      expect(
        validateField(field({'type': 'string', 'pattern': r'^\d+$'}), '123'),
        isNull,
      );
      expect(
        validateField(field({'type': 'string', 'pattern': '('}), 'any'),
        isNull,
      );
    });
  });
}
