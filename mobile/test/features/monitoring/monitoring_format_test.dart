import 'package:croupier_mobile/features/monitoring/monitoring_format.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('formatBytes', () {
    test('1024 进制单位进位', () {
      expect(formatBytes(0), '0B');
      expect(formatBytes(512), '512B');
      expect(formatBytes(1024), '1KB');
      expect(formatBytes(1536), '1.5KB');
      expect(formatBytes(1024 * 1024), '1MB');
      expect(formatBytes(3 * 1024 * 1024 * 1024), '3GB');
    });
  });

  group('formatUptime', () {
    test('两级精度', () {
      expect(formatUptime(0), '-');
      expect(formatUptime(45), '45秒');
      expect(formatUptime(130), '2分钟');
      expect(formatUptime(3 * 3600 + 5 * 60), '3小时5分钟');
      expect(formatUptime(2 * 86400 + 3 * 3600), '2天3小时');
    });
  });

  group('formatPercent', () {
    test('整数不带小数位', () {
      expect(formatPercent(12.0), '12');
      expect(formatPercent(12.5), '12.5');
      expect(formatPercent(0), '0');
    });
  });
}
