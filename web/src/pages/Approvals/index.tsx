import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Card, Tag, Space, Button, Drawer, Descriptions, Select, Input, Tabs } from 'antd';
import { ProTable, type ActionType } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { getMessage } from '@/utils/antdApp';
import {
  approveApproval,
  getApproval,
  listApprovals,
  listDescriptors,
  rejectApproval,
} from '@/services/api';
import type { FunctionDescriptor } from '@/services/api/functions';

type Approval = {
  id: string;
  createdAt: string;
  updatedAt?: string;
  actor: string;
  functionId: string;
  gameId?: string;
  env?: string;
  state: 'pending' | 'approved' | 'rejected';
  mode?: string;
  route?: string;
  reason?: string;
  idempotencyKey?: string;
  targetServiceId?: string;
  hashKey?: string;
  payloadPreview?: string;
  approver?: string;
  reviewedAt?: string;
  reviewedByOther?: boolean;
};

type ViewMode = 'todo' | 'mine' | 'all';

const stateTag = (state: Approval['state']) =>
  state === 'pending' ? 'gold' : state === 'approved' ? 'green' : 'red';
const stateText = (state: Approval['state'], intl: ReturnType<typeof useIntl>) =>
  state === 'pending'
    ? intl.formatMessage({ id: 'pages.approvals.state.pending', defaultMessage: '待审批' })
    : state === 'approved'
      ? intl.formatMessage({ id: 'pages.approvals.state.approved', defaultMessage: '已通过' })
      : intl.formatMessage({ id: 'pages.approvals.state.rejected', defaultMessage: '已拒绝' });

