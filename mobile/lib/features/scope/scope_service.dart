/// Scope 服务（设计稿 §3.3）：游戏/环境列表 + 持久化所选 scope。
library;

import '../../core/api/api_client.dart';
import '../../core/storage/session_store.dart';

class ScopeGame {
  const ScopeGame({
    required this.gameId,
    required this.gameName,
    required this.envs,
  });

  final String gameId;
  final String gameName;
  final List<String> envs;
}

class ScopeService {
  ScopeService({required this.client, required this.sessionStore});

  static const gamesPath = '/api/v1/profile/games';
  static const scopePath = '/api/v1/profile/scope';

  final ApiClient client;
  final SessionStore sessionStore;

  /// 游戏列表（非 scoped 端点）。响应形态不合法时 fail-fast，
  /// 不静默当空列表（对齐 Web 端 mock 兜底教训）。
  Future<List<ScopeGame>> fetchGames() async {
    final data = await client.get<Map<String, Object?>>(gamesPath);
    final raw = data['games'];
    if (raw is! List) {
      throw StateError('invalid games payload');
    }
    final games = <ScopeGame>[];
    for (final item in raw) {
      if (item is! Map) continue;
      final gameId = item['gameId'];
      if (gameId is! String || gameId.isEmpty) continue;
      final envsRaw = item['envs'];
      final envs = envsRaw is List
          ? envsRaw.whereType<String>().toList()
          : const <String>[];
      games.add(
        ScopeGame(
          gameId: gameId,
          gameName: item['gameName'] is String
              ? item['gameName'] as String
              : gameId,
          envs: envs,
        ),
      );
    }
    return games;
  }

  /// 持久化所选 scope（服务端兜底口径）。响应仅 {ok: true}，失败抛 ApiError。
  Future<void> saveScope(String gameId, String env) async {
    await client.put<Map<String, Object?>>(
      scopePath,
      body: {'gameId': gameId, 'env': env},
    );
  }
}
