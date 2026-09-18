/**
 * T-M9 菜单系统端到端验证（real-dashboard）
 *
 * 验收路径（todo.md T-M9）：
 * - 菜单 CRUD 完整流程（管理页 UI：新建 / 加子菜单 / 编辑 / 删除）
 * - 页面挂载菜单：已发布页显式挂到菜单（PUT /pages/:pageKey/menu）后
 *   控制台导航与登录侧边栏可见（menu_items 驱动，发布不自动进菜单）
 * - 权限过滤与继承：带 permission 的菜单对无权限用户隐藏，限制沿子树级联
 *
 * 前置（beforeAll，API）：低权限角色/用户（仅 console:read——保住 /console 侧边栏
 * 可见性，但不含机密菜单的自定义 permission）+ game 授权、
 * operation:mail.send 幂等发布、mail 菜单 + 页面挂载、
 * 机密父子菜单（父带自定义 permission，子无 permission——验证级联继承）。
 */

import { test, expect } from '@playwright/test';
import type { APIRequestContext, Page } from '@playwright/test';
import { readRealFixtureState } from './helpers/realFixture';
import { ensurePageMountedToMenu } from './helpers/menuMount';
import { login, waitForPageReady } from './helpers';

type MenuItemDTO = {
  id: number;
  menuKey: string;
  children?: MenuItemDTO[];
};

type MenuListDTO = { items?: MenuItemDTO[] };

type RolesResponse = { items?: Array<{ id: number; name: string }> };
type AdminsResponse = { items?: Array<{ id: number; username: string }> };

const VIEWER_USERNAME = 'e2e_menu_viewer';
const VIEWER_PASSWORD = 'Croupier#E2E1';
const VIEWER_ROLE = 'e2e-menu-viewer';
const SECRET_PERMISSION = 'e2e:menu:secret';
// 挂载验证用 mail.send（fixture 默认 SDK 函数集自带，提案必然存在；
// operation.spec 的 @sdk-operation-menu 已验证其分类 key=mail）
const MOUNT_MENU_KEY = 'mail';
const MOUNT_MENU_TITLE = 'E2E邮件管理';
const SECRET_MENU_KEY = 'e2e-menu-secret';
const SECRET_MENU_TITLE = 'E2E机密菜单';
const SECRET_CHILD_KEY = 'e2e-menu-secret-child';
const SECRET_CHILD_TITLE = 'E2E机密子菜单';
const MAIL_PROPOSAL_KEY = 'operation:mail.send';
const MAIL_PAGE_KEY = 'operation--mail.send';
const MAIL_PAGE_PATH = `/console/${MOUNT_MENU_KEY}/${MAIL_PAGE_KEY}`;

type ApiContext = { baseURL: string; headers: Record<string, string> };

async function adminAuth(request: APIRequestContext): Promise<ApiContext> {
  const state = readRealFixtureState();
  const response = await request.post(`${state.serverBaseURL}/api/v1/auth/login`, {
    data: { username: 'admin', password: 'admin123' },
  });
  expect(response.status()).toBe(200);
  const session = (await response.json()) as { token?: string };
  expect(session.token).toMatch(/^eyJ/);
  return {
    baseURL: state.serverBaseURL,
    headers: {
      Authorization: `Bearer ${session.token}`,
      'X-Game-ID': state.gameId,
      'X-Env': state.env,
    },
  };
}

async function listMenus(request: APIRequestContext, api: ApiContext): Promise<MenuItemDTO[]> {
  const response = await request.get(`${api.baseURL}/api/v1/menus`, { headers: api.headers });
  expect(response.status()).toBe(200);
  const body = (await response.json()) as MenuListDTO;
  return body.items ?? [];
}

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

async function createMenu(
  request: APIRequestContext,
  api: ApiContext,
  payload: {
    menuKey: string;
    parentId?: number;
    labels: Record<string, string>;
    permission?: string;
  },
): Promise<MenuItemDTO> {
  const response = await request.post(`${api.baseURL}/api/v1/menus`, {
    headers: api.headers,
    data: {
      menuKey: payload.menuKey,
      parentId: payload.parentId ?? 0,
      labels: payload.labels,
      permission: payload.permission ?? '',
      isVisible: true,
    },
  });
  // 已存在（上次中断残留）时复用：beforeAll 的清理通常已删，这里防御
  expect([200, 201, 409]).toContain(response.status());
  if (response.status() !== 409) {
    return (await response.json()) as MenuItemDTO;
  }
  const found = flattenMenus(await listMenus(request, api)).find(
    (item) => item.menuKey === payload.menuKey,
  );
  expect(found).toBeDefined();
  return found as MenuItemDTO;
}

