import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('InMemorySessionStore', () {
    test('save/load 往返；clear 清会话但保留已配置地址', () async {
      final store = InMemorySessionStore();
      expect(await store.load(), isNull);

      const data = SessionData(
        token: 'jwt-1',
        user: {'username': 'admin'},
        gameId: 'demo',
        env: 'prod',
        serverUrl: 'https://gm.example.com',
      );
      await store.save(data);
      final loaded = await store.load();
      expect(loaded?.token, 'jwt-1');
      expect(loaded?.hasCompleteScope, isTrue);

      await store.clear();
      final after = await store.load();
      // 配置与凭据分层：token 清、地址留（登录页预填数据源）。
      expect(after?.token ?? '', isEmpty);
      expect(after?.serverUrl, 'https://gm.example.com');
    });

    test('无地址时 clear 等价无块（load null）', () async {
      final store = InMemorySessionStore();
      await store.save(const SessionData(token: 'jwt-1'));
      await store.clear();
      expect(await store.load(), isNull);
    });

    test('loadServerUrl/saveServerUrl：未配置空串、写入后可读、不动会话', () async {
      final store = InMemorySessionStore();
      expect(await store.loadServerUrl(), isEmpty);

      await store.save(
        const SessionData(token: 'jwt-1', serverUrl: 'http://old.test'),
      );
      await store.saveServerUrl('http://new.test');
      expect(await store.loadServerUrl(), 'http://new.test');
      // saveServerUrl 只写地址，不清会话。
      expect((await store.load())?.token, 'jwt-1');

      // 无会话块也可单独落地址（首启动向导形态：token 空块）。
      final fresh = InMemorySessionStore();
      await fresh.saveServerUrl('https://gm.example.com');
      final block = await fresh.load();
      expect(block, isNotNull);
      expect(block?.token ?? '', isEmpty);
      expect(block?.serverUrl, 'https://gm.example.com');
    });

    test('部分 scope 无效（gameId/env 须成对）', () {
      const partial = SessionData(token: 't', gameId: 'demo');
      expect(partial.hasCompleteScope, isFalse);
    });
  });

  group('SessionData JSON', () {
    test('toJson/fromJson 往返', () {
      const data = SessionData(
        token: 'jwt-2',
        user: {'username': 'op'},
        gameId: 'g',
        env: 'e',
        serverUrl: 'http://x',
      );
      final back = SessionData.fromJson(data.toJson());
      expect(back.token, 'jwt-2');
      expect(back.user, {'username': 'op'});
      expect(back.gameId, 'g');
      expect(back.env, 'e');
    });

    test('缺字段容错（user 非法形态回退空表）', () {
      final back = SessionData.fromJson({'token': 't', 'user': 'bad'});
      expect(back.token, 't');
      expect(back.user, isEmpty);
    });
  });
}
