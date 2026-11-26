/** SUI-046：组件模板端到端走查（回归锚）。以服务端生成器产出的 CRUD 模板
 * （crud--player 的落库同构形态：表格 + 详情弹窗 + 新建/编辑/删除弹窗）为
 * 代表模板，走一次客户端全链「插入（instantiateTemplateDetailed）→ 发布
 * payload（compileTree）」：
 * - 插入：复制后的页面配置全部节点重分配 id、模板内 sectionKey 剥离（编译期
 *   自动重派）、内部引用（行操作 targetSection / 弹窗 onClick.target / 写表单
 *   onSuccess.refreshNode.target）按 idMap 重映射无悬空、行身份参数原样保留；
 * - 发布：编译产物与后端 CompositeSectionRequest 对齐——表格 autoRun、行操作
 *   目标解析为新弹窗组（modal-<id尾6>，声明 sectionKey 剥离后的自动命名）、
 *   新建入口编译为表格工具栏按钮、删除带危险位、写操作成功链刷新表格。
 * 「执行」侧由 PageRenderer 复合渲染器套件与线上 accept-and-publish 取证覆盖
 * （见 docs/review/composite-component-audit-2026-11-26.md）。 */
import { instantiateTemplateDetailed } from '../ComponentLibrary';
import type { ComponentTemplateDTO } from '../ComponentLibrary';
import { compileTree } from '../compiler';
import type { PageNode } from '../model';
import type { CompositeSection } from '@/types/dashboard';

/** 与 internal/api/component/generator.go:buildCRUDTree 落库树同构
 * （crud--player 实测内容，2026-11-26 自 demo-game/demo 线上取回）。 */
function crudPlayerTemplate(): ComponentTemplateDTO {
  const modal = (id: string, title: string, child: PageNode): PageNode => ({
    id,
    type: 'modal',
    props: { title, width: 'medium', sectionKey: id },
    children: [child],
  });
  const form = (id: string, functionId: string): PageNode => ({
    id,
    type: 'fnForm',
    props: {
      functionId,
      span: 24,
      display: 'inline',
      onSuccess: { kind: 'refreshNode', target: 'table' },
    },
  });
  return {
    key: 'crud--player',
    name: { 'zh-CN': '玩家管理', 'en-US': 'Player management' },
    category: '资源管理',
    builtin: true,
    digest: 'f375e4686c93',
    requiredFunctions: ['player.list', 'player.get', 'player.create', 'player.update', 'player.delete'],
    tree: [
      {
        id: 'table',
        type: 'fnTable',
        props: {
          span: 24,
          title: 'player',
          autoRun: true,
          columns: [],
          functionId: 'player.list',
          rowActions: [
            { label: '查看详情', params: { id: '{{row.id}}' }, targetSection: 'detail-modal' },
            { label: '编辑', params: { id: '{{row.id}}' }, targetSection: 'edit-modal' },
            { label: '删除', danger: true, params: { id: '{{row.id}}' }, targetSection: 'delete-modal' },
          ],
        },
      },
      modal('detail-modal', '详情', {
        id: 'detail-fields',
        type: 'fnFields',
        props: { span: 24, autoRun: true, functionId: 'player.get' },
      }),
      {
        id: 'create-btn',
        type: 'button',
        props: { title: '新建', onClick: { kind: 'openModal', target: 'create-modal' } },
      },
      modal('create-modal', '新建', form('create-form', 'player.create')),
      modal('edit-modal', '编辑', form('edit-form', 'player.update')),
      modal('delete-modal', '删除确认', form('delete-form', 'player.delete')),
    ],
  };
}

