/// 描述符驱动动态表单渲染（设计稿 §2.6 映射表）。
///
/// 与 [SchemaFormSpec] 配套：spec 负责分类/默认值/校验（纯逻辑），本文件
/// 负责控件渲染与值收集。提交前由 [SchemaFormController.validate] 统一校验。
library;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'schema_form_model.dart';

/// 表单值 + 校验状态载体。页面持有一份，提交时读 [payload]。
class SchemaFormController extends ChangeNotifier {
  SchemaFormController(SchemaFormSpec spec)
    : values = spec.defaults(),
      _errors = {};

  final Map<String, Object?> values;
  final Map<String, String?> _errors;

  /// 文本字段控制器缓存：避免每次 build 重建控制器导致光标跳到末尾。
  final Map<String, TextEditingController> _textControllers = {};

  /// 原始 JSON 编辑器文本（rawEditor 模式）；非该模式为 null。
  String? rawJson;

  /// 取（或懒建）指定字段的文本控制器，初值取自当前表单值。
  TextEditingController textController(String name, {Object? initial}) {
    return _textControllers.putIfAbsent(
      name,
      () => TextEditingController(
        text: (initial ?? values[name])?.toString() ?? '',
      ),
    );
  }

  String? errorOf(String name) => _errors[name];

  void setValue(String name, Object? value) {
    values[name] = value;
    if (_errors[name] != null) {
      _errors[name] = null;
    }
    notifyListeners();
  }

  void setRawJson(String text) {
    rawJson = text;
    notifyListeners();
  }

  @override
  void dispose() {
    for (final c in _textControllers.values) {
      c.dispose();
    }
    super.dispose();
  }

  /// 校验全部表单字段。返回 null = 通过；否则返回首个错误文案并刷新
  /// 各字段内联错误。
  String? validate(SchemaFormSpec spec) {
    var firstError = '';
    for (final field in spec.fields) {
      if (field.kind == FieldKind.simpleObject) {
        // 简单 object 逐子属性校验（required 只作用于子属性自身声明）。
        final obj = values[field.name];
        if (obj is Map) {
          obj.forEach((key, value) {
            final raw = field.properties[key];
            if (raw is! Map) return;
            final sub = SchemaField(
              name: '$key',
              kind: SchemaFormSpec.classify(Map<String, Object?>.from(raw)),
              propertySchema: Map<String, Object?>.from(raw),
            );
            final err = validateField(sub, value);
            if (err != null) firstError = firstError.isEmpty ? err : firstError;
          });
        }
        continue;
      }
      final err = validateField(field, values[field.name]);
      _errors[field.name] = err;
      if (err != null && firstError.isEmpty) firstError = err;
    }
    notifyListeners();
    return firstError.isEmpty ? null : firstError;
  }

  /// 提交载荷：coerce 标量 + 过滤空值。rawEditor 模式在此不参与
  /// （页面按 rawJson 单独解析），只读参数由页面合并。
  Map<String, Object?> payload(SchemaFormSpec spec) {
    final out = <String, Object?>{};
    for (final field in spec.fields) {
      final value = values[field.name];
      if (value == null) continue;
      if (value is String && value.isEmpty) continue;
      out[field.name] = value;
    }
    return out;
  }
}

class SchemaForm extends StatelessWidget {
  const SchemaForm({required this.spec, required this.controller, super.key});

  final SchemaFormSpec spec;
  final SchemaFormController controller;

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: controller,
      builder: (context, _) {
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (spec.rawEditor)
              _RawJsonField(controller: controller)
            else
              for (final field in spec.fields) ...[
                _FieldTile(field: field, controller: controller),
                const SizedBox(height: 12),
              ],
            if (spec.readOnly.isNotEmpty) _ReadOnlyNote(spec: spec),
          ],
        );
      },
    );
  }
}

class _FieldTile extends StatelessWidget {
  const _FieldTile({required this.field, required this.controller});

  final SchemaField field;
  final SchemaFormController controller;

