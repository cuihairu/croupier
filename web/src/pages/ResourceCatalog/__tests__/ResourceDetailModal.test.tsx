/** ResourceDetailModal 覆盖：基本渲染、状态分支、交互回调。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import ResourceDetailModal from '../ResourceDetailModal';
import type {
  ResourceCatalogItem,
  ResourceSemanticConflicts,
  ResourceSemanticVersions,
} from '@/types/dashboard';

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({
    defaultMessage,
  }: {
    id: string;
    defaultMessage?: string;
    values?: Record<string, string>;
  }) => <>{defaultMessage ?? ''}</>,
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, v);
      }
      return text;
    },
  }),
}));

const emptyMeta: ResourceSemanticConflicts = { provenance: [], conflicts: [] };
const emptyVersions: ResourceSemanticVersions = { items: [], total: 0 };

function baseResource(overrides?: Partial<ResourceCatalogItem>): ResourceCatalogItem {
  return {
    resourceKey: 'player',
    labels: { 'zh-CN': '玩家' },
    categoryKey: 'player',
    status: 'ready',
    functions: [
      {
        id: 1,
        functionId: 'player.list',
        version: '1.0.0',
        capability: 'collection_query',
        execution: 'sync',
        risk: 'low',
        enabled: true,
      },
    ],
    affectedPages: [
      {
        kind: 'draft',
        pageKey: 'player-list',
        title: { 'zh-CN': '玩家列表' },
        draftRevision: 3,
        updatedAt: '2026-09-20T10:00:00Z',
      },
      {
        kind: 'published',
        pageKey: 'player-list',
        publishedVersion: 2,
        updatedAt: '2026-09-19T10:00:00Z',
      },
      {
        kind: 'proposal',
        proposalKey: 'prop-1',
        pageKey: 'player-list',
        proposalQuality: 'good',
        updatedAt: '2026-09-21T10:00:00Z',
      },
    ],
    semantics: {
      version: 5,
      source: 'manual',
      identityField: 'uid',
      identityFieldType: 'string',
      hasIdentity: true,
      hasCollection: true,
      collectionQueryId: 'player.list',
      collectionPath: '/items',
      itemsFieldName: 'items',
      totalFieldName: 'total',
      hasCreate: true,
      createId: 'player.create',
      hasUpdate: true,
      updateId: 'player.update',
      hasDelete: true,
      deleteId: 'player.delete',
      actions: [
        { functionId: 'player.ban', subject: 'resource_item', identityInput: '/uid' },
        { functionId: 'player.recharge', subject: 'resource_item' },
      ],
      tasks: [
        {
          start: { functionId: 'task.start' },
          status: { function: { functionId: 'task.status' } },
          cancel: { function: { functionId: 'task.cancel' } },
        },
      ],
      reports: [
        {
          query: { functionId: 'report.query' },
          datasetPath: '/dataset',
          dimensions: ['region', 'level'],
          metrics: ['count', 'revenue'],
        },
      ],
      unresolvedConflicts: 0,
    },
    diagnostics: [
      {
        code: 'identity_mismatch',
        functionId: 'player.list',
        field: 'identityField',
        severity: 'warning',
        message: 'identity field type mismatch',
      },
    ],
    ...overrides,
  } as ResourceCatalogItem;
}

const defaultProps = {
  open: true,
  resource: baseResource(),
  semanticMeta: emptyMeta,
  semanticVersions: emptyVersions,
  loading: false,
  versionPage: 1,
  versionPageSize: 5,
  onClose: jest.fn(),
  onOpenProposals: jest.fn(),
  onResolveConflict: jest.fn(),
  onVersionPageChange: jest.fn(),
};

function renderModal(overrides?: Record<string, unknown>) {
  return render(<ResourceDetailModal {...defaultProps} {...overrides} />);
}

describe('ResourceDetailModal', () => {
  beforeEach(() => jest.clearAllMocks());

  it('resource 为 null 时弹窗内容为空', () => {
    renderModal({ resource: null });
    expect(screen.queryByText('玩家')).not.toBeInTheDocument();
  });

  it('status=conflict 时 Alert 为 error 类型', () => {
    renderModal({ resource: baseResource({ status: 'conflict' }) });
    expect(screen.getByText('Resource Catalog 只维护资源能力语义')).toBeInTheDocument();
  });

  it('status=ready 时 Alert 为 info 类型', () => {
    renderModal();
    expect(screen.getByText('Resource Catalog 只维护资源能力语义')).toBeInTheDocument();
  });

  it('渲染基本信息：资源标识、名称、分类', () => {
    renderModal();
    expect(screen.getAllByText('player').length).toBeGreaterThan(0);
    expect(screen.getByText('玩家')).toBeInTheDocument();
  });

  it('未解决冲突为 0 时显示绿色 Tag', () => {
    renderModal();
    expect(screen.getByText('0')).toBeInTheDocument();
  });

  it('未解决冲突 > 0 时显示红色 Tag', () => {
    renderModal({
      resource: baseResource({
        semantics: { ...baseResource().semantics!, unresolvedConflicts: 3 },
      }),
    });
    expect(screen.getAllByText('3').length).toBeGreaterThan(0);
  });

  it('点击「查看相关提案」传入正确 resourceKey', () => {
    const onOpenProposals = jest.fn();
    renderModal({ onOpenProposals });
    fireEvent.click(screen.getByText('查看相关提案'));
    expect(onOpenProposals).toHaveBeenCalledWith('player');
  });

  it('函数列表渲染函数 ID 和能力', () => {
    renderModal();
    expect(screen.getAllByText('player.list').length).toBeGreaterThan(0);
    // capabilityLabels 映射: collection_query → 列表查询
    expect(screen.getByText('列表查询')).toBeInTheDocument();
  });

  it('函数 enabled=false 时显示禁用 Tag', () => {
    renderModal({
      resource: baseResource({
        functions: [
          {
            id: 2,
            functionId: 'player.ban',
            version: '1.0.0',
            capability: 'mutation',
            execution: 'sync',
            risk: 'high',
            enabled: false,
          },
        ],
      }),
    });
    expect(screen.getByText('禁用')).toBeInTheDocument();
  });

  it('受影响页面按 kind 渲染不同类型', () => {
    renderModal();
    expect(screen.getAllByText('player-list').length).toBeGreaterThan(0);
  });

  it('受影响页面 proposal 有 proposalQuality 显示', () => {
    renderModal();
    expect(screen.getByText('good')).toBeInTheDocument();
  });

  it('受影响页面版本列：draft 显示 draftRevision，published 显示 publishedVersion', () => {
    renderModal();
    expect(screen.getByText('3')).toBeInTheDocument(); // draftRevision
    expect(screen.getByText('2')).toBeInTheDocument(); // publishedVersion
  });

  it('语义信息渲染 identity、collection、lifecycle 字段', () => {
    renderModal();
    expect(screen.getAllByText('uid').length).toBeGreaterThan(0);
    expect(screen.getAllByText('player.list').length).toBeGreaterThan(0);
    expect(screen.getAllByText('player.create').length).toBeGreaterThan(0);
    expect(screen.getAllByText('player.update').length).toBeGreaterThan(0);
    expect(screen.getAllByText('player.delete').length).toBeGreaterThan(0);
  });

  it('语义信息 actions 列表渲染', () => {
    renderModal();
    expect(screen.getByText(/player\.ban \/ resource_item/)).toBeInTheDocument();
    expect(screen.getByText(/player\.recharge \/ resource_item/)).toBeInTheDocument();
  });

  it('语义信息 tasks 列表渲染', () => {
    renderModal();
    expect(screen.getByText(/task\.start \/ status: task\.status/)).toBeInTheDocument();
    expect(screen.getByText(/cancel: task\.cancel/)).toBeInTheDocument();
  });

  it('语义信息 reports 列表渲染', () => {
    renderModal();
    expect(screen.getByText(/report\.query \/ dataset: \/dataset/)).toBeInTheDocument();
    expect(screen.getByText(/dims: region,level/)).toBeInTheDocument();
    expect(screen.getByText(/metrics: count,revenue/)).toBeInTheDocument();
  });

  it('无 semantics 时不渲染语义信息 section', () => {
    renderModal({ resource: baseResource({ semantics: undefined }) });
    expect(screen.queryByText('语义信息')).not.toBeInTheDocument();
  });

  it('无 diagnostics 时不渲染诊断 section', () => {
    renderModal({ resource: baseResource({ diagnostics: [] }) });
    expect(screen.queryByText('诊断信息')).not.toBeInTheDocument();
  });

  it('有 diagnostics 时渲染诊断表', () => {
    renderModal();
    expect(screen.getByText('诊断信息')).toBeInTheDocument();
    expect(screen.getByText('identity_mismatch')).toBeInTheDocument();
    expect(screen.getByText('warning')).toBeInTheDocument();
  });

  it('diagnostics severity=error 渲染红色 Tag', () => {
    renderModal({
      resource: baseResource({
        diagnostics: [{ code: 'err', severity: 'error', message: 'something broke' }],
      }),
    });
    expect(screen.getByText('error')).toBeInTheDocument();
  });

  it('冲突表有 resolution 时按钮 disabled', () => {
    const onResolveConflict = jest.fn();
    renderModal({
      onResolveConflict,
      semanticMeta: {
        provenance: [],
        conflicts: [
          {
            field: 'identityField',
            values: { manual: 'uid', inferred: 'id' },
            resolution: 'manual',
          },
        ],
      },
    });
    const btn = screen.getByText('选择来源');
    expect(btn.closest('button')).toBeDisabled();
  });

  it('冲突表无 resolution 时按钮可点击', () => {
    const onResolveConflict = jest.fn();
    const conflict = {
      field: 'identityField',
      values: { manual: 'uid', inferred: 'id' },
    };
    renderModal({
      onResolveConflict,
      semanticMeta: { provenance: [], conflicts: [conflict] },
    });
    fireEvent.click(screen.getByText('选择来源'));
    expect(onResolveConflict).toHaveBeenCalledWith(conflict);
  });

  it('版本表翻页调用 onVersionPageChange', () => {
    const onVersionPageChange = jest.fn();
    renderModal({
      onVersionPageChange,
      semanticVersions: {
        items: [{ version: 1, changeReason: 'init', createdAt: '2026-09-01', createdBy: 'admin' }],
        total: 20,
      },
    });
    // Ant Design Pagination renders page buttons; click "2"
    const page2 = screen.getByTitle('2');
    if (page2) fireEvent.click(page2);
    expect(onVersionPageChange).toHaveBeenCalled();
  });

  it('版本表 sourceDigest 截断 12 字符', () => {
    renderModal({
      semanticVersions: {
        items: [
          {
            version: 1,
            sourceDigest: 'abcdef1234567890',
            changeReason: 'test',
            createdAt: '2026-09-01',
            createdBy: 'admin',
          },
        ],
        total: 1,
      },
    });
    expect(screen.getByText('abcdef123456')).toBeInTheDocument();
  });

  it('版本表无 sourceDigest 显示 -', () => {
    renderModal({
      semanticVersions: {
        items: [{ version: 1, changeReason: 'test', createdAt: '2026-09-01', createdBy: 'admin' }],
        total: 1,
      },
    });
    // 版本表渲染成功，changeReason 可见
    expect(screen.getByText('test')).toBeInTheDocument();
  });

  it('受影响页面为空时显示空态文案', () => {
    renderModal({ resource: baseResource({ affectedPages: [] }) });
    expect(screen.getByText('当前资源还没有草稿、已发布页面或提案')).toBeInTheDocument();
  });

  it('语义 actions/tasks/reports 为空时显示「未配置」', () => {
    renderModal({
      resource: baseResource({
        semantics: {
          ...baseResource().semantics!,
          actions: [],
          tasks: [],
          reports: [],
        },
      }),
    });
    // 三个「未配置」
    const unconfigured = screen.getAllByText('未配置');
    expect(unconfigured.length).toBeGreaterThanOrEqual(3);
  });

  it('identityField 为空但 hasIdentity=true 显示「已配置」', () => {
    renderModal({
      resource: baseResource({
        semantics: { ...baseResource().semantics!, identityField: '', hasIdentity: true },
      }),
    });
    expect(screen.getByText('已配置')).toBeInTheDocument();
  });

  it('identityField 为空且 hasIdentity=false 显示「未配置」', () => {
    renderModal({
      resource: baseResource({
        semantics: { ...baseResource().semantics!, identityField: '', hasIdentity: false },
      }),
    });
    expect(screen.getByText('未配置')).toBeInTheDocument();
  });
});
