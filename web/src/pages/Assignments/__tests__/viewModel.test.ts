/**
 * Assignments viewModel 纯函数表驱动单测（覆盖率巡检：0% → 全覆盖）。
 *
 * buildAssignmentOptions / buildGroupedAssignments / buildAssignmentStats
 * 无副作用无 DOM 依赖；表驱动覆盖归一分支（非数组入参、version/resource
 * 缺省、displayName 三级回落）与分组/统计语义。页面间接跑（ListTab）此前
 * 从未触达本模块。
 */
import type { FunctionDescriptor } from '@/services/api';
import {
  buildAssignmentOptions,
  buildGroupedAssignments,
  buildAssignmentStats,
  type AssignmentOption,
} from '../viewModel';

const desc = (
  overrides: Partial<FunctionDescriptor> & Pick<FunctionDescriptor, 'id'>,
): FunctionDescriptor => ({
  ...overrides,
});

const opt = (over: Partial<AssignmentOption> & { value: string }): AssignmentOption => ({
  label: over.value,
  resource: 'player',
  displayName: over.value,
  ...over,
});

describe('buildAssignmentOptions', () => {
  it('非数组入参（undefined/null/对象）统一回退空数组', () => {
    expect(buildAssignmentOptions(undefined as unknown as FunctionDescriptor[])).toEqual([]);
    expect(buildAssignmentOptions(null as unknown as FunctionDescriptor[])).toEqual([]);
    expect(buildAssignmentOptions({} as unknown as FunctionDescriptor[])).toEqual([]);
  });

  it('字段齐备时逐字段映射（label 拼版本、displayName 取 zh-CN）', () => {
    expect(
      buildAssignmentOptions([
        desc({
          id: 'player.ban',
          version: '1.2.0',
          resource: 'player',
          operation: 'ban',
          displayName: { 'zh-CN': '封禁玩家', 'en-US': 'Ban player' },
        }),
      ]),
    ).toEqual([
      {
        label: 'player.ban v1.2.0',
        value: 'player.ban',
        version: '1.2.0',
        resource: 'player',
        operation: 'ban',
        displayName: '封禁玩家',
      },
    ]);
  });

  it.each([
    [
      'version 缺失：label 留空版本、version 为 undefined',
      { id: 'a.b' },
      { label: 'a.b v', version: undefined },
    ],
    ['resource 缺失：归一 unassigned', { id: 'a.b', version: '2' }, { resource: 'unassigned' }],
    [
      'displayName 缺失：回落 summary 的 zh-CN',
      { id: 'a.b', summary: { 'zh-CN': '摘要名' } },
      { displayName: '摘要名' },
    ],
    ['displayName/summary 均缺失：回落 id', { id: 'a.b' }, { displayName: 'a.b' }],
  ])('%s', (_name, partial, expectedPart) => {
    const [option] = buildAssignmentOptions([desc(partial)]);
    expect(option).toMatchObject(expectedPart);
  });
});

describe('buildGroupedAssignments', () => {
  it('空 options 返回空分组', () => {
    expect(buildGroupedAssignments([], ['a.ban'])).toEqual([]);
  });

  it('按 resource 分组、selected 标注 active/disabled、组内计 activeCount', () => {
    const groups = buildGroupedAssignments(
      [
        opt({ value: 'a.ban' }),
        opt({ value: 'a.kick' }),
        opt({ value: 'mail.send', resource: 'mail' }),
      ],
      ['a.ban', 'mail.send'],
    );
    // 组序 = 首次出现序（Object.entries 插入序）
    expect(groups).toMatchObject([
      {
        resource: 'player',
        items: [
          { id: 'a.ban', status: 'active' },
          { id: 'a.kick', status: 'disabled' },
        ],
        activeCount: 1,
      },
      { resource: 'mail', items: [{ id: 'mail.send', status: 'active' }], activeCount: 1 },
    ]);
    // 组内 item 全字段：name=displayName，version 缺省落空串
    expect(groups[0].items[0]).toEqual({
      id: 'a.ban',
      name: 'a.ban',
      version: '',
      resource: 'player',
      operation: undefined,
      status: 'active',
    });
  });

  it('resource 空串归一 unassigned 组；selected 为空全 disabled（activeCount 0）', () => {
    expect(buildGroupedAssignments([opt({ value: 'x', resource: '' })], [])).toEqual([
      {
        resource: 'unassigned',
        items: [
          {
            id: 'x',
            name: 'x',
            version: '',
            resource: '',
            operation: undefined,
            status: 'disabled',
          },
        ],
        activeCount: 0,
      },
    ]);
  });
});

describe('buildAssignmentStats', () => {
  it.each([
    ['空 options/selected 全为 0', [], [], { total: 0, active: 0, inactive: 0, resources: 0 }],
    [
      '正常计数，resource 去重计入 resources',
      [
        opt({ value: 'a', resource: 'player' }),
        opt({ value: 'b', resource: 'mail' }),
        opt({ value: 'c', resource: 'player' }),
      ],
      ['a', 'c'],
      { total: 3, active: 2, inactive: 1, resources: 2 },
    ],
  ])('%s', (_name, options, selected, expected) => {
    expect(buildAssignmentStats(options, selected)).toEqual(expected);
  });
});

describe('directoryDisabled（函数目录总开关禁用标注）', () => {
  it('buildAssignmentOptions：disabledIds 命中标注，未命中/未传不标注', () => {
    const options = buildAssignmentOptions(
      [desc({ id: 'a.ban' }), desc({ id: 'a.kick' })],
      new Set(['a.ban']),
    );
    expect(options[0].directoryDisabled).toBe(true);
    expect(options[1].directoryDisabled).toBeUndefined();

    // 未传集合（拉取失败降级）→ 全部可勾选
    const [fallback] = buildAssignmentOptions([desc({ id: 'a.ban' })]);
    expect(fallback.directoryDisabled).toBeUndefined();
  });

  it('buildGroupedAssignments：directoryDisabled 透传到行项', () => {
    const groups = buildGroupedAssignments(
      [opt({ value: 'a.ban', directoryDisabled: true }), opt({ value: 'a.kick' })],
      ['a.ban'],
    );
    expect(groups[0].items[0]).toMatchObject({ id: 'a.ban', directoryDisabled: true });
    expect(groups[0].items[1].directoryDisabled).toBeUndefined();
  });
});
