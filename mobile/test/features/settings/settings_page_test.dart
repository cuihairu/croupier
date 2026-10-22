import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/settings/settings_page.dart';
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
          // 更换地址流程会做连通探测（GET /api/v1/public/site），
          // 必须落到 fake adapter（否则真 HttpClient 被 test binding 拦 400）。
          apiClientFactoryProvider.overrideWithValue((baseUrl) {
            final client = ApiClient.create(
              baseUrl: baseUrl,
              sessionStore: store,
            );
            client.dio.httpClientAdapter = adapter;
            return client;
          }),
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

  testWidgets('更换服务器地址：探测通过 → 新地址落盘 + 清会话 + 预填', (WidgetTester tester) async {
    final probedUrls = <Uri>[];
    adapter.handler = (options, _) {
      if (options.uri.path == '/api/v1/public/site') {
        probedUrls.add(options.uri);
        return jsonResponse(200, {'title': 'Croupier'});
      }
      return jsonResponse(200, {});
    };
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
      'http://new.test/',
    );
    await tester.tap(find.byKey(const ValueKey('settings-server-save')));
    await tester.pumpAndSettle();

    // 探测打向新地址；归一化去尾斜杠落盘。
    expect(probedUrls.single.host, 'new.test');
    final after = await store.load();
    expect(after?.token ?? '', isEmpty);
    expect(after?.serverUrl, 'http://new.test');
    expect(container?.read(pendingServerUrlProvider), 'http://new.test');
  });

  testWidgets('更换地址探测失败不放行：错误内联展示，会话与地址不动', (WidgetTester tester) async {
    adapter.handler = (options, _) {
      if (options.uri.path == '/api/v1/public/site') {
        return jsonResponse(503, {'error': 'unavailable', 'message': '服务不可用'});
      }
      return jsonResponse(200, {});
    };
    await store.save(
      const SessionData(token: 'jwt', serverUrl: 'http://old.test'),
    );
    await pumpSettings(tester);

    await tester.tap(find.byKey(const ValueKey('settings-change-server')));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const ValueKey('settings-server-input')),
      'http://new.test',
    );
    await tester.tap(find.byKey(const ValueKey('settings-server-save')));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('settings-server-error')), findsOneWidget);
    expect(find.text('服务不可用'), findsOneWidget);
    // 对话框未关闭、会话与地址原样。
    expect(find.byKey(const ValueKey('settings-server-save')), findsOneWidget);
    final after = await store.load();
    expect(after?.token, 'jwt');
    expect(after?.serverUrl, 'http://old.test');
  });

  testWidgets('更换地址格式非法不放行：明确错误且不发探测请求', (WidgetTester tester) async {
    var calls = 0;
    adapter.handler = (options, _) {
      calls++;
      return jsonResponse(200, {});
    };
    await store.save(
      const SessionData(token: 'jwt', serverUrl: 'http://old.test'),
    );
    await pumpSettings(tester);

    await tester.tap(find.byKey(const ValueKey('settings-change-server')));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const ValueKey('settings-server-input')),
      'ftp://new.test',
    );
    await tester.tap(find.byKey(const ValueKey('settings-server-save')));
    await tester.pumpAndSettle();

    expect(find.text('仅支持 http / https 地址'), findsOneWidget);
    expect(calls, 0);
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
