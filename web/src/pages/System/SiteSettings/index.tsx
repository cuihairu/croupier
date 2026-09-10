import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { App, Button, Card, Form, Input, Space, Tabs, Tag, Tooltip, Typography } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  clearSiteSetting,
  fetchSiteConfig,
  setSiteSetting,
  type SettingSource,
} from '@/services/api/sites';
import { extractErrorMessage } from '@/utils/errors';
import AuthTab from './AuthTab';
import FeatureFlagsTab from './FeatureFlagsTab';
import ObservabilityTab from './ObservabilityTab';
import NotificationTab from './NotificationTab';

const { Text } = Typography;

/** 展示文案经 intl 解析（FIELD_KEYS 的 value 是 L3 设置键，行为契约不迁移） */
type FieldMsg = { id: string; defaultMessage: string };

type FieldKey =
  | 'site.name'
  | 'site.logoUrl'
  | 'site.faviconUrl'
  | 'site.description'
  | 'footer.copyright'
  | 'footer.icp';

// 表单字段与 L3 key 的映射（footer.links 走独立编辑，P1 暂用 JSON 输入）
const FIELD_KEYS: Record<string, FieldKey> = {
  siteName: 'site.name',
  logoUrl: 'site.logoUrl',
  faviconUrl: 'site.faviconUrl',
  description: 'site.description',
  copyright: 'footer.copyright',
  icp: 'footer.icp',
};

