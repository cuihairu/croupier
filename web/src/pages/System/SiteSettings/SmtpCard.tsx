/**
 * SMTP 邮件服务卡（运维 Tab，OPEN-ISSUES #55）。
 *
 * 通知邮件通道的 SMTP 传输配置自 NotificationTab 迁入运维家族并扩充：
 * 加密方式（自动/无/SSL/STARTTLS）、认证方式（PLAIN/AUTH LOGIN）、
 * 跳过 TLS 证书校验（自签证书场景），外加既有服务器/端口/用户名/密码
 * （密码即访问令牌口径）/发件人地址。保存即热生效（notify 服务每次发送
 * 前读 L3 快照）。
 *
 * 测试邮件入口已在 #51c 批次补欠（POST /site/notification/test-email）。
 * 边界（诚实）：跳过证书校验仅建议自签内网邮服使用，公网邮服开启有中间人风险。
 */
import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  App,
  Alert,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
} from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  clearSiteSetting,
  fetchNotificationSettings,
  sendTestEmail,
  setSiteSetting,
  type NotificationSettings,
} from '@/services/api/sites';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

type TextMsg = { id: string; defaultMessage: string };

const STRING_FIELDS: { key: string; label: TextMsg; placeholder?: string; secret?: boolean }[] = [
  {
    key: 'notification.smtpHost',
    label: {
      id: 'pages.systemSiteSettings.notification.field.smtpHostLabel',
      defaultMessage: 'SMTP 服务器',
    },
    placeholder: 'smtp.example.com',
  },
  {
    key: 'notification.smtpUser',
    label: {
      id: 'pages.systemSiteSettings.notification.field.smtpUserLabel',
      defaultMessage: 'SMTP 用户名',
    },
    placeholder: 'noreply@example.com',
  },
  {
    key: 'notification.smtpPassword',
    label: {
      id: 'pages.systemSiteSettings.notification.field.smtpPasswordLabel',
      defaultMessage: 'SMTP 密码 / 访问令牌',
    },
    placeholder: '••••••••',
    secret: true,
  },
  {
    key: 'notification.smtpFrom',
    label: {
      id: 'pages.systemSiteSettings.notification.field.fromAddressLabel',
      defaultMessage: '发件人地址',
    },
    placeholder: 'Croupier <noreply@example.com>',
  },
];

