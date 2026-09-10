import { useCallback, useEffect, useState } from 'react';
import { Alert, Button, Drawer, Input, Select, Space, Table, Typography } from 'antd';
import { SummaryOverview } from '@/components';
import {
  listExtensionEvents,
  type ExtensionEventItem,
  type ExtensionInstallationItem,
} from '@/services/api/extensions';
import { adaptEventListResponse } from '@/services/adapters/extensions';
import { formatUnix } from './shared';

/** 扩展事件抽屉：关键词/级别筛选 + 分页事件表（筛选与拉取自包含）。
 * 打开同一安装时重置筛选；关闭态不拉取（对齐原 loadEvents guard）。 */
export default function EventsDrawer({
  open,
  installation,
  onClose,
}: {
  open: boolean;
  installation: ExtensionInstallationItem | null;
  onClose: () => void;
}) {
  const [loading, setLoading] = useState(false);
  const [events, setEvents] = useState<ExtensionEventItem[]>([]);
  const [total, setTotal] = useState(0);
  const [keyword, setKeyword] = useState('');
  const [level, setLevel] = useState<string | undefined>(undefined);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

  const title = installation
    ? `${installation.displayName || installation.extensionId} (#${installation.id})`
    : '';

  // 打开（或切换安装实例）时重置筛选
  useEffect(() => {
    if (open && installation) {
      setKeyword('');
      setLevel(undefined);
      setPage(1);
    }
  }, [open, installation]);

  const loadEvents = useCallback(async () => {
    if (!open || !installation) return;
    setLoading(true);
    try {
      const resp = await listExtensionEvents(installation.id, {
        level,
        keyword: keyword.trim() || undefined,
        page,
        pageSize,
      });
      const vm = adaptEventListResponse(resp);
      setEvents(vm.items);
      setTotal(vm.total);
    } finally {
      setLoading(false);
    }
  }, [open, installation, level, keyword, page, pageSize]);

  useEffect(() => {
    loadEvents();
  }, [loadEvents]);

  return (
    <Drawer open={open} onClose={onClose} width={760} title={`扩展事件: ${title}`}>
      <SummaryOverview
        title="事件筛选"
        description="事件列表主要用于排查安装变更、报错和操作者动作。先按关键词或级别缩小范围，再逐条查看。"
        items={[
          { color: '#1677ff', text: `事件 ${total}` },
          { color: '#722ed1', text: level ? `级别 ${level}` : '全部级别' },
          {
            color: '#13c2c2',
            text: keyword.trim() ? `关键词 ${keyword.trim()}` : '未设置关键词',
          },
        ]}
        hint="推荐路径：先看最近报错和升级事件，再结合安装详情判断是否需要修改配置。"
      />
      <Space style={{ marginBottom: 12 }} wrap>
        <Input
          allowClear
          style={{ width: 260 }}
          placeholder="筛选事件/内容/操作者"
          value={keyword}
          onChange={(e) => {
            setPage(1);
            setKeyword(e.target.value);
          }}
        />
        <Select
          allowClear
          style={{ width: 140 }}
          placeholder="级别"
          value={level}
          onChange={(v) => {
            setPage(1);
            setLevel(v);
          }}
          options={[
            { label: 'info', value: 'info' },
            { label: 'warn', value: 'warn' },
            { label: 'error', value: 'error' },
          ]}
        />
        <Button
          disabled={!keyword.trim() && !level}
          onClick={() => {
            setKeyword('');
            setLevel(undefined);
            setPage(1);
          }}
        >
          清空筛选
        </Button>
      </Space>
      {keyword.trim() || level ? (
        <Alert
          style={{ marginBottom: 12 }}
          type="info"
          showIcon
          message="当前正在查看筛选后的事件范围"
          description={`已生效条件：${[
            keyword.trim() ? `关键词 ${keyword.trim()}` : null,
            level ? `级别 ${level}` : null,
          ]
            .filter(Boolean)
            .join(' / ')}`}
        />
      ) : null}

      <Table<ExtensionEventItem>
        rowKey={(row, idx) => `${row.createdAt}-${row.eventType}-${idx}`}
        loading={loading}
        dataSource={events}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          onChange: (nextPage, nextPageSize) => {
            setPage(nextPage);
            setPageSize(nextPageSize);
          },
        }}
        columns={[
          {
            title: '时间',
            dataIndex: 'createdAt',
            key: 'createdAt',
            render: (v) => formatUnix(v),
          },
          { title: '级别', dataIndex: 'level', key: 'level', width: 100 },
          { title: '事件', dataIndex: 'eventType', key: 'eventType', width: 150 },
          { title: '内容', dataIndex: 'message', key: 'message' },
          {
            title: 'Payload',
            dataIndex: 'payload',
            key: 'payload',
            render: (value: string) =>
              value ? (
                <Typography.Text code ellipsis={{ tooltip: value }} style={{ maxWidth: 260 }}>
                  {value}
                </Typography.Text>
              ) : (
                '-'
              ),
          },
          { title: '操作者', dataIndex: 'createdBy', key: 'createdBy', width: 120 },
        ]}
        locale={{
          emptyText:
            keyword.trim() || level
              ? '当前筛选条件下没有匹配事件，请调整筛选后重试。'
              : '暂时没有事件数据，后续有安装动作后会显示在这里。',
        }}
      />
    </Drawer>
  );
}
