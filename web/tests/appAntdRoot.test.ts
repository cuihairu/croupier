/**
 * <AntdApp> 全局挂载布线守卫（docs/BUGS.md BUG-022 / OPEN-ISSUES #41）。
 *
 * 背景：antd 的 message/notification/modal 必须在 <AntdApp> 上下文内才能渲染。
 * 此前 AppApiRegistrar 只挂在已登录布局的 childrenRender 里，而登录页是
 * `layout: false`，完全没有 App 上下文——登录成功/失败/MFA 的 message 提示
 * 全部被 antd 静默丢弃（登录失败时页面零反馈），开发模式还伴随
 * "You are calling notice in render" 告警。
 *
 * 修复（BUG-022）：innerProvider 全局包裹 + AppApiRegistrar 卸载清理
 * （clearAppApi）。本用例把这份布线固化成静态约束，防止有人把 <AntdApp>
 * 搬回 childrenRender（登录页再次失去上下文），或删掉卸载清理（重新留下
 * 指向死 holder 的实例）。
 *
 * 层级约束（OPEN-ISSUES #41）：挂载层必须是 innerProvider 而非
 * rootContainer。umi 渲染链由外到内是 rootContainer → i18nProvider
 * （plugin-locale 的 zh_CN antd locale 在此注入）→ innerProvider → 路由；
 * AntdApp 的 modal/message holder 渲染在 AntdApp 所在的 React 层，挂
 * rootContainer 时该层位于 zh_CN ConfigProvider 之外——confirm 弹窗按钮
 * 落回 antd 默认英文（取消键渲染成 "Cancel"），page-studio e2e 的
 * 「取 消」定位 20s 超时。本用例以「rootContainer 不得含 <AntdApp>」锁定
 * 该回归：任何把 <AntdApp> 移回 rootContainer 的改动都会在此失败。
 */
import fs from 'node:fs';
import path from 'node:path';

const APP_TSX = path.resolve(__dirname, '..', 'src', 'app.tsx');
const source = fs.readFileSync(APP_TSX, 'utf8');

describe('app.tsx 的 <AntdApp> 全局布线（BUG-022 / OPEN-ISSUES #41）', () => {
  it('innerProvider 导出存在，且在 container 外层包裹 <AntdApp> + AppApiRegistrar', () => {
    const m = source.match(/export\s+const\s+innerProvider[^=]*=\s*(\([\s\S]*?\})\s*;?\s*\n/);
    expect(m).not.toBeNull();
    const body = m?.[1] ?? '';
    expect(body).toContain('<AntdApp>');
    expect(body).toContain('<AppApiRegistrar />');
    // container 必须渲染在 App 内部
    const openIdx = body.indexOf('<AntdApp>');
    const registrarIdx = body.indexOf('<AppApiRegistrar />');
    const containerIdx = body.indexOf('{container}');
    expect(openIdx).toBeGreaterThanOrEqual(0);
    expect(registrarIdx).toBeGreaterThan(openIdx);
    expect(containerIdx).toBeGreaterThan(registrarIdx);
  });

  it('rootContainer 不得再挂 <AntdApp>（#41：holder 会落在 zh_CN locale 之外）', () => {
    // 允许 rootContainer 不存在；但存在时其函数体不得渲染 <AntdApp>/注册器
    // ——保持为「无附加上下文的最薄外壳」，antd 内置文案继承 i18nProvider。
    const m = source.match(/export\s+const\s+rootContainer[^=]*=\s*(\([\s\S]*?\})\s*;?\s*\n/);
    if (!m) return;
    expect(m?.[1] ?? '').not.toContain('<AntdApp>');
    expect(m?.[1] ?? '').not.toContain('<AppApiRegistrar />');
  });

  it('AppApiRegistrar 卸载时调用 clearAppApi（不留死 holder 实例）', () => {
    const registrar = source.match(
      /const\s+AppApiRegistrar[\s\S]{0,600}?useEffect\(\(\)\s*=>\s*\{[\s\S]{0,300}?\}\s*,\s*\[inst\]\)/,
    );
    expect(registrar).not.toBeNull();
    expect(registrar?.[0]).toContain('setAppApi(inst)');
    expect(registrar?.[0]).toContain('clearAppApi(inst)');
  });

  it('childrenRender 不再渲染 <AntdApp>（登录页 layout:false 不经过布局）', () => {
    const m = source.match(/childrenRender:\s*\(children\)\s*=>\s*\{[\s\S]*?\n\s{4}\},/);
    expect(m).not.toBeNull();
    expect(m?.[0]).not.toContain('<AntdApp>');
    expect(m?.[0]).not.toContain('<AppApiRegistrar />');
    // 布局内仍保留 ScopeMenuRefresher（它依赖 layout 运行时闭包，不属于本修复范围）
    expect(m?.[0]).toContain('<ScopeMenuRefresher />');
  });

  it('antdApp 模块导出 clearAppApi', () => {
    const util = fs.readFileSync(
      path.resolve(__dirname, '..', 'src', 'utils', 'antdApp.ts'),
      'utf8',
    );
    expect(util).toMatch(/export\s+function\s+clearAppApi/);
  });
});
