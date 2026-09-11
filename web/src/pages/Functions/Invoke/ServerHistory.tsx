import React, { useCallback, useRef, useState } from 'react';
import { Alert, Button, Space, Tag, Typography } from 'antd';
import { ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components';
import { ReloadOutlined } from '@ant-design/icons';
import {
  getExecutionLog,
  listExecutionLogs,
  type ExecutionLogItem,
} from '@/services/api/executionLogs';
import { FormattedMessage, useIntl } from '@umijs/max';
import { formatDateTime } from '@/utils/format';

const { Text } = Typography;

const PAGE_SIZE = 10;

type LoadedDetail = { loading?: boolean; request?: unknown; response?: unknown };

/** 服务端调用记录（R5）：我的执行留痕（保留期默认 7 天，脱敏+截断）。
 * functionId 传入时默认只看当前函数（可切换），并提供运维全量审计入口。
 * 展开行即自动加载参数（缓存），可点刷新重新拉取。 */
export default function ServerHistoryPanel({ functionId }: { functionId?: string }) {
  const intl = useIntl();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [loadError, setLoadError] = useState<string>('');
  const [onlyCurrent, setOnlyCurrent] = useState<boolean>(!!functionId);
  const [details, setDetails] = useState<Record<number, LoadedDetail>>({});

  const columns: ProColumns<ExecutionLogItem>[] = [
    {
      title: intl.formatMessage({
        id: 'pages.functionsInvoke.serverHistory.column.time',
        defaultMessage: '时间',
      }),
      dataIndex: 'createdAt',
      width: 150,
      render: (_, r) => formatDateTime(r.createdAt),
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsInvoke.serverHistory.column.function',
        defaultMessage: '函数',
      }),
      dataIndex: 'functionId',
      ellipsis: true,
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsInvoke.serverHistory.column.source',
        defaultMessage: '来源',
      }),
      dataIndex: 'source',
      width: 70,
      render: (_, r) =>
        r.source === 'page' ? (
          <Tag>
            <FormattedMessage
              id="pages.functionsInvoke.serverHistory.source.page"
              defaultMessage="页面"
            />
          </Tag>
        ) : (
          <Tag>
            <FormattedMessage
              id="pages.functionsInvoke.serverHistory.source.invoke"
              defaultMessage="调用"
            />
          </Tag>
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsInvoke.serverHistory.column.status',
        defaultMessage: '状态',
      }),
      dataIndex: 'status',
      width: 70,
      render: (_, r) => (
        <Tag color={r.status === 'ok' ? 'green' : 'red'} style={{ marginInlineEnd: 0 }}>
          {r.status === 'ok' ? (
            <FormattedMessage
              id="pages.functionsInvoke.serverHistory.status.success"
              defaultMessage="成功"
            />
          ) : (
            <FormattedMessage
              id="pages.functionsInvoke.serverHistory.status.failed"
              defaultMessage="失败"
            />
          )}
        </Tag>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsInvoke.serverHistory.column.duration',
        defaultMessage: '耗时(ms)',
      }),
      dataIndex: 'durationMs',
      width: 90,
    },
  ];

  const loadDetail = useCallback(async (id: number) => {
    setDetails((prev) => ({ ...prev, [id]: { loading: true } }));
    try {
      const json = await getExecutionLog(id);
      setDetails((prev) => ({
        ...prev,
        [id]: { request: json.requestPayload, response: json.responseBody },
      }));
    } catch {
      setDetails((prev) => ({ ...prev, [id]: {} }));
    }
  }, []);

  const renderDetail = (id: number) => {
    const d = details[id];
    if (d === undefined || d.loading) {
      return (
        <Text type="secondary">
          <FormattedMessage
            id="pages.functionsInvoke.serverHistory.detail.loading"
            defaultMessage="载荷加载中…"
          />
        </Text>
      );
    }
    return (
      <Space direction="vertical" size={8} style={{ width: '100%' }}>
        <div>
          <Space size={8}>
            <Text type="secondary">
              <FormattedMessage
                id="pages.functionsInvoke.serverHistory.detail.requestLabel"
                defaultMessage="请求（已脱敏）："
              />
            </Text>
            <Button size="small" icon={<ReloadOutlined />} onClick={() => void loadDetail(id)}>
              <FormattedMessage
                id="pages.functionsInvoke.serverHistory.detail.refresh"
                defaultMessage="重新拉取"
              />
            </Button>
          </Space>
          <pre style={preStyle}>
            {d.request
              ? JSON.stringify(d.request, null, 2)
              : intl.formatMessage({
                  id: 'pages.functionsInvoke.serverHistory.detail.emptyPayload',
                  defaultMessage: '（无）',
                })}
          </pre>
        </div>
        <div>
          <Text type="secondary">
            <FormattedMessage
              id="pages.functionsInvoke.serverHistory.detail.responseLabel"
              defaultMessage="响应（已脱敏）："
            />
          </Text>
          <pre style={preStyle}>
            {d.response
              ? JSON.stringify(d.response, null, 2)
              : intl.formatMessage({
                  id: 'pages.functionsInvoke.serverHistory.detail.emptyPayload',
                  defaultMessage: '（无）',
                })}
          </pre>
        </div>
      </Space>
    );
  };

  return (
    <Space orientation="vertical" size={8} style={{ width: '100%' }}>
      <Space wrap style={{ justifyContent: 'space-between', width: '100%' }}>
        <label style={{ fontSize: 12 }}>
          <input
            type="checkbox"
            checked={onlyCurrent}
            onChange={(e) => {
              setOnlyCurrent(e.target.checked);
              // 筛选变化回第 1 页：params 变化与 setPageInfo 的双触发由
              // ProTable 内部 debounce + abort 合并，不会出现错序数据
              actionRef.current?.setPageInfo?.({ current: 1 });
            }}
          />{' '}
          {functionId
            ? intl.formatMessage(
                {
                  id: 'pages.functionsInvoke.serverHistory.onlyCurrentWithFunction',
                  defaultMessage: `仅看当前函数（${functionId}）`,
                },
                { functionId },
              )
            : intl.formatMessage({
                id: 'pages.functionsInvoke.serverHistory.onlyCurrent',
                defaultMessage: '仅看当前函数',
              })}
        </label>
        <a
          href="/functions/execution-logs"
          target="_blank"
          rel="noreferrer"
          style={{ fontSize: 12 }}
        >
          <FormattedMessage
            id="pages.functionsInvoke.serverHistory.link.viewAll"
            defaultMessage="查看全部执行留痕 →"
          />
        </a>
      </Space>
      {loadError ? (
        <Alert
          type="error"
          showIcon
          message={intl.formatMessage({
            id: 'pages.functionsInvoke.serverHistory.error.loadFailed',
            defaultMessage: '服务端记录加载失败',
          })}
          description={
            <Space orientation="vertical" size={4}>
              <Text type="secondary">{loadError}</Text>
              <Button
                size="small"
                icon={<ReloadOutlined />}
                onClick={() => actionRef.current?.reload()}
              >
                <FormattedMessage
                  id="pages.functionsInvoke.serverHistory.retry"
                  defaultMessage="重试"
                />
              </Button>
            </Space>
          }
        />
      ) : null}
      <ProTable<ExecutionLogItem>
        actionRef={actionRef}
        rowKey="id"
        size="small"
        columns={columns}
        search={false}
        options={false}
        toolBarRender={false}
        params={{ functionId, onlyCurrent }}
        request={async ({
          current = 1,
          pageSize = PAGE_SIZE,
          functionId: fnId,
          onlyCurrent: onlyFn,
        }) => {
          setLoadError('');
          const params: Record<string, string | number | boolean> = {
            mine: true,
            page: current,
            pageSize,
          };
          if (onlyFn && fnId) params.functionId = fnId;
          try {
            const json = await listExecutionLogs(params);
            return { data: json.items || [], total: json.total || 0, success: true };
          } catch (e) {
            setLoadError(
              e instanceof Error
                ? e.message
                : intl.formatMessage({
                    id: 'pages.functionsInvoke.serverHistory.error.fallback',
                    defaultMessage: '加载失败',
                  }),
            );
            return { data: [], total: 0, success: false };
          }
        }}
        pagination={{ pageSize: PAGE_SIZE, showSizeChanger: true }}
        expandable={{
          onExpand: (expanded, record) => {
            if (expanded && details[record.id] === undefined) {
              void loadDetail(record.id);
            }
          },
          expandedRowRender: (record) => renderDetail(record.id),
        }}
      />
      <Text type="secondary" style={{ fontSize: 12 }}>
        <FormattedMessage
          id="pages.functionsInvoke.serverHistory.scopeNote"
          defaultMessage="仅显示本人记录；点击行首箭头展开查看参数；保留期默认 7 天。"
        />
      </Text>
    </Space>
  );
}

const preStyle: React.CSSProperties = {
  whiteSpace: 'pre-wrap',
  background: '#f6f6f6',
  padding: 8,
  borderRadius: 6,
  maxHeight: 220,
  overflow: 'auto',
  fontSize: 12,
};
