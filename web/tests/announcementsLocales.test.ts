/**
 * 公告页 locale 词条覆盖守卫（docs/BUGS.md BUG-023）。
 *
 * 背景：2c236aa 新增 /admin/announcements 页面时，40 个 pages.announcements.*
 * 词条只有 26 个登记进 locale 目录——剩余 14 个（表单标签、校验提示、行操作、
 * 更新成功提示）zh-CN / en-US 双双缺失，运行时靠 defaultMessage 兜底：
 * 英文界面整页回落中文、console 每次渲染刷
 * "[React Intl] Missing message" 错误；菜单键 menu.AccessControl.Announcements
 * 双语也缺失，每渲染一次侧边栏刷一遍。
 *
 * 本用例扫描页面源码里引用的全部 pages.announcements.* id，断言在
 * zh-CN / en-US 两个目录里都存在；并单独断言菜单键。新增词条时本用例自动
 * 跟进；再出现「代码引用了未登记的 id」即失败。
 */
import fs from 'node:fs';
import path from 'node:path';

const WEB_ROOT = path.resolve(__dirname, '..');
const PAGE_TSX = path.join(WEB_ROOT, 'src', 'pages', 'Admin', 'Announcements', 'index.tsx');

const zh = fs.readFileSync(path.join(WEB_ROOT, 'src', 'locales', 'zh-CN', 'pages.ts'), 'utf8');
const en = fs.readFileSync(path.join(WEB_ROOT, 'src', 'locales', 'en-US', 'pages.ts'), 'utf8');
const zhMenu = fs.readFileSync(path.join(WEB_ROOT, 'src', 'locales', 'zh-CN', 'menu.ts'), 'utf8');
const enMenu = fs.readFileSync(path.join(WEB_ROOT, 'src', 'locales', 'en-US', 'menu.ts'), 'utf8');

const source = fs.readFileSync(PAGE_TSX, 'utf8');

// 页面里所有 t('pages.announcements.xxx', '默认文案') 形式的 id 引用
const idMatches = source.match(/['"]pages\.announcements\.[A-Za-z0-9.]+['"]/g) ?? [];
const ids = Array.from(new Set(idMatches.map((m) => m.slice(1, -1))));

describe('公告页 locale 词条覆盖（BUG-023）', () => {
  it('页面引用了 announcements 词条（守卫自身有效性前置）', () => {
    expect(ids.length).toBeGreaterThanOrEqual(40);
  });

  it.each(['zh-CN', 'en-US'])('%s 目录包含页面引用的全部词条', (locale) => {
    const catalog = locale === 'zh-CN' ? zh : en;
    const missing = ids.filter((id) => !catalog.includes(`'${id}'`));
    expect(missing).toEqual([]);
  });

  it.each(['zh-CN', 'en-US'])('%s 菜单包含 menu.AccessControl.Announcements', (locale) => {
    const catalog = locale === 'zh-CN' ? zhMenu : enMenu;
    expect(catalog).toContain(`'menu.AccessControl.Announcements'`);
  });
});
