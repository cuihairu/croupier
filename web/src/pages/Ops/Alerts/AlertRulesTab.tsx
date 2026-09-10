import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { ModalForm } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  App,
  Button,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Select,
  Switch,
  Table,
  Tag,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  createAlertRule,
  deleteAlertRule,
  listAlertRules,
  updateAlertRule,
  type AlertRuleItem,
} from '@/services/api/ops';
import { extractErrorMessage } from '@/utils/errors';
import { formatDateTime } from '@/utils/format';

const buildMetricOptions = (intl: ReturnType<typeof useIntl>) => [
  {
    label: intl.formatMessage({
      id: 'pages.opsAlerts.rules.metric.cpuUsagePercent',
      defaultMessage: 'CPU 使用率 (%)',
    }),
    value: 'cpu.usagePercent',
  },
  {
    label: intl.formatMessage({
      id: 'pages.opsAlerts.rules.metric.memoryUsagePercent',
      defaultMessage: '内存使用率 (%)',
    }),
    value: 'memory.usagePercent',
  },
  {
    label: intl.formatMessage({
      id: 'pages.opsAlerts.rules.metric.memoryUsedBytes',
      defaultMessage: '内存已用 (字节)',
    }),
    value: 'memory.usedBytes',
  },
  {
    label: intl.formatMessage({
      id: 'pages.opsAlerts.rules.metric.diskUsagePercent',
      defaultMessage: '磁盘使用率（按挂载点）',
    }),
    value: 'disk./data.usedPercent',
  },
  {
    label: intl.formatMessage({
      id: 'pages.opsAlerts.rules.metric.custom',
      defaultMessage: '自定义指标',
    }),
    value: 'custom.',
  },
];

const buildOperatorOptions = (intl: ReturnType<typeof useIntl>) => [
  {
    label: intl.formatMessage({
      id: 'pages.opsAlerts.rules.operator.gt',
      defaultMessage: '> 大于',
    }),
    value: 'gt',
  },
  {
    label: intl.formatMessage({
      id: 'pages.opsAlerts.rules.operator.gte',
      defaultMessage: '>= 大于等于',
    }),
    value: 'gte',
  },
  {
    label: intl.formatMessage({
      id: 'pages.opsAlerts.rules.operator.lt',
      defaultMessage: '< 小于',
    }),
    value: 'lt',
  },
  {
    label: intl.formatMessage({
      id: 'pages.opsAlerts.rules.operator.lte',
      defaultMessage: '<= 小于等于',
    }),
    value: 'lte',
  },
];

const LEVEL_COLOR: Record<AlertRuleItem['level'], string> = {
  info: 'blue',
  warning: 'orange',
  critical: 'red',
};

/** 规则表单值：编辑回填自 AlertRuleItem（operator/level 为具体字面量），新增取弹窗默认值 */
type AlertRuleFormValues = {
  name: string;
  description?: string;
  metric: string;
  operator: AlertRuleItem['operator'];
  threshold: number;
  forCount: number;
  cooldownSeconds: number;
  level: AlertRuleItem['level'];
  agentFilter?: string;
};

