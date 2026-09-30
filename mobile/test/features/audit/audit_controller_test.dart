import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/audit/audit_controller.dart';
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
}
