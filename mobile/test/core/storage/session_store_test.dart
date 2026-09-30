import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('InMemorySessionStore', () {
    test('save/load/clear 往返', () async {
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
      expect(await store.load(), isNull);
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
