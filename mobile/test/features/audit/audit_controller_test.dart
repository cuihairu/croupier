import 'dart:async';
import 'dart:convert';

import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/audit/audit_controller.dart';
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;
  late ProviderContainer container;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
    container = ProviderContainer(
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
    );
    addTearDown(container.dispose);
  });

  Future<void> seedSession() => store.save(
    const SessionData(
      token: 'jwt',
      gameId: 'demo',
      env: 'prod',
      serverUrl: 'http://gm.test',
    ),
  );

  test('applyFilters 提交筛选重查第一页', () async {
    await seedSession();
    final captured = <Uri>[];
    adapter.handler = (options, _) {
      captured.add(options.uri);
      return jsonResponse(200, {
        'items': [
          {'id': 'au-1', 'action': 'invoke'},
        ],
        'total': 1,
      });
    };
    final notifier = container.read(auditControllerProvider.notifier);
    await notifier.refresh();
    await notifier.applyFilters(
      const AuditFilters(actor: 'admin', kinds: {'invoke'}),
    );
    expect(captured.last.queryParameters['actor'], 'admin');
    expect(captured.last.queryParameters['kinds'], 'invoke');
    expect(captured.last.queryParameters['page'], '1');
  });

  test('相同筛选不重复请求', () async {
    await seedSession();
    var calls = 0;
    adapter.handler = (options, _) {
      calls++;
      return jsonResponse(200, {'items': [], 'total': 0});
    };
    final notifier = container.read(auditControllerProvider.notifier);
    await notifier.refresh();
    expect(calls, 1);
    await notifier.applyFilters(const AuditFilters());
    expect(calls, 1);
  });

  test('loadMore 翻页累积 + hasMore 收口', () async {
    await seedSession();
    adapter.handler = (options, _) {
      final page = options.uri.queryParameters['page'];
      return jsonResponse(200, {
        'items': [
          {'id': 'au-$page'},
        ],
        'total': 2,
      });
    };
    final notifier = container.read(auditControllerProvider.notifier);
    await notifier.refresh();
    await notifier.loadMore();
    final state = container.read(auditControllerProvider);
    expect(state.items, hasLength(2));
    expect(state.hasMore, isFalse);
  });

  test('未登录置错误面；HTTP 错误透传 message', () async {
    await container.read(auditControllerProvider.notifier).refresh();
    expect(container.read(auditControllerProvider).error, '未登录或缺少服务器地址');

    await seedSession();
    adapter.handler = (options, _) => jsonResponse(500, {'message': 'boom'});
    await container.read(auditControllerProvider.notifier).refresh();
    expect(container.read(auditControllerProvider).error, 'boom');
  });

  test('形态非法 StateError → error 透传不抛出', () async {
    await seedSession();
    adapter.handler = (options, _) => jsonResponse(200, {'data': <Object>[]});
    await container.read(auditControllerProvider.notifier).refresh();
    final state = container.read(auditControllerProvider);
    expect(state.error, 'invalid audit payload');
    expect(state.loading, isFalse);
    expect(state.loadingMore, isFalse);
  });

  test('loadMore guard：无更多时不发请求', () async {
    await seedSession();
    var calls = 0;
    adapter.handler = (options, _) {
      calls++;
      return jsonResponse(200, {
        'items': [
          {'id': 'a1', 'action': 'invoke'},
        ],
        'total': 1,
      });
    };
    final notifier = container.read(auditControllerProvider.notifier);
    await notifier.refresh();
    expect(calls, 1);
    await notifier.loadMore();
    expect(calls, 1, reason: 'hasMore=false 时 loadMore 直接短路');
  });

  test('loadMore guard：loading 进行中不重复发请求', () async {
    await seedSession();
    var calls = 0;
    final gate = Completer<ResponseBody>();
    adapter.handler = (options, _) {
      calls++;
      return gate.future;
    };
    final notifier = container.read(auditControllerProvider.notifier);
    final inFlight = notifier.refresh();
    // 轮询等 refresh 进入 loading 态（state 先于 dio 适配器到达而置位）。
    for (var i = 0;
        i < 200 && !container.read(auditControllerProvider).loading;
        i++) {
      await Future<void>.delayed(const Duration(milliseconds: 1));
    }
    expect(container.read(auditControllerProvider).loading, isTrue);
    await notifier.loadMore();
    // 等首个请求抵达假适配器（挂起在 gate）。
    for (var i = 0; i < 200 && calls == 0; i++) {
      await Future<void>.delayed(const Duration(milliseconds: 1));
    }
    expect(calls, 1, reason: 'loading=true 时 loadMore 短路，仅 refresh 一发');
    gate.complete(
      ResponseBody.fromString(
        jsonEncode({'items': <Object>[], 'total': 0}),
        200,
        headers: {
          Headers.contentTypeHeader: <String>[Headers.jsonContentType],
        },
      ),
    );
    await inFlight;
    // 收口复核：gate 释放后仍只有 refresh 一发（防竞态假绿——若 loadMore
    // 未短路，第二发也会抵达适配器使 calls=2）。
    expect(calls, 1, reason: 'gate 释放后仍仅 refresh 一发');
  });

  group('AuditFilters 契约', () {
    test('copyWith：覆写指定字段、保留其余', () {
      const base = AuditFilters(actor: 'a', kinds: {'invoke'}, env: 'prod');
      final patched = base.copyWith(actor: 'b');
      expect(patched.actor, 'b');
      expect(patched.env, 'prod');
      expect(patched.kinds, {'invoke'});

      final ranged = base.copyWith(
        start: DateTime(2026, 9, 1),
        end: DateTime(2026, 9, 30, 23, 59, 59),
      );
      expect(ranged.start, DateTime(2026, 9, 1));
      expect(ranged.end, DateTime(2026, 9, 30, 23, 59, 59));
      expect(ranged.actor, 'a');
    });

    test('== / hashCode：kinds 无序相等，任一字段差异即不等', () {
      const a = AuditFilters(actor: 'a', kinds: {'x', 'y'});
      const b = AuditFilters(actor: 'a', kinds: {'y', 'x'});
      expect(a, b);
      expect(a.hashCode, b.hashCode);

      expect(a, isNot(a.copyWith(actor: 'z')));
      expect(a, isNot(a.copyWith(env: 'dev')));
      expect(a, isNot(a.copyWith(ip: '1.1.1.1')));
      expect(a, isNot(a.copyWith(gameId: 'g2')));
      expect(a, isNot(a.copyWith(start: DateTime(2026, 9, 1))));
      expect(a, isNot(a.copyWith(end: DateTime(2026, 9, 2))));
      expect(a, isNot(a.copyWith(kinds: {'x'})));
      // 非 AuditFilters 对象恒不等。
      expect(a == 'x', isFalse);
      // hashCode 全字段参与（含 kinds 无序 hash）。
      expect(
        const AuditFilters(kinds: {'q'}).hashCode,
        const AuditFilters(kinds: {'q'}).hashCode,
      );
    });

    test('SetEquality.equals：空集/同集/异元素/长度不等', () {
      const eq = SetEquality();
      expect(eq.equals(<String>{}, <String>{}), isTrue);
      expect(eq.equals({'a'}, {'a'}), isTrue);
      expect(eq.equals({'a'}, {'b'}), isFalse, reason: '同长异元素');
      expect(eq.equals({'a'}, {'a', 'b'}), isFalse, reason: '长度不等');
    });
  });
}
