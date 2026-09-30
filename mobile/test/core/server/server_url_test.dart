import 'package:croupier_mobile/core/server/server_url.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('validateServerUrl', () {
    test('空与纯空白不放行', () {
      expect(validateServerUrl('').isValid, isFalse);
      expect(validateServerUrl('   ').isValid, isFalse);
      expect(validateServerUrl('').error, '服务器地址不能为空');
    });

    test('无 scheme / 无法解析为 host 的串不放行', () {
      for (final bad in ['gm.test', 'http://', '://x', 'ht tp://x']) {
        final result = validateServerUrl(bad);
        expect(result.isValid, isFalse, reason: bad);
        expect(result.error, contains('格式不正确'), reason: bad);
      }
    });

    test('非 http/https scheme 不放行', () {
      expect(validateServerUrl('ftp://gm.test').isValid, isFalse);
      expect(validateServerUrl('ftp://gm.test').error, '仅支持 http / https 地址');
    });

    test('合法 https/http 地址通过', () {
      expect(
        validateServerUrl('https://gm.example.com').url,
        'https://gm.example.com',
      );
      expect(
        validateServerUrl('http://10.0.2.2:18780').url,
        'http://10.0.2.2:18780',
      );
    });

    test('归一化：trim 与去结尾多余斜杠（保留端口与路径）', () {
      expect(
        validateServerUrl(' https://gm.example.com/ ').url,
        'https://gm.example.com',
      );
      expect(
        validateServerUrl('http://gm.test:18780/base/').url,
        'http://gm.test:18780/base',
      );
      expect(validateServerUrl('http://gm.test///').url, 'http://gm.test');
    });
  });
}
