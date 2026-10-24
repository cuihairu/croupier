/** C1：服务端组件生成器产出的 CRUD 组合模板树必须能编译为可发布的组合页——
 * 新建入口编译为表格工具栏按钮、行操作（详情/编辑/删除）带行身份参数映射
 * 且删除带危险确认、写操作弹窗成功后刷新表格。缺写能力时不生成对应入口。 */
import { compileTree } from '../compiler';
import type { PageNode } from '../model';
import type { CompositeSection } from '@/types/dashboard';

type CrudOpts = { create: boolean; update: boolean; delete: boolean };

/** 与 internal/api/component/generator.go:buildCRUDTree 的输出同构。 */
function crudTree(opts: CrudOpts): PageNode[] {
  const nodes: PageNode[] = [
    {
      id: 'table',
      type: 'fnTable',
      props: {
        functionId: 'player.list',
        title: 'player',
        span: 24,
        autoRun: true,
        columns: [],
        rowActions: [
          { label: '查看详情', targetSection: 'detail-modal', params: { uid: '{{row.uid}}' } },
          ...(opts.update
            ? [{ label: '编辑', targetSection: 'edit-modal', params: { uid: '{{row.uid}}' } }]
            : []),
          ...(opts.delete
            ? [
                {
                  label: '删除',
                  targetSection: 'delete-modal',
                  params: { uid: '{{row.uid}}' },
                  danger: true,
                },
              ]
            : []),
        ],
      },
    },
    {
      id: 'detail-modal',
      type: 'modal',
      props: { title: '详情', width: 'medium', sectionKey: 'detail-modal' },
      children: [
        {
          id: 'detail-fields',
          type: 'fnFields',
          props: { functionId: 'player.get', span: 24, autoRun: true },
        },
      ],
    },
  ];
  if (opts.create) {
    nodes.push(
      {
        id: 'create-btn',
        type: 'button',
        props: { title: '新建', onClick: { kind: 'openModal', target: 'create-modal' } },
      },
      {
        id: 'create-modal',
        type: 'modal',
        props: { title: '新建', width: 'medium', sectionKey: 'create-modal' },
        children: [
          {
            id: 'create-form',
            type: 'fnForm',
            props: {
              functionId: 'player.create',
              span: 24,
              display: 'inline',
              onSuccess: { kind: 'refreshNode', target: 'table' },
            },
          },
        ],
      },
    );
  }
  if (opts.update) {
    nodes.push({
      id: 'edit-modal',
      type: 'modal',
      props: { title: '编辑', width: 'medium', sectionKey: 'edit-modal' },
      children: [
        {
          id: 'edit-form',
          type: 'fnForm',
          props: {
            functionId: 'player.update',
            span: 24,
            display: 'inline',
            onSuccess: { kind: 'refreshNode', target: 'table' },
          },
        },
      ],
    });
  }
  if (opts.delete) {
    nodes.push({
      id: 'delete-modal',
      type: 'modal',
      props: { title: '删除确认', width: 'medium', sectionKey: 'delete-modal' },
      children: [
        {
          id: 'delete-form',
          type: 'fnForm',
          props: {
            functionId: 'player.delete',
            span: 24,
            display: 'inline',
            onSuccess: { kind: 'refreshNode', target: 'table' },
          },
        },
      ],
    });
  }
  return nodes;
}

describe('C1 生成的 CRUD 组合模板树可编译', () => {
  it('全能力：工具栏新建入口 + 三个行操作（删除危险确认）+ 弹窗成功刷新', () => {
    const { sections, blockers, warnings } = compileTree(
      crudTree({ create: true, update: true, delete: true }),
    );
    expect(blockers).toEqual([]);
    expect(warnings).toEqual([]);

    const tableSection = sections.find((s) => s.view === 'table') as CompositeSection;
    // 行操作全部保留（不被「未配置目标的行操作」告警剔除），行身份映射编译为 row.字段
    expect(tableSection.rowActions).toEqual([
      { label: '查看详情', targetSection: 'detail-modal', params: { uid: 'row.uid' } },
      { label: '编辑', targetSection: 'edit-modal', params: { uid: 'row.uid' } },
      {
        label: '删除',
        targetSection: 'delete-modal',
        params: { uid: 'row.uid' },
        danger: true,
      },
    ]);
    // 新建入口编译为表格工具栏按钮
    expect(tableSection.toolbarActions).toEqual([{ label: '新建', targetSection: 'create-modal' }]);
    // 四个弹窗区块（详情 + 三个写操作），行操作 target 各自解析到独立弹窗组
    const dialogSections = sections.filter((s) => s.display === 'dialog');
    expect(dialogSections.map((s) => s.functionId)).toEqual([
      'player.get',
      'player.create',
      'player.update',
      'player.delete',
    ]);
    expect(dialogSections.map((s) => s.group)).toEqual([
      'detail-modal',
      'create-modal',
      'edit-modal',
      'delete-modal',
    ]);
    // 写操作成功事件链刷新表格（编译为 success 事件 → refreshNode 目标区块）
    for (const s of dialogSections.filter((x) => x.view === 'form')) {
      expect(s.events).toEqual([
        { event: 'success', action: { kind: 'refreshNode', target: 'player.list' } },
      ]);
    }
  });

  it('缺写能力：不生成对应入口，不留不可达弹窗', () => {
    const { sections, blockers, warnings } = compileTree(
      crudTree({ create: false, update: false, delete: false }),
    );
    expect(blockers).toEqual([]);
    expect(warnings).toEqual([]);
    const tableSection = sections.find((s) => s.view === 'table') as CompositeSection;
    expect(tableSection.rowActions).toEqual([
      { label: '查看详情', targetSection: 'detail-modal', params: { uid: 'row.uid' } },
    ]);
    expect(tableSection.toolbarActions).toBeUndefined();
    expect(sections.filter((s) => s.view === 'form').map((s) => s.functionId)).toEqual([]);
  });
});
