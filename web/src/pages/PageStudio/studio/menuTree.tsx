import type React from 'react';
import type { MenuItem } from '@/services/api/menu';
import { localizedText } from '@/utils/localizedText';

export interface MenuTreeDatum {
  title: React.ReactNode;
  value: number;
  key: string;
  children?: MenuTreeDatum[];
}

/** MenuItem 树 → TreeSelect treeData（title 用 labels 本地化 + menuKey 标识）。
 *  MenuMountModal 与 EditorModal 共用，勿在调用方再各自内联实现。 */
export function toMenuTreeData(items: MenuItem[], locale: string): MenuTreeDatum[] {
  return items.map((item) => {
    const label = localizedText(item.labels, locale, item.menuKey);
    const node: MenuTreeDatum = {
      title: (
        <span>
          {label}
          <span style={{ color: 'rgba(0,0,0,0.45)', marginLeft: 8 }}>{item.menuKey}</span>
        </span>
      ),
      value: item.id,
      key: item.menuKey,
    };
    const children = toMenuTreeData(item.children ?? [], locale);
    if (children.length > 0) node.children = children;
    return node;
  });
}
