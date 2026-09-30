/// 函数描述符契约模型（设计稿 §2.5 / §3.1）。
///
/// 唯一数据源：`GET /api/v1/functions/descriptors` → `{functions:[FunctionSpec]}`。
/// 一次拉取即覆盖列表展示（名称/摘要/标签）、动态表单（inputSchema）、
/// 与风险治理标记（risk / approval.required），不必再逐个函数查详情。
///
/// wire 出参：internal/dashboard/spec/types.go `spec.FunctionSpec`
/// （risk 为 safe/warning/high/danger；approval 为 {required, policyKey}；
/// execution 为 sync/task；executionState 为 bound/unbound）。
library;

/// 本地化文本归一（对齐 web `normalizeLocalizedText` / `localizedText`）：
/// key 恒为 BCP47（`zh-CN` / `en-US`），禁止自造短 key；缺失取 fallback。
String localizedText(Map<String, Object?> value, String fallback) {
  final zh = value['zh-CN'];
  if (zh is String && zh.isNotEmpty) return zh;
  final en = value['en-US'];
  if (en is String && en.isNotEmpty) return en;
  return fallback;
}

/// 本地化文本归一（裸字符串形态兜底：契约外遗留 / 程序化 payload）。
String localizedTextFrom(Object? raw, String fallback) {
  if (raw is String && raw.isNotEmpty) return raw;
  if (raw is Map) {
    return localizedText(Map<String, Object?>.from(raw), fallback);
  }
  return fallback;
}

class FunctionSpec {
  const FunctionSpec({
    required this.id,
    this.version = '',
    this.enabled = true,
    this.deprecated = false,
    this.executionState = '',
    this.execution = '',
    this.capability = '',
    this.resource = '',
    this.operation = '',
    this.risk = '',
    this.approvalRequired = false,
    this.approvalPolicyKey = '',
    this.permission = '',
    this.tags = const [],
    this.summary = const {},
    this.description = const {},
    this.inputSchema,
    this.outputSchema,
  });

  final String id;
  final String version;
  final bool enabled;
  final bool deprecated;

  /// bound = 运行时已注册可执行；unbound = 纯物料（执行边界 409 executor_unbound）。
  final String executionState;

  /// sync 或 task（task 走异步任务生命周期，见 §2.5）。
  final String execution;
  final String capability;
  final String resource;
  final String operation;

  /// safe / warning / high / danger；空 = 契约未声明（按低风险展示）。
  final String risk;
  final bool approvalRequired;
  final String approvalPolicyKey;
  final String permission;
  final List<String> tags;
  final Map<String, Object?> summary;
  final Map<String, Object?> description;

  /// 请求 JSON Schema（inline JSON，非 base64——后端 JSONSchema.MarshalJSON）。
  final Map<String, Object?>? inputSchema;
  final Map<String, Object?>? outputSchema;

  /// 展示名：summary 缺失时退 id（对齐 web normalizeFunctionDetail）。
  String get displayName {
    final name = localizedText(summary, '');
    if (name.isNotEmpty) return name;
    final desc = localizedText(description, '');
    if (desc.isNotEmpty) return desc;
    return id;
  }

  /// 列表副行：中文摘要缺失时用 description。
  String get displayDescription =>
      localizedText(description, localizedText(summary, ''));

  /// 高危标记（§2.6 审批标签同款口径）。
  bool get isHighRisk => risk == 'high' || risk == 'danger';

  /// 契约声明的 inputSchema 是否可表单化（有 properties 才渲染控件）。
  bool get hasFormSchema {
    final props = inputSchema?['properties'];
    return props is Map && props.isNotEmpty;
  }

  /// execution=task 的函数默认走异步任务模式。
  bool get defaultsToTask => execution == 'task';

  /// 可执行性：契约 enabled 且运行时已绑定（executionState 空按 bound 处理，
  /// 与 web NormalizeExecutionState 一致）。
  bool get executable => enabled && executionState != 'unbound';

  static FunctionSpec? fromJson(Map<String, Object?> json) {
    final id = json['id'];
    if (id is! String || id.isEmpty) return null;
    return FunctionSpec(
      id: id,
      version: json['version'] is String ? json['version'] as String : '',
      // enabled 默认 true：字段缺失按 Web 契约（未显式禁用即可用）处理。
      enabled: json['enabled'] is bool ? json['enabled'] as bool : true,
      deprecated: json['deprecated'] == true,
      executionState: json['executionState'] is String
          ? json['executionState'] as String
          : '',
      execution: json['execution'] is String ? json['execution'] as String : '',
      capability: json['capability'] is String
          ? json['capability'] as String
          : '',
      resource: json['resource'] is String ? json['resource'] as String : '',
      operation: json['operation'] is String ? json['operation'] as String : '',
      risk: json['risk'] is String ? json['risk'] as String : '',
      approvalRequired: _approvalRequired(json['approval']),
      approvalPolicyKey: _approvalPolicyKey(json['approval']),
      permission: json['permission'] is String
          ? json['permission'] as String
          : '',
      tags: _stringList(json['tags']),
      summary: _localizedMap(json['summary']),
      description: _localizedMap(json['description']),
      inputSchema: _jsonMap(json['inputSchema']),
      outputSchema: _jsonMap(json['outputSchema']),
    );
  }

  /// approval.required 嵌套对象；顶层 approvalRequired 布尔为向前兼容
  /// （早期描述符以扁平字段投影）。
  static bool _approvalRequired(Object? raw) {
    if (raw is Map) {
      final required = raw['required'];
      if (required is bool) return required;
    }
    return false;
  }

  static String _approvalPolicyKey(Object? raw) {
    if (raw is Map) {
      final key = raw['policyKey'];
      if (key is String) return key;
    }
    return '';
  }

  static List<String> _stringList(Object? raw) {
    if (raw is! List) return const [];
    return raw.whereType<String>().toList(growable: false);
  }

  /// LocalizedText（BCP47 map）；裸字符串退化为 `{'zh-CN': text}`，
  /// 保证展示层只有一条取值链。
  static Map<String, Object?> _localizedMap(Object? raw) {
    if (raw is Map) return Map<String, Object?>.from(raw);
    if (raw is String && raw.isNotEmpty) return {'zh-CN': raw};
    return const {};
  }

  static Map<String, Object?>? _jsonMap(Object? raw) {
    if (raw is Map) return Map<String, Object?>.from(raw);
    return null;
  }
}