export default function SmtpCard() {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，经 ref 转发保持回调依赖稳定
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [savingKey, setSavingKey] = useState<string | null>(null);
  const [settings, setSettings] = useState<NotificationSettings | null>(null);
  const [sendingTest, setSendingTest] = useState(false);

  // #51c：发送测试邮件（仅校验该输入框，不整表校验）
  const sendTest = async () => {
    try {
      await form.validateFields(['testEmailTo']);
    } catch {
      return; // 校验错误已内联展示
    }
    const to = String(form.getFieldValue('testEmailTo') ?? '').trim();
    if (!to) return;
    setSendingTest(true);
    try {
      await sendTestEmail(to);
      message.success(
        intlRef.current.formatMessage({
          id: 'pages.systemSiteSettings.smtp.testEmailSent',
          defaultMessage: '测试邮件已发送，请查收',
        }),
      );
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.smtp.testEmailFailed',
            defaultMessage: '发送失败',
          }),
        ),
      );
    } finally {
      setSendingTest(false);
    }
  };

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const cfg = await fetchNotificationSettings();
      setSettings(cfg);
      form.setFieldsValue({
        'notification.smtpHost': cfg.smtpHost || undefined,
        'notification.smtpPort': cfg.smtpPort || undefined,
        'notification.smtpUser': cfg.smtpUser || undefined,
        'notification.smtpFrom': cfg.smtpFrom || undefined,
        'notification.smtpEncryption': cfg.smtpEncryption || undefined,
        'notification.smtpAuthType': cfg.smtpAuthType || undefined,
        'notification.smtpInsecureSkipVerify': cfg.smtpInsecureSkipVerify,
      });
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.notification.error.loadFailed',
            defaultMessage: '加载通知配置失败',
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

  const saveKey = async (key: string) => {
    const value = form.getFieldValue(key);
    setSavingKey(key);
    try {
      const trimmed = typeof value === 'string' ? value.trim() : value;
      if (trimmed === undefined || trimmed === '' || trimmed === null) {
        // 空值 = 清除覆盖（回自动/默认语义）。
        await clearSiteSetting(key);
      } else {
        await setSiteSetting(key, trimmed);
      }
      message.success(
        intlRef.current.formatMessage({
          id: 'pages.systemSiteSettings.notification.saved',
          defaultMessage: '已保存',
        }),
      );
      void load();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.notification.error.saveFailed',
            defaultMessage: '保存失败',
          }),
        ),
      );
    } finally {
      setSavingKey(null);
    }
  };

  const toggleBool = async (key: string, next: boolean) => {
    setSavingKey(key);
    try {
      await setSiteSetting(key, next);
      message.success(
        intlRef.current.formatMessage({
          id: next
            ? 'pages.systemSiteSettings.notification.toggle.on'
            : 'pages.systemSiteSettings.notification.toggle.off',
          defaultMessage: next ? '已开启' : '已关闭',
        }),
      );
      void load();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.notification.error.operationFailed',
            defaultMessage: '操作失败',
          }),
        ),
      );
    } finally {
      setSavingKey(null);
    }
  };

  const secretState = (set: boolean, masked?: string) =>
    set ? (
      <Tag color="orange">
        <FormattedMessage
          id="pages.systemSiteSettings.notification.secret.configured"
          defaultMessage={`已配置 ${masked ?? ''}`}
          values={{ masked: masked ?? '' }}
        />
      </Tag>
    ) : (
      <Tag>
        <FormattedMessage
          id="pages.systemSiteSettings.notification.secret.unconfigured"
          defaultMessage="未配置"
        />
      </Tag>
    );

  const fmt = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  const saveButton = (key: string) => (
    <Button type="primary" loading={savingKey === key} onClick={() => void saveKey(key)}>
      <FormattedMessage
        id="pages.systemSiteSettings.notification.action.save"
        defaultMessage="保存"
      />
    </Button>
  );

  return (
    <Card
      title={
        <FormattedMessage id="pages.systemSiteSettings.smtp.title" defaultMessage="SMTP 邮件服务" />
      }
      loading={loading}
    >
      <Text type="secondary">
        <FormattedMessage
          id="pages.systemSiteSettings.smtp.hint"
          defaultMessage="审批与告警事件的邮件通道传输配置；加密方式留空 = 自动（465 端口隐式 TLS，其余端口在服务器宣告时升级 STARTTLS）。保存即热生效。"
        />
      </Text>
      <Form form={form} layout="vertical" style={{ maxWidth: 640, marginTop: 16 }}>
        <Space size="large" style={{ marginBottom: 8 }}>
          <Space>
            <Text strong>
              <FormattedMessage
                id="pages.systemSiteSettings.notification.toggle.email"
                defaultMessage="邮件通知"
              />
            </Text>
            <Switch
              checked={settings?.emailEnabled ?? false}
              loading={savingKey === 'notification.emailEnabled'}
              onChange={(v) => void toggleBool('notification.emailEnabled', v)}
            />
          </Space>
        </Space>

        {settings?.emailEnabled ? (
          <>
            {STRING_FIELDS.map((f) => (
              <Form.Item
                key={f.key}
                label={
                  <Space>
                    {intl.formatMessage(f.label)}
                    {f.secret && settings
                      ? secretState(settings.smtpPasswordSet, settings.smtpPasswordMasked)
                      : null}
                  </Space>
                }
                required={false}
              >
                <Space.Compact style={{ width: '100%' }}>
                  <Form.Item name={f.key} noStyle>
                    <Input.Password
                      placeholder={f.placeholder}
                      visibilityToggle={f.secret}
                      autoComplete="new-password"
                    />
                  </Form.Item>
                  {saveButton(f.key)}
                </Space.Compact>
              </Form.Item>
            ))}
            <Form.Item
              label={fmt('pages.systemSiteSettings.notification.field.smtpPortLabel', 'SMTP 端口')}
              required={false}
            >
              <Space.Compact>
                <Form.Item name="notification.smtpPort" noStyle>
                  <InputNumber min={1} max={65535} placeholder="465" style={{ width: 120 }} />
                </Form.Item>
                {saveButton('notification.smtpPort')}
              </Space.Compact>
            </Form.Item>
            <Form.Item
              label={fmt('pages.systemSiteSettings.smtp.encryption', '加密方式')}
              tooltip={fmt(
                'pages.systemSiteSettings.smtp.encryptionTooltip',
                '自动：465 端口走隐式 TLS，其余端口服务器宣告时升级 STARTTLS',
              )}
              required={false}
            >
              <Space.Compact style={{ width: '100%' }}>
                <Form.Item name="notification.smtpEncryption" noStyle>
                  <Select
                    allowClear
                    placeholder={fmt('pages.systemSiteSettings.smtp.encryptionAuto', '自动')}
                    style={{ width: 200 }}
                    options={[
                      {
                        value: 'none',
                        label: fmt('pages.systemSiteSettings.smtp.encryptionNone', '无（明文）'),
                      },
                      {
                        value: 'ssl',
                        label: fmt(
                          'pages.systemSiteSettings.smtp.encryptionSsl',
                          'SSL/TLS（隐式）',
                        ),
                      },
                      {
                        value: 'starttls',
                        label: fmt(
                          'pages.systemSiteSettings.smtp.encryptionStarttls',
                          'STARTTLS（强制）',
                        ),
                      },
                    ]}
                  />
                </Form.Item>
                {saveButton('notification.smtpEncryption')}
              </Space.Compact>
            </Form.Item>
            <Form.Item
              label={fmt('pages.systemSiteSettings.smtp.authType', '认证方式')}
              tooltip={fmt(
                'pages.systemSiteSettings.smtp.authTypeTooltip',
                '部分邮服仅支持 AUTH LOGIN；PLAIN 为标准默认',
              )}
              required={false}
            >
              <Space.Compact style={{ width: '100%' }}>
                <Form.Item name="notification.smtpAuthType" noStyle>
                  <Select
                    allowClear
                    placeholder={fmt(
                      'pages.systemSiteSettings.smtp.authTypePlain',
                      'PLAIN（默认）',
                    )}
                    style={{ width: 200 }}
                    options={[
                      {
                        value: 'login',
                        label: fmt(
                          'pages.systemSiteSettings.smtp.authTypeLogin',
                          'AUTH LOGIN（强制）',
                        ),
                      },
                    ]}
                  />
                </Form.Item>
                {saveButton('notification.smtpAuthType')}
              </Space.Compact>
            </Form.Item>
            <Form.Item required={false}>
              <Space>
                <Switch
                  checked={settings?.smtpInsecureSkipVerify ?? false}
                  loading={savingKey === 'notification.smtpInsecureSkipVerify'}
                  onChange={(v) => void toggleBool('notification.smtpInsecureSkipVerify', v)}
                />
                <Text strong>
                  <FormattedMessage
                    id="pages.systemSiteSettings.smtp.skipVerify"
                    defaultMessage="跳过 TLS 证书校验"
                  />
                </Text>
              </Space>
            </Form.Item>
            <Alert
              type="warning"
              showIcon
              title={intl.formatMessage({
                id: 'pages.systemSiteSettings.smtp.skipVerifyHint',
                defaultMessage:
                  '跳过校验仅建议自签证书的内网邮服使用；公网邮服开启将暴露中间人风险。',
              })}
            />
            {/* #51c：发送测试邮件（真实发信，验证 SMTP 配置链路） */}
            <Form.Item
              label={fmt('pages.systemSiteSettings.smtp.testEmailLabel', '发送测试邮件')}
              tooltip={fmt(
                'pages.systemSiteSettings.smtp.testEmailTooltip',
                '按当前已保存的 SMTP 配置真实发信（不受上方未保存的表单草稿影响）',
              )}
              required={false}
            >
              <Space.Compact style={{ width: '100%' }}>
                <Form.Item
                  name="testEmailTo"
                  noStyle
                  rules={[
                    {
                      type: 'email',
                      message: fmt(
                        'pages.systemSiteSettings.smtp.testEmailInvalid',
                        '邮箱格式无效',
                      ),
                    },
                  ]}
                >
                  <Input placeholder="you@example.com" autoComplete="off" />
                </Form.Item>
                <Button loading={sendingTest} onClick={() => void sendTest()}>
                  <FormattedMessage
                    id="pages.systemSiteSettings.smtp.testEmailAction"
                    defaultMessage="发送"
                  />
                </Button>
              </Space.Compact>
            </Form.Item>
          </>
        ) : null}
      </Form>
    </Card>
  );
}
