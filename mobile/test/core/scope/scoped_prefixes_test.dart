import 'package:croupier_mobile/core/scope/scoped_prefixes.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('精确命中', () {
    expect(isScopedApiPath('/api/v1/approvals'), isTrue);
    expect(isScopedApiPath('/api/v1/ops'), isTrue);
    expect(isScopedApiPath('/api/v1/tasks'), isTrue);
  });

  test('子路径命中', () {
    expect(isScopedApiPath('/api/v1/ops/nodes'), isTrue);
    expect(isScopedApiPath('/api/v1/approvals/1/approve'), isTrue);
    expect(isScopedApiPath('/api/v1/providers/sdk-stats'), isTrue);
  });

  test('前缀碰撞不误命中（opsfoo ≠ ops）', () {
    expect(isScopedApiPath('/api/v1/opsfoo'), isFalse);
    expect(isScopedApiPath('/api/v1/operations-x'), isFalse);
  });

  test('非 scoped 域不命中', () {
    expect(isScopedApiPath('/api/v1/auth/login'), isFalse);
    expect(isScopedApiPath('/api/v1/audit'), isFalse);
    expect(isScopedApiPath('/api/v1/alerts'), isFalse);
    expect(isScopedApiPath('/healthz'), isFalse);
  });
}
