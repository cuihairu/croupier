# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: dashboard-vnext.spec.ts >> Dashboard PageSpec 验收测试 >> Scope 隔离 - 不同环境数据隔离
- Location: e2e/dashboard-vnext.spec.ts:230:7

# Error details

```
TimeoutError: page.waitForSelector: Timeout 60000ms exceeded.
Call log:
  - waiting for locator('input[placeholder*="admin"]') to be visible

```

# Page snapshot

```yaml
- generic [active] [ref=e1]: "TypeError: (0 , import_path_to_regexp.default) is not a function at getPathReAndKeys (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+preset-umi@4.6.83_@types+node@25.6.0_@types+react@18.3.28_esbuild@0.28.1_jiti@2._aa32045999ebcea6711657dcbda828fb/node_modules/@umijs/preset-umi/dist/features/mock/createMockMiddleware.js:83:48) at /home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+preset-umi@4.6.83_@types+node@25.6.0_@types+react@18.3.28_esbuild@0.28.1_jiti@2._aa32045999ebcea6711657dcbda828fb/node_modules/@umijs/preset-umi/dist/features/mock/createMockMiddleware.js:45:28 at Layer.handle [as handle_request] (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:157:647) at trim_prefix (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:150:2693) at /home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:150:2247 at router.process_params (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:150:2832) at next (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:150:2151) at /home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+preset-umi@4.6.83_@types+node@25.6.0_@types+react@18.3.28_esbuild@0.28.1_jiti@2._aa32045999ebcea6711657dcbda828fb/node_modules/@umijs/preset-umi/dist/features/favicons/favicons.js:61:9 at Layer.handle [as handle_request] (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:157:647) at trim_prefix (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:150:2693) at /home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:150:2247 at router.process_params (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:150:2832) at next (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:150:2151) at /home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+preset-umi@4.6.83_@types+node@25.6.0_@types+react@18.3.28_esbuild@0.28.1_jiti@2._aa32045999ebcea6711657dcbda828fb/node_modules/@umijs/preset-umi/dist/features/devTool/devTool.js:92:16 at Layer.handle [as handle_request] (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:157:647) at trim_prefix (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:150:2693) at /home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:150:2247 at router.process_params (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:150:2832) at next (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:150:2151) at SendStream.error (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:281:1059) at SendStream.emit (node:events:509:28) at SendStream.error (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:274:2959) at SendStream.onStatError (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:274:4938) at next (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:274:8188) at onstat (/home/cui/workspaces/croupier/web/node_modules/.pnpm/@umijs+bundler-utils@4.6.83/node_modules/@umijs/bundler-utils/compiled/express/index.js:274:8021) at FSReqCallback.oncomplete (node:fs:195:21)"
```

# Test source

