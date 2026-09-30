/// 监控域数值展示格式化（字节 / 时长 / 百分比）。
library;

/// 字节人性化：B/KB/MB/GB/TB，1024 进制，一位小数（整数位不带 .0）。
String formatBytes(int bytes) {
  if (bytes < 1024) return '${bytes}B';
  const units = ['KB', 'MB', 'GB', 'TB', 'PB'];
  var value = bytes.toDouble();
  // unit = 已除次数-1 的单位下标（首除落 KB）。
  var unit = -1;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  final text = value >= 100 || value == value.roundToDouble()
      ? value.round().toString()
      : value.toStringAsFixed(1);
  return '$text${units[unit]}';
}

/// 在线时长人性化：x天x小时 / x小时x分钟 / x分钟 / x秒（两级精度）。
String formatUptime(int seconds) {
  if (seconds <= 0) return '-';
  final days = seconds ~/ 86400;
  final hours = (seconds % 86400) ~/ 3600;
  final minutes = (seconds % 3600) ~/ 60;
  final secs = seconds % 60;
  if (days > 0) return '$days天$hours小时';
  if (hours > 0) return '$hours小时$minutes分钟';
  if (minutes > 0) return '$minutes分钟';
  return '$secs秒';
}

/// 百分比：一位小数（整数不带 .0）。
String formatPercent(double pct) {
  return pct == pct.roundToDouble()
      ? pct.round().toString()
      : pct.toStringAsFixed(1);
}

/// 相对时间（「3 分钟前」）：解析失败回退原文。
/// lastSeen 无时区后缀时按本地时区解读（服务端与展示端同机的部署形态）。
String formatRelativeTime(String raw) {
  if (raw.isEmpty) return '-';
  final time = DateTime.tryParse(raw);
  if (time == null) return raw;
  final diff = DateTime.now().difference(time);
  if (diff.inSeconds < 60) return '刚刚';
  if (diff.inMinutes < 60) return '${diff.inMinutes} 分钟前';
  if (diff.inHours < 24) return '${diff.inHours} 小时前';
  return '${diff.inDays} 天前';
}
