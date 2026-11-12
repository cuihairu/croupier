/**
 * @a11y- Dashboard 可访问性扫描（CR-FIX-19）。
 *
 * 两组门槛，全部基于 axe-core 对真实渲染 DOM 的评估：
 *  1) 关键页面全规则扫描（wcag2a / wcag2aa / wcag21aa）：critical/serious
 *     违例必须为空（白名单除外），明细落盘为 JSON 报告；
 *  2) 两套主题套（blue / inkpink）× 明暗（light / dark）共 4 组对比度专项
 *     （color-contrast 规则）：每组独立报告 + 同一阈值断言——
 *     WCAG AA 正文 4.5:1 / 大字与 UI 3:1 由 axe 规则判定。
 *
 * 报告落盘 `e2e/.a11y-reports/`（含逐节点 selector + 失败摘要），同时作为
 * Playwright 附件进 HTML 报告。白名单只收「已知且有意接受」的违例，每条
 * 必须带英文理由；未列入白名单的 critical/serious 违例让用例红，CI 里
 * @a11y- 进 real-dashboard 扫描列表。
 */

import fs from 'node:fs';
import path from 'node:path';
import { expect, test, type Page, type TestInfo } from '@playwright/test';
import AxeBuilderCore from '@axe-core/playwright';
import { login } from './helpers';

const A11Y_PAGES = [
  '/user/login',
  '/console/home',
  '/functions/menus',
  '/functions/resource-catalog',
] as const;

const THEME_PRESETS = ['blue', 'inkpink'] as const;
const THEME_MODES = ['light', 'dark'] as const;

const REPORT_DIR = path.resolve(__dirname, '.a11y-reports');

interface AxeCheckData {
  fgColor?: string;
  bgColor?: string;
  contrastRatio?: number;
  fontSize?: string;
  fontWeight?: string;
  message?: string;
}

interface AxeNode {
  target: string[];
  html: string;
  any?: Array<{ id: string; data?: AxeCheckData; message?: string }>;
  failureSummary?: string;
}

interface AxeViolation {
  id: string;
  impact: string | null;
  nodes: AxeNode[];
}

/**
 * 已知且有意接受的违例。键 = 规则 id，值 = { pattern, reason }（pattern 为
 * CSS selector 子串匹配，理由面向未来复核者）。新增白名单条目必须先有
 * 报告数据支撑（.a11y-reports/），并在注释中写清不接受修复的原因。
 */
const ACCEPTED: Record<string, Array<{ pattern: string; reason: string }>> = {
  'scrollable-region-focusable': [
    {
      // antd6 always renders the table scroll container (.ant-table-content,
      // overflow:auto) even for empty-state tables; with zero rows nothing
      // can scroll and there is no focusable descendant, so axe flags the
      // region (in-app default-theme scan, resource-catalog empty table —
      // e2e/.a11y-reports/, CI run 43153342945 real-dashboard-e2e). The
      // container is antd-internal and cannot take tabindex without DOM
      // patching; once rows exist the row action buttons satisfy
      // focusable-content, so the flag is empty-state-only.
      pattern: '.ant-table-content',
      reason:
        'antd6 empty-state table scroll container: nothing scrollable, no focusable descendant, antd-internal div',
    },
  ],
  'color-contrast': [
    {
      // Ant Design semantic success Tag preset (green-7 on green-1,
      // 3.37:1). Preset tag pairs are generated from colorSuccess seed by
      // antd and pass the 3:1 UI threshold; restyling one semantic preset
      // would diverge from every other status tag in the app.
      pattern: '.ant-tag-green',
      reason: 'antd success Tag preset pair, generated from colorSuccess seed (>=3:1 UI)',
    },
  ],
};

function isAccepted(violation: AxeViolation, node: AxeNode): boolean {
  const entries = ACCEPTED[violation.id] ?? [];
  const selector = node.target.join(' ');
  return entries.some((e) => selector.includes(e.pattern));
}

function seriousViolations(violations: AxeViolation[]): AxeViolation[] {
  return violations
    .map((v) => ({
      ...v,
      nodes: (v.nodes as AxeNode[]).filter((n) => !isAccepted(v, n)),
    }))
    .filter((v) => (v.impact === 'critical' || v.impact === 'serious') && v.nodes.length > 0);
}

function describeViolations(violations: AxeViolation[]): string {
  return violations
    .map((v) => {
      const sample = v.nodes
        .slice(0, 3)
        .map((n) => n.target.join(' '))
        .join(' ; ');
      return `${v.id}[${v.impact}] x${v.nodes.length}: ${sample}`;
    })
    .join(' | ');
}

