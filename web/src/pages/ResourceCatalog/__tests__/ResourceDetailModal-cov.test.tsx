/**
 * ResourceDetailModal 分支补齐：
 * 1. 语义信息四项能力位的 `id || (has* ? 已配置 : 未配置)` 三分支
 *    （collection/create/update/delete 各自的真值与两个兜底）；
 * 2. 可选字段全缺省时的 `-` 回退（identityFieldType/collectionPath/…）；
 * 3. 受影响页面行的状态徽标（status/proposalQuality/stale）、版本列
 *    draft/published 缺修订号回退、updatedAt 缺省；
 * 4. 语义来源表 provenance 行（source 标签渲染）、诊断 severity 非
 *    error/warning 的蓝标分支、task 无 cancel、report 无 datasetPath。
 */
import React from 'react';
import { render, screen, within } from '@testing-library/react';
import ResourceDetailModal from '../ResourceDetailModal';
import type {
  AffectedPageInfo,
  ResourceCatalogItem,
  ResourceSemanticConflicts,
  ResourceSemanticVersions,
  SemanticsInfo,
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

const noop = () => undefined;

function bareResource(overrides?: Partial<ResourceCatalogItem>): ResourceCatalogItem {
  return {
    resourceKey: 'player',
    labels: {},
    status: 'pending',
    functions: [],
    affectedPages: [],
    ...overrides,
  } as ResourceCatalogItem;
}

const defaultProps = {
  open: true,
  resource: bareResource(),
  semanticMeta: emptyMeta,
  semanticVersions: emptyVersions,
  loading: false,
  versionPage: 1,
  versionPageSize: 5,
  onClose: noop,
  onOpenProposals: noop,
  onResolveConflict: noop,
  onVersionPageChange: noop,
};

function renderModal(overrides?: Record<string, unknown>) {
  return render(<ResourceDetailModal {...defaultProps} {...overrides} />);
}

describe('ResourceDetailModal 语义能力位三分支', () => {
  it('collection/create/update/delete 全部缺 id 且 has*=true → 「已配置」', () => {
    const semantics = {
      version: 1,
      source: 'manual',
      hasIdentity: false,
      hasCollection: true,
      hasCreate: true,
      hasUpdate: true,
      hasDelete: true,
      hasActions: false,
      hasTasks: false,
      hasReports: false,
      unresolvedConflicts: 0,
      actions: [],
      tasks: [],
      reports: [],
    } as unknown as SemanticsInfo;
    renderModal({ resource: bareResource({ semantics }) });

    // identityField + actions + tasks + reports 四处「未配置」，
    // collection/create/update/delete 四项均为「已配置」
    expect(screen.getAllByText('未配置')).toHaveLength(4);
    expect(screen.getAllByText('已配置')).toHaveLength(4);
  });

  it('collection/create/update/delete 缺 id 且 has*=false → 「未配置」', () => {
    const semantics = {
      version: 1,
      source: 'manual',
      hasIdentity: true,
      hasCollection: false,
      hasCreate: false,
      hasUpdate: false,
      hasDelete: false,
      hasActions: false,
      hasTasks: false,
      hasReports: false,
      unresolvedConflicts: 0,
    } as unknown as SemanticsInfo;
    renderModal({ resource: bareResource({ semantics }) });

    expect(screen.getAllByText('已配置')).toHaveLength(1); // identityField 走 hasIdentity
    expect(screen.getAllByText('未配置').length).toBeGreaterThanOrEqual(4);
  });

  it('id 存在时直接展示 id（不落入 has* 兜底）', () => {
    const semantics = {
      version: 1,
      source: 'manual',
      hasIdentity: false,
      hasCollection: false,
      hasCreate: false,
      hasUpdate: false,
      hasDelete: false,
      hasActions: false,
      hasTasks: false,
      hasReports: false,
      unresolvedConflicts: 0,
      collectionQueryId: 7,
      createId: 1,
      updateId: 2,
      deleteId: 3,
    } as unknown as SemanticsInfo;
    renderModal({ resource: bareResource({ semantics }) });

    expect(screen.getAllByText('7')).toHaveLength(1);
    expect(screen.getAllByText('1').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('2')).toHaveLength(1);
    expect(screen.getAllByText('3')).toHaveLength(1);
    expect(screen.queryByText('已配置')).not.toBeInTheDocument();
  });
});

describe('ResourceDetailModal 可选字段回退', () => {
  it('语义可选字段全缺省：路径/类型列回退 -，空 actions/tasks/reports 显示未配置', () => {
    const semantics = {
      version: 1,
      source: 'manual',
      hasIdentity: false,
      hasCollection: false,
      hasCreate: false,
      hasUpdate: false,
      hasDelete: false,
      hasActions: false,
      hasTasks: false,
      hasReports: false,
      unresolvedConflicts: 0,
      actions: [],
      tasks: [],
      reports: [],
    } as unknown as SemanticsInfo;
    renderModal({ resource: bareResource({ semantics }) });

    // identityFieldType / collectionPath / itemsFieldName / totalFieldName 四处 '-'
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(4);
    expect(screen.getAllByText('未配置').length).toBeGreaterThanOrEqual(7);
  });

  it('task 无 cancel、report 无 datasetPath：文本不带 cancel 片段、数据集回退 (root)', () => {
    const semantics = {
      version: 1,
      source: 'manual',
      hasIdentity: true,
      hasCollection: true,
      hasCreate: true,
      hasUpdate: true,
      hasDelete: true,
      hasActions: true,
      hasTasks: true,
      hasReports: true,
      unresolvedConflicts: 0,
      identityField: 'uid',
      actions: [{ functionId: 'player.ban', subject: 'resource_item', identityInput: '/uid' }],
      tasks: [
        { start: { functionId: 'task.start' }, status: { function: { functionId: 'task.s' } } },
      ],
      reports: [{ query: { functionId: 'report.query' }, dimensions: [], metrics: [] }],
    } as unknown as SemanticsInfo;
    renderModal({ resource: bareResource({ semantics }) });

    expect(screen.getByText(/task\.start \/ status: task\.s/)).toBeInTheDocument();
    expect(screen.queryByText(/cancel:/)).not.toBeInTheDocument();
    expect(screen.getByText(/dataset: \(root\)/)).toBeInTheDocument();
  });
});

describe('ResourceDetailModal 受影响页面与来源表', () => {
  const pages: AffectedPageInfo[] = [
    {
      kind: 'draft',
      pageKey: 'draft-no-rev',
      title: { 'zh-CN': '草稿页' },
      status: 'drafting',
      updatedAt: '2026-09-20T10:00:00Z',
    },
    {
      kind: 'published',
      pageKey: 'pub-no-ver',
      status: 'live',
      stale: true,
    },
    {
      kind: 'proposal',
      proposalKey: 'prop-9',
      pageKey: 'proposal-page',
      proposalQuality: 'good',
      bindingFreshness: [{ status: 'stale' }],
    } as AffectedPageInfo,
  ];

  it('affectedPages 缺省时按空数组渲染空态；行状态徽标与版本回退', () => {
    renderModal({ resource: bareResource({ affectedPages: undefined }) });
    expect(screen.getByText('当前资源还没有草稿、已发布页面或提案')).toBeInTheDocument();
  });

  it('draft 无 draftRevision / published 无 publishedVersion 回退 -，状态与 stale 徽标渲染', () => {
    renderModal({ resource: bareResource({ affectedPages: pages }) });
    const panel = screen.getByText('受影响页面').closest('h5')?.parentElement;
    expect(panel).not.toBeNull();
    const scope = panel as HTMLElement;

    expect(within(scope).getByText('drafting')).toBeInTheDocument();
    expect(within(scope).getByText('good')).toBeInTheDocument();
    // status 列 stale 徽标 + Freshness 列 stale 汇总
    expect(within(scope).getAllByText('stale').length).toBeGreaterThanOrEqual(2);
    // draft 无 draftRevision、published 无 publishedVersion、proposal 非二者
    expect(within(scope).getAllByText('-').length).toBeGreaterThanOrEqual(3);
    expect(within(scope).getAllByText('无').length).toBeGreaterThanOrEqual(1);
  });

  it('行缺 updatedAt 时时间列回退空串格式化', () => {
    renderModal({
      resource: bareResource({
        affectedPages: [{ kind: 'proposal', proposalKey: 'p', pageKey: 'prop-page' }],
      }),
    });
    expect(screen.getByText('prop-page')).toBeInTheDocument();
    // updatedAt 缺省 → formatDateTime('') 兜底
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(3);
  });
});

describe('ResourceDetailModal 语义版本表', () => {
  it('版本行缺 createdAt → 创建时间列回退空串格式化', () => {
    renderModal({
      resource: bareResource({
        semantics: {
          version: 1,
          source: 'manual',
          hasIdentity: false,
          hasCollection: false,
          hasCreate: false,
          hasUpdate: false,
          hasDelete: false,
          hasActions: false,
          hasTasks: false,
          hasReports: false,
          unresolvedConflicts: 0,
        } as unknown as SemanticsInfo,
      }),
      semanticVersions: {
        items: [{ version: 3, sourceDigest: 'abcdef0123456789', changeReason: '手改' }],
        total: 1,
      } as unknown as ResourceSemanticVersions,
    });
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('abcdef012345')).toBeInTheDocument();
    expect(screen.getByText('手改')).toBeInTheDocument();
  });
});

