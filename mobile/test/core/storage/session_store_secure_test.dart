import 'dart:convert';

import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_secure_storage_platform_interface/flutter_secure_storage_platform_interface.dart';
import 'package:flutter_test/flutter_test.dart';

/// 内存版 secure storage 平台：绕开 MethodChannel，在 VM 测试环境覆盖
/// SecureSessionStore 全链（生产实现走 iOS Keychain / Android
/// EncryptedSharedPreferences，无平台 mock 时这些行为不可达）。
class _FakeSecurePlatform extends FlutterSecureStoragePlatform {
  final Map<String, String> backing = {};

  @override
  Future<void> write({
    required String key,
    required String value,
    required Map<String, String> options,
  }) async {
    backing[key] = value;
  }

  @override
  Future<String?> read({
    required String key,
    required Map<String, String> options,
  }) async => backing[key];

  @override
  Future<bool> containsKey({
    required String key,
    required Map<String, String> options,
  }) async => backing.containsKey(key);

  @override
  Future<void> delete({
    required String key,
    required Map<String, String> options,
  }) async {
    backing.remove(key);
  }

  @override
  Future<Map<String, String>> readAll({
    required Map<String, String> options,
  }) async => Map.of(backing);

  @override
  Future<void> deleteAll({required Map<String, String> options}) async {
    backing.clear();
  }
}

void main() {
  final platform = _FakeSecurePlatform();
  const key = 'croupier.session';
  final original = FlutterSecureStoragePlatform.instance;

  setUp(() {
    FlutterSecureStoragePlatform.instance = platform;
    platform.backing.clear();
  });

  tearDown(() {
    FlutterSecureStoragePlatform.instance = original;
  });

  SecureSessionStore newStore() => SecureSessionStore();

  test('save → load roundtrip（整块 JSON 单 key）', () async {
    final store = newStore();
    const session = SessionData(
      token: 'jwt-x',
      gameId: 'demo',
      env: 'prod',
      serverUrl: 'http://gm.test',
    );
    await store.save(session);

    expect(platform.backing.containsKey(key), isTrue);
    final restored = await store.load();
    expect(restored, isNotNull);
    // SessionData 无 == 重载，逐字段比对。
    expect(restored!.token, 'jwt-x');
    expect(restored.gameId, 'demo');
    expect(restored.env, 'prod');
    expect(restored.serverUrl, 'http://gm.test');
  });

  test('load：无块返回 null', () async {
    expect(await newStore().load(), isNull);
  });

  test('load：损坏 JSON 按无会话处理（FormatException 吞掉）', () async {
    platform.backing[key] = 'not-json{';
    expect(await newStore().load(), isNull);
  });

  test('load：非 map JSON（裸数组）返回 null', () async {
    platform.backing[key] = jsonEncode([1, 2, 3]);
    expect(await newStore().load(), isNull);
  });

  test('clear：已配置地址保留、token 清空（配置与凭据分层）', () async {
    final store = newStore();
    await store.save(
      const SessionData(
        token: 'jwt-x',
        gameId: 'demo',
        env: 'prod',
        serverUrl: 'http://gm.test',
      ),
    );

    await store.clear();

    final restored = await store.load();
    expect(restored, isNotNull);
    expect(restored!.token, '');
    expect(restored.serverUrl, 'http://gm.test');
    expect(restored.gameId, '');
  });

  test('clear：无块/空地址直接删除', () async {
    final store = newStore();
    await store.clear();
    expect(platform.backing.containsKey(key), isFalse);

    // 空地址块同样走删除分支。
    platform.backing[key] = jsonEncode({'token': 't', 'serverUrl': '  '});
    await store.clear();
    expect(platform.backing.containsKey(key), isFalse);
  });

  test('clear：损坏块直接删除（FormatException）', () async {
    platform.backing[key] = 'corrupted';
    await newStore().clear();
    expect(platform.backing.containsKey(key), isFalse);
  });
}
