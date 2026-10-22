import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/function/function_spec.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/functions/function_invoke_page.dart';
import 'package:croupier_mobile/features/functions/functions_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
  });

  Widget wrap(Widget child) {
    return ProviderScope(
      overrides: [
        sessionStoreProvider.overrideWithValue(store),
        apiClientFactoryProvider.overrideWith((ref) {
          return (baseUrl) {
            final client = ApiClient.create(
              baseUrl: baseUrl,
              sessionStore: ref.read(sessionStoreProvider),
            );
            client.dio.httpClientAdapter = adapter;
            return client;
          };
        }),
      ],
      child: MaterialApp(home: child),
    );
  }

  Future<void> login() => store.save(
    const SessionData(
      token: 'jwt',
      gameId: 'demo',
      env: 'prod',
      serverUrl: 'http://gm.test',
    ),
  );

  final kickSpec = FunctionSpec.fromJson({
    'id': 'player.kick',
    'summary': {'zh-CN': '踢人'},
    'risk': 'high',
    'approval': {'required': true},
    'execution': 'sync',
    'inputSchema': {
      'type': 'object',
      'properties': {
        'playerId': {'type': 'integer', 'title': '玩家 ID'},
        'reason': {'type': 'string', 'title': '原因'},
      },
      'required': ['playerId'],
    },
  })!;

  testWidgets('目录页：加载目录 + 高危/审批标签 + 搜索过滤', (WidgetTester tester) async {
    await login();
    adapter.handler = (options, _) => jsonResponse(200, {
      'functions': [
        {
          'id': 'player.kick',
          'summary': {'zh-CN': '踢人'},
          'risk': 'high',
          'approval': {'required': true},
          'execution': 'sync',
        },
        {
          'id': 'mail.batch',
          'summary': {'zh-CN': '批量邮件'},
          'execution': 'task',
        },
      ],
    });
    await tester.pumpWidget(wrap(const FunctionsPage()));
    await tester.pumpAndSettle();

    expect(find.text('player.kick'), findsOneWidget);
    expect(find.text('高危'), findsOneWidget);
    expect(find.text('需审批'), findsOneWidget);

    await tester.enterText(
      find.byKey(const ValueKey('function-search')),
      'mail',
    );
    await tester.pumpAndSettle();
    expect(find.text('player.kick'), findsNothing);
    expect(find.text('mail.batch'), findsOneWidget);
  });

  testWidgets('目录页：空态 + 加载失败重试', (WidgetTester tester) async {
    await login();
    adapter.handler = (options, _) => jsonResponse(500, {'message': 'boom'});
    await tester.pumpWidget(wrap(const FunctionsPage()));
    await tester.pumpAndSettle();
    expect(find.text('boom'), findsOneWidget);

    adapter.handler = (options, _) => jsonResponse(200, {'functions': []});
    await tester.tap(find.byKey(const ValueKey('functions-retry')));
    await tester.pumpAndSettle();
    expect(find.text('当前 scope 下无可调用函数'), findsOneWidget);
  });

  testWidgets('调用页：表单校验 + 同步结果渲染', (WidgetTester tester) async {
    await login();
    final bodies = <String>[];
    adapter.handler = (options, raw) {
      if (options.uri.path == '/api/v1/functions/player.kick/invoke') {
        bodies.add(String.fromCharCodes(raw ?? const <int>[]));
        return jsonResponse(200, {
          'result': {'kicked': true},
        });
      }
      return jsonResponse(200, {});
    };
    await tester.pumpWidget(wrap(FunctionInvokePage(spec: kickSpec)));
    await tester.pumpAndSettle();

    // 审批 banner + 表单字段存在。
    expect(find.byKey(const ValueKey('approval-banner')), findsOneWidget);
    expect(find.byKey(const ValueKey('field-playerId')), findsOneWidget);
    expect(find.byKey(const ValueKey('field-reason')), findsOneWidget);

    // 必填 playerId 空 → 校验拦截，不发请求。
    await tester.tap(find.byKey(const ValueKey('invoke-submit')));
    await tester.pumpAndSettle();
    expect(bodies, isEmpty);

    await tester.enterText(find.byKey(const ValueKey('field-playerId')), '42');
    await tester.tap(find.byKey(const ValueKey('invoke-submit')));
    await tester.pumpAndSettle();

    expect(bodies.single, contains('"playerId":42'));
    expect(find.byKey(const ValueKey('sync-result')), findsOneWidget);
  });

  testWidgets('调用页：approvalRequired 结果提示已提交审批', (WidgetTester tester) async {
    await login();
    adapter.handler = (options, _) =>
        jsonResponse(200, {'approvalRequired': true, 'approvalId': 'ap-9'});
    await tester.pumpWidget(wrap(FunctionInvokePage(spec: kickSpec)));
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(const ValueKey('field-playerId')), '7');
    await tester.tap(find.byKey(const ValueKey('invoke-submit')));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('approval-submitted')), findsOneWidget);
    expect(find.textContaining('ap-9'), findsOneWidget);
  });

  testWidgets('调用页：异步开关 → taskId → 任务卡进度', (WidgetTester tester) async {
    await login();
    final asyncSpec = FunctionSpec.fromJson({
      'id': 'mail.batch',
      'execution': 'task',
      'inputSchema': {
        'type': 'object',
        'properties': {
          'count': {'type': 'integer'},
        },
      },
    })!;
    var mode = '';
    adapter.handler = (options, raw) {
      if (options.uri.path == '/api/v1/functions/mail.batch/invoke') {
        final body = String.fromCharCodes(raw ?? const <int>[]);
        if (body.contains('"mode":"async"')) mode = 'async';
        if (body.contains('"mode":"sync"')) mode = 'sync';
        return jsonResponse(200, {'taskId': 'task-1'});
      }
      return jsonResponse(200, {
        'id': 'task-1',
        'status': 'running',
        'progress': 40,
      });
    };
    await tester.pumpWidget(wrap(FunctionInvokePage(spec: asyncSpec)));
    await tester.pumpAndSettle();

    // execution=task → 默认异步开。
    await tester.tap(find.byKey(const ValueKey('invoke-submit')));
    await tester.pumpAndSettle();

    expect(mode, 'async');
    expect(find.byKey(const ValueKey('task-card')), findsOneWidget);
    expect(find.textContaining('进度 40%'), findsOneWidget);
    expect(find.byKey(const ValueKey('task-cancel')), findsOneWidget);
    // 清理挂起的轮询 timer，避免 pending timer 报错。
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pumpAndSettle();
  });

  testWidgets('调用页：rawEditor（无 properties）走 JSON 编辑器', (
    WidgetTester tester,
  ) async {
    await login();
    final rawSpec = FunctionSpec.fromJson({
      'id': 'raw.fn',
      'inputSchema': {'type': 'object'},
    })!;
    adapter.handler = (options, _) => jsonResponse(200, {'result': 1});
    await tester.pumpWidget(wrap(FunctionInvokePage(spec: rawSpec)));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(const ValueKey('field-__raw__')),
      '{"a":1}',
    );
    await tester.tap(find.byKey(const ValueKey('invoke-submit')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('sync-result')), findsOneWidget);
  });
}