describe('ResourceDetailModal 语义来源与诊断', () => {
  it('provenance 行渲染来源标签、置信度与值', () => {
    renderModal({
      semanticMeta: {
        conflicts: [],
        provenance: [
          {
            field: 'identityField',
            source: 'platform_review',
            confidence: 'high',
            status: 'effective',
            value: '"uid"',
            updatedAt: '2026-09-20T10:00:00Z',
            updatedBy: 'admin',
          },
          {
            field: 'collectionPath',
            source: 'openapi_rest',
            confidence: 'low',
            status: 'overridden',
            updatedAt: '2026-09-20T11:00:00Z',
            updatedBy: 'system',
          },
        ],
      },
    });

    expect(screen.getByText('平台确认')).toBeInTheDocument();
    expect(screen.getByText('OpenAPI REST')).toBeInTheDocument();
    expect(screen.getByText('uid')).toBeInTheDocument();
    expect(screen.getByText('high')).toBeInTheDocument();
  });

  it('诊断 severity=info 走蓝标分支', () => {
    renderModal({
      resource: bareResource({
        diagnostics: [{ code: 'i1', severity: 'info', message: 'just info' }],
      }),
    });
    const tag = screen.getByText('info').closest('.ant-tag');
    expect(tag).not.toBeNull();
    expect(tag?.className).not.toContain('red');
    expect(tag?.className).not.toContain('orange');
  });
});
