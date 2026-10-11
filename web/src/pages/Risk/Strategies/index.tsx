import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  App,
  Alert,
  Button,
  Card,
  Popconfirm,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
  type TableProps,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { ReloadOutlined, RollbackOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  getCaptureGates,
  listCaptureGatesVersions,
  rollbackCaptureGates,
  type CaptureGatesState,
  type CaptureGatesVersionItem,
} from '@/services/api/opsCapture';
import { listOpsNodes, type OpsNode } from '@/services/api/ops';
import { summarizeGatesApply } from '@/utils/captureGatesApply';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

export default function RiskStrategiesPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  const intlRef = useRef(intl);
  intlRef.current = intl;

  const [latest, setLatest] = useState<CaptureGatesState | null>(null);
  const [versions, setVersions] = useState<CaptureGatesVersionItem[]>([]);
  const [nodes, setNodes] = useState<OpsNode[]>([]);
  const [loading, setLoading] = useState(false);
  // 行级回滚 loading：记录正在回滚的版本号（undefined=无进行中的回滚）。
  const [rollingVersion, setRollingVersion] = useState<number | undefined>(undefined);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [s, vs, ns] = await Promise.all([
        getCaptureGates(),
        // 版本历史/节点对账失败不打断主面（空历史/空对账 ≠ 策略不可用）
        listCaptureGatesVersions().catch(() => []),
        listOpsNodes()
          .then((resp) => resp.nodes)
          .catch(() => []),
      ]);
      setLatest(s);
      setVersions(vs);
      setNodes(ns);
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.riskControl.error.loadFailed',
            defaultMessage: '加载策略版本失败',
          }),
        ),
      );
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    load();
  }, [load]);

  // 回滚 = 以历史版本内容再发一版（版本日志不可变，agent 照常轮询热生效）。
  const rollback = useCallback(
    async (version: number) => {
      setRollingVersion(version);
      try {
        await rollbackCaptureGates({ version });
        message.success(
          intlRef.current.formatMessage(
            {
              id: 'pages.riskControl.rollback.success',
              defaultMessage: '已按版本 {version} 内容发布新版本，agent 将在轮询周期内热生效',
            },
            { version },
          ),
        );
        await load();
      } catch (error) {
        message.error(
          extractErrorMessage(
            error,
            intlRef.current.formatMessage({
              id: 'pages.riskControl.error.rollbackFailed',
              defaultMessage: '回滚失败',
            }),
          ),
        );
      } finally {
        setRollingVersion(undefined);
      }
    },
    [load, message],
  );

  const unpublished = !latest || latest.version === 0;
  const desired = latest?.version ?? 0;
  const summary = useMemo(() => summarizeGatesApply(nodes, desired), [nodes, desired]);

  const columns = useMemo(() => {
    const cols: TableProps<CaptureGatesVersionItem>['columns'] = [
      {
        title: <FormattedMessage id="pages.riskControl.column.version" defaultMessage="版本" />,
        dataIndex: 'version',
        width: 90,
        render: (_: unknown, record: CaptureGatesVersionItem) => (
          <Space size={6}>
            <Text strong>v{record.version}</Text>
            {record.version === desired && (
              <Tag color="green">
                <FormattedMessage id="pages.riskControl.current" defaultMessage="当前" />
              </Tag>
            )}
          </Space>
        ),
      },
      {
        title: <FormattedMessage id="pages.riskControl.column.message" defaultMessage="说明" />,
        dataIndex: 'message',
        render: (value: string | undefined) => value || '-',
      },
      {
        title: <FormattedMessage id="pages.riskControl.column.author" defaultMessage="发布人" />,
        dataIndex: 'createdBy',
        width: 120,
        render: (value: string | undefined) => value || '-',
      },
      {
        title: <FormattedMessage id="pages.riskControl.column.time" defaultMessage="发布时间" />,
        dataIndex: 'updatedAt',
        width: 180,
      },
      {
        title: <FormattedMessage id="pages.riskControl.column.action" defaultMessage="操作" />,
        key: 'action',
        width: 110,
        render: (_: unknown, record: CaptureGatesVersionItem) => (
          <Popconfirm
            title={intlRef.current.formatMessage({
              id: 'pages.riskControl.rollback.confirm',
              defaultMessage: '以该历史版本内容再发一版？',
            })}
            onConfirm={() => rollback(record.version)}
            disabled={record.version === desired}
          >
            <Button
              size="small"
              icon={<RollbackOutlined />}
              loading={rollingVersion === record.version}
              disabled={record.version === desired || rollingVersion !== undefined}
            >
              <FormattedMessage id="pages.riskControl.rollback" defaultMessage="回滚" />
            </Button>
          </Popconfirm>
        ),
      },
    ];
    return cols;
  }, [desired, rollingVersion, rollback]);

  return (
    <PageContainer>
      <Card
        title={<FormattedMessage id="pages.riskControl.title" defaultMessage="风控策略" />}
        extra={
          <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
            <FormattedMessage id="pages.riskControl.reload" defaultMessage="重载" />
          </Button>
        }
      >
        <Space direction="vertical" style={{ width: '100%' }} size={12}>
          {unpublished && (
            <Alert
              type="info"
              showIcon
              message={intlRef.current.formatMessage({
                id: 'pages.riskControl.unpublished',
                defaultMessage: '未发布（agent 用文件规则）',
              })}
            />
          )}
          {summary && (
            <div>
              <Text type="secondary">
                <FormattedMessage
                  id="pages.opsCapture.apply.summary"
                  defaultMessage="生效对账（agent 自报）："
                />
              </Text>{' '}
              {summary.total === 0 ? (
                <Tag>
                  <FormattedMessage
                    id="pages.opsCapture.apply.noAgents"
                    defaultMessage="scope 内无在线 capture-agent"
                  />
                </Tag>
              ) : summary.stale.length === 0 ? (
                <Tag color="green">
                  <FormattedMessage
                    id="pages.opsCapture.apply.allApplied"
                    defaultMessage="{count}/{total} 台已生效"
                    values={{ count: summary.applied.length, total: summary.total }}
                  />
                </Tag>
              ) : (
                <Tooltip
                  title={summary.stale
                    .map(
                      (n) =>
                        `${n.id}: ${n.labels?.captureGatesRevision || intlRef.current.formatMessage({ id: 'pages.opsCapture.apply.unknown', defaultMessage: '未上报' })}`,
                    )
                    .join('; ')}
                >
                  <Tag color="gold">
                    <FormattedMessage
                      id="pages.opsCapture.apply.partial"
                      defaultMessage="{count}/{total} 台已生效（其余未上报或滞后）"
                      values={{ count: summary.applied.length, total: summary.total }}
                    />
                  </Tag>
                </Tooltip>
              )}
            </div>
          )}
          <Table<CaptureGatesVersionItem>
            rowKey="version"
            columns={columns}
            dataSource={versions}
            loading={loading}
            size="small"
            pagination={false}
            locale={{
              emptyText: intlRef.current.formatMessage({
                id: 'pages.riskControl.empty',
                defaultMessage: '该 scope 尚未发布过策略版本',
              }),
            }}
          />
          <Alert
            type="info"
            showIcon
            message={intlRef.current.formatMessage({
              id: 'pages.riskControl.boundary',
              defaultMessage:
                '回滚不改写历史：以所选版本内容复制为新版本发布，agent 照常版本轮询热生效；发布成功 ≠ 已生效，以上「生效对账」为 agent 自报（30s 轮询 + 重注册滞后），逐次生效真值在 agent 日志 capture gates version applied。',
            })}
          />
        </Space>
      </Card>
    </PageContainer>
  );
}
