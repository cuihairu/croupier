import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/settings/settings_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  late InMemorySessionStore store;

  setUp(() {
    store = InMemorySessionStore();
  });

  Future<void> pumpSettings(
    WidgetTester tester, {
    void Function(ProviderContainer)? onContainer,
  }) async {
    late final ProviderContainer captured;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sessionStoreProvider.overrideWithValue(store),
          pendingServerUrlProvider.overrideWith((ref) => null),
        ],
        child: Consumer(
          builder: (context, ref, _) {
            captured = ProviderScope.containerOf(context);
            // Scaffold 提供 Material 祖先（MainShell 真实宿主形态）
            return const MaterialApp(home: Scaffold(body: SettingsPage()));
          },
        ),
      ),
    );
    onContainer?.call(captured);
    await tester.pumpAndSettle();
  }

  testWidgets('会话信息展示：用户/scope/服务器地址', (WidgetTester tester) async {
    await store.save(
      const SessionData(
        token: 'jwt',
        user: {'username': 'admin'},
        gameId: 'demo',
        env: 'prod',
        serverUrl: 'http://gm.test',
      ),
    );
    await pumpSettings(tester);

    expect(find.text('admin'), findsOneWidget);
    expect(find.text('demo / prod'), findsOneWidget);
    expect(find.text('http://gm.test'), findsOneWidget);
  });

  testWidgets('更换服务器地址：清会话回登录 + 预填新地址', (WidgetTester tester) async {
    await store.save(
      const SessionData(
        token: 'jwt',
        user: {'username': 'admin'},
        gameId: 'demo',
        env: 'prod',
        serverUrl: 'http://old.test',
      ),
    );
    ProviderContainer? container;
    await pumpSettings(tester, onContainer: (c) => container = c);

    await tester.tap(find.byKey(const ValueKey('settings-change-server')));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(const ValueKey('settings-server-input')),
      'http://new.test',
    );
    await tester.tap(find.byKey(const ValueKey('settings-server-save')));
    await tester.pumpAndSettle();

    // 会话清除 + 预填值写入
    expect(await store.load(), isNull);
    expect(container?.read(pendingServerUrlProvider), 'http://new.test');
  });

  testWidgets('ntfy / 生物门禁为禁用占位（M2/M3 边界明示）', (WidgetTester tester) async {
    await store.save(
      const SessionData(token: 'jwt', serverUrl: 'http://gm.test'),
    );
    await pumpSettings(tester);

    expect(find.text('推送通知（ntfy）'), findsOneWidget);
    expect(find.textContaining('M3 交付'), findsOneWidget);
    expect(find.text('生物门禁'), findsOneWidget);
    expect(find.textContaining('M2 交付'), findsOneWidget);
  });
}
