import 'dart:convert';

import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/function/function_spec.dart';
import 'package:croupier_mobile/core/function/function_spec_service.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

/// 非对象形态（裸数组/裸串）响应快捷构造：helpers 的 jsonResponse 只收 Map。
ResponseBody rawJson(Object? body) => ResponseBody.fromString(
  jsonEncode(body),
  200,
  headers: {
    Headers.contentTypeHeader: <String>[Headers.jsonContentType],
  },
);

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;
  late FunctionSpecService service;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
    final client = ApiClient.create(
      baseUrl: 'http://gm.test',
      sessionStore: store,
    );
    client.dio.httpClientAdapter = adapter;
    service = FunctionSpecService(client: client);
  });

  test('解析 {functions:[...]}（后端实况形态）', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'functions': [
        {
          'id': 'player.kick',
          'version': '1.2.0',
          'enabled': true,
          'execution': 'sync',
          'executionState': 'bound',
          'risk': 'high',
          'approval': {'required': true, 'policyKey': 'two.person'},
          'tags': ['player', 'ops'],
          'summary': {'zh-CN': '踢人', 'en-US': 'Kick'},
          'description': {'zh-CN': '把玩家踢下线'},
          'inputSchema': {
            'type': 'object',
            'properties': {
              'playerId': {'type': 'string'},
            },
            'required': ['playerId'],
          },
        },
      ],
    });

    final specs = await service.list();

    expect(adapter.lastRequest?.path, '/api/v1/functions/descriptors');
    expect(specs, hasLength(1));
    final spec = specs.first;
    expect(spec.id, 'player.kick');
    expect(spec.version, '1.2.0');
    expect(spec.isHighRisk, isTrue);
    expect(spec.approvalRequired, isTrue);
    expect(spec.approvalPolicyKey, 'two.person');
    expect(spec.tags, ['player', 'ops']);
    expect(spec.displayName, '踢人');
    expect(spec.displayDescription, '把玩家踢下线');
    expect(spec.hasFormSchema, isTrue);
    expect(spec.executable, isTrue);
  });

  test('裸数组形态兜底（对齐 web listDescriptors），不经 Map 强转', () async {
    adapter.handler = (options, _) => rawJson([
      {'id': 'player.kick', 'risk': 'high'},
      'not-a-map',
    ]);

    final specs = await service.list();

    expect(specs, hasLength(1));
    expect(specs.first.isHighRisk, isTrue);
  });

  test('非对象非数组（裸串）→ StateError 静默网可接', () async {
    adapter.handler = (options, _) => rawJson('oops');

    await expectLater(service.list(), throwsA(isA<StateError>()));
  });

  test('形态兜底 {items:[...]} 与缺 id 跳过', () {
    final specs = FunctionSpecService.parseSpecs({
      'items': [
        {'id': 'a', 'risk': 'danger'},
        {'name': '缺 id'},
        'not-a-map',
      ],
    });
    expect(specs.map((s) => s.id), ['a']);
    expect(specs.first.isHighRisk, isTrue);
  });

  test('非法形态 fail-fast StateError（防 HTML 兜底当空目录）', () {
    expect(
      () => FunctionSpecService.parseSpecs({'data': <Object?>[]}),
      throwsA(isA<StateError>()),
    );
  });

  test('字段缺失按默认值：risk 空 / enabled true / executionState 空按 bound', () {
    final specs = FunctionSpecService.parseSpecs({
      'functions': [
        {'id': 'a'},
      ],
    });
    final spec = specs.single;
    expect(spec.risk, '');
    expect(spec.isHighRisk, isFalse);
    expect(spec.approvalRequired, isFalse);
    expect(spec.enabled, isTrue);
    expect(spec.executable, isTrue);
    expect(spec.displayName, 'a');
    expect(spec.hasFormSchema, isFalse);
    expect(spec.defaultsToTask, isFalse);
  });

  test('unbound / disabled 不可执行；execution=task 默认异步', () {
    final specs = FunctionSpecService.parseSpecs({
      'functions': [
        {'id': 'u', 'executionState': 'unbound'},
        {'id': 'd', 'enabled': false},
        {'id': 't', 'execution': 'task'},
      ],
    });
    expect(specs[0].executable, isFalse);
    expect(specs[1].executable, isFalse);
    expect(specs[2].defaultsToTask, isTrue);
  });

  test('deprecated / resource / operation / permission 透传', () {
    final specs = FunctionSpecService.parseSpecs({
      'functions': [
        {
          'id': 'player.update',
          'deprecated': true,
          'resource': 'player',
          'operation': 'update',
          'capability': 'update',
          'permission': 'player:write',
        },
      ],
    });
    final spec = specs.single;
    expect(spec.deprecated, isTrue);
    expect(spec.resource, 'player');
    expect(spec.operation, 'update');
    expect(spec.capability, 'update');
    expect(spec.permission, 'player:write');
  });

  group('localizedText 契约（BCP47）', () {
    test('zh-CN 优先，其次 en-US，最后 fallback', () {
      expect(localizedText({'zh-CN': '甲', 'en-US': 'A'}, '兜底'), '甲');
      expect(localizedText({'en-US': 'A'}, '兜底'), 'A');
      expect(localizedText({}, '兜底'), '兜底');
      // 空串视为缺失
      expect(localizedText({'zh-CN': '', 'en-US': 'A'}, '兜底'), 'A');
    });

    test('裸字符串形态归一到 zh-CN 键（不自造短 key）', () {
      final spec = FunctionSpecService.parseSpecs({
        'functions': [
          {'id': 'a', 'summary': '裸字符串摘要'},
        ],
      }).single;
      expect(spec.summary.keys, ['zh-CN']);
      expect(spec.displayName, '裸字符串摘要');
    });

    test('localizedTextFrom 覆盖 map/裸串/非法三类', () {
      expect(localizedTextFrom({'zh-CN': '甲'}, '兜底'), '甲');
      expect(localizedTextFrom('乙', '兜底'), '乙');
      expect(localizedTextFrom(42, '兜底'), '兜底');
    });
  });
}
