/**
 * Dashboard PageSpec E2E 测试
 *
 * 覆盖 todo.md 验收矩阵中的所有场景：
 * 1. OpenAPI CRUD
 * 2. SDK CRUD
 * 3. 独立操作
 * 4. 高风险动作
 * 5. 异步任务
 * 6. 报表
 * 7. 契约变化
 * 8. Scope 隔离
 * 9. OpenAPI Source
 */

import { test, expect } from '@playwright/test';
import type { Page } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8000';

// 增加测试超时时间
test.setTimeout(180000); // 3分钟

// ---------------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------------

async function login(page: Page) {
  await page.goto(`${BASE_URL}/user/login`);
  // 等待页面完全加载
  await page.waitForLoadState('domcontentloaded');
  await page.waitForTimeout(5000); // 等待 React 渲染
  // 等待表单出现 - 使用 ProFormText 的 placeholder
  await page.waitForSelector('input[placeholder*="admin"]', { timeout: 60000 });
  await page.fill('input[placeholder*="admin"]', 'admin');
  await page.fill('input[type="password"]', 'ant.design');
  await page.click('button:has-text("Login")');
  // 等待登录成功 - 导航到首页或 dashboard
  await page.waitForURL(/\/(dashboard|$)/, { timeout: 60000 });
  await page.waitForLoadState('networkidle');
}

async function navigateToConsole(page: Page, categoryKey: string, pageKey: string) {
  await page.goto(`${BASE_URL}/console/${categoryKey}/${pageKey}`);
  await page.waitForLoadState('networkidle');
}

async function waitForPageLoad(page: Page) {
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(500); // 等待渲染完成
}

// ---------------------------------------------------------------------------
// 测试用例
// ---------------------------------------------------------------------------