  @override
  Widget build(BuildContext context) {
    return switch (field.kind) {
      FieldKind.text => _text(context),
      FieldKind.number => _text(context, numeric: true),
      FieldKind.integer => _text(context, numeric: true, integer: true),
      FieldKind.enumSegmented => _segmented(context),
      FieldKind.enumDropdown => _dropdown(context),
      FieldKind.boolean => _boolean(context),
      FieldKind.scalarArray => _scalarArray(context),
      FieldKind.simpleObject => _simpleObject(context),
      FieldKind.readOnlyJson => const SizedBox.shrink(),
      FieldKind.rawJson => const SizedBox.shrink(),
    };
  }

  String get _label => field.required ? '${field.title} *' : field.title;

  Widget _errorText() {
    final err = controller.errorOf(field.name);
    if (err == null) return const SizedBox.shrink();
    return Padding(
      padding: const EdgeInsets.only(top: 4),
      child: Text(err, style: const TextStyle(color: Colors.red, fontSize: 12)),
    );
  }

  Widget _text(
    BuildContext context, {
    bool numeric = false,
    bool integer = false,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        TextField(
          key: ValueKey('field-${field.name}'),
          controller: controller.textController(field.name),
          keyboardType: numeric ? TextInputType.number : TextInputType.text,
          inputFormatters: numeric
              ? [
                  FilteringTextInputFormatter.allow(
                    integer ? RegExp(r'[0-9-]') : RegExp(r'[0-9.-]'),
                  ),
                ]
              : null,
          decoration: InputDecoration(
            labelText: _label,
            helperText: field.description.isEmpty ? null : field.description,
            border: const OutlineInputBorder(),
          ),
          onChanged: (text) =>
              controller.setValue(field.name, coerceScalar(field.kind, text)),
        ),
        _errorText(),
      ],
    );
  }

  Widget _segmented(BuildContext context) {
    final options = field.enumValues;
    final current = controller.values[field.name]?.toString();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(_label, style: Theme.of(context).textTheme.labelLarge),
        const SizedBox(height: 4),
        SegmentedButton<String>(
          key: ValueKey('field-${field.name}'),
          segments: [
            for (final o in options)
              ButtonSegment<String>(value: o, label: Text(o)),
          ],
          selected: current != null && options.contains(current)
              ? {current}
              : const {},
          emptySelectionAllowed: true,
          onSelectionChanged: (set) =>
              controller.setValue(field.name, set.isEmpty ? '' : set.first),
        ),
        _errorText(),
      ],
    );
  }

  Widget _dropdown(BuildContext context) {
    final options = field.enumValues;
    final current = controller.values[field.name]?.toString();
    final valid = current != null && options.contains(current) ? current : null;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        DropdownButtonFormField<String>(
          key: ValueKey('field-${field.name}'),
          initialValue: valid,
          decoration: InputDecoration(
            labelText: _label,
            border: const OutlineInputBorder(),
          ),
          items: [
            for (final o in options)
              DropdownMenuItem<String>(value: o, child: Text(o)),
          ],
          onChanged: (v) => controller.setValue(field.name, v ?? ''),
        ),
        _errorText(),
      ],
    );
  }

  Widget _boolean(BuildContext context) {
    final current = controller.values[field.name] == true;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SwitchListTile(
          key: ValueKey('field-${field.name}'),
          contentPadding: EdgeInsets.zero,
          title: Text(_label),
          subtitle: field.description.isEmpty ? null : Text(field.description),
          value: current,
          onChanged: (v) => controller.setValue(field.name, v),
        ),
      ],
    );
  }

  Widget _scalarArray(BuildContext context) {
    final raw = controller.values[field.name];
    final items = raw is List
        ? raw.map((e) => e.toString()).toList()
        : <String>[];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(_label, style: Theme.of(context).textTheme.labelLarge),
        const SizedBox(height: 4),
        Wrap(
          spacing: 6,
          runSpacing: -8,
          children: [
            for (var i = 0; i < items.length; i++)
              Chip(
                label: Text(items[i]),
                onDeleted: () {
                  final next = [...items]..removeAt(i);
                  controller.setValue(field.name, next);
                },
              ),
          ],
        ),
        TextField(
          key: ValueKey('field-${field.name}-input'),
          decoration: const InputDecoration(hintText: '回车追加', isDense: true),
          onSubmitted: (text) {
            final v = text.trim();
            if (v.isEmpty) return;
            controller.setValue(field.name, [...items, v]);
          },
        ),
        _errorText(),
      ],
    );
  }

  Widget _simpleObject(BuildContext context) {
    final raw = controller.values[field.name];
    final obj = raw is Map
        ? Map<String, Object?>.from(raw)
        : <String, Object?>{};
    return Card(
      margin: EdgeInsets.zero,
      child: ExpansionTile(
        key: ValueKey('field-${field.name}'),
        title: Text(_label),
        childrenPadding: const EdgeInsets.all(12),
        children: [
          for (final entry in field.properties.entries)
            if (entry.value is Map)
              Builder(
                builder: (context) {
                  final prop = Map<String, Object?>.from(entry.value as Map);
                  final kind = SchemaFormSpec.classify(prop);
                  final subField = SchemaField(
                    name: '${field.name}.${entry.key}',
                    kind: kind,
                    propertySchema: prop,
                  );
                  final value = obj[entry.key];
                  return Padding(
                    padding: const EdgeInsets.only(bottom: 8),
                    child: _SimpleObjectField(
                      label: entry.key.toString(),
                      kind: kind,
                      value: value,
                      formController: controller,
                      onChanged: (v) {
                        final next = Map<String, Object?>.from(obj);
                        next[entry.key] = v;
                        controller.setValue(field.name, next);
                      },
                      field: subField,
                    ),
                  );
                },
              ),
        ],
      ),
    );
  }
}

