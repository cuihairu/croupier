/// 会话存储（设计稿 §4.2）：secure storage 存
/// `{token, user, scope(gameId/env), serverUrl}`；登出 / 401 清会话
/// （**保留已配置的 serverUrl**——地址属配置而非凭据，登录页据此预填）。
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

  /// 有存储块即返回对象（token 可为空——「已配置地址未登录」形态）；
  /// 从未配置（无存储块）返回 null。
  Future<SessionData?> load();

  /// 清会话（token/user/scope）；已配置的 serverUrl 保留写回。
  Future<void> clear();

  /// 无条件读服务器地址：未登录 / 无 token 也可读（首启动引导数据源）。
  Future<String> loadServerUrl() async =>
      (await load())?.serverUrl.trim() ?? '';

  /// 持久化服务器地址（只写地址，不动现有会话）。
  Future<void> saveServerUrl(String url) async {
    final current = await load();
    await save(
      (current ?? const SessionData(token: '')).copyWith(serverUrl: url),
    );
  }
}

/// 测试与预览用内存实现。
class InMemorySessionStore extends SessionStore {
  SessionData? _data;

  @override
  Future<void> save(SessionData data) async {
    _data = data;
  }

  @override
  Future<SessionData?> load() async => _data;

  @override
  Future<void> clear() async {
    final url = (_data?.serverUrl ?? '').trim();
    _data = url.isEmpty ? null : SessionData(token: '', serverUrl: url);
  }
}

/// 生产实现：整块 JSON 存单一 secure storage key
/// （iOS Keychain / Android EncryptedSharedPreferences）。
class SecureSessionStore extends SessionStore {
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
        // 有块即返回对象：token 为空的块表示「已配置地址未登录」
        // （首启动向导 / 登出后保留地址），未登录判定由调用方看 token。
        return SessionData.fromJson(decoded);
      }
    } on FormatException {
      // 损坏数据按无会话处理，等价登出
    }
    return null;
  }

  @override
  Future<void> clear() async {
    // 会话整清，已配置地址保留（配置与凭据分层）。
    final raw = await _storage.read(key: _key);
    var url = '';
    if (raw != null && raw.isNotEmpty) {
      try {
        final decoded = jsonDecode(raw);
        if (decoded is Map<String, Object?>) {
          url = ((decoded['serverUrl'] as String?) ?? '').trim();
        }
      } on FormatException {
        // 损坏块直接删除
      }
    }
    if (url.isEmpty) {
      await _storage.delete(key: _key);
    } else {
      await _storage.write(
        key: _key,
        value: jsonEncode(SessionData(token: '', serverUrl: url).toJson()),
      );
    }
  }
}
