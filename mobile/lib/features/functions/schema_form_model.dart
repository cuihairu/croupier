/// JSON Schema → Flutter 控件映射模型（设计稿 §2.6）。
///
/// 收敛原则：**简单参数表单化，复杂结构只读化**。顶层任一参数为复杂形态
/// （嵌套 object / object 数组）时该参数只读展示，不进入表单编辑区。
///
/// 本文件只做纯逻辑（分类 / 默认值 / 值归一 / 校验），不依赖 Flutter，
/// 便于单测覆盖映射矩阵；控件渲染在 `schema_form.dart`。
library;

import '../../core/function/function_spec.dart';

/// 单参数的控件形态（§2.6 映射表）。
enum FieldKind {
  /// string 无 enum → TextFormField
  text,

  /// string + enum（≤4）→ SegmentedButton
  enumSegmented,

  /// string + enum（>4）→ DropdownButtonFormField
  enumDropdown,

  /// number → 数字 TextFormField
  number,

  /// integer → 数字 TextFormField + 取整校验
  integer,

  /// boolean → SwitchListTile
  boolean,

  /// array of string/number → 自绘 TagInput
  scalarArray,

  /// object 且全部属性为简单类型 → ExpansionTile 内递归一层简单控件
  simpleObject,

  /// array of object / 嵌套 object / 无类型 → 只读 JSON 视图
  readOnlyJson,

  /// 无 schema / schema 无 properties → 原始 JSON 编辑器兜底
  rawJson,
}

/// 顶层参数的分类结果（含渲染所需元数据）。
class SchemaField {
  const SchemaField({
    required this.name,
    required this.kind,
    required this.propertySchema,
    this.required = false,
  });

  final String name;
  final FieldKind kind;
  final Map<String, Object?> propertySchema;
  final bool required;

  /// schema 声明的默认值（无则 null）。
  Object? get defaultValue => propertySchema['default'];

  String get title {
    final t = localizedTextFrom(propertySchema['title'], '');
    if (t.isNotEmpty) return t;
    final d = localizedTextFrom(propertySchema['description'], '');
    return d.isNotEmpty ? d : name;
  }

  String get description =>
      localizedTextFrom(propertySchema['description'], '');

  String get type => _typeOf(propertySchema) ?? '';

  List<String> get enumValues {
    final raw = propertySchema['enum'];
    if (raw is! List) return const [];
    return raw.map((e) => e?.toString() ?? '').toList(growable: false);
  }

  /// simpleObject 的子属性（简单类型，递归一层）。
  Map<String, Object?> get properties {
    final raw = propertySchema['properties'];
    if (raw is Map) return Map<String, Object?>.from(raw);
    return const {};
  }
}

/// 一次描述符的顶层表单规格。
class SchemaFormSpec {
  const SchemaFormSpec({
    required this.fields,
    required this.readOnly,
    this.rawEditor = false,
  });

  /// 表单化字段（按 schema properties 顺序）。
  final List<SchemaField> fields;

  /// 只读参数（原始 JSON 展示，提交时取原默认值，不参与编辑）。
  final List<SchemaField> readOnly;

  /// 无可用 properties（或无 schema）→ 顶层退化为原始 JSON 编辑器。
  final bool rawEditor;

  bool get isEmpty => fields.isEmpty && readOnly.isEmpty && !rawEditor;

  static SchemaFormSpec fromSchema(Map<String, Object?>? schema) {
    if (schema == null || schema.isEmpty) {
      return const SchemaFormSpec(fields: [], readOnly: [], rawEditor: true);
    }
    final propsRaw = schema['properties'];
    if (propsRaw is! Map || propsRaw.isEmpty) {
      return const SchemaFormSpec(fields: [], readOnly: [], rawEditor: true);
    }
    final requiredRaw = schema['required'];
    final required = <String>{};
    if (requiredRaw is List) {
      for (final r in requiredRaw) {
        if (r is String) required.add(r);
      }
    }
    final fields = <SchemaField>[];
    final readOnly = <SchemaField>[];
    propsRaw.forEach((key, value) {
      if (key is! String || value is! Map) return;
      final prop = Map<String, Object?>.from(value);
      final kind = classify(prop);
      final field = SchemaField(
        name: key,
        kind: kind,
        propertySchema: prop,
        required: required.contains(key),
      );
      if (kind == FieldKind.readOnlyJson) {
        readOnly.add(field);
      } else {
        fields.add(field);
      }
    });
    return SchemaFormSpec(fields: fields, readOnly: readOnly);
  }

