/**
 * 菜单挂载辅助。
 *
 * 运行控制台导航由菜单系统（menu_items）唯一驱动：已发布页面必须显式
 * 挂到菜单（PUT /pages/:pageKey/menu）才出现在 /console 导航——发布本身
 * 不再自动进菜单。各 spec 的「发布后菜单可见」断言前置调用本 helper。
 */

import { expect } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { readRealFixtureState } from './realFixture';

type MenuItemDTO = { id: number; menuKey: string; children?: MenuItemDTO[] };

function flattenMenus(items: MenuItemDTO[]): MenuItemDTO[] {
  const out: MenuItemDTO[] = [];
  const walk = (nodes: MenuItemDTO[]) => {
    for (const node of nodes) {
      out.push(node);
      walk(node.children ?? []);
    }
  };
  walk(items);
  return out;
}

/**
 * 确保 menuKey 菜单存在（不存在则创建，labels 用 labelZh）且 pageKey
 * 已挂载其下，返回菜单 id。幂等：菜单已存在复用；挂载每次显式设置
 * （重复 PUT 同值无害，改挂载即时生效无需重发页面）。
 */
export async function ensurePageMountedToMenu(
  request: APIRequestContext,
  headers: Record<string, string>,
  menuKey: string,
  pageKey: string,
  labelZh: string,
): Promise<number> {
  const { serverBaseURL } = readRealFixtureState();

  const listResponse = await request.get(`${serverBaseURL}/api/v1/menus`, { headers });
  expect(listResponse.status()).toBe(200);
  const body = (await listResponse.json()) as { items?: MenuItemDTO[] };
  let menu = flattenMenus(body.items ?? []).find((item) => item.menuKey === menuKey);

  if (!menu) {
    const createResponse = await request.post(`${serverBaseURL}/api/v1/menus`, {
      headers,
      data: {
        menuKey,
        parentId: 0,
        labels: { 'zh-CN': labelZh },
        permission: '',
        isVisible: true,
      },
    });
    // 409：上次中断残留——回查复用
    expect([200, 201, 409]).toContain(createResponse.status());
    if (createResponse.status() !== 409) {
      menu = (await createResponse.json()) as MenuItemDTO;
    } else {
      menu = flattenMenus(
        (
          (await (await request.get(`${serverBaseURL}/api/v1/menus`, { headers })).json()) as {
            items?: MenuItemDTO[];
          }
        ).items ?? [],
      ).find((item) => item.menuKey === menuKey);
      expect(menu).toBeDefined();
      menu = menu as MenuItemDTO;
    }
  }

  const mountResponse = await request.put(
    `${serverBaseURL}/api/v1/pages/${encodeURIComponent(pageKey)}/menu`,
    { headers, data: { menuId: menu.id } },
  );
  expect(mountResponse.status()).toBe(200);

  return menu.id;
}
