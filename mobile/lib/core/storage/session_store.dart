/// 会话存储（设计稿 §4.2）：secure storage 存
/// `{token, user, scope(gameId/env), serverUrl}`；登出 / 401 整块清除。
/// 不落密码、TOTP 种子、生物特征数据。
library;

import 'dart:convert';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class SessionData {
  const SessionData({
    required this.token,
    this.user = const {},
    this.gameId = '',
    this.env = '',
    this.serverUrl = '',
  });

  final String token;
  final Map<String, Object?> user;
  final String gameId;
  final String env;
  final String serverUrl;

  /// scope 必须成对才有效——部分 scope 不允许出现（服务端原子拒绝）。
  bool get hasCompleteScope =>
      gameId.trim().isNotEmpty && env.trim().isNotEmpty;

  SessionData copyWith({
    String? token,
    Map<String, Object?>? user,
    String? gameId,
    String? env,
    String? serverUrl,
  }) {
    return SessionData(
      token: token ?? this.token,
      user: user ?? this.user,
      gameId: gameId ?? this.gameId,
      env: env ?? this.env,
      serverUrl: serverUrl ?? this.serverUrl,
    );
  }

  Map<String, Object?> toJson() => {
    'token': token,
    'user': user,
    'gameId': gameId,
    'env': env,
    'serverUrl': serverUrl,
  };

  static SessionData fromJson(Map<String, Object?> json) {
    final user = json['user'];
    return SessionData(
      token: (json['token'] as String?) ?? '',
      user: user is Map<String, Object?> ? user : const {},
      gameId: (json['gameId'] as String?) ?? '',
      env: (json['env'] as String?) ?? '',
      serverUrl: (json['serverUrl'] as String?) ?? '',
    );
  }
}

abstract class SessionStore {
  Future<void> save(SessionData data);
  Future<SessionData?> load();
  Future<void> clear();
}

/// 测试与预览用内存实现。
class InMemorySessionStore implements SessionStore {
  SessionData? _data;

  @override
  Future<void> save(SessionData data) async {
    _data = data;
  }

  @override
  Future<SessionData?> load() async => _data;

  @override
  Future<void> clear() async {
    _data = null;
  }
}

/// 生产实现：整块 JSON 存单一 secure storage key
/// （iOS Keychain / Android EncryptedSharedPreferences）。
class SecureSessionStore implements SessionStore {
  SecureSessionStore({FlutterSecureStorage? storage})
    : _storage =
          storage ??
          const FlutterSecureStorage(
            aOptions: AndroidOptions(encryptedSharedPreferences: true),
          );

  static const _key = 'croupier.session';

  final FlutterSecureStorage _storage;

  @override
  Future<void> save(SessionData data) async {
    await _storage.write(key: _key, value: jsonEncode(data.toJson()));
  }

  @override
  Future<SessionData?> load() async {
    final raw = await _storage.read(key: _key);
    if (raw == null || raw.isEmpty) return null;
    try {
      final decoded = jsonDecode(raw);
      if (decoded is Map<String, Object?>) {
        final data = SessionData.fromJson(decoded);
        if (data.token.isEmpty) return null;
        return data;
      }
    } on FormatException {
      // 损坏数据按无会话处理，等价登出
    }
    return null;
  }

  @override
  Future<void> clear() async {
    await _storage.delete(key: _key);
  }
}
