/**
 * OPEN-ISSUES #19 回归：新增用户角色下拉带简短中文说明。
 *
 * 下拉选项由 buildRoleOptions 构造（label=角色名用于选中 tag，description
 * 走 optionRender 副行），搜索经 roleOptionFilter 同时命中名称与描述。
 */
import { buildRoleOptions, roleOptionFilter } from '../roleOptions';

describe('buildRoleOptions', () => {
  it('选项 value/label 为角色名，description 原样透传', () => {
    const options = buildRoleOptions([
      { id: 1, name: 'super_admin', description: '拥有全部权限（超管）' },
      { id: 2, name: 'producer', description: '整体项目负责人，产品方向/上线决策' },
    ]);
    expect(options).toEqual([
      { value: 'super_admin', label: 'super_admin', description: '拥有全部权限（超管）' },
      {
        value: 'producer',
        label: 'producer',
        description: '整体项目负责人，产品方向/上线决策',
      },
    ]);
  });

  it('缺省/空白描述归一为空串（副行不渲染由组件判断）', () => {
    const options = buildRoleOptions([
      { id: 1, name: 'ops' },
      { id: 2, name: 'viewer', description: '   ' },
    ]);
    expect(options.map((o) => o.description)).toEqual(['', '']);
  });
});

describe('roleOptionFilter', () => {
  const option = { value: 'super_admin', description: '拥有全部权限（超管）' };

  it('按角色名命中（不区分大小写）', () => {
    expect(roleOptionFilter('SUPER', option)).toBe(true);
    expect(roleOptionFilter('admin', option)).toBe(true);
  });

  it('按中文描述命中', () => {
    expect(roleOptionFilter('超管', option)).toBe(true);
    expect(roleOptionFilter('权限', option)).toBe(true);
  });

  it('名称与描述都未命中时过滤掉', () => {
    expect(roleOptionFilter('财务', option)).toBe(false);
  });

  it('空输入不过滤（展示全部选项）', () => {
    expect(roleOptionFilter('  ', option)).toBe(true);
  });

  it('option 缺失或键缺失：安全兜底为空串，不命中即过滤（antd 索引签名形状容错）', () => {
    expect(roleOptionFilter('admin')).toBe(false); // option 未传
    expect(roleOptionFilter('admin', null)).toBe(false); // option 为 null
    expect(roleOptionFilter('admin', {})).toBe(false); // value/description 均缺失
    expect(roleOptionFilter('admin', { value: 'ops' })).toBe(false); // 仅 description 缺失
    expect(roleOptionFilter('admin', { description: '运营' })).toBe(false); // 仅 value 缺失
    expect(roleOptionFilter('运营', { description: '运营' })).toBe(true); // 仅描述可命中
    expect(roleOptionFilter('ops', { value: 'ops' })).toBe(true); // 仅名称可命中
  });
});
