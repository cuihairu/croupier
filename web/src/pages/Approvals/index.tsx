import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Card, Table, Tag, Space, Button, Drawer, Descriptions, Select, Input, Tabs } from 'antd';
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
const stateText = (state: Approval['state']) =>
  state === 'pending' ? '待审批' : state === 'approved' ? '已通过' : '已拒绝';

export default function ApprovalsPage() {
  const [data, setData] = useState<Approval[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
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
  const descMap = useMemo(() => {
    const m: Record<string, FunctionDescriptor> = {};
    (descs || []).forEach((d: FunctionDescriptor) => {
      if (d?.id) m[d.id] = d;
    });
    return m;
  }, [descs]);

  const list = useCallback(async () => {
    setLoading(true);
    const qs = new URLSearchParams();
    // 我发起的：服务端强制按当前登录人过滤；其余视图按状态过滤
    if (viewMode === 'mine') qs.set('mine', 'true');
    if (state) qs.set('status', state);
    if (functionId) qs.set('functionId', functionId);
    if (gameId) qs.set('gameId', gameId);
    if (env) qs.set('env', env);
    if (viewMode !== 'mine' && actor) qs.set('actor', actor);
    if (riskFilter) qs.set('risk', riskFilter);
    qs.set('page', String(page));
    qs.set('pageSize', String(size));
    let json: Awaited<ReturnType<typeof listApprovals>>;
    try {
      json = await listApprovals(Object.fromEntries(qs));
    } catch (e) {
      const msg = e instanceof Error ? e.message : '加载失败';
      getMessage()?.error(msg);
      setLoading(false);
      return;
    }
    setData(json.approvals || []);
    setTotal(json.total || 0);
    setLoading(false);
  }, [viewMode, state, functionId, gameId, env, actor, riskFilter, page, size]);

  const filtered = useMemo(() => {
    const wantRisk = (riskFilter || '').trim().toLowerCase();
    if (!wantRisk) return data || [];
    return (data || []).filter((r) => {
      const d = descMap[r.functionId || ''];
      return (d?.risk || '').toString().toLowerCase() === wantRisk;
    });
  }, [data, riskFilter, descMap]);

  async function view(id: string) {
    let json: Awaited<ReturnType<typeof getApproval>>;
    try {
      json = await getApproval(id);
    } catch (e) {
      const msg = e instanceof Error ? e.message : '加载失败';
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
      const text = window.prompt(`高风险函数，请输入函数ID确认：${funcId}`) || '';
      if (funcId && text.trim() !== funcId) {
        getMessage()?.warning('确认文本不匹配');
        return;
      }
    }
    const otp = window.prompt('动态验证码（若未开启可留空）') || '';
    try {
      await approveApproval({ id, otp });
    } catch (e) {
      const msg = e instanceof Error ? e.message : '批准失败';
      getMessage()?.error(msg);
      return;
    }
    getMessage()?.success('已批准');
    await list();
    await view(id);
  }

  async function reject(id: string) {
    const reason = window.prompt('请输入拒绝原因') || '';
    try {
      await rejectApproval({ id, reason });
    } catch (e) {
      const msg = e instanceof Error ? e.message : '拒绝失败';
      getMessage()?.error(msg);
      return;
    }
    getMessage()?.success('已拒绝');
    await list();
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
    list();
  }, [list]);
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
    <Card title="审批中心">
      <Tabs
        activeKey={viewMode}
        onChange={(key) => {
          const next = key as ViewMode;
          setViewMode(next);
          setPage(1);
          // 待我审批聚焦 pending；我发起的/全部默认查全部状态
          setState(next === 'todo' ? 'pending' : '');
        }}
        items={[
          { key: 'todo', label: '待我审批' },
          { key: 'mine', label: '我发起的' },
          { key: 'all', label: '全部' },
        ]}
      />
      <Space style={{ marginBottom: 16 }} wrap>
        <span>状态:</span>
        <Select
          style={{ width: 160 }}
          value={state}
          onChange={setState}
          options={[
            { label: '全部', value: '' },
            { label: '待审批', value: 'pending' },
            { label: '已通过', value: 'approved' },
            { label: '已拒绝', value: 'rejected' },
          ]}
        />
        <Input
          placeholder="函数ID"
          value={functionId}
          onChange={(e) => setFunctionId(e.target.value)}
          style={{ width: 240 }}
        />
        <Input
          placeholder="游戏"
          value={gameId}
          onChange={(e) => setGameId(e.target.value)}
          style={{ width: 160 }}
        />
        <Input
          placeholder="环境"
          value={env}
          onChange={(e) => setEnv(e.target.value)}
          style={{ width: 120 }}
        />
        {viewMode !== 'mine' && (
          <Input
            placeholder="申请人"
            value={actor}
            onChange={(e) => setActor(e.target.value)}
            style={{ width: 160 }}
          />
        )}
        <Select
          placeholder="风险"
          style={{ width: 140 }}
          value={riskFilter}
          onChange={setRiskFilter}
          options={[
            { label: '全部', value: '' },
            { label: '高', value: 'high' },
            { label: '中', value: 'medium' },
            { label: '低', value: 'low' },
          ]}
        />
        <Button
          onClick={() => {
            setPage(1);
            list();
          }}
          type="primary"
        >
          查询
        </Button>
      </Space>
      <Table
        rowKey="id"
        loading={loading}
        dataSource={filtered}
        pagination={{
          current: page,
          pageSize: size,
          total,
          onChange: (p, ps) => {
            setPage(p);
            setSize(ps);
          },
        }}
        columns={[
          { title: '创建时间', dataIndex: 'createdAt' },
          { title: '申请人', dataIndex: 'actor' },
          {
            title: '函数',
            dataIndex: 'functionId',
            render: (v) => {
              const d = descMap[v];
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
                  {v}
                  {tags}
                </Space>
              );
            },
          },
          { title: '游戏/环境', render: (_, r) => `${r.gameId || ''}/${r.env || ''}` },
          {
            title: '状态',
            dataIndex: 'state',
            render: (v) => <Tag color={stateTag(v)}>{stateText(v)}</Tag>,
          },
          {
            title: '审批信息',
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
                      两人复核
                    </Tag>
                  )}
                </Space>
              );
            },
          },
          { title: '模式', dataIndex: 'mode' },
          {
            title: '操作',
            render: (_, r) => (
              <Space>
                <Button size="small" onClick={() => view(r.id)}>
                  查看
                </Button>
                {/* 我发起的视图只读：两人规则下申请人无权审批自己的申请 */}
                {viewMode !== 'mine' && r.state === 'pending' && (
                  <Button size="small" type="primary" onClick={() => approve(r.id)}>
                    通过
                  </Button>
                )}
                {viewMode !== 'mine' && r.state === 'pending' && (
                  <Button size="small" danger onClick={() => reject(r.id)}>
                    拒绝
                  </Button>
                )}
              </Space>
            ),
          },
        ]}
      />
      <Drawer
        title={`审批详情 ${current?.id || ''}`}
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
                  查看审计（申请人）
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
                  查看审计（批准）
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
                  查看审计（拒绝）
                </Button>
              )}
              <Button size="small" onClick={exportDetailJSON}>
                导出 JSON
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
                导出预览文本
              </Button>
            </Space>
            <Descriptions size="small" column={1} bordered>
              <Descriptions.Item label="申请人">{current.actor}</Descriptions.Item>
              <Descriptions.Item label="函数">{current.functionId}</Descriptions.Item>
              <Descriptions.Item label="游戏/环境">
                {current.gameId || ''}/{current.env || ''}
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={stateTag(current.state)}>{stateText(current.state)}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="模式">{current.mode}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{current.createdAt}</Descriptions.Item>
              {current.approver && (
                <Descriptions.Item label="审批人">
                  <Space size={4}>
                    <span>
                      {current.approver}
                      {current.reviewedAt ? ` / ${current.reviewedAt}` : ''}
                    </span>
                    {current.reviewedByOther && (
                      <Tag color="green" style={{ marginInlineEnd: 0 }}>
                        两人复核
                      </Tag>
                    )}
                  </Space>
                </Descriptions.Item>
              )}
              <Descriptions.Item label="幂等键">{current.idempotencyKey}</Descriptions.Item>
              <Descriptions.Item label="路由">{current.route}</Descriptions.Item>
              <Descriptions.Item label="目标服务">{current.targetServiceId}</Descriptions.Item>
              <Descriptions.Item label="Hash Key">{current.hashKey}</Descriptions.Item>
              {current.reason && (
                <Descriptions.Item label="原因">{current.reason}</Descriptions.Item>
              )}
            </Descriptions>
            <h4 style={{ marginTop: 16 }}>载荷预览</h4>
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
