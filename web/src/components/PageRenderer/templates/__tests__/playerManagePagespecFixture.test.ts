/** demo-game E2E fixture 漂移锁：scripts/e2e/fixtures/demo-game/
 * player-manage.pagespec.json 必须与 playerManagePageSpec 常量一一对应
 * （导出变换见 toSaveRequestBody）。模板改动后重新导出 fixture，否则
 * 本用例红；E2E 侧 publish/execute 断言以该 JSON 为准。 */
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { playerManagePageSpec } from '../playerManage';

/** 与一次性导出脚本同一变换：PageSpec → PUT /pages/:pageKey 请求体。 */
function toSaveRequestBody(spec: typeof playerManagePageSpec): Record<string, unknown> {
  return {
    type: spec.type,
    resourceKey: spec.resourceKey,
    title: spec.title,
    description: spec.description,
    category: spec.category,
    order: spec.order ?? 0,
    icon: spec.icon ?? '',
    resource: spec.resource,
    bindings: spec.bindings,
  };
}

test('demo-game e2e pagespec fixture matches playerManagePageSpec', () => {
  const fixturePath = resolve(
    __dirname,
    '../../../../../../scripts/e2e/fixtures/demo-game/player-manage.pagespec.json',
  );
  const committed = JSON.parse(readFileSync(fixturePath, 'utf-8')) as Record<string, unknown>;
  const expected = JSON.parse(JSON.stringify(toSaveRequestBody(playerManagePageSpec)));
  expect(committed).toEqual(expected);
});
