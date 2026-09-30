import 'package:croupier_mobile/app/app.dart';
import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:dio/dio.dart';
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

  Future<void> pumpApp(WidgetTester tester) async {
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
        child: const CroupierApp(),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('全新冷启动（无存储块）进设置向导，不进登录页', (WidgetTester tester) async {
    await pumpApp(tester);
    expect(find.byKey(const ValueKey('setup-server')), findsOneWidget);
    expect(find.byKey(const ValueKey('setup-submit')), findsOneWidget);
    expect(find.byKey(const ValueKey('login-submit')), findsNothing);
  });

  testWidgets('空地址不放行：明确错误且不发探测请求', (WidgetTester tester) async {
    var calls = 0;
    adapter.handler = (options, _) {
      calls++;
      return jsonResponse(200, {});
    };
    await pumpApp(tester);

    await tester.tap(find.byKey(const ValueKey('setup-submit')));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('setup-error')), findsOneWidget);
    expect(find.text('服务器地址不能为空'), findsOneWidget);
    expect(calls, 0);
  });

  testWidgets('格式非法不放行：格式错误文案且不发探测请求', (WidgetTester tester) async {
    var calls = 0;
    adapter.handler = (options, _) {
      calls++;
      return jsonResponse(200, {});
    };
    await pumpApp(tester);

    await tester.enterText(
      find.byKey(const ValueKey('setup-server')),
      'not a url',
    );
    await tester.tap(find.byKey(const ValueKey('setup-submit')));
    await tester.pumpAndSettle();

    expect(find.textContaining('格式不正确'), findsOneWidget);
    expect(calls, 0);
  });

  testWidgets('探测失败（5xx）不放行：错误透传且停留向导', (WidgetTester tester) async {
    adapter.handler = (options, _) =>
        jsonResponse(500, {'error': 'internal', 'message': '服务不可用'});
    await pumpApp(tester);

    await tester.enterText(
      find.byKey(const ValueKey('setup-server')),
      'http://gm.test',
    );
    await tester.tap(find.byKey(const ValueKey('setup-submit')));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('setup-error')), findsOneWidget);
    expect(find.text('服务不可用'), findsOneWidget);
    expect(find.byKey(const ValueKey('setup-submit')), findsOneWidget);
    expect(await store.load(), isNull);
  });

  testWidgets('200 HTML 兜底不放行：连接的不是 Croupier 服务', (WidgetTester tester) async {
    adapter.handler = (options, _) => ResponseBody.fromString(
      '<!DOCTYPE html><html></html>',
      200,
      headers: {
        Headers.contentTypeHeader: <String>['text/html'],
      },
    );
    await pumpApp(tester);

    await tester.enterText(
      find.byKey(const ValueKey('setup-server')),
      'http://spa.test',
    );
    await tester.tap(find.byKey(const ValueKey('setup-submit')));
    await tester.pumpAndSettle();

    expect(find.textContaining('不是 Croupier 服务'), findsOneWidget);
    expect(await store.load(), isNull);
  });

  testWidgets('探测通过保存地址：落盘 + 切登录页预填 + 探测打向该地址', (WidgetTester tester) async {
    adapter.handler = (options, _) {
      if (options.uri.path == '/api/v1/public/site') {
        return jsonResponse(200, {'title': 'Croupier'});
      }
      return jsonResponse(404, {
        'error': 'not_found',
        'message': options.uri.path,
      });
    };
    await pumpApp(tester);

    await tester.enterText(
      find.byKey(const ValueKey('setup-server')),
      'http://gm.test/',
    );
    await tester.tap(find.byKey(const ValueKey('setup-submit')));
    await tester.pumpAndSettle();

    // 地址落盘（归一化去尾斜杠）+ 切登录页并预填。
    expect((await store.load())?.serverUrl, 'http://gm.test');
    expect(find.byKey(const ValueKey('login-server')), findsOneWidget);
    expect(find.text('http://gm.test'), findsOneWidget);
    // 探测请求确实打向用户输入的地址（host 生效）。
    expect(adapter.lastRequest?.uri.host, 'gm.test');
  });
}
