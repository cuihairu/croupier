import React, { useCallback, useEffect, useState } from 'react';
import { Alert, Button, Space, Table, Tag, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import {
  getExecutionLog,
  listExecutionLogs,
  type ExecutionLogItem,
} from '@/services/api/executionLogs';

const { Text } = Typography;

const PAGE_SIZE = 10;

/** 服务端调用记录（R5）：我的执行留痕（保留期默认 7 天，脱敏+截断）。 */
export default function ServerHistoryPanel() {
  const [items, setItems] = useState<ExecutionLogItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string>('');
  const [detail, setDetail] = useState<null | {
    id: number;
    loading: boolean;
    request?: unknown;
    response?: unknown;
  }>(null);

  const list = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const json = await listExecutionLogs({ mine: true, page, pageSize: PAGE_SIZE });
      setItems(json.items || []);
      setTotal(json.total || 0);
    } catch (e) {
      setItems([]);
      setTotal(0);
      setLoadError(e instanceof Error ? e.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    void list();
  }, [list]);

  const viewDetail = async (id: number) => {
    setDetail({ id, loading: true });
    try {
      const json = await getExecutionLog(id);
      setDetail({ id, loading: false, request: json.requestPayload, response: json.responseBody });
    } catch {
      setDetail({ id, loading: false });
    }
  };

  return (
    <Space direction="vertical" size={8} style={{ width: '100%' }}>
      {loadError ? (
        <Alert
          type="error"
          showIcon
          message="服务端记录加载失败"
          description={
            <Space direction="vertical" size={4}>
              <Text type="secondary">{loadError}</Text>
              <Button size="small" icon={<ReloadOutlined />} onClick={() => void list()}>
                重试
              </Button>
            </Space>
          }
        />
      ) : null}
      <Table
        rowKey="id"
        size="small"
        loading={loading}
        dataSource={items}
        pagination={{
          current: page,
          pageSize: PAGE_SIZE,
          total,
          onChange: setPage,
          showSizeChanger: false,
        }}
        expandable={{
          expandedRowRender: (record) => (
            <div>
              {detail?.id === record.id ? (
                detail.loading ? (
                  <Text type="secondary">加载中…</Text>
                ) : (
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    <div>
                      <Text type="secondary">请求：</Text>
                      <pre style={preStyle}>
                        {detail.request ? JSON.stringify(detail.request, null, 2) : '（无）'}
                      </pre>
                    </div>
                    <div>
                      <Text type="secondary">响应：</Text>
                      <pre style={preStyle}>
                        {detail.response ? JSON.stringify(detail.response, null, 2) : '（无）'}
                      </pre>
                    </div>
                  </Space>
                )
              ) : (
                <Button size="small" onClick={() => void viewDetail(record.id)}>
                  查看载荷
                </Button>
              )}
            </div>
          ),
        }}
        columns={[
          {
            title: '时间',
            dataIndex: 'createdAt',
            width: 150,
            render: (v: string) => new Date(v).toLocaleString(),
          },
          { title: '函数', dataIndex: 'functionId', ellipsis: true },
          {
            title: '来源',
            dataIndex: 'source',
            width: 70,
            render: (v: string) => (v === 'page' ? <Tag>页面</Tag> : <Tag>调用</Tag>),
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 70,
            render: (v: string) => (
              <Tag color={v === 'ok' ? 'green' : 'red'} style={{ marginInlineEnd: 0 }}>
                {v === 'ok' ? '成功' : '失败'}
              </Tag>
            ),
          },
          { title: '耗时(ms)', dataIndex: 'durationMs', width: 90 },
        ]}
      />
      <Text type="secondary" style={{ fontSize: 12 }}>
        仅显示本人记录；载荷已脱敏，保留期默认 7 天。
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
