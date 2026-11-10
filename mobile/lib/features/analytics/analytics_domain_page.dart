/// 分域详情页（MOB-ANA-001）：已建域走 facade query 拉域数据；
/// ❌ 待建域直接渲染「数据源待建」占位（不发请求，诚实边界）。
/// 加载/错误重试/空态齐全；数据渲染是通用形态（键值行 / 列表）。
library;

import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import 'analytics_service.dart';

class AnalyticsDomainPage extends ConsumerStatefulWidget {
  const AnalyticsDomainPage({
    super.key,
    required this.domain,
    required this.label,
    this.pending = false,
  });

  final String domain;
  final String label;
  final bool pending;

  @override
  ConsumerState<AnalyticsDomainPage> createState() =>
      _AnalyticsDomainPageState();
}

class _AnalyticsDomainPageState extends ConsumerState<AnalyticsDomainPage> {
  Object? _data;
  bool _loading = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    if (!widget.pending) {
      Future.microtask(_fetch);
    }
  }

  Future<AnalyticsService?> _serviceAsync() async {
    final session = await ref.read(sessionStoreProvider).load();
    final serverUrl = session?.serverUrl ?? '';
    if (serverUrl.isEmpty) return null;
    return AnalyticsService(
      client: ref.read(apiClientFactoryProvider)(serverUrl),
    );
  }

  Future<void> _fetch() async {
    final service = await _serviceAsync();
    if (!mounted) return;
    if (service == null) {
      _failure('未登录或缺少服务器地址');
      return;
    }
    final session = await ref.read(sessionStoreProvider).load();
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final data = await service.query(
        domain: widget.domain,
        gameId: session?.gameId.trim() ?? '',
        env: session?.env.trim() ?? '',
      );
      if (!mounted) return;
      setState(() {
        _data = data;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      _failure(e.toString());
    }
  }

  void _failure(String message) {
    setState(() {
      _loading = false;
      _error = message;
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(widget.label)),
      body: widget.pending
          ? _pendingView()
          : _loading
              ? const Center(child: CircularProgressIndicator())
              : _error != null
                  ? _errorView()
                  : _DataView(data: _data),
    );
  }

  Widget _pendingView() {
    return Center(
      key: const ValueKey('analytics-domain-pending'),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.construction_outlined,
              size: 48, color: Colors.orange.withValues(alpha: 0.6)),
          const SizedBox(height: 12),
          const Text('数据源待建'),
          const SizedBox(height: 4),
          const Text(
            '该域后端尚未接入，移动端不做合成数据',
            style: TextStyle(color: Colors.grey, fontSize: 12),
          ),
        ],
      ),
    );
  }

  Widget _errorView() {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 24),
            child: Text(_error!),
          ),
          const SizedBox(height: 12),
          FilledButton(
            key: const ValueKey('analytics-domain-retry'),
            onPressed: _fetch,
            child: const Text('重试'),
          ),
        ],
      ),
    );
  }
}

/// 通用数据渲染：Map → 键值行；List → 分块条目；标量 → 文本。
class _DataView extends StatelessWidget {
  const _DataView({this.data});

  final Object? data;

  @override
  Widget build(BuildContext context) {
    final data = this.data;
    if (data == null ||
        (data is Map && data.isEmpty) ||
        (data is List && data.isEmpty)) {
      return const Center(key: ValueKey('analytics-domain-empty'), child: Text('暂无数据'));
    }
    if (data is Map) {
      final rows = <Widget>[];
      data.forEach((key, value) {
        rows.add(_row(key.toString(), _scalar(value)));
      });
      return ListView(
        padding: const EdgeInsets.all(16),
        children: rows,
      );
    }
    if (data is List) {
      return ListView(
        padding: const EdgeInsets.all(16),
        children: [
          for (final item in data) ...[
            Card(
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: Text(_scalar(item)),
              ),
            ),
            const SizedBox(height: 8),
          ],
        ],
      );
    }
    return Center(child: Text(_scalar(data)));
  }

  Widget _row(String key, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 128,
            child: Text(key, style: const TextStyle(color: Colors.grey)),
          ),
          Expanded(child: Text(value)),
        ],
      ),
    );
  }
}

String _scalar(Object? value) {
  if (value == null) return '-';
  if (value is Map || value is List) {
    return jsonEncode(value);
  }
  return value.toString();
}
