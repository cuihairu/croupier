import React, { useCallback, useEffect, useRef, useState } from 'react';
import { App, Button, Card, Form, Input, Space, Tag, Tooltip, Typography } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  clearSiteSetting,
  fetchObservabilitySettings,
  setSiteSetting,
  type ObservabilitySettings,
  type SettingSource,
} from '@/services/api/sites';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

/** 展示文案经 intl 解析（key 是 L3 设置键，行为契约不迁移） */
type FieldMsg = { id: string; defaultMessage: string };

const FIELDS: Record<
  string,
  { key: string; label: FieldMsg; placeholder: string; help: FieldMsg }
> = {
  alertmanagerUrl: {
    key: 'obs.alertmanagerUrl',
    label: {
      id: 'pages.systemSiteSettings.observability.field.alertmanagerUrlLabel',
      defaultMessage: 'Alertmanager 地址',
    },
    placeholder: 'http://alertmanager:9093',
    help: {
      id: 'pages.systemSiteSettings.observability.field.alertmanagerUrlHelp',
      defaultMessage: '运维中心告警页跳转用的 Alertpush 源',
    },
  },
  grafanaExploreUrl: {
    key: 'obs.grafanaExploreUrl',
    label: {
      id: 'pages.systemSiteSettings.observability.field.grafanaExploreUrlLabel',
      defaultMessage: 'Grafana Explore 地址',
    },
    placeholder: 'http://grafana:3000/explore',
    help: {
      id: 'pages.systemSiteSettings.observability.field.grafanaExploreUrlHelp',
      defaultMessage: '指标下钻跳转的 Grafana 入口',
    },
  },
  jaegerUrl: {
    key: 'obs.jaegerUrl',
    label: {
      id: 'pages.systemSiteSettings.observability.field.jaegerUrlLabel',
      defaultMessage: 'Jaeger 地址',
    },
    placeholder: 'http://jaeger:16686',
    help: {
      id: 'pages.systemSiteSettings.observability.field.jaegerUrlHelp',
      defaultMessage: '链路追踪查询入口',
    },
  },
};

export default function ObservabilityTab() {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让请求
  // effect 无限重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
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
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.observability.error.loadFailed',
            defaultMessage: '加载观测配置失败',
          }),
        ),
      );
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
      message.success(
        intl.formatMessage({
          id: 'pages.systemSiteSettings.observability.saved',
          defaultMessage: '已保存并即时生效',
        }),
      );
      load();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.systemSiteSettings.observability.error.saveFailed',
            defaultMessage: '保存失败',
          }),
        ),
      );
    } finally {
      setSavingKey(null);
    }
  };

  const sourceBadge = (field: string) => {
    const src: SettingSource | undefined = settings?.sources?.[FIELDS[field].key];
    if (src === 'database')
      return (
        <Tag color="orange">
          <FormattedMessage
            id="pages.systemSiteSettings.observability.source.dbOverride"
            defaultMessage="数据库覆盖"
          />
        </Tag>
      );
    if (src === 'config')
      return (
        <Tag color="blue">
          <FormattedMessage
            id="pages.systemSiteSettings.observability.source.envVar"
            defaultMessage="环境变量"
          />
        </Tag>
      );
    return (
      <Tag>
        <FormattedMessage
          id="pages.systemSiteSettings.observability.source.unconfigured"
          defaultMessage="未配置"
        />
      </Tag>
    );
  };

  return (
    <Card loading={loading}>
      <Text type="secondary">
        <FormattedMessage
          id="pages.systemSiteSettings.observability.hint"
          defaultMessage="观测平台集成入口：配置后运维中心的告警/指标/链路页会携带这些地址做跳转。 存入数据库后重启不丢失；清空输入保存即恢复跟随环境变量默认。"
        />
      </Text>
      <Form form={form} layout="vertical" style={{ maxWidth: 640, marginTop: 16 }}>
        {Object.entries(FIELDS).map(([field, meta]) => (
          <Form.Item
            key={field}
            label={
              <Space>
                {intl.formatMessage(meta.label)}
                {sourceBadge(field)}
              </Space>
            }
            help={intl.formatMessage(meta.help)}
            required={false}
          >
            <Space.Compact style={{ width: '100%' }}>
              <Form.Item name={field} noStyle>
                <Input placeholder={meta.placeholder} />
              </Form.Item>
              <Button type="primary" loading={savingKey === field} onClick={() => saveField(field)}>
                <FormattedMessage
                  id="pages.systemSiteSettings.observability.action.save"
                  defaultMessage="保存"
                />
              </Button>
              {settings?.sources?.[meta.key] === 'database' ? (
                <Tooltip
                  title={intl.formatMessage({
                    id: 'pages.systemSiteSettings.observability.resetTooltip',
                    defaultMessage: '删除数据库覆盖，恢复为环境变量/默认值',
                  })}
                >
                  <Button
                    loading={savingKey === field}
                    onClick={() => {
                      form.setFieldValue(field, '');
                      saveField(field);
                    }}
                  >
                    <FormattedMessage
                      id="pages.systemSiteSettings.observability.action.restore"
                      defaultMessage="恢复"
                    />
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
