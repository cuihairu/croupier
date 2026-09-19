/**
 * components barrel（index.ts）导出面冒烟：统一出口的每个 re-export
 * 都可访问且为真值（覆盖编译产物的导出 getter），并按形态分类断言，
 * 防止 re-export 路径/拼写回归静默产出 undefined。
 */
import {
  AvatarDropdown,
  AvatarName,
  DASHBOARD_PAGE_TOKENS,
  Footer,
  OperationPageRenderer,
  PageRenderer,
  PageStatePanel,
  PlayerManageTemplate,
  Question,
  ReportPageRenderer,
  ResourcePageRenderer,
  SchemaFormRenderer,
  SelectLang,
  StandardFilterBar,
  StandardListSection,
  SummaryOverview,
  TaskPageRenderer,
  createPlayerManageDemoExecute,
  playerManagePageSpec,
} from '../index';

describe('components barrel 导出面', () => {
  it('全部 19 个导出经 barrel 访问为真值', () => {
    const exports: Record<string, unknown> = {
      Footer,
      Question,
      SelectLang,
      AvatarDropdown,
      AvatarName,
      SummaryOverview,
      StandardFilterBar,
      StandardListSection,
      PageStatePanel,
      DASHBOARD_PAGE_TOKENS,
      PageRenderer,
      ResourcePageRenderer,
      OperationPageRenderer,
      TaskPageRenderer,
      ReportPageRenderer,
      SchemaFormRenderer,
      PlayerManageTemplate,
      playerManagePageSpec,
      createPlayerManageDemoExecute,
    };
    expect(Object.keys(exports)).toHaveLength(19);
    for (const [name, value] of Object.entries(exports)) {
      if (value == null) {
        throw new Error(`barrel 导出 ${name} 不应为空`);
      }
    }
  });

  it('布局与标准页组件为可渲染函数', () => {
    const components: Record<string, unknown> = {
      Footer,
      Question,
      SelectLang,
      AvatarDropdown,
      AvatarName,
      SummaryOverview,
      StandardFilterBar,
      StandardListSection,
      PageStatePanel,
    };
    for (const [name, comp] of Object.entries(components)) {
      if (typeof comp !== 'function') {
        throw new Error(`布局组件 ${name} 应为函数，实际 ${typeof comp}`);
      }
    }
  });

  it('设计令牌与模板 spec 为对象；demo 执行器为函数', () => {
    expect(typeof DASHBOARD_PAGE_TOKENS).toBe('object');
    expect(typeof playerManagePageSpec).toBe('object');
    expect(typeof createPlayerManageDemoExecute).toBe('function');
    // PageRenderer 族与 SchemaFormRenderer 经 default 导出（函数或
    // memo/forwardRef 包装对象，typeof 为 object），仅断言真值形态
    for (const renderer of [
      PageRenderer,
      ResourcePageRenderer,
      OperationPageRenderer,
      TaskPageRenderer,
      ReportPageRenderer,
      SchemaFormRenderer,
      PlayerManageTemplate,
    ]) {
      expect(renderer != null).toBe(true);
    }
  });
});