export default function ApprovalsPage() {
  const intl = useIntl();
  // 当前页数据副本：审批动作按 id 在当前页定位记录，在 request 成功后同步
  const [data, setData] = useState<Approval[]>([]);
  const [viewMode, setViewMode] = useState<ViewMode>('todo');
  const [state, setState] = useState<string>('pending');
  const [functionId, setFunctionId] = useState<string>('');
  const [gameId, setGameId] = useState<string>('');
  const [env, setEnv] = useState<string>('');
  const [actor, setActor] = useState<string>('');
  const [riskFilter, setRiskFilter] = useState<string>('');
  const [open, setOpen] = useState(false);
  const [current, setCurrent] = useState<Approval | undefined>();
  const [preview, setPreview] = useState<string>('');
  const [descs, setDescs] = useState<FunctionDescriptor[]>([]);
  const actionRef = useRef<ActionType | undefined>(undefined);
  const descMap = useMemo(() => {
    const m: Record<string, FunctionDescriptor> = {};
    (descs || []).forEach((d: FunctionDescriptor) => {
      if (d?.id) m[d.id] = d;
    });
    return m;
  }, [descs]);

  async function view(id: string) {
    let json: Awaited<ReturnType<typeof getApproval>>;
    try {
      json = await getApproval(id);
    } catch (e) {
      const msg =
        e instanceof Error
          ? e.message
          : intl.formatMessage({
              id: 'pages.approvals.error.loadFailed',
              defaultMessage: '加载失败',
            });
      getMessage()?.error(msg);
      return;
    }
    setCurrent(json as Approval);
    setPreview(json.payloadPreview || '');
    setOpen(true);
  }

  async function approve(id: string) {
    const a = data.find((x) => x.id === id);
    const funcId = a?.functionId || '';
    const desc = funcId ? descMap[funcId] : undefined;
    const risk = (desc?.risk || '').toString().toLowerCase();
    if (risk === 'high') {
      // Require typing the function id as a simple safeguard
      const text =
        window.prompt(
          intl.formatMessage(
            {
              id: 'pages.approvals.prompt.highRiskConfirm',
              defaultMessage: '高风险函数，请输入函数ID确认：{functionId}',
            },
            { functionId: funcId },
          ),
        ) || '';
      if (funcId && text.trim() !== funcId) {
        getMessage()?.warning(
          intl.formatMessage({
            id: 'pages.approvals.warning.confirmMismatch',
            defaultMessage: '确认文本不匹配',
          }),
        );
        return;
      }
    }
    const otp =
      window.prompt(
        intl.formatMessage({
          id: 'pages.approvals.prompt.otp',
          defaultMessage: '动态验证码（若未开启可留空）',
        }),
      ) || '';
    try {
      await approveApproval({ id, otp });
    } catch (e) {
      const msg =
        e instanceof Error
          ? e.message
          : intl.formatMessage({
              id: 'pages.approvals.error.approveFailed',
              defaultMessage: '批准失败',
            });
      getMessage()?.error(msg);
      return;
    }
    getMessage()?.success(
      intl.formatMessage({ id: 'pages.approvals.message.approved', defaultMessage: '已批准' }),
    );
    await actionRef.current?.reload();
    await view(id);
  }

  async function reject(id: string) {
    const reason =
      window.prompt(
        intl.formatMessage({
          id: 'pages.approvals.prompt.rejectReason',
          defaultMessage: '请输入拒绝原因',
        }),
      ) || '';
    try {
      await rejectApproval({ id, reason });
    } catch (e) {
      const msg =
        e instanceof Error
          ? e.message
          : intl.formatMessage({
              id: 'pages.approvals.error.rejectFailed',
              defaultMessage: '拒绝失败',
            });
      getMessage()?.error(msg);
      return;
    }
    getMessage()?.success(
      intl.formatMessage({ id: 'pages.approvals.message.rejected', defaultMessage: '已拒绝' }),
    );
    await actionRef.current?.reload();
    await view(id);
  }

  const exportDetailJSON = () => {
    if (!current) return;
    const obj = {
      id: current.id,
      createdAt: current.createdAt,
      actor: current.actor,
      functionId: current.functionId,
      gameId: current.gameId,
      env: current.env,
      state: current.state,
      mode: current.mode,
      route: current.route,
      idempotencyKey: current.idempotencyKey,
      targetServiceId: current.targetServiceId,
      hashKey: current.hashKey,
      approver: current.approver,
      reviewedAt: current.reviewedAt,
      reviewedByOther: current.reviewedByOther,
      reason: current.reason,
      payloadPreview: preview,
    };
    const blob = new Blob([JSON.stringify(obj, null, 2)], {
      type: 'application/json;charset=utf-8;',
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `approval_${obj.id || ''}.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  useEffect(() => {
    listDescriptors()
      .then((d) => setDescs(d || []))
      .catch(() => {});
  }, []);

  // deep-link：/approvals?approvalId=xxx 直接打开详情（站内信/申请人跳转入口）
  useEffect(() => {
    const approvalId = new URLSearchParams(window.location.search).get('approvalId');
    if (approvalId) void view(approvalId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <Card title={intl.formatMessage({ id: 'pages.approvals.title', defaultMessage: '审批中心' })}>
      <Tabs
        activeKey={viewMode}
        onChange={(key) => {
          const next = key as ViewMode;
          setViewMode(next);
          // 待我审批聚焦 pending；我发起的/全部默认查全部状态
          setState(next === 'todo' ? 'pending' : '');
          // 筛选变化回第 1 页：params 变化与 setPageInfo 的双触发由
          // ProTable 内部 debounce + abort 合并，不会出现错序数据
          actionRef.current?.setPageInfo?.({ current: 1 });
        }}
        items={[
          {
            key: 'todo',
            label: intl.formatMessage({
              id: 'pages.approvals.tab.todo',
              defaultMessage: '待我审批',
            }),
          },
          {
            key: 'mine',
            label: intl.formatMessage({
              id: 'pages.approvals.tab.mine',
              defaultMessage: '我发起的',
            }),
          },
          {
            key: 'all',
            label: intl.formatMessage({ id: 'pages.approvals.tab.all', defaultMessage: '全部' }),
          },
        ]}
      />
      <Space style={{ marginBottom: 16 }} wrap>
        <span>
          {intl.formatMessage({
            id: 'pages.approvals.filter.stateLabel',
            defaultMessage: '状态:',
          })}
        </span>
        <Select
          style={{ width: 160 }}
          value={state}
          onChange={(v) => {
            setState(v);
            actionRef.current?.setPageInfo?.({ current: 1 });
          }}
          options={[
            {
              label: intl.formatMessage({
                id: 'pages.approvals.option.all',
                defaultMessage: '全部',
              }),
              value: '',
            },
            {
              label: intl.formatMessage({
                id: 'pages.approvals.state.pending',
                defaultMessage: '待审批',
              }),
              value: 'pending',
            },
            {
              label: intl.formatMessage({
                id: 'pages.approvals.state.approved',
                defaultMessage: '已通过',
              }),
              value: 'approved',
            },
            {
              label: intl.formatMessage({
                id: 'pages.approvals.state.rejected',
                defaultMessage: '已拒绝',
              }),
              value: 'rejected',
            },
          ]}
        />
        <Input
          placeholder={intl.formatMessage({
            id: 'pages.approvals.filter.placeholder.functionId',
            defaultMessage: '函数ID',
          })}
          value={functionId}
          onChange={(e) => setFunctionId(e.target.value)}
          style={{ width: 240 }}
        />
        <Input
          placeholder={intl.formatMessage({
            id: 'pages.approvals.filter.placeholder.gameId',
            defaultMessage: '游戏',
          })}
          value={gameId}
          onChange={(e) => setGameId(e.target.value)}
          style={{ width: 160 }}
        />
        <Input
          placeholder={intl.formatMessage({
            id: 'pages.approvals.filter.placeholder.env',
            defaultMessage: '环境',
          })}
          value={env}
          onChange={(e) => setEnv(e.target.value)}
          style={{ width: 120 }}
        />
        {viewMode !== 'mine' && (
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.approvals.filter.placeholder.actor',
              defaultMessage: '申请人',
            })}
            value={actor}
            onChange={(e) => setActor(e.target.value)}
            style={{ width: 160 }}
          />
        )}
        <Select
          placeholder={intl.formatMessage({
            id: 'pages.approvals.filter.placeholder.risk',
            defaultMessage: '风险',
          })}
          style={{ width: 140 }}
          value={riskFilter}
          onChange={(v) => {
            setRiskFilter(v);
            actionRef.current?.setPageInfo?.({ current: 1 });
          }}
          options={[
            {
              label: intl.formatMessage({
                id: 'pages.approvals.option.all',
                defaultMessage: '全部',
              }),
              value: '',
            },
            {
              label: intl.formatMessage({ id: 'pages.approvals.risk.high', defaultMessage: '高' }),
              value: 'high',
            },
            {
              label: intl.formatMessage({
                id: 'pages.approvals.risk.medium',
                defaultMessage: '中',
              }),
              value: 'medium',
            },
            {
              label: intl.formatMessage({ id: 'pages.approvals.risk.low', defaultMessage: '低' }),
              value: 'low',
            },
          ]}
        />
        <Button
          onClick={() => {
            actionRef.current?.setPageInfo?.({ current: 1 });
          }}
          type="primary"
        >
          <FormattedMessage id="pages.approvals.button.query" defaultMessage="查询" />
        </Button>
      </Space>
      <ProTable<Approval>
        actionRef={actionRef}
        rowKey="id"
        search={false}
        options={false}
        toolBarRender={false}
        params={{ viewMode, state, functionId, gameId, env, actor, riskFilter, descs }}
        request={async ({
          current = 1,
          pageSize = 20,
          viewMode: vm,
          state: stateFilter,
          functionId: fnFilter,
          gameId: gameFilter,
          env: envFilter,
          actor: actorFilter,
          riskFilter: risk,
        }) => {
          const qs = new URLSearchParams();
          // 我发起的：服务端强制按当前登录人过滤；其余视图按状态过滤
          if (vm === 'mine') qs.set('mine', 'true');
          if (stateFilter) qs.set('status', stateFilter);
          if (fnFilter) qs.set('functionId', fnFilter);
          if (gameFilter) qs.set('gameId', gameFilter);
          if (envFilter) qs.set('env', envFilter);
          if (vm !== 'mine' && actorFilter) qs.set('actor', actorFilter);
          if (risk) qs.set('risk', risk);
          qs.set('page', String(current));
          qs.set('pageSize', String(pageSize));
          try {
            const json = await listApprovals(Object.fromEntries(qs));
            const rows = json.approvals || [];
            // 审批动作按当前页记录定位，同步一份到本地 state
            setData(rows);
            // 原 filtered 语义：风险条件在服务端过滤外，再按 descriptor 风险
            // 对当前页做客户端二次过滤（descs 进入 params，descriptors 晚到时
            // 触发一次重查以对齐原“descMap 就绪后自动重算”的行为）
            const wantRisk = (risk || '').trim().toLowerCase();
            const visible = wantRisk
              ? rows.filter((r) => {
                  const d = descMap[r.functionId || ''];
                  return (d?.risk || '').toString().toLowerCase() === wantRisk;
                })
              : rows;
            return { data: visible, total: json.total || 0, success: true };
          } catch (e) {
            const msg =
              e instanceof Error
                ? e.message
                : intl.formatMessage({
                    id: 'pages.approvals.error.loadFailed',
                    defaultMessage: '加载失败',
                  });
            getMessage()?.error(msg);
            return { data: [], total: 0, success: false };
          }
        }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        columns={[
          {
            title: intl.formatMessage({
              id: 'pages.approvals.column.createdAt',
              defaultMessage: '创建时间',
            }),
            dataIndex: 'createdAt',
          },
          {
            title: intl.formatMessage({
              id: 'pages.approvals.column.actor',
              defaultMessage: '申请人',
            }),
            dataIndex: 'actor',
          },
          {
            title: intl.formatMessage({
              id: 'pages.approvals.column.function',
              defaultMessage: '函数',
            }),
            dataIndex: 'functionId',
            render: (_, r) => {
              const d = descMap[r.functionId];
              const risk = (d?.risk || '').toString().toLowerCase();
              const tags: React.ReactNode[] = [];
              if (risk)
                tags.push(
                  <Tag key="risk" color={risk === 'high' ? 'red' : 'gold'}>
                    {risk}
                  </Tag>,
                );
              if (risk === 'high')
                tags.push(
                  <Tag key="otp" color="blue">
                    OTP
                  </Tag>,
                );
              return (
                <Space size={4}>
                  {r.functionId}
                  {tags}
                </Space>
              );
            },
          },
          {
            title: intl.formatMessage({
              id: 'pages.approvals.column.gameEnv',
              defaultMessage: '游戏/环境',
            }),
            render: (_, r) => `${r.gameId || ''}/${r.env || ''}`,
          },
          {
            title: intl.formatMessage({
              id: 'pages.approvals.column.state',
              defaultMessage: '状态',
            }),
            dataIndex: 'state',
            render: (_, r) => <Tag color={stateTag(r.state)}>{stateText(r.state, intl)}</Tag>,
          },
          {
            title: intl.formatMessage({
              id: 'pages.approvals.column.reviewInfo',
              defaultMessage: '审批信息',
            }),
            render: (_, r) => {
              if (!r.approver) return '-';
              return (
                <Space size={4}>
                  <span>
                    {r.approver}
                    {r.reviewedAt ? ` / ${r.reviewedAt}` : ''}
                  </span>
                  {r.reviewedByOther && (
                    <Tag color="green" style={{ marginInlineEnd: 0 }}>
                      <FormattedMessage
                        id="pages.approvals.tag.twoPersonReview"
                        defaultMessage="两人复核"
                      />
                    </Tag>
                  )}
                </Space>
              );
            },
          },
          {
            title: intl.formatMessage({
              id: 'pages.approvals.column.mode',
              defaultMessage: '模式',
            }),
            dataIndex: 'mode',
          },
          {
            title: intl.formatMessage({
              id: 'pages.approvals.column.actions',
              defaultMessage: '操作',
            }),
            render: (_, r) => (
              <Space>
                <Button size="small" onClick={() => view(r.id)}>
                  <FormattedMessage id="pages.approvals.button.view" defaultMessage="查看" />
                </Button>
                {/* 我发起的视图只读：两人规则下申请人无权审批自己的申请 */}
                {viewMode !== 'mine' && r.state === 'pending' && (
                  <Button size="small" type="primary" onClick={() => approve(r.id)}>
                    <FormattedMessage id="pages.approvals.button.approve" defaultMessage="通过" />
                  </Button>
                )}
                {viewMode !== 'mine' && r.state === 'pending' && (
                  <Button size="small" danger onClick={() => reject(r.id)}>
                    <FormattedMessage id="pages.approvals.button.reject" defaultMessage="拒绝" />
                  </Button>
                )}
              </Space>
            ),
          },
        ]}
      />
      <Drawer
        title={intl.formatMessage(
          { id: 'pages.approvals.drawer.title', defaultMessage: '审批详情 {id}' },
          { id: current?.id || '' },
        )}
        width={720}
        open={open}
        onClose={() => setOpen(false)}
      >
        {current && (
          <>
            <Space style={{ marginBottom: 12 }} wrap>
              {current.actor && (
                <Button
                  size="small"
                  onClick={() =>
                    window.open(
                      `/ops/audit?actor=${encodeURIComponent(current.actor || '')}`,
                      '_blank',
                    )
                  }
                >
                  <FormattedMessage
                    id="pages.approvals.button.auditActor"
                    defaultMessage="查看审计（申请人）"
                  />
                </Button>
              )}
              {current.state === 'approved' && (
                <Button
                  size="small"
                  onClick={() =>
                    window.open(
                      `/ops/audit?actor=${encodeURIComponent(current.approver || current.actor || '')}&kind=approval_approve`,
                      '_blank',
                    )
                  }
                >
                  <FormattedMessage
                    id="pages.approvals.button.auditApprove"
                    defaultMessage="查看审计（批准）"
                  />
                </Button>
              )}
              {current.state === 'rejected' && (
                <Button
                  size="small"
                  onClick={() =>
                    window.open(
                      `/ops/audit?actor=${encodeURIComponent(current.approver || current.actor || '')}&kind=approval_reject`,
                      '_blank',
                    )
                  }
                >
                  <FormattedMessage
                    id="pages.approvals.button.auditReject"
                    defaultMessage="查看审计（拒绝）"
                  />
                </Button>
              )}
              <Button size="small" onClick={exportDetailJSON}>
                <FormattedMessage
                  id="pages.approvals.button.exportJson"
                  defaultMessage="导出 JSON"
                />
              </Button>
              <Button
                size="small"
                onClick={() => {
                  const blob = new Blob([preview || ''], { type: 'text/plain;charset=utf-8;' });
                  const url = URL.createObjectURL(blob);
                  const a = document.createElement('a');
                  a.href = url;
                  a.download = `approval_preview_${current.id || ''}.txt`;
                  a.click();
                  URL.revokeObjectURL(url);
                }}
              >
                <FormattedMessage
                  id="pages.approvals.button.exportPreview"
                  defaultMessage="导出预览文本"
                />
              </Button>
            </Space>
            <Descriptions size="small" column={1} bordered>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.approvals.desc.actor',
                  defaultMessage: '申请人',
                })}
              >
                {current.actor}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.approvals.desc.function',
                  defaultMessage: '函数',
                })}
              >
                {current.functionId}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.approvals.desc.gameEnv',
                  defaultMessage: '游戏/环境',
                })}
              >
                {current.gameId || ''}/{current.env || ''}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.approvals.desc.state',
                  defaultMessage: '状态',
                })}
              >
                <Tag color={stateTag(current.state)}>{stateText(current.state, intl)}</Tag>
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.approvals.desc.mode',
                  defaultMessage: '模式',
                })}
              >
                {current.mode}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.approvals.desc.createdAt',
                  defaultMessage: '创建时间',
                })}
              >
                {current.createdAt}
              </Descriptions.Item>
              {current.approver && (
                <Descriptions.Item
                  label={intl.formatMessage({
                    id: 'pages.approvals.desc.approver',
                    defaultMessage: '审批人',
                  })}
                >
                  <Space size={4}>
                    <span>
                      {current.approver}
                      {current.reviewedAt ? ` / ${current.reviewedAt}` : ''}
                    </span>
                    {current.reviewedByOther && (
                      <Tag color="green" style={{ marginInlineEnd: 0 }}>
                        <FormattedMessage
                          id="pages.approvals.tag.twoPersonReview"
                          defaultMessage="两人复核"
                        />
                      </Tag>
                    )}
                  </Space>
                </Descriptions.Item>
              )}
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.approvals.desc.idempotencyKey',
                  defaultMessage: '幂等键',
                })}
              >
                {current.idempotencyKey}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.approvals.desc.route',
                  defaultMessage: '路由',
                })}
              >
                {current.route}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.approvals.desc.targetService',
                  defaultMessage: '目标服务',
                })}
              >
                {current.targetServiceId}
              </Descriptions.Item>
              <Descriptions.Item label="Hash Key">{current.hashKey}</Descriptions.Item>
              {current.reason && (
                <Descriptions.Item
                  label={intl.formatMessage({
                    id: 'pages.approvals.desc.reason',
                    defaultMessage: '原因',
                  })}
                >
                  {current.reason}
                </Descriptions.Item>
              )}
            </Descriptions>
            <h4 style={{ marginTop: 16 }}>
              <FormattedMessage
                id="pages.approvals.drawer.payloadPreview"
                defaultMessage="载荷预览"
              />
            </h4>
            <pre
              style={{
                whiteSpace: 'pre-wrap',
                background: '#f6f6f6',
                padding: 8,
                border: '1px solid #eee',
              }}
            >
              {preview}
            </pre>
          </>
        )}
      </Drawer>
    </Card>
  );
}
