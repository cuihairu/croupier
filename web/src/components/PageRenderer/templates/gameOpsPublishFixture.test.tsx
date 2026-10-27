/** C2：PageRenderer fixture 链延长——提案接受 / 发布快照生成 /
 * 控制台执行回读 / 未发布草稿保护。
 *
 * 沿用 gameOpsCrudFixture 的玩家工作台 base Spec 与 gameOpsDemo 内存数据，
 * 不接真实 Dashboard API，仅证明机制：
 *  ① accept-and-publish 将 pending 提案置 accepted 并产出不可变快照；
 *  ② 快照冻结 version/rendererSchemaVersion/bindingContracts；
 *  ③ 控制台从 PublishedPageSpec 进入 ResourcePageRenderer 执行并回读列表；
 *  ④ 只落草稿未发布的页面不进控制台发布面、execute 被保护拒绝。
 *
 * 真实游戏页面与真实浏览器 E2E 留独立环境，不因本 fixture 通过宣称 C2 交付。
 *
 * @module components/PageRenderer/templates/gameOpsPublishFixture
 */

import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { App } from 'antd';
import ResourcePageRenderer from '@/components/PageRenderer/ResourcePageRenderer';
import { playerManagePageSpec } from './playerManage';
import { createPlayerManageDemoExecute } from './playerManage';
import { createGameOpsDemoStore } from './gameOpsDemoData';
import type {
  BindingContractSnapshot,
  BindingExecutionContext,
  PageExecuteFn,
  PageExecutionResult,
  PageProposal,
  PageSpec,
  PageSpecDraft,
  PublishedPageSpec,
} from '@/types/dashboard';

jest.mock('@/components/PageRenderer', () => ({
  __esModule: true,
  default: () => null,
}));

jest.mock('@/components/SchemaFormRenderer', () => {
  const formMockState = { validate: true, values: {} as Record<string, unknown> };
  const Stub = React.forwardRef<unknown, Record<string, unknown>>((_props, ref) => {
    React.useImperativeHandle(ref, () => ({
      validate: () => formMockState.validate,
      getValues: () => formMockState.values,
    }));
    return <div data-testid="schema-form-stub" />;
  });
  Stub.displayName = 'SchemaFormRendererStub';
  return { __esModule: true, default: Stub, formMockState };
});