/// 简单 object 的子控件（string/number/boolean 三种简单类型）。
class _SimpleObjectField extends StatelessWidget {
  const _SimpleObjectField({
    required this.label,
    required this.kind,
    required this.value,
    required this.onChanged,
    required this.field,
    required this.formController,
  });

  final String label;
  final FieldKind kind;
  final Object? value;
  final ValueChanged<Object?> onChanged;
  final SchemaField field;
  final SchemaFormController formController;

  @override
  Widget build(BuildContext context) {
    if (kind == FieldKind.boolean) {
      return SwitchListTile(
        contentPadding: EdgeInsets.zero,
        dense: true,
        title: Text(label),
        value: value == true,
        onChanged: onChanged,
      );
    }
    return TextField(
      controller: formController.textController(
        '${field.name}.$label',
        initial: value,
      ),
      keyboardType: (kind == FieldKind.number || kind == FieldKind.integer)
          ? TextInputType.number
          : TextInputType.text,
      decoration: InputDecoration(labelText: label, isDense: true),
      onChanged: (text) => onChanged(coerceScalar(kind, text)),
    );
  }
}

class _ReadOnlyNote extends StatelessWidget {
  const _ReadOnlyNote({required this.spec});

  final SchemaFormSpec spec;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(8),
      decoration: BoxDecoration(
        color: Colors.grey.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        '以下复杂参数只读展示，编辑请回 Web：'
        '${spec.readOnly.map((f) => f.name).join('、')}',
        style: const TextStyle(fontSize: 12, color: Colors.grey),
      ),
    );
  }
}

/// rawEditor 模式：原始 JSON 编辑器（jsonDecode 校验由页面在提交时执行）。
class _RawJsonField extends StatelessWidget {
  const _RawJsonField({required this.controller});

  final SchemaFormController controller;

  @override
  Widget build(BuildContext context) {
    return TextField(
      key: const ValueKey('field-__raw__'),
      controller: TextEditingController(text: controller.rawJson ?? ''),
      maxLines: 8,
      minLines: 6,
      keyboardType: TextInputType.multiline,
      decoration: const InputDecoration(
        labelText: '请求参数（JSON）',
        hintText: '{}',
        border: OutlineInputBorder(),
      ),
      onChanged: controller.setRawJson,
    );
  }
}