/** 删除本 spec 创建的全部菜单（顶级删除级联子树）。 */
async function cleanupMenus(request: APIRequestContext, api: ApiContext): Promise<void> {
  for (const top of await listMenus(request, api)) {
    if (top.menuKey.startsWith('e2e-menu-') || top.menuKey === MOUNT_MENU_KEY) {
      await request.delete(`${api.baseURL}/api/v1/menus/${top.id}`, { headers: api.headers });
    }
  }
}

async function cleanupViewer(request: APIRequestContext, api: ApiContext): Promise<void> {
  // 先删用户再删角色：角色仍被用户引用时删除会被拒
  const adminsResponse = await request.get(`${api.baseURL}/api/v1/admin`, { headers: api.headers });
  if (adminsResponse.status() === 200) {
    const admins = (await adminsResponse.json()) as AdminsResponse;
    for (const admin of admins.items ?? []) {
      if (admin.username === VIEWER_USERNAME) {
        await request.delete(`${api.baseURL}/api/v1/admin/${admin.id}`, { headers: api.headers });
      }
    }
  }
  const rolesResponse = await request.get(`${api.baseURL}/api/v1/roles`, { headers: api.headers });
  if (rolesResponse.status() === 200) {
    const roles = (await rolesResponse.json()) as RolesResponse;
    for (const role of roles.items ?? []) {
      if (role.name === VIEWER_ROLE) {
        await request.delete(`${api.baseURL}/api/v1/roles/${role.id}`, { headers: api.headers });
      }
    }
  }
}

/** 幂等发布 mail.send 操作页（operation.spec 同款模式）。 */
async function ensureMailPagePublished(request: APIRequestContext, api: ApiContext): Promise<void> {
  const pageURL = `${api.baseURL}/api/v1/console/pages/${MAIL_PAGE_KEY}`;
  const current = await request.get(pageURL, { headers: api.headers });
  if (current.status() === 200) return;
  expect(current.status()).toBe(404);

  const publish = await request.post(
    `${api.baseURL}/api/v1/proposals/${encodeURIComponent(MAIL_PROPOSAL_KEY)}/accept-and-publish`,
    { headers: api.headers },
  );
  expect(publish.status()).toBe(200);

  const published = await request.get(pageURL, { headers: api.headers });
  expect(published.status()).toBe(200);
}

/** 展开侧边栏「运行控制台」子树并确认挂载菜单可见。 */
async function expandConsoleSubtree(page: Page): Promise<void> {
  // 两个坑：
  // 1. 路由命中 /console/* 时 antd 默认展开「运行控制台」，无条件点击会把它
  //    收起（inline 菜单收起动画后子树 display:none）——按 aria-expanded
  //    守卫仅在收起时点击；
  // 2. ProLayout 的 submenu-title 内层（.ant-pro-base-menu-inline-item-title）
  //    会拦截外层 div 的 pointer events，直接点 title div 常报 intercepts——
  //    统一点文字本体。
  const consoleTitle = page.locator('.ant-menu-submenu-title', { hasText: '运行控制台' }).first();
  if ((await consoleTitle.getAttribute('aria-expanded')) !== 'true') {
    await consoleTitle.getByText('运行控制台').click();
  }
  await expect(
    page.locator('.ant-menu-submenu-title', { hasText: MOUNT_MENU_TITLE }).first(),
  ).toBeVisible();
}

