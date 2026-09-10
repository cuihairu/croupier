import React, { useCallback, useEffect, useRef, useState } from 'react';
import { App, Button, Card, Form, Input, InputNumber, Space, Switch, Tag, Typography } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  clearSiteSetting,
  fetchNotificationSettings,
  setSiteSetting,
  type NotificationSettings,
} from '@/services/api/sites';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

/** 展示文案经 intl 解析（key 是 L3 设置键，行为契约不迁移） */
type FieldMsg = { id: string; defaultMessage: string };

type FieldDef = {
  key: string;
  label: FieldMsg;
  placeholder?: string;
  placeholderMsg?: FieldMsg;
  help?: FieldMsg;
  secret?: boolean;
};

const SMTP_FIELDS: FieldDef[] = [
  {
    key: 'notification.smtpHost',
    label: {
      id: 'pages.systemSiteSettings.notification.field.smtpHostLabel',
      defaultMessage: 'SMTP 服务器',
    },
    placeholder: 'smtp.example.com',
  },
  {
    key: 'notification.smtpPort',
    label: {
      id: 'pages.systemSiteSettings.notification.field.smtpPortLabel',
      defaultMessage: 'SMTP 端口',
    },
    placeholder: '465',
    kind: 'int',
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
      defaultMessage: 'SMTP 密码',
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
] as FieldDef[];

const DINGTALK_FIELDS: FieldDef[] = [
  {
    key: 'notification.dingtalkUrl',
    label: {
      id: 'pages.systemSiteSettings.notification.field.groupWebhookLabel',
      defaultMessage: '群机器人 Webhook',
    },
    placeholder: 'https://oapi.dingtalk.com/robot/send?access_token=…',
    help: {
      id: 'pages.systemSiteSettings.notification.field.dingtalkUrlHelp',
      defaultMessage: '钉钉群 → 群设置 → 机器人 → 添加"自定义"机器人',
    },
  },
  {
    key: 'notification.dingtalkSecret',
    label: {
      id: 'pages.systemSiteSettings.notification.field.dingtalkSecretLabel',
      defaultMessage: '加签密钥（SEC…）',
    },
    placeholder: 'SEC…',
    secret: true,
    help: {
      id: 'pages.systemSiteSettings.notification.field.dingtalkSecretHelp',
      defaultMessage: '机器人安全设置选择"加签"时必填',
    },
  },
];

const WECOM_FIELDS: FieldDef[] = [
  {
    key: 'notification.wecomUrl',
    label: {
      id: 'pages.systemSiteSettings.notification.field.groupWebhookLabel',
      defaultMessage: '群机器人 Webhook',
    },
    placeholder: 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=…',
    help: {
      id: 'pages.systemSiteSettings.notification.field.wecomUrlHelp',
      defaultMessage: '企业微信群 → 群设置 → 群机器人 → 添加机器人（key 由 URL 携带，无加签）',
    },
  },
];

const FEISHU_FIELDS: FieldDef[] = [
  {
    key: 'notification.feishuUrl',
    label: {
      id: 'pages.systemSiteSettings.notification.field.groupWebhookLabel',
      defaultMessage: '群机器人 Webhook',
    },
    placeholder: 'https://open.feishu.cn/open-apis/bot/v2/hook/…',
    help: {
      id: 'pages.systemSiteSettings.notification.field.feishuUrlHelp',
      defaultMessage: '飞书群 → 设置 → 群机器人 → 添加"自定义机器人"',
    },
  },
  {
    key: 'notification.feishuSecret',
    label: {
      id: 'pages.systemSiteSettings.notification.field.feishuSecretLabel',
      defaultMessage: '加签密钥',
    },
    placeholderMsg: {
      id: 'pages.systemSiteSettings.notification.field.feishuSecretPlaceholder',
      defaultMessage: '签名校验密钥',
    },
    secret: true,
    help: {
      id: 'pages.systemSiteSettings.notification.field.feishuSecretHelp',
      defaultMessage: '机器人安全设置开启"签名校验"时必填',
    },
  },
];

const WEBHOOK_FIELDS: FieldDef[] = [
  {
    key: 'notification.webhookUrl',
    label: {
      id: 'pages.systemSiteSettings.notification.field.webhookUrlLabel',
      defaultMessage: 'Webhook 地址',
    },
    placeholder: 'https://your-receiver.example.com/hook',
  },
  {
    key: 'notification.webhookSecret',
    label: {
      id: 'pages.systemSiteSettings.notification.field.webhookSecretLabel',
      defaultMessage: '签名密钥',
    },
    placeholderMsg: {
      id: 'pages.systemSiteSettings.notification.field.webhookSecretPlaceholder',
      defaultMessage: 'HMAC-SHA256 密钥',
    },
    secret: true,
    help: {
      id: 'pages.systemSiteSettings.notification.field.webhookSecretHelp',
      defaultMessage: '请求头 X-Croupier-Signature: sha256=…（对 body 的 HMAC）',
    },
  },
];

export default function NotificationTab() {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让请求
  // effect 无限重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [savingKey, setSavingKey] = useState<string | null>(null);
  const [settings, setSettings] = useState<NotificationSettings | null>(null);

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
        'notification.dingtalkUrl': cfg.dingtalkUrl || undefined,
        'notification.dingtalkSecret': undefined,
        'notification.webhookUrl': cfg.webhookUrl || undefined,
        'notification.webhookSecret': undefined,
        'notification.wecomUrl': cfg.wecomUrl || undefined,
        'notification.feishuUrl': cfg.feishuUrl || undefined,
        'notification.feishuSecret': undefined,
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
    load();
  }, [load]);

  const saveKey = async (key: string) => {
    const value = form.getFieldValue(key);
    setSavingKey(key);
    try {
      const trimmed = typeof value === 'string' ? value.trim() : value;
      if (trimmed === undefined || trimmed === '' || trimmed === null) {
        // 空值 = 清除覆盖。
        await clearSiteSetting(key);
      } else {
        await setSiteSetting(key, trimmed);
      }
      message.success(
        intl.formatMessage({
          id: 'pages.systemSiteSettings.notification.saved',
          defaultMessage: '已保存',
        }),
      );
      load();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
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
        intl.formatMessage({
          id: next
            ? 'pages.systemSiteSettings.notification.toggle.on'
            : 'pages.systemSiteSettings.notification.toggle.off',
          defaultMessage: next ? '已开启' : '已关闭',
        }),
      );
      load();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
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

  const renderField = (f: FieldDef) => (
    <Form.Item
      key={f.key}
      label={
        <Space>
          {intl.formatMessage(f.label)}
          {f.secret && settings
            ? secretState(
                f.key === 'notification.smtpPassword'
                  ? settings.smtpPasswordSet
                  : f.key === 'notification.dingtalkSecret'
                    ? settings.dingtalkSecretSet
                    : settings.webhookSecretSet,
                f.key === 'notification.smtpPassword'
                  ? settings.smtpPasswordMasked
                  : f.key === 'notification.dingtalkSecret'
                    ? settings.dingtalkSecretMasked
                    : settings.webhookSecretMasked,
              )
            : null}
        </Space>
      }
      help={f.help ? intl.formatMessage(f.help) : undefined}
      required={false}
    >
      <Space.Compact style={{ width: '100%' }}>
        <Form.Item name={f.key} noStyle>
          <Input.Password
            placeholder={f.placeholderMsg ? intl.formatMessage(f.placeholderMsg) : f.placeholder}
            visibilityToggle={f.secret}
            autoComplete="new-password"
          />
        </Form.Item>
        <Button type="primary" loading={savingKey === f.key} onClick={() => saveKey(f.key)}>
          <FormattedMessage
            id="pages.systemSiteSettings.notification.action.save"
            defaultMessage="保存"
          />
        </Button>
      </Space.Compact>
    </Form.Item>
  );

  const smtpPortField = (
    <Form.Item
      key="notification.smtpPort"
      label={intl.formatMessage({
        id: 'pages.systemSiteSettings.notification.field.smtpPortLabel',
        defaultMessage: 'SMTP 端口',
      })}
      required={false}
    >
      <Space.Compact>
        <Form.Item name="notification.smtpPort" noStyle>
          <InputNumber min={1} max={65535} placeholder="465" style={{ width: 120 }} />
        </Form.Item>
        <Button
          type="primary"
          loading={savingKey === 'notification.smtpPort'}
          onClick={() => saveKey('notification.smtpPort')}
        >
          <FormattedMessage
            id="pages.systemSiteSettings.notification.action.save"
            defaultMessage="保存"
          />
        </Button>
      </Space.Compact>
    </Form.Item>
  );

  return (
    <Card loading={loading}>
      <Text type="secondary">
        <FormattedMessage
          id="pages.systemSiteSettings.notification.hint"
          defaultMessage="审批与告警事件的通知渠道。站内信默认开启（零配置）；钉钉/通用 Webhook/邮件按需配置，保存即生效。密钥只回显尾 4 位，留空保存即清除。"
        />
      </Text>

      <Form form={form} layout="vertical" style={{ maxWidth: 640, marginTop: 16 }}>
        <Space size="large" style={{ marginBottom: 8 }}>
          <Space>
            <Text strong>
              <FormattedMessage
                id="pages.systemSiteSettings.notification.toggle.inApp"
                defaultMessage="站内信"
              />
            </Text>
            <Switch
              checked={settings?.inAppEnabled ?? true}
              loading={savingKey === 'notification.inAppEnabled'}
              onChange={(v) => toggleBool('notification.inAppEnabled', v)}
            />
          </Space>
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
              onChange={(v) => toggleBool('notification.emailEnabled', v)}
            />
          </Space>
        </Space>

        {settings?.emailEnabled ? (
          <>
            {SMTP_FIELDS.filter((f) => f.key !== 'notification.smtpPort').map(renderField)}
            {smtpPortField}
          </>
        ) : null}

        <Typography.Title level={5} style={{ marginTop: 16 }}>
          <FormattedMessage
            id="pages.systemSiteSettings.notification.channel.dingtalk"
            defaultMessage="钉钉群机器人"
          />
        </Typography.Title>
        {DINGTALK_FIELDS.map(renderField)}

        <Typography.Title level={5} style={{ marginTop: 16 }}>
          <FormattedMessage
            id="pages.systemSiteSettings.notification.channel.wecom"
            defaultMessage="企业微信群机器人"
          />
        </Typography.Title>
        {WECOM_FIELDS.map(renderField)}

        <Typography.Title level={5} style={{ marginTop: 16 }}>
          <FormattedMessage
            id="pages.systemSiteSettings.notification.channel.feishu"
            defaultMessage="飞书群机器人"
          />
        </Typography.Title>
        {FEISHU_FIELDS.map(renderField)}

        <Typography.Title level={5} style={{ marginTop: 16 }}>
          <FormattedMessage
            id="pages.systemSiteSettings.notification.channel.webhook"
            defaultMessage="通用 Webhook"
          />
        </Typography.Title>
        {WEBHOOK_FIELDS.map(renderField)}
      </Form>
    </Card>
  );
}