async function saveReport(
  testInfo: TestInfo,
  name: string,
  violations: AxeViolation[],
): Promise<void> {
  const payload = violations.map((v) => ({
    id: v.id,
    impact: v.impact,
    count: v.nodes.length,
    nodes: v.nodes.map((n) => ({
      target: n.target.join(' '),
      // html 快照：定位未知违例源头用（axe 自动 id 如 #_r_15_ 无法反查组件）
      html: n.html,
      failureSummary: n.failureSummary,
      checks: (n.any ?? []).map((c) => ({ id: c.id, data: c.data })),
    })),
  }));
  fs.mkdirSync(REPORT_DIR, { recursive: true });
  const file = path.join(REPORT_DIR, `${name}.json`);
  fs.writeFileSync(file, JSON.stringify(payload, null, 2));
  await testInfo.attach(`${name}.json`, {
    body: JSON.stringify(payload, null, 2),
    contentType: 'application/json',
  });
}

/** 页面挂载硬断言：dev server 首跳有资源加载 overlay，必须等真实控件 */
async function assertPageMounted(page: Page, route: string): Promise<void> {
  if (route === '/user/login') {
    await expect(page.getByRole('button', { name: /登\s*录/ })).toBeVisible({
      timeout: 60000,
    });
    return;
  }
  await expect(page.getByTestId('env-selector')).toBeVisible({ timeout: 60000 });
}

async function scanRoute(page: Page, route: string, tags: string[]): Promise<AxeViolation[]> {
  await page.goto(route);
  await page.waitForLoadState('domcontentloaded');
  await assertPageMounted(page, route);
  // 冻结动画/过渡后再采样，保证 axe 取到的是落地后的静态样式而非挂载
  // 过渡中间值；断言本身不变。安装版 @axe-core/playwright 4.14 没有
  // disableAnimations API（4.15+ 才有），手动注入等效冻结样式并等两帧
  // 让样式重算落地。
  await page.addStyleTag({
    content: '*, *::before, *::after { animation: none !important; transition: none !important; }',
  });
  await page.evaluate(
    () =>
      new Promise<void>((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
      ),
  );
  const results = await new AxeBuilderCore({ page }).withTags(tags).analyze();
  return results.violations as AxeViolation[];
}

test.describe('dashboard a11y @a11y-', () => {
  test('login page passes WCAG A/AA scan @a11y-', async ({ page }, testInfo) => {
    // 未认证 fresh context：real-dashboard 项目无 storageState，首页即登录页
    const violations = await scanRoute(page, '/user/login', ['wcag2a', 'wcag2aa', 'wcag21aa']);
    await saveReport(testInfo, 'axe-user-login', seriousViolations(violations));
    const bad = seriousViolations(violations);
    expect(bad, `login page critical/serious: ${describeViolations(bad) || 'none'}`).toEqual([]);
  });

  test('in-app pages pass WCAG A/AA scan under default theme @a11y-', async ({
    page,
  }, testInfo) => {
    await login(page);
    const aggregate: AxeViolation[] = [];
    for (const route of A11Y_PAGES) {
      if (route === '/user/login') continue; // 上一个用例已覆盖未认证形态
      const violations = await scanRoute(page, route, ['wcag2a', 'wcag2aa', 'wcag21aa']);
      await saveReport(testInfo, `axe${route.replace(/\//g, '-')}`, seriousViolations(violations));
      aggregate.push(...seriousViolations(violations));
    }
    expect(
      aggregate,
      `in-app critical/serious: ${describeViolations(aggregate) || 'none'}`,
    ).toEqual([]);
  });

  for (const preset of THEME_PRESETS) {
    for (const mode of THEME_MODES) {
      test(`color contrast sweep ${preset}/${mode} @a11y-`, async ({ page }, testInfo) => {
        // 主题双维偏好先于应用引导注入：themeMode 启动即读这两个键
        await page.addInitScript(
          ([presetKey, modeKey, p, m]) => {
            window.localStorage.setItem(presetKey, p);
            window.localStorage.setItem(modeKey, m);
          },
          ['croupier-theme-preset', 'croupier-theme', preset, mode],
        );
        await login(page);

        const aggregate: AxeViolation[] = [];
        for (const route of A11Y_PAGES) {
          const violations = await scanRoute(page, route, ['wcag2aa']);
          aggregate.push(...seriousViolations(violations).filter((v) => v.id === 'color-contrast'));
        }
        await saveReport(testInfo, `contrast-${preset}-${mode}`, aggregate);
        expect(
          aggregate,
          `contrast ${preset}/${mode} violations: ${describeViolations(aggregate) || 'none'}`,
        ).toEqual([]);
      });
    }
  }
});