export default function SiteSettingsPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让请求
  // effect 无限重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [savingKey, setSavingKey] = useState<string | null>(null);
  const [sources, setSources] = useState<Record<string, SettingSource>>({});
  // 记录每个字段的 L3 是否被覆盖（决定显示「恢复跟随配置文件」）
  const [overridden, setOverridden] = useState<Record<string, boolean>>({});

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const cfg = await fetchSiteConfig();
      form.setFieldsValue({
        siteName: cfg.siteName,
        logoUrl: cfg.logoUrl,
        faviconUrl: cfg.faviconUrl,
        description: cfg.description,
        copyright: cfg.footerCopyright,
        icp: cfg.footerIcp,
      });
      const src = (cfg as { sources?: Record<string, SettingSource> }).sources || {};
      setSources(src);
      setOverridden({
        siteName: src['site.name'] === 'database',
        logoUrl: src['site.logoUrl'] === 'database',
        faviconUrl: src['site.faviconUrl'] === 'database',
        description: src['site.description'] === 'database',
        copyright: src['footer.copyright'] === 'database',
        icp: src['footer.icp'] === 'database',
      });
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.error.loadFailed',
            defaultMessage: '加载站点配置失败',
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
    const key = FIELD_KEYS[field];
    if (!key) return;
    const value = (form.getFieldValue(field) as string | undefined)?.trim() ?? '';
    if (!value) return;
    setSavingKey(field);
    try {
      await setSiteSetting(key, value);
      message.success(
        intl.formatMessage({
          id: 'pages.systemSiteSettings.saved',
          defaultMessage: '已保存并即时生效',
        }),
      );
      load();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.systemSiteSettings.error.saveFailed',
            defaultMessage: '保存失败',
          }),
        ),
      );
    } finally {
      setSavingKey(null);
    }
  };

  const resetField = async (field: string) => {
    const key = FIELD_KEYS[field];
    if (!key) return;
    setSavingKey(field);
    try {
      await clearSiteSetting(key);
      message.success(
        intl.formatMessage({
          id: 'pages.systemSiteSettings.resetSuccess',
          defaultMessage: '已恢复跟随配置文件',
        }),
      );
      load();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.systemSiteSettings.error.operationFailed',
            defaultMessage: '操作失败',
          }),
        ),
      );
    } finally {
      setSavingKey(null);
    }
  };

  const sourceBadge = (field: string, key: FieldKey) => {
    const src = sources[key];
    if (!src) return null;
    if (src === 'database') {
      return (
        <Tag color="orange">
          <FormattedMessage
            id="pages.systemSiteSettings.source.dbOverride"
            defaultMessage="数据库覆盖"
          />
        </Tag>
      );
    }
    if (src === 'config') {
      return (
        <Tag color="blue">
          <FormattedMessage
            id="pages.systemSiteSettings.source.configFile"
            defaultMessage="跟随配置文件"
          />
        </Tag>
      );
    }
    return (
      <Tag>
        <FormattedMessage id="pages.systemSiteSettings.source.default" defaultMessage="默认" />
      </Tag>
    );
  };

  const fieldWithActions = (
    field: string,
    label: FieldMsg,
    placeholder?: string | FieldMsg,
    textArea?: boolean,
  ) => {
    const key = FIELD_KEYS[field];
    const placeholderText =
      typeof placeholder === 'string' || placeholder === undefined
        ? placeholder
        : intl.formatMessage(placeholder);
    return (
      <Form.Item
        label={
          <Space>
            {intl.formatMessage(label)}
            {sourceBadge(field, key)}
          </Space>
        }
        required={false}
      >
        <Space.Compact style={{ width: '100%' }}>
          {textArea ? (
            <Form.Item name={field} noStyle>
              <Input.TextArea rows={2} placeholder={placeholderText} />
            </Form.Item>
          ) : (
            <Form.Item name={field} noStyle>
              <Input placeholder={placeholderText} />
            </Form.Item>
          )}
          <Button type="primary" loading={savingKey === field} onClick={() => saveField(field)}>
            <FormattedMessage id="pages.systemSiteSettings.action.save" defaultMessage="保存" />
          </Button>
          {overridden[field] ? (
            <Tooltip
              title={intl.formatMessage({
                id: 'pages.systemSiteSettings.resetTooltip',
                defaultMessage: '删除数据库覆盖，恢复为配置文件中的值',
              })}
            >
              <Button loading={savingKey === field} onClick={() => resetField(field)}>
                <FormattedMessage
                  id="pages.systemSiteSettings.action.reset"
                  defaultMessage="恢复"
                />
              </Button>
            </Tooltip>
          ) : null}
        </Space.Compact>
      </Form.Item>
    );
  };

  const siteTab = useMemo(
    () => (
      <Card loading={loading}>
        <Text type="secondary">
          <FormattedMessage
            id="pages.systemSiteSettings.hint"
            defaultMessage="配置分层：代码默认 ← 配置文件 ← 此处覆盖（最高）。「恢复」按钮会删除覆盖、回到配置文件值。修改即时生效，无需重启。"
          />
        </Text>
        <Form form={form} layout="vertical" style={{ maxWidth: 640, marginTop: 16 }}>
          {fieldWithActions(
            'siteName',
            {
              id: 'pages.systemSiteSettings.field.siteName',
              defaultMessage: '站点名称',
            },
            'Croupier',
          )}
          {fieldWithActions(
            'logoUrl',
            {
              id: 'pages.systemSiteSettings.field.logoUrl',
              defaultMessage: 'Logo 地址',
            },
            {
              id: 'pages.systemSiteSettings.field.logoUrlPlaceholder',
              defaultMessage: '/logo.svg 或 https://…',
            },
          )}
          {fieldWithActions(
            'faviconUrl',
            {
              id: 'pages.systemSiteSettings.field.faviconUrl',
              defaultMessage: 'Favicon 地址',
            },
            '/favicon.ico',
          )}
          {fieldWithActions(
            'description',
            {
              id: 'pages.systemSiteSettings.field.description',
              defaultMessage: '登录页副标题',
            },
            '',
            true,
          )}
          {fieldWithActions(
            'copyright',
            {
              id: 'pages.systemSiteSettings.field.copyright',
              defaultMessage: '页脚版权',
            },
            '© 2026 Your Company',
          )}
          {fieldWithActions(
            'icp',
            {
              id: 'pages.systemSiteSettings.field.icp',
              defaultMessage: 'ICP 备案号',
            },
            {
              id: 'pages.systemSiteSettings.field.icpPlaceholder',
              defaultMessage: '京ICP备XXXXXXXX号',
            },
          )}
        </Form>
      </Card>
    ),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [loading, form, sources, overridden, savingKey, intl],
  );

  return (
    <PageContainer>
      <Tabs
        defaultActiveKey="site"
        items={[
          {
            key: 'site',
            label: intl.formatMessage({
              id: 'pages.systemSiteSettings.tab.site',
              defaultMessage: '站点信息',
            }),
            children: siteTab,
          },
          {
            key: 'features',
            label: intl.formatMessage({
              id: 'pages.systemSiteSettings.tab.features',
              defaultMessage: '功能开关',
            }),
            children: <FeatureFlagsTab />,
          },
          {
            key: 'auth',
            label: intl.formatMessage({
              id: 'pages.systemSiteSettings.tab.auth',
              defaultMessage: '登录方式',
            }),
            children: <AuthTab />,
          },
          {
            key: 'notification',
            label: intl.formatMessage({
              id: 'pages.systemSiteSettings.tab.notification',
              defaultMessage: '通知设置',
            }),
            children: <NotificationTab />,
          },
          {
            key: 'observability',
            label: intl.formatMessage({
              id: 'pages.systemSiteSettings.tab.observability',
              defaultMessage: '观测集成',
            }),
            children: <ObservabilityTab />,
          },
        ]}
      />
    </PageContainer>
  );
}
