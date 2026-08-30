import { acceptsChild, getComponent, registerComponent, resetRegistryForTest } from '../registry';
import type { PageNode } from '../model';

function stubDef(type: PageNode['type']) {
  return {
    type,
    name: type,
    icon: null,
    category: 'basic' as const,
    propSchema: () => ({ type: 'object', properties: {} }),
    scaffold: () => ({}),
    Preview: () => null,
  };
}

describe('editor v3 registry', () => {
  beforeEach(() => resetRegistryForTest());

  it('注册与获取', () => {
    registerComponent(stubDef('button'));
    expect(getComponent('button')?.name).toBe('button');
    expect(getComponent('modal')).toBeUndefined();
  });

  it('重复注册抛错', () => {
    registerComponent(stubDef('button'));
    expect(() => registerComponent(stubDef('button'))).toThrow(/already registered/);
  });

  it('子节点约束', () => {
    registerComponent({ ...stubDef('modal'), allowedChildren: ['fnForm'] });
    const modal: PageNode = { id: 'm1', type: 'modal', props: {} };
    expect(acceptsChild(modal, 'fnForm')).toBe(true);
    expect(acceptsChild(modal, 'fnTable')).toBe(false);
    // 无 allowedChildren 定义（button）不接受任何子节点
    registerComponent(stubDef('button'));
    const btn: PageNode = { id: 'b1', type: 'button', props: {} };
    expect(acceptsChild(btn, 'fnForm')).toBe(false);
  });
});
