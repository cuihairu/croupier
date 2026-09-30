/// dio 客户端装配（设计稿 §3.3 / §3.4 / §5.1）：
/// 拦截器链 = token 注入 → scope 头（注入 + 剥除外部同名头防注入）→
/// 错误归一 ApiError；401（登录端点除外）清会话并回调回登录页。
library;

import 'package:dio/dio.dart';

import 'api_error.dart';
import '../auth/login_service.dart';
import '../scope/scoped_prefixes.dart';
import '../storage/session_store.dart';

class ApiClient {
  ApiClient._(this.dio);

  final Dio dio;

  /// 登录端点路径：其 401（mfa_required）必须透传给登录流，
  /// 不得触发清会话回登录（否则双步登录第二跳被拦）。
  static const _loginPath = LoginService.loginPath;

  /// [onUnauthorized] 401 时回调（清会话后由 UI 跳登录页）。
  factory ApiClient.create({
    required String baseUrl,
    required SessionStore sessionStore,
    void Function()? onUnauthorized,
  }) {
    final dio = Dio(
      BaseOptions(
        baseUrl: baseUrl,
        connectTimeout: const Duration(seconds: 10),
        receiveTimeout: const Duration(seconds: 30),
        // 4xx/5xx 一律抛 DioException，由统一错误面归一；
        // 登录双步需要读 401 body 里的 mfa_required，同样走异常通道。
        validateStatus: (code) => code != null && code < 400,
      ),
    );

    dio.interceptors.addAll([
      InterceptorsWrapper(
        onRequest: (options, handler) async {
          // 1) token 注入
          final session = await sessionStore.load();
          if (session != null && session.token.isNotEmpty) {
            options.headers['Authorization'] = 'Bearer ${session.token}';
          }

          // 2) scope 头：命中 scoped 前缀且本地 scope 成对才注入；
          //    先剥除外部传入的同名头（对齐 web 拦截器防注入语义）
          final path = options.uri.path;
          options.headers.remove('X-Game-ID');
          options.headers.remove('X-Env');
          if (isScopedApiPath(path) &&
              session != null &&
              session.hasCompleteScope) {
            options.headers['X-Game-ID'] = session.gameId.trim();
            options.headers['X-Env'] = session.env.trim();
          }
          handler.next(options);
        },
        onError: (e, handler) async {
          // 3) 401 → 清会话回登录（登录端点自身的 401 分支除外）
          final status = e.response?.statusCode;
          final isLogin = e.requestOptions.uri.path == _loginPath;
          if (status == 401 && !isLogin) {
            await sessionStore.clear();
            onUnauthorized?.call();
          }
          handler.next(e);
        },
      ),
    ]);
    return ApiClient._(dio);
  }

  /// GET 并归一响应/错误。泛型由调用方按端点契约指定。
  Future<T> get<T>(
    String path, {
    Map<String, Object?>? query,
    Options? options,
  }) async {
    try {
      final res = await dio.get<T>(
        path,
        queryParameters: query,
        options: options,
      );
      return res.data as T;
    } on DioException catch (e) {
      throw ApiError.fromDio(e);
    }
  }

  /// POST 并归一响应/错误。
  Future<T> post<T>(String path, {Object? body, Options? options}) async {
    try {
      final res = await dio.post<T>(path, data: body, options: options);
      return res.data as T;
    } on DioException catch (e) {
      throw ApiError.fromDio(e);
    }
  }
}
