import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  App,
  Button,
  Card,
  Col,
  Form,
  Input,
  Row,
  Space,
  Switch,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import { ApiOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import {
  clearSiteSetting,
  fetchAuthSnapshot,
  setSiteSetting,
  testAuthConnection,
  type AuthProviderSnapshot,
  type AuthSnapshot,
} from '@/services/api/sites';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

/** 字段来源标签：database=UI 覆盖，config=配置文件，default=未配置 */
function SourceTag({ source }: { source?: string }) {
  if (!source) return null;
  const meta: Record<string, { color: string; label: string }> = {
    database: { color: 'blue', label: 'UI' },
    yaml: { color: 'orange', label: '配置文件' },
    config: { color: 'orange', label: '配置文件' },
    default: { color: 'default', label: '默认' },
  };
  const m = meta[source];
  if (!m) return null;
  return (
    <Tag color={m.color} style={{ marginRight: 0 }}>
      {m.label}
    </Tag>
  );
}

type LDAPFormValues = {
  enabled: boolean;
  addr: string;
  baseDn: string;
  bindDn: string;
  bindPassword: string;
  userFilter: string;
  startTls: boolean;
  defaultRoles: string;
};

type OIDCFormValues = {
  enabled: boolean;
  issuer: string;
  clientId: string;
  clientSecret: string;
  redirectUrl: string;
  defaultRoles: string;
};

/** 保存一组键值：空字符串清除覆盖（回落配置文件），secret 留空跳过。 */
async function saveKeys(
  entries: Array<{ key: string; value: unknown; isSecret?: boolean }>,
): Promise<number> {
  let saved = 0;
  for (const { key, value, isSecret } of entries) {
    if (isSecret && (value === '' || value === undefined || value === null)) continue;
    if (value === '' || value === undefined || value === null) {
      await clearSiteSetting(key);
    } else {
      await setSiteSetting(key, value);
    }
    saved += 1;
  }
  return saved;
}

function LDAPCard({
  snapshot,
  onReload,
}: {
  snapshot: AuthProviderSnapshot | undefined;
  onReload: () => Promise<void>;
}) {
  const { message, modal } = App.useApp();
  const [form] = Form.useForm<LDAPFormValues>();
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);

  useEffect(() => {
    const f = snapshot?.fields ?? {};
    form.setFieldsValue({
      enabled: snapshot?.enabled ?? false,
      addr: f.addr ?? '',
      baseDn: f.baseDn ?? '',
      bindDn: f.bindDn ?? '',
      bindPassword: '',
      userFilter: f.userFilter ?? '',
      startTls: f.startTls === 'true',
      defaultRoles: f.defaultRoles ?? '',
    });
  }, [snapshot, form]);

  const collect = (v: LDAPFormValues) => [
    { key: 'auth.ldap.enabled', value: v.enabled },
    { key: 'auth.ldap.addr', value: v.addr?.trim() ?? '' },
    { key: 'auth.ldap.baseDn', value: v.baseDn?.trim() ?? '' },
    { key: 'auth.ldap.bindDn', value: v.bindDn?.trim() ?? '' },
    { key: 'auth.ldap.bindPassword', value: v.bindPassword, isSecret: true },
    { key: 'auth.ldap.userFilter', value: v.userFilter?.trim() ?? '' },
    { key: 'auth.ldap.startTls', value: v.startTls },
    { key: 'auth.ldap.defaultRoles', value: v.defaultRoles?.trim() ?? '' },
  ];

  const handleSave = async (test: boolean) => {
    try {
      const v = await form.validateFields();
      setSaving(true);
      await saveKeys(collect(v));
      await onReload();
      if (test) {
        setTesting(true);
        try {
          const r = await testAuthConnection('ldap');
          if (r.ok) message.success(r.message);
          else modal.warning({ title: 'LDAP 连接失败', content: r.message });
        } catch (error) {
          modal.warning({
            title: 'LDAP 连接失败',
            content: extractErrorMessage(error, '测试失败'),
          });
        } finally {
          setTesting(false);
        }
      } else {
        message.success('LDAP 配置已保存');
      }
    } catch (error) {
      if ((error as { errorFields?: unknown }).errorFields) return; // 表单校验错误已提示
      message.error(extractErrorMessage(error, '保存失败'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card
      size="small"
      title={
        <Space size={6}>
          <SafetyCertificateOutlined />
          <Text strong>LDAP 目录</Text>
          {snapshot?.enabled ? <Tag color="green">已启用</Tag> : <Tag>未启用</Tag>}
        </Space>
      }
      extra={<Text type="secondary">用户名/密码在原登录框输入，本地校验失败自动级联</Text>}
    >
      <Form form={form} layout="vertical" size="small">
        <Row gutter={12}>
          <Col span={12}>
            <Form.Item
              name="addr"
              label={
                <Space size={4}>
                  目录地址
                  <SourceTag source={snapshot?.sources?.addr} />
                </Space>
              }
              rules={[{ required: true, message: 'ldap://host:389 或 ldaps://host:636' }]}
            >
              <Input placeholder="ldap://ldap.example.com:389" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="baseDn"
              label={
                <Space size={4}>
                  Base DN
                  <SourceTag source={snapshot?.sources?.baseDn} />
                </Space>
              }
            >
              <Input placeholder="dc=example,dc=com" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="bindDn"
              label={
                <Space size={4}>
                  Bind DN（只读账号）
                  <SourceTag source={snapshot?.sources?.bindDn} />
                </Space>
              }
            >
              <Input placeholder="cn=readonly,dc=example,dc=com" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="bindPassword"
              label={
                <Space size={4}>
                  Bind 密码
                  {snapshot?.secretSet ? (
                    <Tooltip title={`已保存：${snapshot.secretMasked}，留空保持不变`}>
                      <Tag color="purple" style={{ marginRight: 0 }}>
                        {snapshot.secretMasked}
                      </Tag>
                    </Tooltip>
                  ) : null}
                </Space>
              }
            >
              <Input.Password
                placeholder={snapshot?.secretSet ? '留空保持不变' : '未设置'}
                autoComplete="new-password"
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="userFilter"
              label={
                <Space size={4}>
                  用户过滤器
                  <SourceTag source={snapshot?.sources?.userFilter} />
                </Space>
              }
              tooltip="占位符 {username} 会替换为登录输入；留空走 userDnTemplate（配置文件）"
            >
              <Input placeholder="(&(objectClass=person)(uid={username}))" />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="startTls" label="StartTLS" valuePropName="checked">
              <Switch size="small" />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item
              name="defaultRoles"
              label={
                <Space size={4}>
                  JIT 角色
                  <SourceTag source={snapshot?.sources?.defaultRoles} />
                </Space>
              }
              tooltip="首次登录自动建号时赋予的角色（逗号分隔）"
            >
              <Input placeholder="viewer" />
            </Form.Item>
          </Col>
          <Col span={24}>
            <Form.Item name="enabled" label="启用 LDAP 登录" valuePropName="checked">
              <Switch
                checkedChildren="启用"
                unCheckedChildren="停用"
                onChange={(checked) => form.setFieldValue('enabled', checked)}
              />
            </Form.Item>
          </Col>
        </Row>
        <Space>
          <Button
            type="primary"
            size="small"
            loading={saving}
            onClick={() => void handleSave(false)}
          >
            保存
          </Button>
          <Button
            size="small"
            icon={<ApiOutlined />}
            loading={testing}
            onClick={() => void handleSave(true)}
          >
            保存并测试连接
          </Button>
        </Space>
      </Form>
    </Card>
  );
}

function OIDCCard({
  snapshot,
  onReload,
}: {
  snapshot: AuthProviderSnapshot | undefined;
  onReload: () => Promise<void>;
}) {
  const { message, modal } = App.useApp();
  const [form] = Form.useForm<OIDCFormValues>();
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);

  useEffect(() => {
    const f = snapshot?.fields ?? {};
    form.setFieldsValue({
      enabled: snapshot?.enabled ?? false,
      issuer: f.issuer ?? '',
      clientId: f.clientId ?? '',
      clientSecret: '',
      redirectUrl: f.redirectUrl ?? '',
      defaultRoles: f.defaultRoles ?? '',
    });
  }, [snapshot, form]);

  const handleSave = async (test: boolean) => {
    try {
      const v = await form.validateFields();
      setSaving(true);
      await saveKeys([
        { key: 'auth.oidc.enabled', value: v.enabled },
        { key: 'auth.oidc.issuer', value: v.issuer?.trim() ?? '' },
        { key: 'auth.oidc.clientId', value: v.clientId?.trim() ?? '' },
        { key: 'auth.oidc.clientSecret', value: v.clientSecret, isSecret: true },
        { key: 'auth.oidc.redirectUrl', value: v.redirectUrl?.trim() ?? '' },
        { key: 'auth.oidc.defaultRoles', value: v.defaultRoles?.trim() ?? '' },
      ]);
      await onReload();
      if (test) {
        setTesting(true);
        try {
          const r = await testAuthConnection('oidc');
          if (r.ok) message.success(r.message);
          else modal.warning({ title: 'OIDC 连接失败', content: r.message });
        } catch (error) {
          modal.warning({
            title: 'OIDC 连接失败',
            content: extractErrorMessage(error, '测试失败'),
          });
        } finally {
          setTesting(false);
        }
      } else {
        message.success('OIDC 配置已保存');
      }
    } catch (error) {
      if ((error as { errorFields?: unknown }).errorFields) return;
      message.error(extractErrorMessage(error, '保存失败'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card
      size="small"
      title={
        <Space size={6}>
          <ApiOutlined />
          <Text strong>OIDC 单点登录</Text>
          {snapshot?.enabled ? <Tag color="green">已启用</Tag> : <Tag>未启用</Tag>}
        </Space>
      }
      extra={<Text type="secondary">启用后登录页出现「SSO 登录」入口</Text>}
    >
      <Form form={form} layout="vertical" size="small">
        <Row gutter={12}>
          <Col span={12}>
            <Form.Item
              name="issuer"
              label={
                <Space size={4}>
                  Issuer
                  <SourceTag source={snapshot?.sources?.issuer} />
                </Space>
              }
              rules={[{ required: true, message: '如 https://sso.example.com' }]}
            >
              <Input placeholder="https://sso.example.com" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="clientId"
              label={
                <Space size={4}>
                  Client ID
                  <SourceTag source={snapshot?.sources?.clientId} />
                </Space>
              }
              rules={[{ required: true }]}
            >
              <Input placeholder="croupier-console" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="clientSecret"
              label={
                <Space size={4}>
                  Client Secret
                  {snapshot?.secretSet ? (
                    <Tooltip title={`已保存：${snapshot.secretMasked}，留空保持不变`}>
                      <Tag color="purple" style={{ marginRight: 0 }}>
                        {snapshot.secretMasked}
                      </Tag>
                    </Tooltip>
                  ) : null}
                </Space>
              }
            >
              <Input.Password
                placeholder={snapshot?.secretSet ? '留空保持不变' : '未设置'}
                autoComplete="new-password"
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="redirectUrl"
              label={
                <Space size={4}>
                  回调地址
                  <SourceTag source={snapshot?.sources?.redirectUrl} />
                </Space>
              }
              tooltip="身份源侧登记的回调：https://<host>/api/v1/auth/oidc/callback"
            >
              <Input placeholder="https://croupier.example.com/api/v1/auth/oidc/callback" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="defaultRoles"
              label={
                <Space size={4}>
                  JIT 角色
                  <SourceTag source={snapshot?.sources?.defaultRoles} />
                </Space>
              }
              tooltip="首次登录自动建号时赋予的角色（逗号分隔）"
            >
              <Input placeholder="viewer" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="enabled" label="启用 SSO 登录" valuePropName="checked">
              <Switch checkedChildren="启用" unCheckedChildren="停用" />
            </Form.Item>
          </Col>
        </Row>
        <Space>
          <Button
            type="primary"
            size="small"
            loading={saving}
            onClick={() => void handleSave(false)}
          >
            保存
          </Button>
          <Button
            size="small"
            icon={<ApiOutlined />}
            loading={testing}
            onClick={() => void handleSave(true)}
          >
            保存并测试发现端点
          </Button>
        </Space>
      </Form>
    </Card>
  );
}

/** 登录方式 Tab：LDAP 直连级联 + OIDC 重定向 SSO（Harbor 模式热配置）。 */
export default function AuthTab() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [snapshot, setSnapshot] = useState<AuthSnapshot | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setSnapshot(await fetchAuthSnapshot());
    } catch (error) {
      message.error(extractErrorMessage(error, '加载登录方式配置失败'));
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    void load();
  }, [load]);

  const extra = useMemo(
    () => (
      <Text type="secondary" style={{ fontSize: 12 }}>
        配置文件仅作初始值，此处保存后热生效（无需重启）；本地账号登录始终可用
      </Text>
    ),
    [],
  );

  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      {extra}
      <LDAPCard snapshot={snapshot?.ldap} onReload={load} />
      <OIDCCard snapshot={snapshot?.oidc} onReload={load} />
    </Space>
  );
}