export default function AlertRulesTab() {
  const { message } = App.useApp();
  const intl = useIntl();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<AlertRuleItem[]>([]);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<AlertRuleItem | null>(null);

  const metricOptions = useMemo(() => buildMetricOptions(intl), [intl]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await listAlertRules();
      setRows(res?.items || []);
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.opsAlerts.rules.error.loadFailed',
            defaultMessage: '加载告警规则失败',
          }),
        ),
      );
    } finally {
      setLoading(false);
    }
  }, [message, intl]);

  useEffect(() => {
    load();
  }, [load]);

  const openCreate = () => {
    setEditing(null);
    setOpen(true);
  };

  const openEdit = (row: AlertRuleItem) => {
    setEditing(row);
    setOpen(true);
  };

  const onFinish = async (values: AlertRuleFormValues) => {
    const payload = {
      name: values.name,
      description: values.description,
      metric: values.metric,
      operator: values.operator,
      threshold: values.threshold,
      forCount: values.forCount,
      cooldownSeconds: values.cooldownSeconds,
      level: values.level,
      agentFilter: values.agentFilter || '',
    };
    try {
      if (editing) {
        await updateAlertRule(editing.id, payload);
        message.success(
          intl.formatMessage({
            id: 'pages.opsAlerts.rules.success.updated',
            defaultMessage: '已更新',
          }),
        );
      } else {
        await createAlertRule(payload);
        message.success(
          intl.formatMessage({
            id: 'pages.opsAlerts.rules.success.created',
            defaultMessage: '已创建，下次指标上报即生效',
          }),
        );
      }
      load();
      return true;
    } catch (error) {
      // 原语义：提交失败本地 toast（校验失败由 ModalForm 内置拦截，不走到这里）
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.opsAlerts.rules.error.saveFailed',
            defaultMessage: '保存失败',
          }),
        ),
      );
      return false;
    }
  };

  const toggleEnabled = async (row: AlertRuleItem, next: boolean) => {
    try {
      await updateAlertRule(row.id, { enabled: next });
      message.success(
        next
          ? intl.formatMessage({
              id: 'pages.opsAlerts.rules.success.enabled',
              defaultMessage: '已启用',
            })
          : intl.formatMessage({
              id: 'pages.opsAlerts.rules.success.disabled',
              defaultMessage: '已停用',
            }),
      );
      load();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.opsAlerts.rules.error.operationFailed',
            defaultMessage: '操作失败',
          }),
        ),
      );
    }
  };

  const columns: ColumnsType<AlertRuleItem> = [
    {
      title: intl.formatMessage({ id: 'pages.opsAlerts.rules.field.name', defaultMessage: '名称' }),
      dataIndex: 'name',
      width: 150,
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsAlerts.rules.column.condition',
        defaultMessage: '条件',
      }),
      dataIndex: 'metric',
      width: 260,
      render: (_: unknown, r: AlertRuleItem) => (
        <code style={{ fontSize: 12 }}>
          {r.metric} {r.operator} {r.threshold}
        </code>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsAlerts.rules.field.level',
        defaultMessage: '级别',
      }),
      dataIndex: 'level',
      width: 90,
      render: (_: unknown, r: AlertRuleItem) => (
        <Tag color={LEVEL_COLOR[r.level] || 'default'}>{r.level}</Tag>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsAlerts.rules.column.forCount',
        defaultMessage: '连续命中',
      }),
      dataIndex: 'forCount',
      width: 90,
      render: (_: unknown, r: AlertRuleItem) =>
        r.forCount > 1
          ? intl.formatMessage(
              { id: 'pages.opsAlerts.rules.forCount.times', defaultMessage: '{count} 次' },
              { count: r.forCount },
            )
          : intl.formatMessage({
              id: 'pages.opsAlerts.rules.forCount.immediate',
              defaultMessage: '立即',
            }),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsAlerts.rules.column.cooldown',
        defaultMessage: '冷却',
      }),
      dataIndex: 'cooldownSeconds',
      width: 90,
      render: (v: number) =>
        intl.formatMessage(
          { id: 'pages.opsAlerts.rules.cooldown.minutes', defaultMessage: '{minutes} 分钟' },
          { minutes: Math.round(v / 60) },
        ),
    },
    {
      title: 'Agent',
      dataIndex: 'agentFilter',
      width: 110,
      render: (v?: string) =>
        v ||
        intl.formatMessage({ id: 'pages.opsAlerts.rules.agentFilter.all', defaultMessage: '全部' }),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsAlerts.rules.column.lastFiredAt',
        defaultMessage: '最近触发',
      }),
      dataIndex: 'lastFiredAt',
      width: 160,
      render: (v?: string) => formatDateTime(v ?? ''),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsAlerts.rules.column.enabled',
        defaultMessage: '启用',
      }),
      dataIndex: 'enabled',
      width: 80,
      render: (_: unknown, r: AlertRuleItem) => (
        <Switch checked={r.enabled} onChange={(v) => toggleEnabled(r, v)} size="small" />
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsAlerts.rules.column.actions',
        defaultMessage: '操作',
      }),
      key: 'actions',
      width: 120,
      render: (_: unknown, row: AlertRuleItem) => [
        <a key="edit" onClick={() => openEdit(row)}>
          <FormattedMessage id="pages.opsAlerts.rules.action.edit" defaultMessage="编辑" />
        </a>,
        <Popconfirm
          key="del"
          title={intl.formatMessage({
            id: 'pages.opsAlerts.rules.confirm.delete',
            defaultMessage: '确认删除该规则？',
          })}
          onConfirm={async () => {
            try {
              await deleteAlertRule(row.id);
              message.success(
                intl.formatMessage({
                  id: 'pages.opsAlerts.rules.success.deleted',
                  defaultMessage: '已删除',
                }),
              );
              load();
            } catch (error) {
              message.error(
                extractErrorMessage(
                  error,
                  intl.formatMessage({
                    id: 'pages.opsAlerts.rules.error.deleteFailed',
                    defaultMessage: '删除失败',
                  }),
                ),
              );
            }
          }}
        >
          <a style={{ color: '#cf1322' }}>
            <FormattedMessage id="pages.opsAlerts.rules.action.delete" defaultMessage="删除" />
          </a>
        </Popconfirm>,
      ],
    },
  ];

  return (
    <>
      <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'flex-end' }}>
        <Button type="primary" onClick={openCreate}>
          <FormattedMessage id="pages.opsAlerts.rules.action.create" defaultMessage="新建规则" />
        </Button>
      </div>
      <Table<AlertRuleItem>
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={rows}
        pagination={false}
        size="small"
      />

      <ModalForm<AlertRuleFormValues>
        title={
          editing
            ? intl.formatMessage({
                id: 'pages.opsAlerts.rules.modal.editTitle',
                defaultMessage: '编辑规则',
              })
            : intl.formatMessage({
                id: 'pages.opsAlerts.rules.action.create',
                defaultMessage: '新建规则',
              })
        }
        open={open}
        onOpenChange={setOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        submitter={{
          searchConfig: {
            submitText: intl.formatMessage({
              id: 'pages.opsAlerts.rules.modal.submit',
              defaultMessage: '确定',
            }),
          },
        }}
        layout="vertical"
        // destroyOnHidden 使弹窗每次关闭即卸载表单，重开时按最新 initialValues
        // 重新挂载，新增/编辑切换不会残留上一次的预填值
        initialValues={
          editing ?? {
            operator: 'gt',
            forCount: 1,
            cooldownSeconds: 300,
            level: 'warning',
            metric: 'cpu.usagePercent',
            threshold: 90,
          }
        }
        onFinish={onFinish}
      >
        <Form.Item
          name="name"
          label={intl.formatMessage({
            id: 'pages.opsAlerts.rules.field.name',
            defaultMessage: '名称',
          })}
          rules={[{ required: true }]}
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.opsAlerts.rules.form.namePlaceholder',
              defaultMessage: 'CPU 持续高负载',
            })}
          />
        </Form.Item>
        <Form.Item
          name="description"
          label={intl.formatMessage({
            id: 'pages.opsAlerts.rules.form.description',
            defaultMessage: '说明',
          })}
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.opsAlerts.rules.form.descriptionPlaceholder',
              defaultMessage: '用于…（可选）',
            })}
          />
        </Form.Item>
        <Form.Item
          name="metric"
          label={intl.formatMessage({
            id: 'pages.opsAlerts.rules.form.metric',
            defaultMessage: '指标',
          })}
          rules={[{ required: true }]}
        >
          <Select options={metricOptions} />
        </Form.Item>
        <Form.Item noStyle shouldUpdate={(p, c) => p.metric !== c.metric}>
          {({ getFieldValue }) => {
            const metric: string = getFieldValue('metric') || '';
            const isCustom = metric.startsWith('custom.');
            const isPreset = !isCustom && metricOptions.some((o) => o.value === metric);
            return (
              <Form.Item
                name="metric"
                label={
                  isCustom
                    ? intl.formatMessage({
                        id: 'pages.opsAlerts.rules.form.metricKeyLabel',
                        defaultMessage: '自定义指标 key',
                      })
                    : undefined
                }
                rules={[
                  { required: true },
                  // addonBefore 仅是装饰，值不带前缀提交后规则永不命中，
                  // 改为校验拦截（值本身含前缀，避免 addon 再叠一层显示）。
                  ...(isCustom
                    ? [
                        {
                          pattern: /^custom\./,
                          message: intl.formatMessage({
                            id: 'pages.opsAlerts.rules.form.metricPatternMessage',
                            defaultMessage: '自定义指标需以 custom. 开头（如 custom.queueDepth）',
                          }),
                        },
                      ]
                    : []),
                ]}
                style={isPreset ? { display: 'none' } : undefined}
              >
                {isCustom ? (
                  <Input placeholder="custom.queueDepth" />
                ) : (
                  <Input
                    placeholder={intl.formatMessage({
                      id: 'pages.opsAlerts.rules.form.metricManualPlaceholder',
                      defaultMessage: 'disk./data.usedPercent 或 custom.queueDepth',
                    })}
                  />
                )}
              </Form.Item>
            );
          }}
        </Form.Item>
        <Form.Item
          name="operator"
          label={intl.formatMessage({
            id: 'pages.opsAlerts.rules.form.operator',
            defaultMessage: '比较',
          })}
          rules={[{ required: true }]}
        >
          <Select options={buildOperatorOptions(intl)} style={{ width: 160 }} />
        </Form.Item>
        <Form.Item
          name="threshold"
          label={intl.formatMessage({
            id: 'pages.opsAlerts.rules.form.threshold',
            defaultMessage: '阈值',
          })}
          rules={[{ required: true }]}
        >
          <InputNumber style={{ width: '100%' }} placeholder="90" />
        </Form.Item>
        <Form.Item
          name="forCount"
          label={intl.formatMessage({
            id: 'pages.opsAlerts.rules.form.forCountLabel',
            defaultMessage: '连续命中次数（>1 表示持续窗口）',
          })}
          initialValue={1}
        >
          <InputNumber min={1} max={60} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item
          name="cooldownSeconds"
          label={intl.formatMessage({
            id: 'pages.opsAlerts.rules.form.cooldownLabel',
            defaultMessage: '冷却（秒）',
          })}
          initialValue={300}
        >
          <InputNumber min={60} max={86400} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item
          name="level"
          label={intl.formatMessage({
            id: 'pages.opsAlerts.rules.field.level',
            defaultMessage: '级别',
          })}
          initialValue="warning"
        >
          <Select
            options={[
              {
                label: intl.formatMessage({
                  id: 'pages.opsAlerts.rules.form.levelInfo',
                  defaultMessage: '提示 info',
                }),
                value: 'info',
              },
              {
                label: intl.formatMessage({
                  id: 'pages.opsAlerts.rules.form.levelWarning',
                  defaultMessage: '警告 warning',
                }),
                value: 'warning',
              },
              {
                label: intl.formatMessage({
                  id: 'pages.opsAlerts.rules.form.levelCritical',
                  defaultMessage: '严重 critical',
                }),
                value: 'critical',
              },
            ]}
            style={{ width: 200 }}
          />
        </Form.Item>
        <Form.Item
          name="agentFilter"
          label={intl.formatMessage({
            id: 'pages.opsAlerts.rules.form.agentFilterLabel',
            defaultMessage: '限定 Agent（空 = 全部）',
          })}
        >
          <Input placeholder="agent-1" />
        </Form.Item>
      </ModalForm>
    </>
  );
}
