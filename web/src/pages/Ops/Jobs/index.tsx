import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Card,
  Descriptions,
  Drawer,
  Input,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import { listOpsJobs, type OpsJob, listOpsFunctions } from '@/services/api/ops';
import { cancelJob, fetchJobResult, openJobEventSource } from '@/services/api/functions';
import { StandardFilterBar, StandardListSection, SummaryOverview } from '@/components';
const { Paragraph, Text } = Typography;

function getJobStatusMeta(state?: string) {
  if (state === 'running') return { color: 'blue', text: '运行中' };
  if (state === 'succeeded') return { color: 'green', text: '已成功' };
  if (state === 'failed') return { color: 'red', text: '已失败' };
  if (state === 'canceled') return { color: 'default', text: '已取消' };
  return { color: 'default', text: state || '-' };
}

export default function OpsJobsPage() {
  const { message } = App.useApp();
  const [rows, setRows] = useState<OpsJob[]>([]);
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<string>('');
  const [fid, setFid] = useState<string>('');
  const [actor, setActor] = useState<string>('');
  const [funcs, setFuncs] = useState<string[]>([]);
  const [detail, setDetail] = useState<OpsJob | null>(null);
  const [stream, setStream] = useState<string[]>([]);
  const [result, setResult] = useState<{ state?: string; payload?: any; error?: string } | null>(
    null,
  );
  const esRef = useRef<EventSource | null>(null);

  const load = async () => {
    setLoading(true);
    try {
      const params: Record<string, string> = {};
      if (status) params.status = status;
      if (fid) params.functionId = fid;
      const actorValue = actor.trim();
      if (actorValue) params.actor = actorValue;
      const r = await listOpsJobs(params);
      setRows(r.jobs || []);
    } catch (e: any) {
      message.error(e?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    load();
  }, [status, fid, actor]);
  useEffect(() => {
    (async () => {
      try {
        const s = await listOpsFunctions();
        setFuncs((s.functions || []).map((x) => x.id));
      } catch {}
    })();
  }, []);

  const summary = useMemo(() => {
    const total = rows.length;
    const runningCount = rows.filter((item) => item.state === 'running').length;
    const succeededCount = rows.filter((item) => item.state === 'succeeded').length;
    const failedCount = rows.filter((item) => item.state === 'failed').length;
    const functionCount = new Set(rows.map((item) => item.functionId).filter(Boolean)).size;
    return { total, runningCount, succeededCount, failedCount, functionCount };
  }, [rows]);

  const resultRows = useMemo(() => {
    return rows.filter((item) => {
      if (status && item.state !== status) return false;
      if (fid && item.functionId !== fid) return false;
      const actorValue = actor.trim().toLowerCase();
      if (actorValue && !(item.actor || '').toLowerCase().includes(actorValue)) return false;
      return true;
    });
  }, [actor, fid, rows, status]);

  const hasFilters = Boolean(status || fid || actor.trim());
  const filterSummary = [
    status ? `状态 ${getJobStatusMeta(status).text}` : null,
    fid ? `函数 ${fid}` : null,
    actor.trim() ? `操作者 ${actor.trim()}` : null,
  ]
    .filter(Boolean)
    .join(' / ');

  // Auto-connect SSE for running job when opening detail
  useEffect(() => {
    if (!detail) return;
    setResult(null);
    setStream([]);
    if (detail.state !== 'running') return;
    try {
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }
    } catch {}
    const id = detail.id;
    const es = openJobEventSource(id);
    const push = (type: string, data: any) => {
      setStream((prev) =>
        [...prev, `${type}: ${typeof data === 'string' ? data : JSON.stringify(data)}`].slice(-200),
      );
    };
    const handle = (ev: MessageEvent) => {
      // generic handler for default messages (if any)
      push('message', ev.data);
    };
    es.onmessage = handle;
    // listen typed events
    ['stdout', 'stderr', 'progress', 'log'].forEach((t) =>
      es.addEventListener(t, (ev: MessageEvent) => push(t, (ev as any).data)),
    );
    es.addEventListener('error', (ev: MessageEvent) => push('error', (ev as any).data));
    es.addEventListener('done', async (_ev: MessageEvent) => {
      // close stream, fetch final result and refresh list
      try {
        es.close();
      } catch {}
      esRef.current = null;
      try {
        const r = await fetchJobResult(id);
        setResult(r as any);
      } catch {}
      load();
    });
    esRef.current = es;
    return () => {
      try {
        es.close();
      } catch {}
      esRef.current = null;
    };
  }, [detail]);

  const handleCancelJob = async (job: OpsJob) => {
    try {
      await cancelJob(job.id);
      message.success('已取消');
      if (detail?.id === job.id) {
        setDetail({ ...job, state: 'canceled' });
      }
      load();
    } catch (e: any) {
      message.error(e?.message || '取消失败');
    }
  };

  const columns: ColumnsType<OpsJob> = [
    { title: 'JobID', dataIndex: 'id', width: 220 },
    { title: '函数', dataIndex: 'functionId', width: 220 },
    {
      title: '状态',
      dataIndex: 'state',
      width: 120,
      render: (v) => {
        const meta = getJobStatusMeta(v);
        return <Tag color={meta.color}>{meta.text}</Tag>;
      },
    },
    { title: '操作者', dataIndex: 'actor', width: 140 },
    {
      title: '游戏/环境',
      key: 'ge',
      width: 160,
      render: (_: any, r: any) => `${r.gameId || ''}/${r.env || ''}`,
    },
    {
      title: '耗时',
      dataIndex: 'durationMs',
      width: 120,
      render: (v: any) => (typeof v === 'number' && v > 0 ? `${(v / 1000).toFixed(2)}s` : '-'),
    },
    {
      title: '开始时间',
      dataIndex: 'startedAt',
      width: 180,
      render: (v: any) => (v ? new Date(v).toLocaleString() : '-'),
    },
    {
      title: '结束时间',
      dataIndex: 'endedAt',
      width: 180,
      render: (v: any) => (v ? new Date(v).toLocaleString() : '-'),
    },
    { title: '服务地址', dataIndex: 'rpcAddr', ellipsis: true },
    {
      title: '操作',
      width: 160,
      render: (_: any, r) => (
        <Space>
          <Button size="small" type="primary" ghost onClick={() => setDetail(r)}>
            查看详情
          </Button>
          {r.state === 'running' && (
            <Button size="small" danger onClick={() => handleCancelJob(r)}>
              取消
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer title="任务监控" subTitle="先缩小任务范围，再查看执行细节、事件流和最终结果">
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <SummaryOverview
          title="任务概览"
          description="这个页面优先服务排查和追踪，不把所有信息一次性堆进表格。先按状态、函数或操作者收敛范围，再进详情查看。"
          items={[
            { color: '#1677ff', text: `任务 ${summary.total}` },
            { color: '#2f54eb', text: `运行中 ${summary.runningCount}` },
            { color: '#52c41a', text: `成功 ${summary.succeededCount}` },
            { color: '#ff4d4f', text: `失败 ${summary.failedCount}` },
            { color: '#722ed1', text: `函数 ${summary.functionCount}` },
          ]}
          hint="推荐路径：先筛选任务，再打开详情查看 SSE 事件流和结果，不必在主表里同时处理所有上下文。"
        />

        <StandardListSection title="任务列表" extra={<Button onClick={load}>刷新</Button>}>
          <StandardFilterBar
            resultText={`当前结果 ${resultRows.length} 个任务`}
            controls={
              <>
                <Select
                  placeholder="状态"
                  allowClear
                  style={{ width: 140 }}
                  value={status || undefined}
                  onChange={(v) => setStatus(v || '')}
                  options={[
                    { label: '运行中', value: 'running' },
                    { label: '已成功', value: 'succeeded' },
                    { label: '已失败', value: 'failed' },
                    { label: '已取消', value: 'canceled' },
                  ]}
                />
                <Select
                  showSearch
                  placeholder="函数"
                  allowClear
                  style={{ width: 240 }}
                  value={fid || undefined}
                  onChange={(v) => setFid(v || '')}
                  options={funcs.map((id) => ({ label: id, value: id }))}
                />
                <Input
                  allowClear
                  placeholder="按操作者过滤"
                  value={actor}
                  onChange={(e) => setActor(e.target.value)}
                  style={{ width: 180 }}
                />
                {hasFilters && (
                  <Button
                    onClick={() => {
                      setStatus('');
                      setFid('');
                      setActor('');
                    }}
                  >
                    清空筛选
                  </Button>
                )}
              </>
            }
          />
          {hasFilters ? (
            <Alert
              style={{ marginBottom: 12 }}
              type="info"
              showIcon
              message="当前正在查看筛选后的任务范围"
              description={`已生效条件：${filterSummary}`}
            />
          ) : null}
          <Table
            rowKey={(r) => r.id}
            loading={loading}
            dataSource={resultRows}
            columns={columns}
            pagination={{ pageSize: 10 }}
            onRow={(rec) => ({ onDoubleClick: () => setDetail(rec) })}
            scroll={{ x: 1280 }}
            locale={{
              emptyText: hasFilters
                ? '当前筛选条件下没有匹配任务，请调整筛选后重试。'
                : '暂时没有任务数据，先触发任务后再回来查看。',
            }}
          />
        </StandardListSection>
      </Space>

      <Drawer
        title="作业详情"
        width={720}
        open={!!detail}
        onClose={() => {
          setDetail(null);
          setStream([]);
          setResult(null);
          if (esRef.current) {
            esRef.current.close();
            esRef.current = null;
          }
        }}
        extra={
          <Space>
            {detail?.state === 'running' && (
              <Button
                danger
                onClick={async () => {
                  if (!detail) return;
                  await handleCancelJob(detail);
                }}
              >
                取消
              </Button>
            )}
            <Button
              onClick={async () => {
                if (!detail) return;
                try {
                  const r = await fetchJobResult(detail.id);
                  setResult(r as any);
                  message.success(`状态：${r?.state}`);
                  load();
                } catch (e: any) {
                  message.error(e?.message || '查询失败');
                }
              }}
            >
              刷新结果
            </Button>
          </Space>
        }
      >
        {detail && (
          <Space direction="vertical" style={{ width: '100%' }}>
            <SummaryOverview
              title="任务状态"
              description="先判断任务当前处于运行、成功还是失败，再决定要不要刷新结果或查看事件流。"
              items={[
                {
                  color: getJobStatusMeta(detail.state).color,
                  text: getJobStatusMeta(detail.state).text,
                },
                { color: '#1677ff', text: detail.functionId || '-' },
                { color: '#722ed1', text: detail.actor || '未知操作者' },
                {
                  color: '#13c2c2',
                  text:
                    typeof detail.durationMs === 'number' && detail.durationMs > 0
                      ? `耗时 ${(detail.durationMs / 1000).toFixed(2)}s`
                      : '耗时未知',
                },
              ]}
              hint={
                detail.state === 'running'
                  ? '任务仍在运行，优先观察事件流；只有确认需要中止时再取消。'
                  : detail.state === 'failed'
                  ? '任务已经失败，建议先看错误信息，再核对结果和事件流。'
                  : '任务已结束，可以直接查看结果和事件流。'
              }
              hintType={detail.state === 'failed' ? 'warning' : 'info'}
            />

            <Descriptions bordered size="small" column={1}>
              <Descriptions.Item label="JobID">
                <Text code copyable>
                  {detail.id}
                </Text>
              </Descriptions.Item>
              <Descriptions.Item label="函数">{detail.functionId}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={getJobStatusMeta(detail.state).color}>
                  {getJobStatusMeta(detail.state).text}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="操作者">{detail.actor || '-'}</Descriptions.Item>
              <Descriptions.Item label="游戏/环境">
                {(detail.gameId || '') + '/' + (detail.env || '')}
              </Descriptions.Item>
              <Descriptions.Item label="服务地址">{detail.rpcAddr || '-'}</Descriptions.Item>
              <Descriptions.Item label="Trace">{detail.traceId || '-'}</Descriptions.Item>
              <Descriptions.Item label="开始时间">
                {detail.startedAt ? new Date(detail.startedAt).toLocaleString() : '-'}
              </Descriptions.Item>
              <Descriptions.Item label="结束时间">
                {detail.endedAt ? new Date(detail.endedAt).toLocaleString() : '-'}
              </Descriptions.Item>
              <Descriptions.Item label="耗时">
                {typeof detail.durationMs === 'number' && detail.durationMs > 0
                  ? `${(detail.durationMs / 1000).toFixed(2)}s`
                  : '-'}
              </Descriptions.Item>
            </Descriptions>
            {detail.error && (
              <Paragraph copyable={{ text: detail.error }} style={{ color: '#ff4d4f' }}>
                错误：{detail.error}
              </Paragraph>
            )}
            <Card size="small" title={`结果${result?.state ? `（${result.state}）` : ''}`}>
              {result ? (
                <Space direction="vertical" size={10} style={{ width: '100%' }}>
                  {result.error && (
                    <Paragraph copyable={{ text: result.error }} style={{ color: '#ff4d4f' }}>
                      错误：{result.error}
                    </Paragraph>
                  )}
                  {result.payload ? (
                    <Paragraph copyable={{ text: JSON.stringify(result.payload) }}>
                      <pre style={{ whiteSpace: 'pre-wrap' }}>
                        {JSON.stringify(result.payload, null, 2)}
                      </pre>
                    </Paragraph>
                  ) : (
                    <Alert
                      type="info"
                      showIcon
                      message="当前还没有结果数据"
                      description="运行中的任务通常要先看事件流；已结束但没有结果时，可以手动刷新结果后再确认。"
                    />
                  )}
                </Space>
              ) : (
                <Alert
                  type="info"
                  showIcon
                  message="结果尚未加载"
                  description="建议先点击“刷新结果”或等待任务完成后自动拉取。"
                />
              )}
            </Card>
            <Card
              size="small"
              title="事件流（SSE）"
              extra={
                <Space>
                  <Button
                    size="small"
                    onClick={() => {
                      if (!detail) return;
                      try {
                        if (esRef.current) {
                          esRef.current.close();
                          esRef.current = null;
                        }
                      } catch {}
                      const es = openJobEventSource(detail.id);
                      const push = (type: string, data: any) =>
                        setStream((prev) =>
                          [
                            ...prev,
                            `${type}: ${typeof data === 'string' ? data : JSON.stringify(data)}`,
                          ].slice(-200),
                        );
                      es.onmessage = (ev) => push('message', ev.data);
                      ['stdout', 'stderr', 'progress', 'log'].forEach((t) =>
                        es.addEventListener(t, (ev: MessageEvent) => push(t, (ev as any).data)),
                      );
                      es.addEventListener('error', (ev: MessageEvent) =>
                        push('error', (ev as any).data),
                      );
                      es.addEventListener('done', async () => {
                        try {
                          es.close();
                        } catch {}
                        esRef.current = null;
                        try {
                          const r = await fetchJobResult(detail.id);
                          setResult(r as any);
                        } catch {}
                        load();
                      });
                      es.onerror = () => {
                        try {
                          es.close();
                        } catch {}
                      };
                      esRef.current = es;
                    }}
                  >
                    连接
                  </Button>
                  <Button
                    size="small"
                    onClick={() => {
                      try {
                        if (esRef.current) esRef.current.close();
                      } catch {}
                      esRef.current = null;
                    }}
                  >
                    断开
                  </Button>
                </Space>
              }
            >
              <div
                style={{
                  maxHeight: 200,
                  overflow: 'auto',
                  fontFamily: 'monospace',
                  fontSize: 12,
                  background: '#fafafa',
                  padding: 8,
                  border: '1px solid #f0f0f0',
                }}
              >
                {stream.length > 0 ? (
                  stream.map((ln, i) => <div key={i}>{ln}</div>)
                ) : (
                  <Typography.Text type="secondary">
                    还没有事件流输出。运行中的任务可以先连接 SSE；已结束任务可能不会再产生新事件。
                  </Typography.Text>
                )}
              </div>
            </Card>
          </Space>
        )}
      </Drawer>
    </PageContainer>
  );
}