  /// 单属性分类（§2.6 映射表）。可单测独立覆盖。
  static FieldKind classify(Map<String, Object?> prop) {
    final type = _typeOf(prop);
    switch (type) {
      case 'string':
        final enumRaw = prop['enum'];
        if (enumRaw is List && enumRaw.isNotEmpty) {
          return enumRaw.length <= 4
              ? FieldKind.enumSegmented
              : FieldKind.enumDropdown;
        }
        return FieldKind.text;
      case 'number':
        return FieldKind.number;
      case 'integer':
        return FieldKind.integer;
      case 'boolean':
        return FieldKind.boolean;
      case 'array':
        final items = prop['items'];
        if (items is Map) {
          final itemType = _typeOf(Map<String, Object?>.from(items));
          if (itemType == 'string' ||
              itemType == 'number' ||
              itemType == 'integer') {
            return FieldKind.scalarArray;
          }
        }
        return FieldKind.readOnlyJson;
      case 'object':
        final nested = prop['properties'];
        if (nested is Map && nested.isNotEmpty && _allSimple(nested)) {
          return FieldKind.simpleObject;
        }
        return FieldKind.readOnlyJson;
      default:
        // 无 type（anyOf/oneOf/缺声明）→ 只读兜底，不猜控件。
        return FieldKind.readOnlyJson;
    }
  }

  /// object 的全部属性是否都是简单类型（string/number/integer/boolean
  /// 含 enum）；出现嵌套 object/array 即判复杂。
  static bool _allSimple(Map nested) {
    for (final value in nested.values) {
      if (value is! Map) return false;
      final type = _typeOf(Map<String, Object?>.from(value));
      if (type != 'string' &&
          type != 'number' &&
          type != 'integer' &&
          type != 'boolean') {
        return false;
      }
    }
    return true;
  }

  /// 表单初值：schema 默认值优先；boolean 缺省 false，标量数组缺省 []，
  /// 简单 object 缺省 {}（对齐 web buildSchemaDefaults）。
  Map<String, Object?> defaults() {
    final values = <String, Object?>{};
    for (final f in fields) {
      values[f.name] = _defaultFor(f);
    }
    return values;
  }

  static Object? _defaultFor(SchemaField f) {
    final declared = f.defaultValue;
    if (declared != null) return declared;
    switch (f.kind) {
      case FieldKind.boolean:
        return false;
      case FieldKind.scalarArray:
        return <Object?>[];
      case FieldKind.simpleObject:
        final obj = <String, Object?>{};
        f.properties.forEach((key, value) {
          if (value is! Map) return;
          final prop = Map<String, Object?>.from(value);
          final subKind = classify(prop);
          obj[key] =
              prop['default'] ?? (subKind == FieldKind.boolean ? false : '');
        });
        return obj;
      default:
        return '';
    }
  }
}

String? _typeOf(Map<String, Object?> prop) {
  final t = prop['type'];
  if (t is String) return t;
  return null;
}

/// 字符串输入 → 目标类型的归一（对齐 web normalizeConfigBySchema）。
/// 解析失败返回 [sentinelError] 之外的原字符串，交由校验层拦截。
Object? coerceScalar(FieldKind kind, String raw) {
  final text = raw.trim();
  switch (kind) {
    case FieldKind.integer:
      return int.tryParse(text) ?? text;
    case FieldKind.number:
      return num.tryParse(text) ?? text;
    default:
      return raw;
  }
}

/// 必填 + 数值边界 + 字符串长度/正则校验（§2.6 validator 映射）。
/// 返回 null = 通过；非 null = 错误文案。
String? validateField(SchemaField field, Object? value) {
  if (field.required) {
    if (value == null) return '必填';
    if (value is String && value.trim().isEmpty) return '必填';
    if (value is List && value.isEmpty) return '必填';
  }
  if (value is String && value.isNotEmpty) {
    final minLength = field.propertySchema['minLength'];
    if (minLength is int && value.length < minLength) {
      return '至少 $minLength 个字符';
    }
    final maxLength = field.propertySchema['maxLength'];
    if (maxLength is int && value.length > maxLength) {
      return '最多 $maxLength 个字符';
    }
    final pattern = field.propertySchema['pattern'];
    if (pattern is String && pattern.isNotEmpty) {
      try {
        if (!RegExp(pattern).hasMatch(value)) return '格式不匹配';
      } on FormatException {
        // 坏正则降级忽略，不阻塞表单。
      }
    }
  }
  if (value is num) {
    final minimum = field.propertySchema['minimum'];
    if (minimum is num && value < minimum) return '不得小于 $minimum';
    final maximum = field.propertySchema['maximum'];
    if (maximum is num && value > maximum) return '不得大于 $maximum';
  }
  return null;
}
