import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { App, Button, Card, Form, Input, Space, Tabs, Tag, Tooltip, Typography } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import {
  clearSiteSetting,
  fetchSiteConfig,
  setSiteSetting,
  type SettingSource,
} from '@/services/api/sites';
import { extractErrorMessage } from '@/utils/errors';
import FeatureFlagsTab from './FeatureFlagsTab';
import ObservabilityTab from './ObservabilityTab';

const { Text } = Typography;

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
      message.error(extractErrorMessage(error, '加载站点配置失败'));
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
      message.success('已保存并即时生效');
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '保存失败'));
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
      message.success('已恢复跟随配置文件');
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '操作失败'));
    } finally {
      setSavingKey(null);
    }
  };

  const sourceBadge = (field: string, key: FieldKey) => {
    const src = sources[key];
    if (!src) return null;
    if (src === 'database') {
      return <Tag color="orange">数据库覆盖</Tag>;
    }
    if (src === 'config') {
      return <Tag color="blue">跟随配置文件</Tag>;
    }
    return <Tag>默认</Tag>;
  };

  const fieldWithActions = (
    field: string,
    label: string,
    placeholder: string,
    textArea?: boolean,
  ) => {
    const key = FIELD_KEYS[field];
    return (
      <Form.Item
        label={
          <Space>
            {label}
            {sourceBadge(field, key)}
          </Space>
        }
        required={false}
      >
        <Space.Compact style={{ width: '100%' }}>
          {textArea ? (
            <Form.Item name={field} noStyle>
              <Input.TextArea rows={2} placeholder={placeholder} />
            </Form.Item>
          ) : (
            <Form.Item name={field} noStyle>
              <Input placeholder={placeholder} />
            </Form.Item>
          )}
          <Button type="primary" loading={savingKey === field} onClick={() => saveField(field)}>
            保存
          </Button>
          {overridden[field] ? (
            <Tooltip title="删除数据库覆盖，恢复为配置文件中的值">
              <Button loading={savingKey === field} onClick={() => resetField(field)}>
                恢复
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
          配置分层：代码默认 ← 配置文件 ← 此处覆盖（最高）。「恢复」按钮会删除覆盖、回到配置文件值。
          修改即时生效，无需重启。
        </Text>
        <Form form={form} layout="vertical" style={{ maxWidth: 640, marginTop: 16 }}>
          {fieldWithActions('siteName', '站点名称', 'Croupier')}
          {fieldWithActions('logoUrl', 'Logo 地址', '/logo.svg 或 https://…')}
          {fieldWithActions('faviconUrl', 'Favicon 地址', '/favicon.ico')}
          {fieldWithActions('description', '登录页副标题', '', true)}
          {fieldWithActions('copyright', '页脚版权', '© 2026 Your Company')}
          {fieldWithActions('icp', 'ICP 备案号', '京ICP备XXXXXXXX号')}
        </Form>
      </Card>
    ),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [loading, form, sources, overridden, savingKey],
  );

  return (
    <PageContainer>
      <Tabs
        defaultActiveKey="site"
        items={[
          { key: 'site', label: '站点信息', children: siteTab },
          { key: 'features', label: '功能开关', children: <FeatureFlagsTab /> },
          { key: 'observability', label: '观测集成', children: <ObservabilityTab /> },
        ]}
      />
    </PageContainer>
  );
}