test.describe('菜单系统端到端', () => {
  let api: ApiContext;

  test.beforeAll(async ({ request }) => {
    api = await adminAuth(request);

    // 幂等清理上次残留（中断跑 / 重跑），再重建全部前置数据
    await cleanupMenus(request, api);
    await cleanupViewer(request, api);

    // 低权限角色 + 用户 + fixture scope 的 game 授权。
    // console:read 让 /console 侧边栏对该用户可见（access.canConsoleRead），
    // 但不含 SECRET_PERMISSION，机密菜单的过滤语义不受影响。
    const roleResponse = await request.post(`${api.baseURL}/api/v1/roles`, {
      headers: api.headers,
      data: {
        name: VIEWER_ROLE,
        description: 'T-M9 e2e viewer',
        permissions: ['console:read'],
      },
    });
    expect(roleResponse.status()).toBe(200);

    const viewerResponse = await request.post(`${api.baseURL}/api/v1/admin`, {
      headers: api.headers,
      data: {
        username: VIEWER_USERNAME,
        password: VIEWER_PASSWORD,
        nickname: 'T-M9 Viewer',
        email: 'e2e-menu-viewer@example.com',
        phone: '',
        roles: [VIEWER_ROLE],
      },
    });
    expect(viewerResponse.status()).toBe(200);
    const viewer = (await viewerResponse.json()) as { id?: number | string };
    expect(viewer.id).toBeDefined();
    const state = readRealFixtureState();
    const gamesResponse = await request.put(`${api.baseURL}/api/v1/admin/${viewer.id}/games`, {
      headers: api.headers,
      data: {
        games: [{ gameId: state.gameId, gameName: state.gameId, envs: [state.env] }],
      },
    });
    expect(gamesResponse.status()).toBe(200);

    // 发布页 + mail 菜单 + 显式挂载（menu_items 驱动导航）
    await ensureMailPagePublished(request, api);
    await createMenu(request, api, {
      menuKey: MOUNT_MENU_KEY,
      labels: { 'zh-CN': MOUNT_MENU_TITLE },
    });
    await ensurePageMountedToMenu(
      request,
      api.headers,
      MOUNT_MENU_KEY,
      MAIL_PAGE_KEY,
      MOUNT_MENU_TITLE,
    );
    const secret = await createMenu(request, api, {
      menuKey: SECRET_MENU_KEY,
      labels: { 'zh-CN': SECRET_MENU_TITLE },
      permission: SECRET_PERMISSION,
    });
    await createMenu(request, api, {
      menuKey: SECRET_CHILD_KEY,
      parentId: secret.id,
      labels: { 'zh-CN': SECRET_CHILD_TITLE },
    });
  });

  test.afterAll(async ({ request }) => {
    if (!api) return;
    await cleanupMenus(request, api);
    await cleanupViewer(request, api);
  });

  test('@menu- 菜单管理页 CRUD 完整流程', async ({ page }) => {
    await login(page);
    await page.goto('/functions/menus');
    await waitForPageReady(page);

    // 新建顶级菜单（menuKey 用字段 id 定位：placeholder「如 resource（…）」
    // 与 permission 字段的「如 resource:read（…）」前缀相同，会撞 strict mode）
    await page.getByRole('button', { name: '新建菜单' }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await dialog.locator('#menuKey').fill('e2e-menu-crud');
    await dialog.locator('input[placeholder="请输入菜单名称"]').fill('E2E菜单');
    // ModalForm 默认提交按钮是「确 定」（单测 __tests__/index.test.tsx:126 同款定位）
    await dialog.getByRole('button', { name: /确\s*定/ }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByText('E2E菜单', { exact: true })).toBeVisible();
    await expect(page.getByText('e2e-menu-crud')).toBeVisible();

    // 加子菜单（数据变化时受控全展开，子项直接可见）。
    // 树节点按 menuKey Tag 精确匹配：'e2e-menu-crud' 是 'e2e-menu-crud-child'
    // 的前缀，hasText 模糊匹配会命中两个节点触发 strict mode violation。
    const nodeByKey = (key: string) =>
      page
        .locator('.ant-tree-node-content-wrapper')
        .filter({ has: page.locator('.ant-tag', { hasText: new RegExp(`^${key}$`) }) });
    await nodeByKey('e2e-menu-crud').getByRole('button', { name: '加子菜单' }).click();
    const childDialog = page.getByRole('dialog');
    await expect(childDialog).toBeVisible();
    await childDialog.locator('#menuKey').fill('e2e-menu-crud-child');
    await childDialog.locator('input[placeholder="请输入菜单名称"]').fill('E2E子菜单');
    await childDialog.getByRole('button', { name: /确\s*定/ }).click();
    await expect(childDialog).toBeHidden();
    await expect(page.getByText('E2E子菜单', { exact: true })).toBeVisible();
    await expect(page.getByText('e2e-menu-crud-child')).toBeVisible();

    // 编辑改名
    await nodeByKey('e2e-menu-crud-child')
      .getByRole('button', { name: /编\s*辑/ })
      .click();
    const editDialog = page.getByRole('dialog');
    await expect(editDialog).toBeVisible();
    await editDialog.locator('input[placeholder="请输入菜单名称"]').fill('E2E子菜单改');
    await editDialog.getByRole('button', { name: /确\s*定/ }).click();
    await expect(editDialog).toBeHidden();
    await expect(page.getByText('E2E子菜单改', { exact: true })).toBeVisible();

    // 删除子菜单（Popconfirm 渲染在 ant-popover 容器，antd5 无 ant-popconfirm
    // 类；限定其内避免误点页面上其他「确定」按钮）
    await nodeByKey('e2e-menu-crud-child')
      .getByRole('button', { name: /删\s*除/ })
      .click();
    await page.locator('.ant-popover .ant-btn-primary').click();
    await expect(page.getByText('e2e-menu-crud-child')).toHaveCount(0);

    // 删除顶级菜单
    await nodeByKey('e2e-menu-crud')
      .getByRole('button', { name: /删\s*除/ })
      .click();
    await page.locator('.ant-popover .ant-btn-primary').click();
    await expect(page.getByText('e2e-menu-crud')).toHaveCount(0);
  });

  test('@menu- 页面挂载菜单后控制台导航与侧边栏可见', async ({ page, request }) => {
    await login(page);

    // API 预检：ConsoleMenuSpec 按菜单树 + 挂载页面组装（mail 菜单组）
    const menuResponse = await request.get('/api/v1/console/menu', { headers: api.headers });
    expect(menuResponse.status()).toBe(200);
    const consoleMenu = (await menuResponse.json()) as {
      items?: Array<{ key: string; children?: Array<{ key: string; path: string }> }>;
    };
    const mailCategory = (consoleMenu.items || []).find(
      (category) => category.key === MOUNT_MENU_KEY,
    );
    expect(mailCategory).toBeDefined();
    expect((mailCategory?.children || []).some((item) => item.key === MAIL_PAGE_KEY)).toBe(true);

    // UI：登录后侧边栏由 ConsoleMenuSpec（menu_items 驱动）渲染，mail 菜单可见
    await page.goto('/console/home');
    await waitForPageReady(page);
    await expandConsoleSubtree(page);

    // 展开菜单：显式挂载的发布页（beforeAll PUT /pages/:pageKey/menu）为子项。
    // 同 expandConsoleSubtree：点文字本体，避开 ProLayout 内层拦截。
    // antd inline 菜单子项收起时 DOM 保留（hidden），若 submenu 原本已展开，
    // 无条件点击会把它收起——按 aria-expanded 守卫仅在收起时点击。
    const mailTitle = page
      .locator('.ant-menu-submenu-title', { hasText: MOUNT_MENU_TITLE })
      .first();
    if ((await mailTitle.getAttribute('aria-expanded')) !== 'true') {
      await mailTitle.getByText(MOUNT_MENU_TITLE).click();
    }
    await expect(page.locator(`a[href="${MAIL_PAGE_PATH}"]`)).toBeVisible();
  });

  test('@menu- 菜单权限过滤与继承', async ({ page, request }) => {
    // API 正例：admin（admin:all）的 accessible 树含机密父子（继承=子随父可见）
    const accessibleResponse = await request.get(`${api.baseURL}/api/v1/menus/accessible`, {
      headers: api.headers,
    });
    expect(accessibleResponse.status()).toBe(200);
    const accessible = flattenMenus(((await accessibleResponse.json()) as MenuListDTO).items ?? []);
    expect(accessible.some((item) => item.menuKey === SECRET_MENU_KEY)).toBe(true);
    expect(accessible.some((item) => item.menuKey === SECRET_CHILD_KEY)).toBe(true);

    // UI 负例：低权限用户只见无 permission 的菜单，机密父子均隐藏。
    // 先展开「运行控制台」并确认 profile 菜单可见（证明子树已渲染），
    // 此时对机密菜单的 count=0 断言才有验证力。
    await login(page, VIEWER_USERNAME, VIEWER_PASSWORD);
    await page.goto('/console/home');
    await waitForPageReady(page);
    await expandConsoleSubtree(page);
    await expect(page.getByText(SECRET_MENU_TITLE)).toHaveCount(0);
    await expect(page.getByText(SECRET_CHILD_TITLE)).toHaveCount(0);
  });
});
