import React, { useEffect, useState } from 'react';
import {
  Card,
  Table,
  Space,
  Button,
  Modal,
  Form,
  InputNumber,
  Select,
  Input,
  App,
  Tag,
  Checkbox,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { ColumnsType } from 'antd/es/table';
import {
  listRateLimits,
  putRateLimits,
  deleteRateLimit,
  type RateLimitRule,
  type RateLimitPreviewAgent,
  listOpsFunctions,
  previewRateLimit,
  listOpsNodes,
} from '@/services/api/ops';
import { exportToCSV } from '@/utils/export';

type RateLimitFormValues = {
  scope: 'function' | 'service';
  key: string;
  limitQps: number;
  percent?: number;
  matchGameId?: string;
  matchEnv?: string;
  matchRegion?: string;
  matchZone?: string;
  matchLabels?: string;
};

export default function OpsRateLimitsPage() {
  const { message, modal } = App.useApp();
  const intl = useIntl();
  const [loading, setLoading] = useState(false);
  const [rules, setRules] = useState<RateLimitRule[]>([]);
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm<RateLimitFormValues>();
  const [functions, setFunctions] = useState<string[]>([]);
  const [agents, setAgents] = useState<string[]>([]);
  const [preview, setPreview] = useState<{
    matched: number;
    agents: RateLimitPreviewAgent[];
  } | null>(null);
  const [pvTick, setPvTick] = useState(0);
  const [onlyOver, setOnlyOver] = useState(false);
  // auto-preview trigger on form changes
  const onFormValuesChange = () => setPvTick((x) => x + 1);
  useEffect(() => {
    if (!open) return;
    const id = setTimeout(async () => {
      try {
        const v = form.getFieldsValue();
        if (v && v.scope === 'service' && v.key && v.limitQps) {
          const res = await previewRateLimit({
            scope: 'service',
            key: v.key,
            limitQps: v.limitQps,
            percent: v.percent,
            matchGameId: v.matchGameId,
            matchEnv: v.matchEnv,
            matchRegion: v.matchRegion,
            matchZone: v.matchZone,
          });
          setPreview(res);
        } else {
          setPreview(null);
        }
      } catch {
        /* ignore */
      }
    }, 200);
    return () => clearTimeout(id);
  }, [pvTick, open, form]);

  const load = async () => {
    setLoading(true);
    try {
      const res = await listRateLimits();
      setRules(res.rules || []);
      // 从函数描述符加载函数ID列表（用于下拉选择）
      try {
        const s = await listOpsFunctions();
        const funcs = (s.functions || []).map((f) => f.id).filter(Boolean);
        setFunctions(funcs);
      } catch {}
      // 载入 agent 列表
      try {
        const s2 = await listOpsNodes();
        setAgents(
          (s2.nodes || [])
            .filter((s) => (s.type || 'agent') === 'agent')
            .map((s) => s.id || s.addr)
            .filter(Boolean) as string[],
        );
      } catch {}
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    load();
  }, []);

  const columns: ColumnsType<RateLimitRule> = [
    {
      title: intl.formatMessage({ id: 'pages.scope' }),
      dataIndex: 'scope',
      width: 120,
      render: (v) =>
        v === 'function' ? (
          <Tag color="blue">{intl.formatMessage({ id: 'pages.rate.limits.functions' })}</Tag>
        ) : (
          <Tag color="purple">{intl.formatMessage({ id: 'pages.rate.limits.services' })}</Tag>
        ),
    },
    { title: 'Key', dataIndex: 'key', width: 240 },
    { title: 'QPS', dataIndex: 'limitQps', width: 100 },
    {
      title: intl.formatMessage({ id: 'pages.rate.limits.percentage' }),
      dataIndex: 'percent',
      width: 100,
      render: (v) => v || 100,
    },
    {
      title: intl.formatMessage({ id: 'pages.rate.limits.match' }).replace('（可选）', ''),
      dataIndex: 'match',
      width: 200,
      render: (m: Record<string, string> | undefined) =>
        m
          ? Object.entries(m).map(([k, v]) => (
              <Tag key={k}>
                {k}:{String(v)}
              </Tag>
            ))
          : '-',
    },
    {
      title: intl.formatMessage({ id: 'pages.permissions.actions' }),
      render: (_: unknown, r: RateLimitRule) => (
        <Space>
          <Button
            size="small"
            onClick={() => {
              setOpen(true);
              // 表单字段是平铺的 matchGameId/matchEnv/...，而规则里的 match 是
              // 嵌套对象：回填时映射标准四键，其余键还原为 labels JSON 文本，
              // 否则提交侧按空输入重建 match，静默清空全部匹配条件。
              const { match, ...rest } = r;
              const standard: Record<string, string> = {};
              const labels: Record<string, string> = {};
              const keyMap: Record<string, string> = {
                gameId: 'matchGameId',
                env: 'matchEnv',
                region: 'matchRegion',
                zone: 'matchZone',
              };
              Object.entries(match || {}).forEach(([k, v]) => {
                const formKey = keyMap[k];
                if (formKey) standard[formKey] = String(v);
                else labels[k] = String(v);
              });
              form.setFieldsValue({
                ...rest,
                ...standard,
                matchLabels: Object.keys(labels).length
                  ? JSON.stringify(labels, null, 2)
                  : undefined,
              });
            }}
          >
            {intl.formatMessage({ id: 'pages.permissions.edit.button' })}
          </Button>
          <Button
            size="small"
            danger
            onClick={() =>
              modal.confirm({
                title: intl
                  .formatMessage({ id: 'pages.rate.limits.management' })
                  .replace('限速管理', '删除限速'),
                content: intl.formatMessage(
                  { id: 'pages.rate.limits.delete.confirm' },
                  { scope: r.scope, key: r.key },
                ),
                onOk: async () => {
                  await deleteRateLimit(r.scope, r.key);
                  message.success(
                    intl
                      .formatMessage({ id: 'pages.permissions.save.success' })
                      .replace('已保存权限配置', '已删除'),
                  );
                  load();
                },
              })
            }
          >
            {intl.formatMessage({ id: 'pages.permissions.edit.button' }).replace('编辑', '删除')}
          </Button>
        </Space>
      ),
    },
  ];

  const onSubmit = async () => {
    const v = await form.validateFields();
    const match: Record<string, string> = {};
    if (v.matchGameId) match.gameId = v.matchGameId;
    if (v.matchEnv) match.env = v.matchEnv;
    if (v.matchRegion) match.region = v.matchRegion;
    if (v.matchZone) match.zone = v.matchZone;
    const rule: RateLimitRule = { scope: v.scope, key: v.key, limitQps: v.limitQps };
    if (Object.keys(match).length > 0) rule.match = match;
    if (v.percent && v.percent > 0 && v.percent <= 100) rule.percent = v.percent;
    // optional labels JSON
    try {
      const txt = (v.matchLabels || '').trim();
      if (txt) {
        const m = JSON.parse(txt);
        if (typeof m === 'object' && !Array.isArray(m)) {
          rule.match = { ...(rule.match || {}), ...(m as Record<string, string>) };
        }
      }
    } catch {
      message.warning(
        intl.formatMessage({
          id: 'pages.opsRateLimits.message.labelsIgnored',
          defaultMessage: '标签JSON解析失败，已忽略',
        }),
      );
    }
    await putRateLimits([rule]);
    setOpen(false);
    message.success(
      intl.formatMessage({ id: 'pages.opsRateLimits.message.saved', defaultMessage: '已保存' }),
    );
    load();
  };
  const onPreview = async () => {
    const v = await form.validateFields();
    if (v.scope !== 'service') {
      message.info(
        intl.formatMessage({
          id: 'pages.opsRateLimits.message.servicePreviewOnly',
          defaultMessage: '仅支持服务级预览',
        }),
      );
      return;
    }
    try {
      const res = await previewRateLimit({
        scope: 'service',
        key: v.key,
        limitQps: v.limitQps,
        percent: v.percent,
        matchGameId: v.matchGameId,
        matchEnv: v.matchEnv,
        matchRegion: v.matchRegion,
        matchZone: v.matchZone,
      });
      setPreview(res);
    } catch (e) {
      const errMsg =
        e instanceof Error
          ? e.message
          : intl.formatMessage({
              id: 'pages.opsRateLimits.error.operationFailed',
              defaultMessage: '操作失败',
            });
      message.error(
        errMsg ||
          intl.formatMessage({
            id: 'pages.opsRateLimits.error.previewFailed',
            defaultMessage: '预览失败',
          }),
      );
    }
  };

  return (
    <PageContainer>
      <Card
        title={intl.formatMessage({ id: 'pages.rate.limits.management' })}
        extra={
          <Button
            type="primary"
            onClick={() => {
              setOpen(true);
              // 先清空：上次编辑的 key 与 match 字段残留在 form 实例中，
              // 会对错误函数/agent 建立限速规则
              form.resetFields();
              form.setFieldsValue({ scope: 'function', limitQps: 10, percent: 100 });
            }}
          >
            {intl.formatMessage({ id: 'pages.rate.limits.new.rule' })}
          </Button>
        }
      >
        <Table
          scroll={{ x: 850 }}
          rowKey={(r) => `${r.scope}:${r.key}`}
          loading={loading}
          dataSource={rules}
          columns={columns}
          pagination={{ pageSize: 10 }}
        />
      </Card>

      <Modal
        title={intl.formatMessage({ id: 'pages.rate.limits.edit.rule' })}
        open={open}
        onOk={onSubmit}
        onCancel={() => {
          setOpen(false);
          setPreview(null);
        }}
        destroyOnHidden
      >
        <Form
          form={form}
          layout="vertical"
          onValuesChange={onFormValuesChange}
          initialValues={{ scope: 'function', limitQps: 10, percent: 100 }}
        >
          <Form.Item
            label={intl.formatMessage({ id: 'pages.scope' })}
            name="scope"
            rules={[{ required: true }]}
          >
            <Select
              options={[
                {
                  label: intl.formatMessage({ id: 'pages.rate.limits.functions' }),
                  value: 'function',
                },
                {
                  label: intl.formatMessage({ id: 'pages.rate.limits.services' }),
                  value: 'service',
                },
              ]}
              onChange={() => form.setFieldValue('key', '')}
            />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(prev, cur) => prev.scope !== cur.scope}>
            {() => {
              const scope = form.getFieldValue('scope');
              return (
                <Form.Item
                  label={
                    scope === 'service'
                      ? intl.formatMessage({ id: 'pages.rate.limits.key.agent' })
                      : intl.formatMessage({ id: 'pages.rate.limits.key.function' })
                  }
                  name="key"
                  rules={[{ required: true }]}
                >
                  <Select
                    showSearch
                    placeholder={scope === 'service' ? 'agent_id' : 'function_id'}
                    options={(scope === 'service' ? agents : functions).map((id) => ({
                      label: id,
                      value: id,
                    }))}
                  />
                </Form.Item>
              );
            }}
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({ id: 'pages.rate.limits.qps' })}
            name="limitQps"
            rules={[{ required: true, type: 'number', min: 1 }]}
          >
            {' '}
            <InputNumber min={1} />{' '}
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({ id: 'pages.rate.limits.percentage' })}
            name="percent"
            tooltip={intl.formatMessage({
              id: 'pages.opsRateLimits.form.percentTooltip',
              defaultMessage: '按比例生效（函数灰度为按 trace 采样；服务灰度折算 QPS）',
            })}
          >
            {' '}
            <InputNumber min={1} max={100} />{' '}
          </Form.Item>
          <Form.Item label={intl.formatMessage({ id: 'pages.rate.limits.match' })}>
            <Space>
              <Form.Item name="matchGameId" noStyle>
                {' '}
                <Input placeholder="game_id" style={{ width: 160 }} />{' '}
              </Form.Item>
              <Form.Item name="matchEnv" noStyle>
                {' '}
                <Input placeholder="env" style={{ width: 120 }} />{' '}
              </Form.Item>
              <Form.Item name="matchRegion" noStyle>
                {' '}
                <Input placeholder="region" style={{ width: 120 }} />{' '}
              </Form.Item>
              <Form.Item name="matchZone" noStyle>
                {' '}
                <Input placeholder="zone" style={{ width: 120 }} />{' '}
              </Form.Item>
            </Space>
          </Form.Item>
          <Space>
            <Button onClick={onPreview}>
              {intl.formatMessage({ id: 'pages.rate.limits.preview' })}
            </Button>
            {preview && (
              <span>
                {intl.formatMessage(
                  {
                    id: 'pages.opsRateLimits.preview.matched',
                    defaultMessage: '命中实例：{count}',
                  },
                  { count: preview.matched },
                )}
              </span>
            )}
            {preview && (
              <Checkbox checked={onlyOver} onChange={(e) => setOnlyOver(e.target.checked)}>
                <FormattedMessage
                  id="pages.opsRateLimits.preview.onlyOverCheckbox"
                  defaultMessage="仅显示超限（当前QPS>限速）"
                />
              </Checkbox>
            )}
            {preview && (
              <Button
                onClick={() => {
                  try {
                    const rows = (preview.agents || []).map((a) => [
                      a.agentId,
                      a.gameId || '',
                      a.env || '',
                      a.region || '',
                      a.zone || '',
                      a.addr || '',
                      a.qps || '',
                      (a.qps1m || 0).toFixed(2),
                    ]);
                    rows.unshift([
                      'agentId',
                      'gameId',
                      'env',
                      'region',
                      'zone',
                      'addr',
                      'qpsLimit',
                      'qps1m',
                    ]);
                    exportToCSV('rate_limit_preview.csv', rows);
                  } catch {}
                }}
              >
                {intl.formatMessage({ id: 'pages.rate.limits.export.csv' })}
              </Button>
            )}
          </Space>
          {preview && (
            <div
              style={{
                maxHeight: 180,
                overflow: 'auto',
                border: '1px solid #f0f0f0',
                padding: 8,
                marginTop: 8,
              }}
            >
              {(() => {
                const arr = (preview?.agents || [])
                  .map((a) => ({ ...a, qps1m: Number(a.qps1m || 0) }))
                  .sort((a, b) => b.qps1m - a.qps1m)
                  .filter((a) => !onlyOver || a.qps1m > (a.qps || 0));
                return arr.map((a) => (
                  <div key={a.agentId}>
                    <Tag>{a.agentId}</Tag> {a.gameId || ''}/{a.env || ''}{' '}
                    {a.region ? `/${a.region}` : ''} {a.zone ? `/${a.zone}` : ''}
                    {' '}
                    <FormattedMessage
                      id="pages.opsRateLimits.preview.currentQpsLabel"
                      defaultMessage="当前QPS: "
                    />
                    <b>{a.qps1m.toFixed(2)}</b> {' / '}
                    <FormattedMessage
                      id="pages.opsRateLimits.preview.limitLabel"
                      defaultMessage="限速: "
                    />
                    <b>{a.qps}</b>
                  </div>
                ));
              })()}
            </div>
          )}
        </Form>
      </Modal>
    </PageContainer>
  );
}
