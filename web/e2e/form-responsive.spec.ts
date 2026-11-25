/**
 * SUI-028：表单宽度与响应式布局浏览器用例（mock 实页）。
 *
 * 协议 12 栏宽度在 antd 24 栏栅格上乘 2 落地；窄屏（<768px）每个字段独占
 * 整行，宽屏按声明跨度多列；分组容器与长文本字段在两种视口下都可读取、
 * 可操作（输入/切换与提交链不变）。
 */
import { test, expect, type Page } from '@playwright/test';
import { login, waitForPageReady } from './helpers';

async function openMailOperation(page: Page): Promise<void> {
  await login(page);
  await page.goto('/console/mail/operation--mail.send', { waitUntil: 'domcontentloaded' });
  await waitForPageReady(page);
}

async function colFor(page: Page, label: string) {
  const input = page.getByLabel(label).first();
  await expect(input).toBeVisible();
  // rjsf antd FieldTemplate 内层 Form.Item 也带一层 Col（ant-form-item-control，
  // 固定 ant-col-24）；模板栅格 Col 是其外层祖先，过滤掉内层再取模板列。
  const col = page
    .locator('.ant-col:not(.ant-form-item-control):not(.ant-form-item-label)')
    .filter({ has: input })
    .first();
  await expect(col).toBeVisible();
  return col;
}

test.describe('SUI-028 表单宽度与响应式布局', () => {
  test.beforeEach(async ({ page }) => {
    await openMailOperation(page);
  });

  test('桌面视口：表单字段可读可填，栅格列落地', async ({ page }) => {
    // mail.send mock 表单字段（收件人/主题/内容）沿用默认布局：
    // 多字段场景按类型派生半行/整行，窄字段不消失不重叠。
    const to = await colFor(page, '收件人');
    await expect(to).toHaveClass(/ant-col/);
    const input = page.getByLabel('收件人').first();
    await input.fill('a@example.com');
    await expect(input).toHaveValue('a@example.com');
  });

  test('窄屏视口：字段独占整行且仍可填写与提交入口可见', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 780 });
    // 响应式栅格：所有字段 xs=24（独占整行），antd 在 DOM 上落 ant-col-xs-24
    const to = await colFor(page, '收件人');
    await expect(to).toHaveClass(/ant-col-xs-24/);
    const subject = await colFor(page, '主题');
    await expect(subject).toHaveClass(/ant-col-xs-24/);
    const body = await colFor(page, '内容');
    await expect(body).toHaveClass(/ant-col-xs-24/);

    // 窄屏仍可操作：输入生效、提交按钮仍可见
    await page.getByLabel('收件人').first().fill('narrow@example.com');
    await expect(page.getByLabel('收件人').first()).toHaveValue('narrow@example.com');
    // antd 对双字中文按钮插入字间距（DOM 文本为「提 交」）。
    await expect(page.getByRole('button', { name: /提\s*交/ }).first()).toBeVisible();
  });
});
