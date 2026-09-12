import { localizedText } from '@/utils/localizedText';
import type { ComponentTemplateUsage } from '@/types/dashboard';
import type { ComponentTemplateDTO } from './ComponentLibrary';

/**
 * 模板新鲜度比对（U11 更新提醒）：页面快照（key+digest，随保存提交）与
 * 模板库当前列表比对，产出「已更新」模板显示名列表。
 *
 * 门禁（缺一不算 stale，避免误报）：
 * - 双方都有 digest 且不一致才算（旧快照/未迁移模板无 digest 时无法判定，跳过）；
 * - 模板已删除（不在当前列表）不算 stale——删除不是「有新版本」。
 */
export function computeStaleTemplateNames(
  usage: ComponentTemplateUsage[],
  items: ComponentTemplateDTO[],
): string[] {
  const byKey = new Map(items.map((t) => [t.key, t]));
  const stale: string[] = [];
  for (const u of usage) {
    const cur = byKey.get(u.key);
    if (cur && u.digest && cur.digest && u.digest !== cur.digest) {
      stale.push(localizedText(cur.name, 'zh-CN', cur.key));
    }
  }
  return stale;
}