describe('SUI-046 代表模板走查：插入（复制后的页面配置）', () => {
  it('id 全量重分配 + sectionKey 剥离 + 内部引用重映射无悬空 + 参数原样保留', () => {
    const tpl = crudPlayerTemplate();
    const originalIds = ['table', 'detail-modal', 'detail-fields', 'create-btn', 'create-modal', 'create-form', 'edit-modal', 'edit-form', 'delete-modal', 'delete-form'];
    const { nodes, dangling } = instantiateTemplateDetailed(tpl);

    // 顶层 6 个节点（表格 + 4 弹窗 + 新建按钮），全部换新 id
    expect(nodes.map((n) => n.type)).toEqual([
      'fnTable',
      'modal',
      'button',
      'modal',
      'modal',
      'modal',
    ]);
    const allIds: string[] = [];
    const walk = (list: PageNode[]): void => {
      for (const n of list) {
        allIds.push(n.id);
        if (n.children) walk(n.children);
      }
    };
    walk(nodes);
    expect(allIds).toHaveLength(originalIds.length);
    for (const id of allIds) expect(originalIds).not.toContain(id);

    // 模板内 sectionKey 不随实例复制（U5）：三个弹窗均剥离，编译期自动重派
    for (const n of nodes) {
      if (n.type === 'modal') expect(n.props.sectionKey).toBeUndefined();
    }

    // 悬空引用为空：模板内引用（行操作/弹窗/刷新链）全部可重映射
    expect(dangling).toEqual([]);

    // 行操作 targetSection 重映射到对应弹窗的「新」节点 id
    const [table, detailModal, createBtn, createModal, editModal, deleteModal] = nodes;
    expect(table.props.rowActions).toEqual([
      { label: '查看详情', params: { id: '{{row.id}}' }, targetSection: detailModal.id },
      { label: '编辑', params: { id: '{{row.id}}' }, targetSection: editModal.id },
      { label: '删除', danger: true, params: { id: '{{row.id}}' }, targetSection: deleteModal.id },
    ]);
    // 新建按钮的弹窗目标与写表单的刷新目标同样落到新 id
    expect(createBtn.props.onClick).toEqual({ kind: 'openModal', target: createModal.id });
    for (const m of [createModal, editModal, deleteModal]) {
      const formNode = (m.children ?? [])[0] as PageNode;
      expect(formNode.props.onSuccess).toEqual({ kind: 'refreshNode', target: table.id });
    }
    // 函数依赖与行身份参数逐项保留（复制语义不篡改绑定）
    expect(table.props.functionId).toBe('player.list');
    expect((createModal.children ?? [])[0].props.functionId).toBe('player.create');
  });
});

describe('SUI-046 代表模板走查：发布 payload（编译产物）', () => {
  it('表格 autoRun + 行操作/工具栏解析为新弹窗组 + 写操作成功链刷新表格', () => {
    const { nodes } = instantiateTemplateDetailed(crudPlayerTemplate());
    const { sections, blockers, warnings } = compileTree(nodes);
    expect(blockers).toEqual([]);
    expect(warnings).toEqual([]);

    const tableSection = sections.find((s) => s.view === 'table') as CompositeSection;
    expect(tableSection.functionId).toBe('player.list');
    expect(tableSection.autoRun).toBe(true);
    // sectionKey 剥离后弹窗组自动命名为 modal-<新id尾6>，行操作/工具栏目标
    // 与之逐一对应（重映射链：模板旧 id → 实例新 id → 编译组名）
    const groupOf = (node: PageNode): string => `modal-${node.id.slice(-6)}`;
    const [, detailModal, , createModal, editModal, deleteModal] = nodes;
    expect(tableSection.rowActions).toEqual([
      { label: '查看详情', targetSection: groupOf(detailModal), params: { id: 'row.id' } },
      { label: '编辑', targetSection: groupOf(editModal), params: { id: 'row.id' } },
      {
        label: '删除',
        targetSection: groupOf(deleteModal),
        params: { id: 'row.id' },
        danger: true,
      },
    ]);
    expect(tableSection.toolbarActions).toEqual([
      { label: '新建', targetSection: groupOf(createModal) },
    ]);

    // 四个弹窗区块：详情（fields）+ 新建/编辑/删除（form），组名与目标一致
    const dialogSections = sections.filter((s) => s.display === 'dialog');
    expect(dialogSections.map((s) => s.functionId)).toEqual([
      'player.get',
      'player.create',
      'player.update',
      'player.delete',
    ]);
    expect(dialogSections.map((s) => s.group)).toEqual([
      groupOf(detailModal),
      groupOf(createModal),
      groupOf(editModal),
      groupOf(deleteModal),
    ]);
    // 写操作成功事件链编译为 refreshNode → 表格区块 key（函数 id 兜底）
    for (const s of dialogSections.filter((x) => x.view === 'form')) {
      expect(s.events).toEqual([
        { event: 'success', action: { kind: 'refreshNode', target: 'player.list' } },
      ]);
    }
    // 唯一表格区块的 key 即刷新链目标（同函数多实例时 fid-2/fid-3 递增）
    expect(tableSection.key).toBe('player.list');
  });
});