```ts
  1   | /**
  2   |  * Dashboard PageSpec E2E 测试
  3   |  *
  4   |  * 覆盖 todo.md 验收矩阵中的所有场景：
  5   |  * 1. OpenAPI CRUD
  6   |  * 2. SDK CRUD
  7   |  * 3. 独立操作
  8   |  * 4. 高风险动作
  9   |  * 5. 异步任务
  10  |  * 6. 报表
  11  |  * 7. 契约变化
  12  |  * 8. Scope 隔离
  13  |  * 9. OpenAPI Source
  14  |  */
  15  | 
  16  | import { test, expect } from '@playwright/test';
  17  | import type { Page } from '@playwright/test';
  18  | 
  19  | const BASE_URL = process.env.BASE_URL || 'http://localhost:8000';
  20  | 
  21  | // 增加测试超时时间
  22  | test.setTimeout(180000); // 3分钟
  23  | 
  24  | // ---------------------------------------------------------------------------
  25  | // 辅助函数
  26  | // ---------------------------------------------------------------------------
  27  | 
  28  | async function login(page: Page) {
  29  |   await page.goto(`${BASE_URL}/user/login`);
  30  |   // 等待页面完全加载
  31  |   await page.waitForLoadState('domcontentloaded');
  32  |   await page.waitForTimeout(5000); // 等待 React 渲染
  33  |   // 等待表单出现 - 使用 ProFormText 的 placeholder
> 34  |   await page.waitForSelector('input[placeholder*="admin"]', { timeout: 60000 });
      |              ^ TimeoutError: page.waitForSelector: Timeout 60000ms exceeded.
  35  |   await page.fill('input[placeholder*="admin"]', 'admin');
  36  |   await page.fill('input[type="password"]', 'ant.design');
  37  |   await page.click('button:has-text("Login")');
  38  |   // 等待登录成功 - 导航到首页或 dashboard
  39  |   await page.waitForURL(/\/(dashboard|$)/, { timeout: 60000 });
  40  |   await page.waitForLoadState('networkidle');
  41  | }
  42  | 
  43  | async function navigateToConsole(page: Page, categoryKey: string, pageKey: string) {
  44  |   await page.goto(`${BASE_URL}/console/${categoryKey}/${pageKey}`);
  45  |   await page.waitForLoadState('networkidle');
  46  | }
  47  | 
  48  | async function waitForPageLoad(page: Page) {
  49  |   await page.waitForLoadState('networkidle');
  50  |   await page.waitForTimeout(500); // 等待渲染完成
  51  | }
  52  | 
  53  | // ---------------------------------------------------------------------------
  54  | // 测试用例
  55  | // ---------------------------------------------------------------------------
  56  | 
  57  | test.describe('Dashboard PageSpec 验收测试', () => {
  58  |   test.beforeEach(async ({ page }) => {
  59  |     await login(page);
  60  |   });
  61  | 
  62  |   // 场景 1: OpenAPI CRUD
  63  |   test('OpenAPI CRUD - 资源页面完整功能', async ({ page }) => {
  64  |     // 导航到资源页面
  65  |     await navigateToConsole(page, 'players', 'players.manage');
  66  |     await waitForPageLoad(page);
  67  | 
  68  |     // 验证列表视图
  69  |     await expect(page.locator('.ant-pro-table')).toBeVisible();
  70  |     await expect(page.locator('text=玩家列表')).toBeVisible();
  71  | 
  72  |     // 测试分页
  73  |     await page.click('text=下一页');
  74  |     await waitForPageLoad(page);
  75  | 
  76  |     // 测试创建
  77  |     await page.click('text=新建');
  78  |     await expect(page.locator('.ant-modal')).toBeVisible();
  79  |     await page.fill('input[name="name"]', '测试玩家');
  80  |     await page.click('text=确定');
  81  |     await waitForPageLoad(page);
  82  | 
  83  |     // 测试详情
  84  |     await page.click('text=查看');
  85  |     await expect(page.locator('.ant-drawer')).toBeVisible();
  86  |     await page.click('.ant-drawer-close');
  87  |     await waitForPageLoad(page);
  88  | 
  89  |     // 测试编辑
  90  |     await page.click('text=编辑');
  91  |     await expect(page.locator('.ant-modal')).toBeVisible();
  92  |     await page.fill('input[name="name"]', '更新玩家');
  93  |     await page.click('text=确定');
  94  |     await waitForPageLoad(page);
  95  | 
  96  |     // 测试删除
  97  |     await page.click('text=删除');
  98  |     await expect(page.locator('.ant-popconfirm')).toBeVisible();
  99  |     await page.click('text=确认删除');
  100 |     await waitForPageLoad(page);
  101 | 
  102 |     // 测试行操作（封禁）
  103 |     await page.click('text=封禁');
  104 |     await expect(page.locator('.ant-popconfirm')).toBeVisible();
  105 |     await page.click('text=确认');
  106 |     await waitForPageLoad(page);
  107 |   });
  108 | 
  109 |   // 场景 2: SDK CRUD
  110 |   test('SDK CRUD - 资源页面功能', async ({ page }) => {
  111 |     await navigateToConsole(page, 'inventory', 'inventory.manage');
  112 |     await waitForPageLoad(page);
  113 | 
  114 |     // 验证资源页面加载
  115 |     await expect(page.locator('.ant-pro-table')).toBeVisible();
  116 | 
  117 |     // 测试 CRUD 操作
  118 |     await page.click('text=新建');
  119 |     await expect(page.locator('.ant-modal')).toBeVisible();
  120 |     await page.click('text=取消');
  121 |   });
  122 | 
  123 |   // 场景 3: 独立操作
  124 |   test('独立操作 - OperationPage 完整功能', async ({ page }) => {
  125 |     await navigateToConsole(page, 'mail', 'mail.send');
  126 |     await waitForPageLoad(page);
  127 | 
  128 |     // 验证表单
  129 |     await expect(page.locator('.ant-form')).toBeVisible();
  130 | 
  131 |     // 填写表单
  132 |     await page.fill('input[name="to"]', 'test@example.com');
  133 |     await page.fill('textarea[name="content"]', '测试邮件内容');
  134 | 
```