jest.mock('@ant-design/pro-components', () => {
  const ProTableStub = ({
    request,
    columns,
    rowKey,
    actionRef,
    toolBarRender,
    headerTitle,
    pagination,
  }: {
    request?: (params: Record<string, unknown>) => Promise<{ data: unknown[]; total: number }>;
    columns?: Array<{
      key?: string;
      dataIndex?: string;
      render?: (dom: unknown, record: Record<string, unknown>) => React.ReactNode;
    }>;
    rowKey?: (record: Record<string, unknown>) => string;
    actionRef?: React.MutableRefObject<{ reload?: () => void } | null>;
    toolBarRender?: () => React.ReactNode[];
    headerTitle?: React.ReactNode;
    pagination?:
      false | { showTotal?: (total: number) => React.ReactNode; defaultPageSize?: number };
  }) => {
    const [rows, setRows] = React.useState<Record<string, unknown>[]>([]);
    const reload = React.useCallback(() => {
      void request?.({}).then((r) => setRows(r.data as Record<string, unknown>[]));
    }, [request]);
    React.useEffect(() => {
      void reload();
    }, [reload]);
    React.useEffect(() => {
      if (actionRef) actionRef.current = { reload };
    }, [actionRef, reload]);
    return (
      <div>
        <div data-testid="protable-title">{headerTitle}</div>
        <div>{toolBarRender?.()}</div>
        <div data-testid="rows-loaded" data-count={String(rows.length)} />
        {pagination ? (
          <div data-testid="pagination-bar">{pagination.showTotal?.(rows.length)}</div>
        ) : null}
        <table>
          <tbody>
            {rows.map((row) => (
              <tr key={rowKey?.(row) ?? 'row'}>
                {columns?.map((column) => (
                  <td key={column.key ?? ''}>
                    {column.render
                      ? column.render(null, row)
                      : String(row[column.dataIndex ?? ''] ?? '')}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  };
  const ProDescriptionsStub = ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="prodescriptions">{children}</div>
  );
  (
    ProDescriptionsStub as unknown as {
      Item: React.FC<{ label?: React.ReactNode; children?: React.ReactNode }>;
    }
  ).Item = ({ label, children }) => (
    <div data-testid="prodescriptions-item">
      <span>{label}</span>
      <span>{children}</span>
    </div>
  );
  const PageContainerStub = ({ children }: { children?: React.ReactNode }) => <div>{children}</div>;
  return {
    ProTable: jest.fn(ProTableStub),
    ProDescriptions: ProDescriptionsStub,
    PageContainer: PageContainerStub,
  };
});

const { formMockState } = jest.requireMock('@/components/SchemaFormRenderer') as {
  formMockState: { validate: boolean; values: Record<string, unknown> };
};

const PLAYER_PAGE_KEY = 'template--player-manage';
const PLAYER_PROPOSAL_KEY = 'resource:players';
const PUBLISHED_AT = '2026-10-28T08:00:00.000Z';

function cloneSpec(spec: PageSpec): PageSpec {
  return JSON.parse(JSON.stringify(spec)) as PageSpec;
}

function buildPlayerProposal(): PageProposal {
  return {
    id: 1,
    gameId: 'fixture-game',
    env: 'development',
    proposalKey: PLAYER_PROPOSAL_KEY,
    pageKey: PLAYER_PAGE_KEY,
    pageType: 'resource',
    resourceKey: 'players',
    quality: 'ready',
    generatorVersion: 'page-generator:1',
    functionDigest: 'fixture-fn-digest-v1',
    semanticsDigest: 'fixture-sem-digest-v1',
    title: playerManagePageSpec.title,
    description: playerManagePageSpec.description,
    pageSpec: cloneSpec(playerManagePageSpec),
    diagnostics: [],
    status: 'pending',
    updatedAt: '2026-10-28T00:00:00.000Z',
  };
}

function buildBindingContracts(spec: PageSpec): BindingContractSnapshot[] {
  return spec.bindings.map((binding) => ({
    bindingId: binding.id,
    functionId: binding.functionId,
    functionVersion: 'fixture:v1',
    inputSchemaDigest: `fixture:${binding.id}:in`,
    outputSchemaDigest: `fixture:${binding.id}:out`,
    risk: 'warning',
    permission: `pages:execute:${binding.functionId}`,
    approval: { required: false },
    executionMode: binding.execution.mode,
    rendererSchemaVersion: 'page-spec:1',
  }));
}

/** 内存发布链 fixture：proposal → 接受并发布快照 → 草稿库 → 控制台执行面。 */
function createConsolePublishFixture() {
  const published = new Map<string, PublishedPageSpec>();
  const drafts = new Map<string, PageSpecDraft>();
  let versionSeq = 0;
  return {
    publish(proposal: PageProposal): PublishedPageSpec {
      versionSeq += 1;
      const snapshot: PublishedPageSpec = {
        ...cloneSpec(proposal.pageSpec),
        gameId: proposal.gameId,
        env: proposal.env,
        version: versionSeq,
        publishedAt: PUBLISHED_AT,
        publishedBy: 'fixture-operator',
        rendererSchemaVersion: 'page-spec:1',
        baseProposalKey: proposal.proposalKey,
        baseProposalVersion: versionSeq,
        functionDigest: proposal.functionDigest,
        semanticsDigest: proposal.semanticsDigest,
        generatorVersion: proposal.generatorVersion,
        bindingContracts: buildBindingContracts(proposal.pageSpec),
      };
      published.set(snapshot.pageKey, snapshot);
      proposal.status = 'accepted';
      return snapshot;
    },
    saveManualDraft(draft: PageSpecDraft): void {
      drafts.set(draft.pageKey, draft);
    },
    getDraft(pageKey: string): PageSpecDraft | undefined {
      return drafts.get(pageKey);
    },
    getPublished(pageKey: string): PublishedPageSpec | undefined {
      return published.get(pageKey);
    },
    execute(
      pageKey: string,
      bindingId: string,
      context: BindingExecutionContext,
      executor: PageExecuteFn,
    ): Promise<PageExecutionResult> {
      if (!published.has(pageKey)) {
        return Promise.reject(new Error(`草稿未发布，控制台不可执行：${pageKey}`));
      }
      return executor(bindingId, context);
    },
  };
}

describe('C2 gameOps 发布链 fixture：提案→快照→控制台', () => {
  beforeEach(() => {
    formMockState.validate = true;
    formMockState.values = {};
  });

  it('提案接受 → 发布快照生成 → 控制台执行回读（草稿不回流快照）', async () => {
    const proposal = buildPlayerProposal();
    expect(proposal.status).toBe('pending');
    const fixture = createConsolePublishFixture();
    const store = createGameOpsDemoStore();
    const executor = jest.fn(createPlayerManageDemoExecute(store));
    const onExecute = ((bindingId: string, context: BindingExecutionContext) =>
      fixture.execute(
        PLAYER_PAGE_KEY,
        bindingId,
        context,
        executor as unknown as PageExecuteFn,
      )) as PageExecuteFn;

    // ① 提案接受：accept-and-publish 等效落 accepted，产出发布身份快照
    const snapshot = fixture.publish(proposal);
    expect(proposal.status).toBe('accepted');
    expect(snapshot.pageKey).toBe(PLAYER_PAGE_KEY);
    expect(snapshot.baseProposalKey).toBe(PLAYER_PROPOSAL_KEY);
    expect(snapshot.baseProposalVersion).toBe(1);

    // ② 发布快照生成：冻结 version / renderer / binding 契约快照
    expect(snapshot.version).toBe(1);
    expect(snapshot.publishedAt).toBe(PUBLISHED_AT);
    expect(snapshot.rendererSchemaVersion).toBe('page-spec:1');
    expect(snapshot.bindingContracts).toHaveLength(snapshot.bindings.length);
    expect(snapshot.bindingContracts[0]).toMatchObject({
      bindingId: 'list',
      functionId: 'player.list',
      executionMode: 'sync',
    });
    expect(snapshot.type).toBe('resource');

    // 人工未发布草稿领先修订：发布快照不回流
    fixture.saveManualDraft({
      ...cloneSpec(snapshot),
      status: 'draft',
      draftRevision: 2,
      publishedVersion: 1,
      title: { 'zh-CN': '人工草稿改名', 'en-US': 'Manual Draft Rename' },
      updatedAt: '2026-10-28T09:00:00.000Z',
    } as PageSpecDraft);
    expect(fixture.getDraft(PLAYER_PAGE_KEY)?.draftRevision).toBe(2);
    expect(fixture.getPublished(PLAYER_PAGE_KEY)?.title).toEqual(playerManagePageSpec.title);
    expect(fixture.getPublished(PLAYER_PAGE_KEY)?.bindingContracts[0].functionId).toBe(
      'player.list',
    );

    // ③ 控制台执行回读：Console 只消费 PublishedPageSpec，真实触发 list/create 并刷新
    const resourceSpec = snapshot.type === 'resource' ? snapshot.resource : undefined;
    expect(resourceSpec).toBeDefined();
    render(
      <App>
        <ResourcePageRenderer
          spec={resourceSpec!}
          bindings={snapshot.bindings}
          onExecute={onExecute}
        />
      </App>,
    );

    await waitFor(() => expect(screen.getByText('玩家1号')).toBeInTheDocument());
    expect(screen.getByTestId('rows-loaded')).toHaveAttribute('data-count', '10');
    expect(executor).toHaveBeenCalledWith('list', { form: {} });

    fireEvent.click(screen.getByRole('button', { name: /新\s*建/ }));
    await waitFor(() => expect(screen.getByTestId('schema-form-stub')).toBeInTheDocument());
    formMockState.values = { nickname: 'console-created', level: 21, vip: 5 };
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(screen.getByText('创建成功')).toBeInTheDocument());
    expect(executor).toHaveBeenCalledWith('create', {
      form: { nickname: 'console-created', level: 21, vip: 5 },
    });
    await waitFor(() => expect(screen.getByText('console-created')).toBeInTheDocument());
    // 初始查询 + 写成功刷新：列表被回读两次
    expect(executor.mock.calls.filter(([id]) => id === 'list')).toHaveLength(2);
  }, 90000);

  it('未发布草稿保护：草稿不进控制台发布面，未发布页面 execute 直接拒绝', async () => {
    const fixture = createConsolePublishFixture();
    const proposal = buildPlayerProposal();
    const snapshot = fixture.publish(proposal);

    // 已发布页面的人工草稿（领先未发布修订）：发布面仍返回 version 1 快照
    fixture.saveManualDraft({
      ...cloneSpec(snapshot),
      status: 'draft',
      draftRevision: 3,
      publishedVersion: 1,
      title: { 'zh-CN': '未发布草稿', 'en-US': 'Unpublished Draft' },
      updatedAt: '2026-10-28T09:30:00.000Z',
    } as PageSpecDraft);
    expect(fixture.getDraft(PLAYER_PAGE_KEY)?.draftRevision).toBe(3);
    expect(fixture.getPublished(PLAYER_PAGE_KEY)?.version).toBe(1);
    expect(fixture.getPublished(PLAYER_PAGE_KEY)?.title).toEqual(playerManagePageSpec.title);

    // 只落草稿从未发布：不进发布面，执行在进入 executor 前被保护
    const unpublishedKey = 'template--player-manage-draft-only';
    fixture.saveManualDraft({
      ...cloneSpec(snapshot),
      pageKey: unpublishedKey,
      status: 'draft',
      draftRevision: 1,
      updatedAt: '2026-10-28T09:30:00.000Z',
    } as PageSpecDraft);
    expect(fixture.getPublished(unpublishedKey)).toBeUndefined();

    const executor = jest.fn(createPlayerManageDemoExecute(createGameOpsDemoStore()));
    await expect(
      fixture.execute(unpublishedKey, 'list', { form: {} }, executor as unknown as PageExecuteFn),
    ).rejects.toThrow(/未发布/);
    expect(executor).not.toHaveBeenCalled();
  });
});
