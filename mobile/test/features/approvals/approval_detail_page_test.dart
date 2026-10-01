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

  testWidgets('step-up：otp_required → 动态码弹窗 → 带 otp 重批成功（#75）', (
    WidgetTester tester,
  ) async {
    var state = 'pending';
    final otpBodies = <String?>[];
    var approveCalls = 0;
    adapter.handler = (options, raw) {
      final path = options.uri.path;
      if (path == '/api/v1/approvals/a1/approve') {
        approveCalls += 1;
        // raw 是 dio 出站 JSON 字节流，解码后断言 otp 字段
        otpBodies.add(_extractOtp(rawBodyString(raw)));
        if (approveCalls == 1) {
          return jsonResponse(403, {
            'error': 'otp_required',
            'message': '高危审批需要动态验证码',
          });
        }
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

    // 首次无码 → 403 → 弹动态码输入
    expect(otpBodies.first, isNull);
    expect(find.byKey(const ValueKey('otp-input')), findsOneWidget);

    await tester.enterText(find.byKey(const ValueKey('otp-input')), '123456');
    await tester.tap(find.byKey(const ValueKey('otp-confirm')));
    await tester.pumpAndSettle();

    expect(approveCalls, 2, reason: '带码重试恰好一次');
    expect(otpBodies.last, '123456');
    expect(find.text('approved'), findsOneWidget);
  });

  testWidgets('step-up：取消弹窗不重发，审批保持 pending', (WidgetTester tester) async {
    var approveCalls = 0;
    adapter.handler = (options, _) {
      final path = options.uri.path;
      if (path == '/api/v1/approvals/a1/approve') {
        approveCalls += 1;
        return jsonResponse(403, {
          'error': 'otp_required',
          'message': '需要动态验证码',
        });
      }
      return jsonResponse(200, {
        'id': 'a1',
        'functionId': 'player.kick',
        'actor': 'op1',
        'state': 'pending',
      });
    };
    await pumpDetail(tester, 'a1');

    await tester.tap(find.byKey(const ValueKey('approve-btn')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('otp-input')), findsOneWidget);

    await tester.tap(find.text('取消'));
    await tester.pumpAndSettle();

    expect(approveCalls, 1, reason: '取消后不重发');
    expect(find.text('pending'), findsOneWidget);
    expect(find.byKey(const ValueKey('approve-btn')), findsOneWidget);
  });

  testWidgets('step-up：otp_invalid 提示后重弹，正确码批准成功', (WidgetTester tester) async {
    var state = 'pending';
    final otpBodies = <String?>[];
    var approveCalls = 0;
    adapter.handler = (options, raw) {
      final path = options.uri.path;
      if (path == '/api/v1/approvals/a1/approve') {
        approveCalls += 1;
        otpBodies.add(rawBodyString(raw));
        if (approveCalls == 1) {
          return jsonResponse(403, {
            'error': 'otp_required',
            'message': '需要动态验证码',
          });
        }
        if (approveCalls == 2) {
          return jsonResponse(400, {
            'error': 'otp_invalid',
            'message': '动态验证码错误或已过期',
          });
        }
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
    await tester.enterText(find.byKey(const ValueKey('otp-input')), '000000');
    await tester.tap(find.byKey(const ValueKey('otp-confirm')));
    await tester.pumpAndSettle();

    // 错码 snackbar + 弹窗重现
    expect(find.textContaining('错误或已过期'), findsOneWidget);
    expect(find.byKey(const ValueKey('otp-input')), findsOneWidget);

    await tester.enterText(find.byKey(const ValueKey('otp-input')), '123456');
    await tester.tap(find.byKey(const ValueKey('otp-confirm')));
    await tester.pumpAndSettle();

    expect(approveCalls, 3);
    expect(find.text('approved'), findsOneWidget);
  });
}

/// 从 dio 出站 body 原始字节解 JSON 取 otp 字段（无码请求为空 → null）。
String? _extractOtp(String? body) {
  if (body == null || body.isEmpty) return null;
  final match = RegExp('"otp"\\s*:\\s*"([^"]+)"').firstMatch(body);
  return match?.group(1);
}

String? rawBodyString(List<int>? raw) {
  if (raw == null || raw.isEmpty) return null;
  return String.fromCharCodes(raw);
}
