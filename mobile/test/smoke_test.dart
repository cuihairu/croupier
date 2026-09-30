import 'package:croupier_mobile/app/app.dart';
import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('未登录冷启动渲染登录页首屏', (WidgetTester tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sessionStoreProvider.overrideWithValue(InMemorySessionStore()),
        ],
        child: const CroupierApp(),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('服务器地址'), findsOneWidget);
    expect(find.text('用户名'), findsOneWidget);
    expect(find.text('密码'), findsOneWidget);
    expect(find.byKey(const ValueKey('login-submit')), findsOneWidget);
  });
}
