import React, { useCallback, useEffect, useState } from 'react';
import { App, Button, Card, Form, Input, Space, Tag, Tooltip, Typography } from 'antd';
import {
  clearSiteSetting,
  fetchObservabilitySettings,
  setSiteSetting,
  type ObservabilitySettings,
  type SettingSource,
} from '@/services/api/sites';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

const FIELDS: Record<string, { key: string; label: string; placeholder: string; help: string }> = {
  alertmanagerUrl: {
    key: 'obs.alertmanagerUrl',
    label: 'Alertmanager 地址',
    placeholder: 'http://alertmanager:9093',
    help: '运维中心告警页跳转用的 Alertpush 源',
  },
  grafanaExploreUrl: {
    key: 'obs.grafanaExploreUrl',
    label: 'Grafana Explore 地址',
    placeholder: 'http://grafana:3000/explore',
    help: '指标下钻跳转的 Grafana 入口',
  },
  jaegerUrl: {
    key: 'obs.jaegerUrl',
    label: 'Jaeger 地址',
    placeholder: 'http://jaeger:16686',
    help: '链路追踪查询入口',
  },
};

export default function ObservabilityTab() {
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [savingKey, setSavingKey] = useState<string | null>(null);
  const [settings, setSettings] = useState<ObservabilitySettings | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const cfg = await fetchObservabilitySettings();
      setSettings(cfg);
      form.setFieldsValue({
        alertmanagerUrl: cfg.alertmanagerUrl,
        grafanaExploreUrl: cfg.grafanaExploreUrl,
        jaegerUrl: cfg.jaegerUrl,
      });
    } catch (error) {
      message.error(extractErrorMessage(error, '加载观测配置失败'));
    } finally {
      setLoading(false);
    }
  }, [form, message]);

  useEffect(() => {
    load();
  }, [load]);

  const saveField = async (field: string) => {
    const meta = FIELDS[field];
    if (!meta) return;
    const value = (form.getFieldValue(field) as string | undefined)?.trim() ?? '';
    setSavingKey(field);
    try {
      if (value) {
        await setSiteSetting(meta.key, value);
      } else {
        // 空值 = 清除覆盖，恢复跟随环境变量/默认。
        await clearSiteSetting(meta.key);
      }
      message.success('已保存并即时生效');
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '保存失败'));
    } finally {
      setSavingKey(null);
    }
  };

  const sourceBadge = (field: string) => {
    const src: SettingSource | undefined = settings?.sources?.[FIELDS[field].key];
    if (src === 'database') return <Tag color="orange">数据库覆盖</Tag>;
    if (src === 'config') return <Tag color="blue">环境变量</Tag>;
    return <Tag>未配置</Tag>;
  };

  return (
    <Card loading={loading}>
      <Text type="secondary">
        观测平台集成入口：配置后运维中心的告警/指标/链路页会携带这些地址做跳转。
        存入数据库后重启不丢失；清空输入保存即恢复跟随环境变量默认。
      </Text>
      <Form form={form} layout="vertical" style={{ maxWidth: 640, marginTop: 16 }}>
        {Object.entries(FIELDS).map(([field, meta]) => (
          <Form.Item
            key={field}
            label={
              <Space>
                {meta.label}
                {sourceBadge(field)}
              </Space>
            }
            help={meta.help}
            required={false}
          >
            <Space.Compact style={{ width: '100%' }}>
              <Form.Item name={field} noStyle>
                <Input placeholder={meta.placeholder} />
              </Form.Item>
              <Button type="primary" loading={savingKey === field} onClick={() => saveField(field)}>
                保存
              </Button>
              {settings?.sources?.[meta.key] === 'database' ? (
                <Tooltip title="删除数据库覆盖，恢复为环境变量/默认值">
                  <Button
                    loading={savingKey === field}
                    onClick={() => {
                      form.setFieldValue(field, '');
                      saveField(field);
                    }}
                  >
                    恢复
                  </Button>
                </Tooltip>
              ) : null}
            </Space.Compact>
          </Form.Item>
        ))}
      </Form>
    </Card>
  );
}
