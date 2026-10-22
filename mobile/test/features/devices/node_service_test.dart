import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/api/api_error.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/devices/node_service.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;
  late NodeService service;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
    final client = ApiClient.create(
      baseUrl: 'http://gm.test',
      sessionStore: store,
    );
    client.dio.httpClientAdapter = adapter;
    service = NodeService(client: client);
  });

  final nodePayload = {
    'id': 'agent-1',
    'hostname': 'gm-node-a',
    'addr': '10.0.0.2:19091',
    'gameId': 'demo',
    'env': 'prod',
    'status': 'active',
    'labels': {'region': 'cn', 'role': 'gm'},
    'lastSeen': '2026-09-30 10:00:00',
    'sdkLanguage': 'go',
    'sdkVersion': '1.2.0',
    'sdkName': 'croupier-sdk-go',
    'version': 'v0.9.3',
    'functions': 12,
    'expiresInSec': 3600,
    'cpu': {'usagePercent': 12.5, 'cores': 8},
    'memory': {'totalBytes': 1024, 'usedBytes': 512, 'usagePercent': 50},
    'disks': [
      {
        'mountPoint': '/',
        'totalBytes': 2048,
        'usedBytes': 1024,
        'usagePercent': 50,
      },
    ],
  };

  test('list 解析 {nodes:[...]}，缺 id 条目跳过', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'nodes': [
        nodePayload,
        {'hostname': '缺 id'},
      ],
    });
    final nodes = await service.list();
    expect(nodes, hasLength(1));
    final node = nodes.single;
    expect(node.hostname, 'gm-node-a');
    expect(node.labels['region'], 'cn');
    expect(node.band, DeviceBand.online);
    expect(node.displayVersion, 'v0.9.3');
    expect(node.functions, 12);
    expect(node.cpu?.usagePercent, 12.5);
    expect(node.disks.single.mountPoint, '/');
  });

  test('band 四档归并 + 未知值兜底', () {
    Map<String, Object?> withStatus(String status) => {
      'id': 'x',
      'status': status,
    };
    expect(NodeDevice.fromJson(withStatus('online'))!.band, DeviceBand.online);
    expect(
      NodeDevice.fromJson(withStatus('drained'))!.band,
      DeviceBand.drained,
    );
    expect(NodeDevice.fromJson(withStatus('stale'))!.band, DeviceBand.degraded);
    expect(
      NodeDevice.fromJson(withStatus('offline'))!.band,
      DeviceBand.offline,
    );
    expect(NodeDevice.fromJson(withStatus('weird'))!.band, DeviceBand.unknown);
  });

  test('displayVersion 回退 sdkVersion', () {
    final node = NodeDevice.fromJson({'id': 'x', 'sdkVersion': '1.0.0'})!;
    expect(node.displayVersion, '1.0.0');
    expect(NodeDevice.fromJson({'id': 'x'})!.displayVersion, isEmpty);
  });

  test('list 请求 scoped 头', () async {
    await store.save(
      const SessionData(token: 'jwt', gameId: 'demo', env: 'prod'),
    );
    adapter.handler = (options, _) => jsonResponse(200, {'nodes': []});
    await service.list();
    expect(adapter.lastRequest?.uri.path, '/api/v1/ops/nodes');
    expect(adapter.lastRequest?.headers['X-Game-ID'], 'demo');
  });

  test('list 非 Map 体 fail-fast', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'foo': []});
    await expectLater(service.list(), throwsStateError);
  });

  test('detail 解析 {node:{...}}，404 归一 ApiError', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'node': nodePayload});
    final node = await service.detail('agent-1');
    expect(adapter.lastRequest?.uri.path, '/api/v1/ops/nodes/agent-1');
    expect(node.id, 'agent-1');

    adapter.handler = (options, _) =>
        jsonResponse(404, {'error': 'not_found', 'message': '节点不存在'});
    try {
      await service.detail('missing');
      fail('should throw');
    } on ApiError catch (e) {
      expect(e.status, 404);
      expect(e.message, '节点不存在');
    }
  });

  test('detail node 键缺失 fail-fast', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'foo': {}});
    await expectLater(service.detail('x'), throwsStateError);
  });
}
