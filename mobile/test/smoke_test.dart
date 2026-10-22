import 'package:croupier_mobile/app/app.dart';
import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('全新冷启动（未配置地址）渲染设置向导首屏', (WidgetTester tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sessionStoreProvider.overrideWithValue(InMemorySessionStore()),
        ],
        child: const CroupierApp(),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('setup-server')), findsOneWidget);
    expect(find.byKey(const ValueKey('setup-submit')), findsOneWidget);
    expect(find.byKey(const ValueKey('login-submit')), findsNothing);
  });

  testWidgets('地址已配置未登录渲染登录页并预填地址', (WidgetTester tester) async {
    final store = InMemorySessionStore();
    await store.saveServerUrl('https://gm.example.com');
    await tester.pumpWidget(
      ProviderScope(
        overrides: [sessionStoreProvider.overrideWithValue(store)],
        child: const CroupierApp(),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('login-server')), findsOneWidget);
    expect(find.byKey(const ValueKey('login-submit')), findsOneWidget);
    // 预填地址直读 controller（find.text 会与同值 hintText 双匹配）。
    final serverField = tester.widget<TextField>(
      find.byKey(const ValueKey('login-server')),
    );
    expect(serverField.controller?.text, 'https://gm.example.com');
  });
}
