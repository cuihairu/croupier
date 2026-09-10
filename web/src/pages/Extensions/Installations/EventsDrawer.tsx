import { useEffect, useRef, useState } from 'react';
import { Alert, Button, Drawer, Input, Select, Space, Typography } from 'antd';
import { ProTable, type ActionType } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { SummaryOverview } from '@/components';
import {
  listExtensionEvents,
  type ExtensionEventItem,
  type ExtensionInstallationItem,
} from '@/services/api/extensions';
import { adaptEventListResponse } from '@/services/adapters/extensions';
import { formatUnix } from './shared';

/** 扩展事件抽屉：关键词/级别筛选 + 分页事件表（筛选与拉取自包含）。
 * 打开同一安装时重置筛选；关闭态不拉取（抽屉内容未挂载时不发起请求）。 */
export default function EventsDrawer({
  open,
  installation,
  onClose,
}: {
  open: boolean;
  installation: ExtensionInstallationItem | null;
  onClose: () => void;
}) {
  // 事件总数副本：抽屉顶部概览依赖它，在 request 成功后同步
  const intl = useIntl();
  const [total, setTotal] = useState(0);
  const [keyword, setKeyword] = useState('');
  const [level, setLevel] = useState<string | undefined>(undefined);
  const actionRef = useRef<ActionType | undefined>(undefined);

  const title = installation
    ? `${installation.displayName || installation.extensionId} (#${installation.id})`
    : '';

  // 关键词/级别筛选的展示文案：概览 chips 与「已生效条件」拼接共用
  const keywordText = keyword.trim()
    ? intl.formatMessage(
        {
          id: 'pages.extensionsInstallations.events.overview.keyword',
          defaultMessage: `关键词 ${keyword.trim()}`,
        },
        { value: keyword.trim() },
      )
    : intl.formatMessage({
        id: 'pages.extensionsInstallations.events.overview.keywordUnset',
        defaultMessage: '未设置关键词',
      });
  const levelText = level
    ? intl.formatMessage(
        {
          id: 'pages.extensionsInstallations.events.overview.level',
          defaultMessage: `级别 ${level}`,
        },
        { value: level },
      )
    : intl.formatMessage({
        id: 'pages.extensionsInstallations.events.overview.levelAll',
        defaultMessage: '全部级别',
      });
  const activeConditions = [keyword.trim() ? keywordText : null, level ? levelText : null]
    .filter(Boolean)
    .join(' / ');

  // 打开（或切换安装实例）时重置筛选并重新拉取（对齐原 reset + load 行为；
  // 首次打开时表格随抽屉挂载自动请求，reload 的双触发由内部 abort 合并）
  useEffect(() => {
    if (open && installation) {
      setKeyword('');
      setLevel(undefined);
      actionRef.current?.reload();
    }
  }, [open, installation]);

  return (
    <Drawer
      open={open}
      onClose={onClose}
      width={760}
      title={intl.formatMessage(
        {
          id: 'pages.extensionsInstallations.events.drawer.title',
          defaultMessage: `扩展事件: ${title}`,
        },
        { title },
      )}
    >
      <SummaryOverview
        title={intl.formatMessage({
          id: 'pages.extensionsInstallations.events.overview.title',
          defaultMessage: '事件筛选',
        })}
        description={intl.formatMessage({
          id: 'pages.extensionsInstallations.events.overview.description',
          defaultMessage:
            '事件列表主要用于排查安装变更、报错和操作者动作。先按关键词或级别缩小范围，再逐条查看。',
        })}
        items={[
          {
            color: '#1677ff',
            text: intl.formatMessage(
              {
                id: 'pages.extensionsInstallations.events.overview.total',
                defaultMessage: `事件 ${total}`,
              },
              { count: total },
            ),
          },
          { color: '#722ed1', text: levelText },
          { color: '#13c2c2', text: keywordText },
        ]}
        hint={intl.formatMessage({
          id: 'pages.extensionsInstallations.events.overview.hint',
          defaultMessage: '推荐路径：先看最近报错和升级事件，再结合安装详情判断是否需要修改配置。',
        })}
      />
      <Space style={{ marginBottom: 12 }} wrap>
        <Input
          allowClear
          style={{ width: 260 }}
          placeholder={intl.formatMessage({
            id: 'pages.extensionsInstallations.events.filter.placeholder',
            defaultMessage: '筛选事件/内容/操作者',
          })}
          value={keyword}
          onChange={(e) => {
            // 筛选变化回第 1 页：params 变化与 setPageInfo 的双触发由
            // ProTable 内部 debounce + abort 合并，不会出现错序数据
            setKeyword(e.target.value);
            actionRef.current?.setPageInfo?.({ current: 1 });
          }}
        />
        <Select
          allowClear
          style={{ width: 140 }}
          placeholder={intl.formatMessage({
            id: 'pages.extensionsInstallations.events.filter.levelPlaceholder',
            defaultMessage: '级别',
          })}
          value={level}
          onChange={(v) => {
            setLevel(v);
            actionRef.current?.setPageInfo?.({ current: 1 });
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
            actionRef.current?.setPageInfo?.({ current: 1 });
          }}
        >
          <FormattedMessage
            id="pages.extensionsInstallations.events.filter.clear"
            defaultMessage="清空筛选"
          />
        </Button>
      </Space>
      {keyword.trim() || level ? (
        <Alert
          style={{ marginBottom: 12 }}
          type="info"
          showIcon
          message={intl.formatMessage({
            id: 'pages.extensionsInstallations.events.filter.activeMessage',
            defaultMessage: '当前正在查看筛选后的事件范围',
          })}
          description={intl.formatMessage(
            {
              id: 'pages.extensionsInstallations.events.filter.activeDescription',
              defaultMessage: `已生效条件：${activeConditions}`,
            },
            { conditions: activeConditions },
          )}
        />
      ) : null}

      <ProTable<ExtensionEventItem>
        actionRef={actionRef}
        rowKey={(row, idx) => `${row.createdAt}-${row.eventType}-${idx}`}
        search={false}
        options={false}
        toolBarRender={false}
        params={{ installationId: installation?.id, keyword, level }}
        request={async ({ current = 1, pageSize = 10, installationId, keyword: kw, level: lv }) => {
          if (!installationId) {
            // 对齐原 loadEvents guard：无安装实例时不发起请求
            return { data: [], total: 0, success: true };
          }
          try {
            const resp = await listExtensionEvents(installationId, {
              level: lv,
              keyword: kw?.trim() || undefined,
              page: current,
              pageSize,
            });
            const vm = adaptEventListResponse(resp);
            setTotal(vm.total);
            return { data: vm.items, total: vm.total, success: true };
          } catch {
            // 原实现不本地弹错（全局请求拦截器已 toast），保持该语义
            return { data: [], total: 0, success: false };
          }
        }}
        pagination={{ pageSize: 10, showSizeChanger: true }}
        columns={[
          {
            title: intl.formatMessage({
              id: 'pages.extensionsInstallations.events.column.createdAt',
              defaultMessage: '时间',
            }),
            dataIndex: 'createdAt',
            key: 'createdAt',
            render: (_, row) => formatUnix(row.createdAt),
          },
          {
            title: intl.formatMessage({
              id: 'pages.extensionsInstallations.events.column.level',
              defaultMessage: '级别',
            }),
            dataIndex: 'level',
            key: 'level',
            width: 100,
          },
          {
            title: intl.formatMessage({
              id: 'pages.extensionsInstallations.events.column.eventType',
              defaultMessage: '事件',
            }),
            dataIndex: 'eventType',
            key: 'eventType',
            width: 150,
          },
          {
            title: intl.formatMessage({
              id: 'pages.extensionsInstallations.events.column.message',
              defaultMessage: '内容',
            }),
            dataIndex: 'message',
            key: 'message',
          },
          {
            title: 'Payload',
            dataIndex: 'payload',
            key: 'payload',
            render: (_, row) =>
              row.payload ? (
                <Typography.Text code ellipsis={{ tooltip: row.payload }} style={{ maxWidth: 260 }}>
                  {row.payload}
                </Typography.Text>
              ) : (
                '-'
              ),
          },
          {
            title: intl.formatMessage({
              id: 'pages.extensionsInstallations.events.column.createdBy',
              defaultMessage: '操作者',
            }),
            dataIndex: 'createdBy',
            key: 'createdBy',
            width: 120,
          },
        ]}
        locale={{
          emptyText:
            keyword.trim() || level
              ? intl.formatMessage({
                  id: 'pages.extensionsInstallations.events.empty.filtered',
                  defaultMessage: '当前筛选条件下没有匹配事件，请调整筛选后重试。',
                })
              : intl.formatMessage({
                  id: 'pages.extensionsInstallations.events.empty.default',
                  defaultMessage: '暂时没有事件数据，后续有安装动作后会显示在这里。',
                }),
        }}
      />
    </Drawer>
  );
}
