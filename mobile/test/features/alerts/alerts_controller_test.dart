import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/alerts/alerts_controller.dart';
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

  test('refresh + switchLevel 重查第一页', () async {
    await seedSession();
    final captured = <Uri>[];
    adapter.handler = (options, _) {
      captured.add(options.uri);
      return jsonResponse(200, {
        'items': [
          {'id': 'al-1', 'level': 'warning'},
        ],
        'total': 1,
      });
    };
    final notifier = container.read(alertsControllerProvider.notifier);
    await notifier.refresh();
    expect(container.read(alertsControllerProvider).items, hasLength(1));

    await notifier.switchLevel('warning');
    expect(captured.last.queryParameters['level'], 'warning');
    expect(captured.last.queryParameters['page'], '1');
  });

  test('loadMore 翻页累积', () async {
    await seedSession();
    var call = 0;
    adapter.handler = (options, _) {
      call++;
      final page = options.uri.queryParameters['page'];
      return jsonResponse(200, {
        'items': [
          {'id': 'al-$page-$call'},
        ],
        'total': 2,
      });
    };
    final notifier = container.read(alertsControllerProvider.notifier);
    await notifier.refresh();
    await notifier.loadMore();
    final state = container.read(alertsControllerProvider);
    expect(state.items, hasLength(2));
    expect(state.hasMore, isFalse);
  });

  test('silence 提交后刷新列表', () async {
    await seedSession();
    final posts = <Uri>[];
    adapter.handler = (options, _) {
      if (options.method == 'POST') {
        posts.add(options.uri);
        return jsonResponse(200, {'message': '操作成功'});
      }
      return jsonResponse(200, {
        'items': [
          {'id': 'al-1', 'status': 'firing'},
        ],
        'total': 1,
      });
    };
    await container
        .read(alertsControllerProvider.notifier)
        .silence('al-1', durationMinutes: 30, reason: '维护');
    expect(posts.single.path, '/api/v1/alerts/al-1/silence');
    // 静默后触发列表刷新。
    expect(container.read(alertsControllerProvider).items, hasLength(1));
  });

  test('loadSilences 载入规则 / 错误面', () async {
    await seedSession();
    adapter.handler = (options, _) {
      if (options.uri.path == '/api/v1/alerts/silences') {
        return jsonResponse(200, {
          'items': [
            {'id': 'sl-1', 'alertType': 'agent_offline'},
          ],
        });
      }
      return jsonResponse(200, {'items': [], 'total': 0});
    };
    await container.read(alertsControllerProvider.notifier).loadSilences();
    expect(container.read(alertsControllerProvider).silences.single.id, 'sl-1');

    adapter.handler = (options, _) {
      if (options.uri.path == '/api/v1/alerts/silences') {
        return jsonResponse(500, {'message': 'boom'});
      }
      return jsonResponse(200, {'items': [], 'total': 0});
    };
    await container.read(alertsControllerProvider.notifier).loadSilences();
    expect(container.read(alertsControllerProvider).silencesError, 'boom');
  });
}
