import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  App,
  Button,
  Card,
  Col,
  Form,
  InputNumber,
  Row,
  Space,
  Switch,
  Tag,
  Typography,
} from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import { clearSiteSetting, fetchSecuritySettings, setSiteSetting } from '@/services/api/sites';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

/** security.* 账号安全策略快照（Source: layered.go SecurityPolicySnapshot） */
type SecuritySettings = {
  mfaRequired?: boolean;
  passwordMinLength?: number;
  passwordRequireUppercase?: boolean;
  passwordRequireSpecial?: boolean;
  passwordMaxAgeDays?: number;
};

const BOOL_KEYS = [
  {
    key: 'security.mfaRequired',
    field: 'mfaRequired' as const,
    label: {
      id: 'pages.systemSiteSettings.security.mfaRequired',
      defaultMessage: '强制二次验证 (TOTP)',
    },
    help: {
      id: 'pages.systemSiteSettings.security.mfaRequiredHelp',
      defaultMessage:
        '开启后所有本地账号必须绑定 TOTP：未绑定账号登录时收到引导，其余 API 返回 403 mfa_required（外部身份源账号由 IdP 负责，不受影响）',
    },
  },
  {
    key: 'security.passwordRequireUppercase',
    field: 'passwordRequireUppercase' as const,
    label: {
      id: 'pages.systemSiteSettings.security.requireUppercase',
      defaultMessage: '密码必须含大写字母',
    },
    help: {
      id: 'pages.systemSiteSettings.security.requireUppercaseHelp',
      defaultMessage: '作用于建号、重置密码与自助修改密码',
    },
  },
  {
    key: 'security.passwordRequireSpecial',
    field: 'passwordRequireSpecial' as const,
    label: {
      id: 'pages.systemSiteSettings.security.requireSpecial',
      defaultMessage: '密码必须含特殊字符',
    },
    help: {
      id: 'pages.systemSiteSettings.security.requireSpecialHelp',
      defaultMessage: '作用于建号、重置密码与自助修改密码',
    },
  },
];

const INT_KEYS = [
  {
    key: 'security.passwordMinLength',
    field: 'passwordMinLength' as const,
    label: { id: 'pages.systemSiteSettings.security.minLength', defaultMessage: '密码最小长度' },
    help: {
      id: 'pages.systemSiteSettings.security.minLengthHelp',
      defaultMessage: '0 = 沿用内置基线 8（上限 128）；仅可收紧不可放宽',
    },
  },
  {
    key: 'security.passwordMaxAgeDays',
    field: 'passwordMaxAgeDays' as const,
    label: {
      id: 'pages.systemSiteSettings.security.maxAgeDays',
      defaultMessage: '密码有效期（天）',
    },
    help: {
      id: 'pages.systemSiteSettings.security.maxAgeDaysHelp',
      defaultMessage: '0 = 永不过期；开启后改密/建号自当刻起计时，过期登录强制走改密流程',
    },
  },
];

export default function SecurityTab() {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用；经 ref 转发保持回调依赖稳定
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [savingKey, setSavingKey] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const cfg = await fetchSecuritySettings();
      form.setFieldsValue({
        mfaRequired: !!cfg.mfaRequired,
        passwordRequireUppercase: !!cfg.passwordRequireUppercase,
        passwordRequireSpecial: !!cfg.passwordRequireSpecial,
        passwordMinLength: cfg.passwordMinLength || 0,
        passwordMaxAgeDays: cfg.passwordMaxAgeDays || 0,
      });
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.security.error.loadFailed',
            defaultMessage: '加载账号安全策略失败',
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

  // field = Form.Item 名（表单取值路径），key = settings 键（存储路径），两者分开传
  const saveKey = async (key: string, field: string) => {
    const value = form.getFieldValue(field);
    setSavingKey(key);
    try {
      const normalized =
        typeof value === 'string'
          ? value.trim()
          : value === undefined || value === null
            ? ''
            : value;
      if (normalized === '' || normalized === false || normalized === 0) {
        // 空值/关闭/0 = 清除覆盖回到默认（全关基线）
        await clearSiteSetting(key);
      } else {
        await setSiteSetting(key, normalized);
      }
      message.success(
        intl.formatMessage({
          id: 'pages.systemSiteSettings.security.saved',
          defaultMessage: '已保存',
        }),
      );
      load();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.security.error.saveFailed',
            defaultMessage: '保存失败',
          }),
        ),
      );
    } finally {
      setSavingKey(null);
    }
  };

  return (
    <Card loading={loading}>
      <Form form={form} layout="vertical">
        <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
          <Text type="secondary">
            <FormattedMessage
              id="pages.systemSiteSettings.security.intro"
              defaultMessage="全部默认关闭：关闭时维持内置基线（密码 8-128 位、弱密码拦截、至少两类字符、不限期、TOTP 自助绑定）。"
            />
          </Text>
          {BOOL_KEYS.map((item) => (
            <Row key={item.key} gutter={12} align="middle">
              <Col>
                <Form.Item name={item.field} valuePropName="checked" style={{ marginBottom: 0 }}>
                  <Switch />
                </Form.Item>
              </Col>
              <Col flex="auto">
                <Space size={4}>
                  <Text strong>{intl.formatMessage(item.label)}</Text>
                  <Button
                    size="small"
                    type="link"
                    loading={savingKey === item.key}
                    onClick={() => saveKey(item.key, item.field)}
                  >
                    <FormattedMessage
                      id="pages.systemSiteSettings.security.save"
                      defaultMessage="保存"
                    />
                  </Button>
                </Space>
                <div>
                  <Text type="secondary">{intl.formatMessage(item.help)}</Text>
                </div>
              </Col>
            </Row>
          ))}
          {INT_KEYS.map((item) => (
            <Row key={item.key} gutter={12} align="middle">
              <Col>
                <Form.Item name={item.field} style={{ marginBottom: 0 }}>
                  <InputNumber min={0} max={3650} style={{ width: 120 }} />
                </Form.Item>
              </Col>
              <Col flex="auto">
                <Space size={4}>
                  <Text strong>
                    {intl.formatMessage(item.label)} <Tag>{item.key}</Tag>
                  </Text>
                  <Button
                    size="small"
                    type="link"
                    loading={savingKey === item.key}
                    onClick={() => saveKey(item.key, item.field)}
                  >
                    <FormattedMessage
                      id="pages.systemSiteSettings.security.save"
                      defaultMessage="保存"
                    />
                  </Button>
                </Space>
                <div>
                  <Text type="secondary">{intl.formatMessage(item.help)}</Text>
                </div>
              </Col>
            </Row>
          ))}
        </Space>
      </Form>
    </Card>
  );
}
