import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/approvals/approval_detail_page.dart';
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
    store.save(
      const SessionData(
        token: 'jwt',
        user: {'username': 'admin'},
        gameId: 'demo',
        env: 'prod',
        serverUrl: 'http://gm.test',
      ),
    );
  });

  Future<void> pumpDetail(WidgetTester tester, String id) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sessionStoreProvider.overrideWithValue(store),
          apiClientFactoryProvider.overrideWithValue((baseUrl) {
            final client = ApiClient.create(
              baseUrl: baseUrl,
              sessionStore: store,
            );
            client.dio.httpClientAdapter = adapter;
            return client;
          }),
        ],
        child: MaterialApp(home: ApprovalDetailPage(approvalId: id)),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('详情渲染元数据 + payloadPreview 折叠 + 动作按钮', (
    WidgetTester tester,
  ) async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'id': 'a1',
      'functionId': 'player.kick',
      'actor': 'op1',
      'state': 'pending',
      'mode': 'invoke',
      'gameId': 'demo',
      'env': 'prod',
      'createdAt': '2026-09-30 10:00',
      'payloadPreview': '{"playerId": 42}',
    });
    await pumpDetail(tester, 'a1');

    expect(find.text('op1'), findsOneWidget);
    expect(find.text('pending'), findsOneWidget);
    expect(find.byKey(const ValueKey('payload-preview')), findsOneWidget);
    expect(find.byKey(const ValueKey('approve-btn')), findsOneWidget);
    expect(find.byKey(const ValueKey('reject-btn')), findsOneWidget);
  });

  testWidgets('两人规则：actor==当前用户 → 只读横幅 + 按钮禁用', (WidgetTester tester) async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'id': 'a1',
      'functionId': 'player.kick',
      'actor': 'admin',
      'state': 'pending',
    });
    await pumpDetail(tester, 'a1');

    expect(find.byKey(const ValueKey('two-person-banner')), findsOneWidget);
    expect(find.byKey(const ValueKey('approve-btn')), findsNothing);
    expect(find.byKey(const ValueKey('reject-btn')), findsNothing);
  });

  testWidgets('批准：POST 后刷新详情到 approved', (WidgetTester tester) async {
    var state = 'pending';
    var approveCalled = '';
    adapter.handler = (options, _) {
      final path = options.uri.path;
      if (path == '/api/v1/approvals/a1/approve') {
        approveCalled = path;
        state = 'approved';
        return jsonResponse(200, {'id': 'a1', 'state': 'approved'});
      }
      return jsonResponse(200, {
        'id': 'a1',
        'functionId': 'player.kick',
        'actor': 'op1',
        'state': state,
      });
    };
    await pumpDetail(tester, 'a1');

    await tester.tap(find.byKey(const ValueKey('approve-btn')));
    await tester.pumpAndSettle();

    // approve 后详情自动刷新，lastRequest 已被 GET 覆盖，用捕获变量断言
    expect(approveCalled, '/api/v1/approvals/a1/approve');
    expect(find.text('approved'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('approve-btn')),
      findsNothing,
      reason: '已批状态不再显示动作按钮',
    );
  });

  testWidgets('拒绝：reason 必填 → 提交后刷新到 rejected', (WidgetTester tester) async {
    var state = 'pending';
    adapter.handler = (options, _) {
      final path = options.uri.path;
      if (path == '/api/v1/approvals/a1/reject') {
        state = 'rejected';
        return jsonResponse(200, {'id': 'a1', 'state': 'rejected'});
      }
      return jsonResponse(200, {
        'id': 'a1',
        'functionId': 'player.kick',
        'actor': 'op1',
        'state': state,
      });
    };
    await pumpDetail(tester, 'a1');

    await tester.tap(find.byKey(const ValueKey('reject-btn')));
    await tester.pumpAndSettle();

    // reason 为空时确认按钮不关闭对话框
    await tester.tap(find.byKey(const ValueKey('reject-confirm')));
    await tester.pump();
    expect(find.byKey(const ValueKey('reject-reason')), findsOneWidget);

    await tester.enterText(find.byKey(const ValueKey('reject-reason')), '不当操作');
    await tester.tap(find.byKey(const ValueKey('reject-confirm')));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('reject-reason')), findsNothing);
    expect(find.text('rejected'), findsOneWidget);
  });

  testWidgets('409 竞态：冲突提示 + 自动刷新详情', (WidgetTester tester) async {
    var handled = false;
    adapter.handler = (options, _) {
      final path = options.uri.path;
      if (path == '/api/v1/approvals/a1/approve') {
        handled = true;
        return jsonResponse(409, {'error': 'conflict', 'message': '审批已被他人处理'});
      }
      return jsonResponse(200, {
        'id': 'a1',
        'functionId': 'player.kick',
        'actor': 'op1',
        'state': handled ? 'approved' : 'pending',
        'approver': handled ? 'op2' : '',
      });
    };
    await pumpDetail(tester, 'a1');

    await tester.tap(find.byKey(const ValueKey('approve-btn')));
    await tester.pumpAndSettle();

    expect(find.textContaining('已被处理'), findsOneWidget);
    expect(find.text('approved'), findsOneWidget, reason: '409 后自动刷新详情');
    expect(find.text('op2'), findsOneWidget);
  });
}