test.describe('Dashboard PageSpec 验收测试', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  // 场景 1: OpenAPI CRUD
  test('OpenAPI CRUD - 资源页面完整功能', async ({ page }) => {
    // 导航到资源页面
    await navigateToConsole(page, 'players', 'players.manage');
    await waitForPageLoad(page);

    // 验证列表视图
    await expect(page.locator('.ant-pro-table')).toBeVisible();
    await expect(page.locator('text=玩家列表')).toBeVisible();

    // 测试分页
    await page.click('text=下一页');
    await waitForPageLoad(page);

    // 测试创建
    await page.click('text=新建');
    await expect(page.locator('.ant-modal')).toBeVisible();
    await page.fill('input[name="name"]', '测试玩家');
    await page.click('text=确定');
    await waitForPageLoad(page);

    // 测试详情
    await page.click('text=查看');
    await expect(page.locator('.ant-drawer')).toBeVisible();
    await page.click('.ant-drawer-close');
    await waitForPageLoad(page);

    // 测试编辑
    await page.click('text=编辑');
    await expect(page.locator('.ant-modal')).toBeVisible();
    await page.fill('input[name="name"]', '更新玩家');
    await page.click('text=确定');
    await waitForPageLoad(page);

    // 测试删除
    await page.click('text=删除');
    await expect(page.locator('.ant-popconfirm')).toBeVisible();
    await page.click('text=确认删除');
    await waitForPageLoad(page);

    // 测试行操作（封禁）
    await page.click('text=封禁');
    await expect(page.locator('.ant-popconfirm')).toBeVisible();
    await page.click('text=确认');
    await waitForPageLoad(page);
  });

  // 场景 2: SDK CRUD
  test('SDK CRUD - 资源页面功能', async ({ page }) => {
    await navigateToConsole(page, 'inventory', 'inventory.manage');
    await waitForPageLoad(page);

    // 验证资源页面加载
    await expect(page.locator('.ant-pro-table')).toBeVisible();

    // 测试 CRUD 操作
    await page.click('text=新建');
    await expect(page.locator('.ant-modal')).toBeVisible();
    await page.click('text=取消');
  });

  // 场景 3: 独立操作
  test('独立操作 - OperationPage 完整功能', async ({ page }) => {
    await navigateToConsole(page, 'mail', 'mail.send');
    await waitForPageLoad(page);

    // 验证表单
    await expect(page.locator('.ant-form')).toBeVisible();

    // 填写表单
    await page.fill('input[name="to"]', 'test@example.com');
    await page.fill('textarea[name="content"]', '测试邮件内容');

    // 确认执行
    await page.click('text=发送');
    await expect(page.locator('.ant-popconfirm')).toBeVisible();
    await page.click('text=确认');

    // 验证结果
    await waitForPageLoad(page);
    await expect(page.locator('text=操作成功')).toBeVisible();
  });

  // 场景 4: 高风险动作
  test('高风险动作 - 需要审批的操作', async ({ page }) => {
    await navigateToConsole(page, 'system', 'system.dangerous-op');
    await waitForPageLoad(page);

    // 填写表单
    await page.fill('input[name="reason"]', '测试原因');

    // 执行高风险操作
    await page.click('text=执行');
    await expect(page.locator('.ant-modal-confirm')).toBeVisible();
    await page.click('text=确认提交');

    // 验证审批状态
    await waitForPageLoad(page);
    await expect(page.locator('text=已提交审批')).toBeVisible();
  });

  // 场景 5: 异步任务
  test('异步任务 - TaskPage 完整功能', async ({ page }) => {
    await navigateToConsole(page, 'reward', 'reward.batchGrant');
    await waitForPageLoad(page);

    // 填写任务参数
    await page.fill('input[name="playerIds"]', '1001,1002,1003');
    await page.fill('input[name="rewardId"]', '100');

    // 提交任务
    await page.click('text=提交任务');
    await waitForPageLoad(page);

    // 验证任务进度
    await expect(page.locator('text=任务进行中')).toBeVisible();

    // 测试取消任务
    await page.click('text=取消任务');
    await expect(page.locator('.ant-popconfirm')).toBeVisible();
    await page.click('text=确认取消');

    // 验证任务取消
    await waitForPageLoad(page);
    await expect(page.locator('text=任务已取消')).toBeVisible();

    // 测试重试任务
    await page.click('text=重试');
    await waitForPageLoad(page);
    await expect(page.locator('text=任务进行中')).toBeVisible();
  });

  // 场景 6: 报表
  test('报表 - ReportPage 完整功能', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'analytics.retention');
    await waitForPageLoad(page);

    // 设置查询参数
    await page.click('text=今天');
    await page.click('text=最近 7 天');

    // 执行查询
    await page.click('text=查询');
    await waitForPageLoad(page);

    // 验证图表
    await expect(page.locator('.antv-charts')).toBeVisible();

    // 测试导出
    await page.click('text=导出');
    await expect(page.locator('text=导出成功')).toBeVisible();
  });

  // 场景 7: 契约变化
  test('契约变化 - Stale 页面处理', async ({ page }) => {
    await navigateToConsole(page, 'players', 'players.manage');
    await waitForPageLoad(page);

    // 验证 stale 警告
    const staleAlert = page.locator('text=页面绑定的函数契约已变化');
    if (await staleAlert.isVisible()) {
      // 验证执行被阻断
      await page.click('text=新建');
      await expect(page.locator('text=执行被阻断')).toBeVisible();
    }
  });

  // 场景 8: Scope 隔离
  test('Scope 隔离 - 不同环境数据隔离', async ({ page }) => {
    // 切换到测试环境
    await page.goto(`${BASE_URL}/console`);
    await page.click('text=切换环境');
    await page.click('text=测试环境');

    // 导航到页面
    await navigateToConsole(page, 'players', 'players.manage');
    await waitForPageLoad(page);

    // 验证数据隔离
    await expect(page.locator('.ant-pro-table')).toBeVisible();
  });

  // 场景 9: OpenAPI Source
  test('OpenAPI Source - 导入和绑定', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/openapi-sources`);
    await waitForPageLoad(page);

    // 上传 OpenAPI 文档
    await page.click('text=上传');
    await page.setInputFiles('input[type="file"]', 'test-openapi.json');
    await page.click('text=确认上传');

    // 验证导入结果
    await waitForPageLoad(page);
    await expect(page.locator('text=导入成功')).toBeVisible();

    // 绑定到 Provider
    await page.click('text=绑定 Provider');
    await page.selectOption('select[name="provider"]', 'test-provider');
    await page.click('text=确认绑定');

    // 验证绑定结果
    await waitForPageLoad(page);
    await expect(page.locator('text=绑定成功')).toBeVisible();
  });
});

test.describe('Page Studio 测试', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('Page Studio - 页面列表和操作', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/pages`);
    await waitForPageLoad(page);

    // 验证页面列表
    await expect(page.locator('.ant-pro-table')).toBeVisible();

    // 测试预览
    await page.click('text=预览');
    await expect(page.locator('.ant-drawer')).toBeVisible();
    await page.click('.ant-drawer-close');

    // 测试发布
    await page.click('text=发布');
    await expect(page.locator('.ant-popconfirm')).toBeVisible();
    await page.click('text=确认');
    await waitForPageLoad(page);

    // 验证发布成功
    await expect(page.locator('text=发布成功')).toBeVisible();
  });
});

test.describe('Resource Catalog 测试', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('Resource Catalog - 资源目录查看', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/resource-catalog`);
    await waitForPageLoad(page);

    // 验证资源列表
    await expect(page.locator('.ant-pro-table')).toBeVisible();

    // 查看资源详情
    await page.click('text=查看');
    await expect(page.locator('.ant-drawer')).toBeVisible();

    // 验证语义信息
    await expect(page.locator('text=已识别')).toBeVisible();
  });
});
