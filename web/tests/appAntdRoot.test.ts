/**
 * <AntdApp> 全局挂载布线守卫（docs/BUGS.md BUG-022）。
 *
 * 背景：antd 的 message/notification/modal 必须在 <AntdApp> 上下文内才能渲染。
 * 此前 AppApiRegistrar 只挂在已登录布局的 childrenRender 里，而登录页是
 * `layout: false`，完全没有 App 上下文——登录成功/失败/MFA 的 message 提示
 * 全部被 antd 静默丢弃（登录失败时页面零反馈），开发模式还伴随
 * "You are calling notice in render" 告警。
 *
 * 修复：rootContainer 全局包裹 + AppApiRegistrar 卸载清理（clearAppApi）。
 * 本用例把这份布线固化成静态约束，防止有人把 <AntdApp> 搬回 childrenRender
 * （登录页再次失去上下文），或删掉卸载清理（重新留下指向死 holder 的实例）。
 */
import fs from 'node:fs';
import path from 'node:path';

const APP_TSX = path.resolve(__dirname, '..', 'src', 'app.tsx');
const source = fs.readFileSync(APP_TSX, 'utf8');

describe('app.tsx 的 <AntdApp> 全局布线（BUG-022）', () => {
  it('rootContainer 导出存在，且在 container 外层包裹 <AntdApp> + AppApiRegistrar', () => {
    const m = source.match(/export\s+const\s+rootContainer[^=]*=\s*(\([\s\S]*?\})\s*;?\s*\n/);
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
