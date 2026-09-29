/**
 * 出站安全与限制卡（账号安全 Tab 第二卡，OPEN-ISSUES #56）。
 *
 * sec.* 四键：允许端口清单 / 允许 IP/CIDR 清单 / 域名后缀白名单 /
 * SSRF 保护开关。清单为空 = 不限（清除覆盖即回不限）；保存即热生效
 * （外呼前每次读 L3 快照）。
 *
 * 边界（诚实）：守卫只作用于用户可配置 URL 的出站 HTTP——通知 webhook
 * （钉钉/飞书/企微/通用）与「检查更新」拉取；agent/DB/SDK 通道不经过
 * 守卫。默认全关零行为变更。SSRF 保护同时做静态解析校验与真实连接前
 * 复核（消除 DNS TOCTOU）。
 */
import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Alert, App, Button, Card, Form, Input, Space, Switch, Typography } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import { clearSiteSetting, fetchOutboundSettings, setSiteSetting } from '@/services/api/sites';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

type TextMsg = { id: string; defaultMessage: string };

const LIST_FIELDS: {
  key: string;
  field: string;
  label: TextMsg;
  help: TextMsg;
  placeholder: string;
}[] = [
  {
    key: 'sec.allowPorts',
    field: 'allowPorts',
    label: { id: 'pages.systemSiteSettings.outbound.allowPorts', defaultMessage: '允许的端口' },
    help: {
      id: 'pages.systemSiteSettings.outbound.allowPortsHelp',
      defaultMessage: '逗号分隔端口清单（1-65535），如 443,8080；空 = 不限',
    },
    placeholder: '443,8080',
  },
  {
    key: 'sec.allowIPs',
    field: 'allowIPs',
    label: { id: 'pages.systemSiteSettings.outbound.allowIPs', defaultMessage: '允许的私有 IP' },
    help: {
      id: 'pages.systemSiteSettings.outbound.allowIPsHelp',
      defaultMessage:
        'SSRF 保护拦截内网目标时的放行清单（单 IP 或 CIDR，如 10.0.0.0/8）；空 = 无放行',
    },
    placeholder: '10.0.0.0/8,127.0.0.1',
  },
  {
    key: 'sec.domainFilter',
    field: 'domainFilter',
    label: {
      id: 'pages.systemSiteSettings.outbound.domainFilter',
      defaultMessage: '域名过滤（允许清单）',
    },
    help: {
      id: 'pages.systemSiteSettings.outbound.domainFilterHelp',
      defaultMessage:
        '逗号分隔域名后缀，子域自动放行（example.com 覆盖 api.example.com）；空 = 不限',
    },
    placeholder: 'example.com,foo.io',
  },
];

export default function OutboundSecurityCard() {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用；经 ref 转发保持回调依赖稳定
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [savingKey, setSavingKey] = useState<string | null>(null);
  const [ssrfProtection, setSsrfProtection] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const cfg = await fetchOutboundSettings();
      form.setFieldsValue({
        allowPorts: cfg.allowPorts || undefined,
        allowIPs: cfg.allowIPs || undefined,
        domainFilter: cfg.domainFilter || undefined,
      });
      setSsrfProtection(!!cfg.ssrfProtection);
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.outbound.error.loadFailed',
            defaultMessage: '加载出站安全配置失败',
          }),
        ),
      );
    } finally {
      setLoading(false);
    }
  }, [form, message]);

  useEffect(() => {
    void load();
  }, [load]);

  const saveKey = async (key: string, field: string) => {
    const value = form.getFieldValue(field);
    setSavingKey(key);
    try {
      const trimmed = typeof value === 'string' ? value.trim() : '';
      if (trimmed === '') {
        // 空值 = 清除覆盖（回「不限」语义）。
        await clearSiteSetting(key);
      } else {
        await setSiteSetting(key, trimmed);
      }
      message.success(
        intlRef.current.formatMessage({
          id: 'pages.systemSiteSettings.outbound.saved',
          defaultMessage: '已保存',
        }),
      );
      void load();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.outbound.error.saveFailed',
            defaultMessage: '保存失败',
          }),
        ),
      );
    } finally {
      setSavingKey(null);
    }
  };

  const toggleSsrf = async (next: boolean) => {
    setSavingKey('sec.ssrfProtection');
    try {
      if (next) {
        await setSiteSetting('sec.ssrfProtection', true);
      } else {
        // 关闭 = 清除覆盖回默认（不拦截）。
        await clearSiteSetting('sec.ssrfProtection');
      }
      setSsrfProtection(next);
      message.success(
        intlRef.current.formatMessage({
          id: 'pages.systemSiteSettings.outbound.saved',
          defaultMessage: '已保存',
        }),
      );
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.outbound.error.saveFailed',
            defaultMessage: '保存失败',
          }),
        ),
      );
    } finally {
      setSavingKey(null);
    }
  };

  return (
    <Card
      title={
        <FormattedMessage
          id="pages.systemSiteSettings.outbound.title"
          defaultMessage="出站安全与限制"
        />
      }
      loading={loading}
    >
      <Text type="secondary">
        <FormattedMessage
          id="pages.systemSiteSettings.outbound.hint"
          defaultMessage="限制平台对外发起的 HTTP 请求（通知 webhook、检查更新）。全部默认关闭：关闭时 outbound 行为与既往完全一致。"
        />
      </Text>
      <Form form={form} layout="vertical" style={{ maxWidth: 640, marginTop: 16 }}>
        {LIST_FIELDS.map((f) => (
          <Form.Item
            key={f.key}
            label={intl.formatMessage(f.label)}
            extra={intl.formatMessage(f.help)}
            required={false}
          >
            <Space.Compact style={{ width: '100%' }}>
              <Form.Item name={f.field} noStyle>
                <Input placeholder={f.placeholder} allowClear autoComplete="off" />
              </Form.Item>
              <Button
                type="primary"
                loading={savingKey === f.key}
                onClick={() => void saveKey(f.key, f.field)}
              >
                <FormattedMessage
                  id="pages.systemSiteSettings.outbound.save"
                  defaultMessage="保存"
                />
              </Button>
            </Space.Compact>
          </Form.Item>
        ))}
        <Form.Item required={false}>
          <Space>
            <Switch
              checked={ssrfProtection}
              loading={savingKey === 'sec.ssrfProtection'}
              onChange={(v) => void toggleSsrf(v)}
            />
            <Text strong>
              <FormattedMessage
                id="pages.systemSiteSettings.outbound.ssrfProtection"
                defaultMessage="SSRF 保护"
              />
            </Text>
          </Space>
        </Form.Item>
        <Text type="secondary">
          <FormattedMessage
            id="pages.systemSiteSettings.outbound.ssrfProtectionHelp"
            defaultMessage="开启后出站目标解析或连接到私有/回环/链路本地地址即拒绝（可用「允许的私有 IP」放行内网依赖）；真实连接前二次复核，DNS 重绑定无效。"
          />
        </Text>
        <Alert
          style={{ marginTop: 16 }}
          type="info"
          showIcon
          title={intl.formatMessage({
            id: 'pages.systemSiteSettings.outbound.boundary',
            defaultMessage:
              '守卫仅覆盖用户可配置 URL 的出站 HTTP（通知 webhook 与检查更新）；agent / 数据库 / SDK 通道不受限。域名过滤为允许清单语义（配置后仅清单内域名可出站）。',
          })}
        />
      </Form>
    </Card>
  );
}
