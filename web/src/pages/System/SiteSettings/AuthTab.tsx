import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
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
import {
  ApiOutlined,
  GithubOutlined,
  KeyOutlined,
  LockOutlined,
  QrcodeOutlined,
  SafetyCertificateOutlined,
  UserAddOutlined,
} from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  clearSiteSetting,
  fetchAuthSnapshot,
  setSiteSetting,
  testAuthConnection,
  type AuthProviderSnapshot,
  type EmailPolicySnapshot,
  type AuthSnapshot,
  type LocalAuthSnapshot,
} from '@/services/api/sites';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

/** 字段来源标签：database=UI 覆盖，config=配置文件，default=未配置 */
function SourceTag({ source }: { source?: string }) {
  const intl = useIntl();
  if (!source) return null;
  // key 为来源枚举（行为契约）；展示文案经 textId/textDefault 走 intl
  const meta: Record<string, { color: string; textId: string; textDefault: string }> = {
    database: {
      color: 'blue',
      textId: 'pages.systemSiteSettings.auth.sourceTag.ui',
      textDefault: 'UI',
    },
    yaml: {
      color: 'orange',
      textId: 'pages.systemSiteSettings.auth.sourceTag.configFile',
      textDefault: '配置文件',
    },
    config: {
      color: 'orange',
      textId: 'pages.systemSiteSettings.auth.sourceTag.configFile',
      textDefault: '配置文件',
    },
    default: {
      color: 'default',
      textId: 'pages.systemSiteSettings.auth.sourceTag.default',
      textDefault: '默认',
    },
  };
  const m = meta[source];
  if (!m) return null;
  return (
    <Tag color={m.color} style={{ marginRight: 0 }}>
      {intl.formatMessage({ id: m.textId, defaultMessage: m.textDefault })}
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

type LocalFormValues = { enabled: boolean };

/** 本地账号密码登录开关（OPEN-ISSUES #51a）：显式覆盖 auth.local.enabled=false 才停用。 */
function LocalCard({
  snapshot,
  onReload,
}: {
  snapshot: LocalAuthSnapshot | undefined;
  onReload: () => Promise<void>;
}) {
  const { message } = App.useApp();
  const intl = useIntl();
  const [form] = Form.useForm<LocalFormValues>();
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    form.setFieldsValue({ enabled: snapshot?.enabled ?? true });
  }, [snapshot, form]);

  const handleSave = async () => {
    try {
      const v = await form.validateFields();
      setSaving(true);
      await saveKeys([{ key: 'auth.local.enabled', value: v.enabled }]);
      await onReload();
      message.success(
        intl.formatMessage({
          id: 'pages.systemSiteSettings.auth.saved.local',
          defaultMessage: '本地登录配置已保存',
        }),
      );
    } catch (error) {
      if ((error as { errorFields?: unknown }).errorFields) return; // 表单校验错误已提示
      // 后端防锁死守卫拒绝（全部登录方式关闭）时经此提示，设置已被回滚
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.systemSiteSettings.auth.error.saveFailed',
            defaultMessage: '保存失败',
          }),
        ),
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card
      size="small"
      title={
        <Space size={6}>
          <LockOutlined />
          <Text strong>
            <FormattedMessage
              id="pages.systemSiteSettings.auth.local.title"
              defaultMessage="本地账号密码"
            />
          </Text>
          {snapshot?.enabled ? (
            <Tag color="green">
              <FormattedMessage
                id="pages.systemSiteSettings.auth.provider.enabled"
                defaultMessage="已启用"
              />
            </Tag>
          ) : (
            <Tag>
              <FormattedMessage
                id="pages.systemSiteSettings.auth.provider.disabled"
                defaultMessage="未启用"
              />
            </Tag>
          )}
        </Space>
      }
      extra={
        <Text type="secondary">
          <FormattedMessage
            id="pages.systemSiteSettings.auth.hint.localCard"
            defaultMessage="停用后登录页隐藏账密表单；须至少保留一种登录方式"
          />
        </Text>
      }
    >
      <Form form={form} name="auth-local" layout="vertical" size="small">
        <Form.Item
          name="enabled"
          label={intl.formatMessage({
            id: 'pages.systemSiteSettings.auth.local.enableLabel',
            defaultMessage: '启用账号密码登录',
          })}
          valuePropName="checked"
          tooltip={intl.formatMessage({
            id: 'pages.systemSiteSettings.auth.local.enableTooltip',
            defaultMessage:
              '内置管理员账号走此通道；停用前请先确保 LDAP/OIDC/GitHub 之一可用，否则保存会被拒绝以防锁死',
          })}
        >
          <Switch
            checkedChildren={intl.formatMessage({
              id: 'pages.systemSiteSettings.auth.switch.enable',
              defaultMessage: '启用',
            })}
            unCheckedChildren={intl.formatMessage({
              id: 'pages.systemSiteSettings.auth.switch.disable',
              defaultMessage: '停用',
            })}
          />
        </Form.Item>
        <Button type="primary" size="small" loading={saving} onClick={() => void handleSave()}>
          <FormattedMessage id="pages.systemSiteSettings.auth.action.save" defaultMessage="保存" />
        </Button>
      </Form>
    </Card>
  );
}

function LDAPCard({
  snapshot,
  onReload,
}: {
  snapshot: AuthProviderSnapshot | undefined;
  onReload: () => Promise<void>;
}) {
  const { message, modal } = App.useApp();
  const intl = useIntl();
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
          else
            modal.warning({
              title: intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.connectionFailed.ldap',
                defaultMessage: 'LDAP 连接失败',
              }),
              content: r.message,
            });
        } catch (error) {
          modal.warning({
            title: intl.formatMessage({
              id: 'pages.systemSiteSettings.auth.connectionFailed.ldap',
              defaultMessage: 'LDAP 连接失败',
            }),
            content: extractErrorMessage(
              error,
              intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.error.testFailed',
                defaultMessage: '测试失败',
              }),
            ),
          });
        } finally {
          setTesting(false);
        }
      } else {
        message.success(
          intl.formatMessage({
            id: 'pages.systemSiteSettings.auth.saved.ldap',
            defaultMessage: 'LDAP 配置已保存',
          }),
        );
      }
    } catch (error) {
      if ((error as { errorFields?: unknown }).errorFields) return; // 表单校验错误已提示
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.systemSiteSettings.auth.error.saveFailed',
            defaultMessage: '保存失败',
          }),
        ),
      );
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
          <Text strong>
            <FormattedMessage
              id="pages.systemSiteSettings.auth.ldap.title"
              defaultMessage="LDAP 目录"
            />
          </Text>
          {snapshot?.enabled ? (
            <Tag color="green">
              <FormattedMessage
                id="pages.systemSiteSettings.auth.provider.enabled"
                defaultMessage="已启用"
              />
            </Tag>
          ) : (
            <Tag>
              <FormattedMessage
                id="pages.systemSiteSettings.auth.provider.disabled"
                defaultMessage="未启用"
              />
            </Tag>
          )}
        </Space>
      }
      extra={
        <Text type="secondary">
          <FormattedMessage
            id="pages.systemSiteSettings.auth.hint.ldapCard"
            defaultMessage="用户名/密码在原登录框输入，本地校验失败自动级联"
          />
        </Text>
      }
    >
      <Form form={form} name="auth-ldap" layout="vertical" size="small">
        <Row gutter={12}>
          <Col span={12}>
            <Form.Item
              name="addr"
              label={
                <Space size={4}>
                  <FormattedMessage
                    id="pages.systemSiteSettings.auth.dirAddrLabel"
                    defaultMessage="目录地址"
                  />
                  <SourceTag source={snapshot?.sources?.addr} />
                </Space>
              }
              rules={[
                {
                  required: true,
                  message: intl.formatMessage({
                    id: 'pages.systemSiteSettings.auth.dirAddrRule',
                    defaultMessage: 'ldap://host:389 或 ldaps://host:636',
                  }),
                },
              ]}
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
                  <FormattedMessage
                    id="pages.systemSiteSettings.auth.bindDnLabel"
                    defaultMessage="Bind DN（只读账号）"
                  />
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
                  <FormattedMessage
                    id="pages.systemSiteSettings.auth.bindPasswordLabel"
                    defaultMessage="Bind 密码"
                  />
                  {snapshot?.secretSet ? (
                    <Tooltip
                      title={intl.formatMessage(
                        {
                          id: 'pages.systemSiteSettings.auth.secretSavedTooltip',
                          defaultMessage: `已保存：${snapshot.secretMasked}，留空保持不变`,
                        },
                        { masked: snapshot.secretMasked },
                      )}
                    >
                      <Tag color="purple" style={{ marginRight: 0 }}>
                        {snapshot.secretMasked}
                      </Tag>
                    </Tooltip>
                  ) : null}
                </Space>
              }
            >
              <Input.Password
                placeholder={
                  snapshot?.secretSet
                    ? intl.formatMessage({
                        id: 'pages.systemSiteSettings.auth.secretPlaceholderKeep',
                        defaultMessage: '留空保持不变',
                      })
                    : intl.formatMessage({
                        id: 'pages.systemSiteSettings.auth.secretPlaceholderUnset',
                        defaultMessage: '未设置',
                      })
                }
                autoComplete="new-password"
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="userFilter"
              label={
                <Space size={4}>
                  <FormattedMessage
                    id="pages.systemSiteSettings.auth.userFilterLabel"
                    defaultMessage="用户过滤器"
                  />
                  <SourceTag source={snapshot?.sources?.userFilter} />
                </Space>
              }
              tooltip={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.userFilterTooltip',
                // 字面量 {username} 按 ICU 语法转义（真实 intl 渲染为 {username}）
                defaultMessage:
                  "占位符 '{'username'}' 会替换为登录输入；留空走 userDnTemplate（配置文件）",
              })}
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
                  <FormattedMessage
                    id="pages.systemSiteSettings.auth.jitRolesLabel"
                    defaultMessage="JIT 角色"
                  />
                  <SourceTag source={snapshot?.sources?.defaultRoles} />
                </Space>
              }
              tooltip={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.jitRolesTooltip',
                defaultMessage: '首次登录自动建号时赋予的角色（逗号分隔）',
              })}
            >
              <Input placeholder="viewer" />
            </Form.Item>
          </Col>
          <Col span={24}>
            <Form.Item
              name="enabled"
              label={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.enableLdapLabel',
                defaultMessage: '启用 LDAP 登录',
              })}
              valuePropName="checked"
            >
              <Switch
                checkedChildren={intl.formatMessage({
                  id: 'pages.systemSiteSettings.auth.switch.enable',
                  defaultMessage: '启用',
                })}
                unCheckedChildren={intl.formatMessage({
                  id: 'pages.systemSiteSettings.auth.switch.disable',
                  defaultMessage: '停用',
                })}
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
            <FormattedMessage
              id="pages.systemSiteSettings.auth.action.save"
              defaultMessage="保存"
            />
          </Button>
          <Button
            size="small"
            icon={<ApiOutlined />}
            loading={testing}
            onClick={() => void handleSave(true)}
          >
            <FormattedMessage
              id="pages.systemSiteSettings.auth.saveAndTest.ldap"
              defaultMessage="保存并测试连接"
            />
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
  const intl = useIntl();
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
          else
            modal.warning({
              title: intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.connectionFailed.oidc',
                defaultMessage: 'OIDC 连接失败',
              }),
              content: r.message,
            });
        } catch (error) {
          modal.warning({
            title: intl.formatMessage({
              id: 'pages.systemSiteSettings.auth.connectionFailed.oidc',
              defaultMessage: 'OIDC 连接失败',
            }),
            content: extractErrorMessage(
              error,
              intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.error.testFailed',
                defaultMessage: '测试失败',
              }),
            ),
          });
        } finally {
          setTesting(false);
        }
      } else {
        message.success(
          intl.formatMessage({
            id: 'pages.systemSiteSettings.auth.saved.oidc',
            defaultMessage: 'OIDC 配置已保存',
          }),
        );
      }
    } catch (error) {
      if ((error as { errorFields?: unknown }).errorFields) return;
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.systemSiteSettings.auth.error.saveFailed',
            defaultMessage: '保存失败',
          }),
        ),
      );
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
          <Text strong>
            <FormattedMessage
              id="pages.systemSiteSettings.auth.oidc.title"
              defaultMessage="OIDC 单点登录"
            />
          </Text>
          {snapshot?.enabled ? (
            <Tag color="green">
              <FormattedMessage
                id="pages.systemSiteSettings.auth.provider.enabled"
                defaultMessage="已启用"
              />
            </Tag>
          ) : (
            <Tag>
              <FormattedMessage
                id="pages.systemSiteSettings.auth.provider.disabled"
                defaultMessage="未启用"
              />
            </Tag>
          )}
        </Space>
      }
      extra={
        <Text type="secondary">
          <FormattedMessage
            id="pages.systemSiteSettings.auth.hint.oidcCard"
            defaultMessage="启用后登录页出现「SSO 登录」入口"
          />
        </Text>
      }
    >
      <Form form={form} name="auth-oidc" layout="vertical" size="small">
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
              rules={[
                {
                  required: true,
                  message: intl.formatMessage({
                    id: 'pages.systemSiteSettings.auth.issuerRule',
                    defaultMessage: '如 https://sso.example.com',
                  }),
                },
              ]}
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
                    <Tooltip
                      title={intl.formatMessage(
                        {
                          id: 'pages.systemSiteSettings.auth.secretSavedTooltip',
                          defaultMessage: `已保存：${snapshot.secretMasked}，留空保持不变`,
                        },
                        { masked: snapshot.secretMasked },
                      )}
                    >
                      <Tag color="purple" style={{ marginRight: 0 }}>
                        {snapshot.secretMasked}
                      </Tag>
                    </Tooltip>
                  ) : null}
                </Space>
              }
            >
              <Input.Password
                placeholder={
                  snapshot?.secretSet
                    ? intl.formatMessage({
                        id: 'pages.systemSiteSettings.auth.secretPlaceholderKeep',
                        defaultMessage: '留空保持不变',
                      })
                    : intl.formatMessage({
                        id: 'pages.systemSiteSettings.auth.secretPlaceholderUnset',
                        defaultMessage: '未设置',
                      })
                }
                autoComplete="new-password"
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="redirectUrl"
              label={
                <Space size={4}>
                  <FormattedMessage
                    id="pages.systemSiteSettings.auth.callbackUrlLabel"
                    defaultMessage="回调地址"
                  />
                  <SourceTag source={snapshot?.sources?.redirectUrl} />
                </Space>
              }
              tooltip={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.callbackUrlTooltip',
                defaultMessage: '身份源侧登记的回调：https://<host>/api/v1/auth/oidc/callback',
              })}
            >
              <Input placeholder="https://croupier.example.com/api/v1/auth/oidc/callback" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="defaultRoles"
              label={
                <Space size={4}>
                  <FormattedMessage
                    id="pages.systemSiteSettings.auth.jitRolesLabel"
                    defaultMessage="JIT 角色"
                  />
                  <SourceTag source={snapshot?.sources?.defaultRoles} />
                </Space>
              }
              tooltip={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.jitRolesTooltip',
                defaultMessage: '首次登录自动建号时赋予的角色（逗号分隔）',
              })}
            >
              <Input placeholder="viewer" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="enabled"
              label={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.enableSsoLabel',
                defaultMessage: '启用 SSO 登录',
              })}
              valuePropName="checked"
            >
              <Switch
                checkedChildren={intl.formatMessage({
                  id: 'pages.systemSiteSettings.auth.switch.enable',
                  defaultMessage: '启用',
                })}
                unCheckedChildren={intl.formatMessage({
                  id: 'pages.systemSiteSettings.auth.switch.disable',
                  defaultMessage: '停用',
                })}
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
            <FormattedMessage
              id="pages.systemSiteSettings.auth.action.save"
              defaultMessage="保存"
            />
          </Button>
          <Button
            size="small"
            icon={<ApiOutlined />}
            loading={testing}
            onClick={() => void handleSave(true)}
          >
            <FormattedMessage
              id="pages.systemSiteSettings.auth.saveAndTest.oidc"
              defaultMessage="保存并测试发现端点"
            />
          </Button>
        </Space>
      </Form>
    </Card>
  );
}

type GitHubFormValues = {
  enabled: boolean;
  clientId: string;
  clientSecret: string;
  redirectUrl: string;
  defaultRoles: string;
  successUrl: string;
};

/** GitHub OAuth 登录（OPEN-ISSUES #51a）：复用 OIDC 的 state/JIT 建号链路。 */
function GitHubCard({
  snapshot,
  onReload,
}: {
  snapshot: AuthProviderSnapshot | undefined;
  onReload: () => Promise<void>;
}) {
  const { message } = App.useApp();
  const intl = useIntl();
  const [form] = Form.useForm<GitHubFormValues>();
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const f = snapshot?.fields ?? {};
    form.setFieldsValue({
      enabled: snapshot?.enabled ?? false,
      clientId: f.clientId ?? '',
      clientSecret: '',
      redirectUrl: f.redirectUrl ?? '',
      defaultRoles: f.defaultRoles ?? '',
      successUrl: f.successUrl ?? '',
    });
  }, [snapshot, form]);

  const handleSave = async () => {
    try {
      const v = await form.validateFields();
      setSaving(true);
      await saveKeys([
        { key: 'auth.github.enabled', value: v.enabled },
        { key: 'auth.github.clientId', value: v.clientId?.trim() ?? '' },
        { key: 'auth.github.clientSecret', value: v.clientSecret, isSecret: true },
        { key: 'auth.github.redirectUrl', value: v.redirectUrl?.trim() ?? '' },
        { key: 'auth.github.defaultRoles', value: v.defaultRoles?.trim() ?? '' },
        { key: 'auth.github.successUrl', value: v.successUrl?.trim() ?? '' },
      ]);
      await onReload();
      // 启用但凭证不全时后端会拒绝并回滚本次保存（错误信息已说明缺哪项）
      message.success(
        intl.formatMessage({
          id: 'pages.systemSiteSettings.auth.saved.github',
          defaultMessage: 'GitHub 配置已保存',
        }),
      );
    } catch (error) {
      if ((error as { errorFields?: unknown }).errorFields) return;
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.systemSiteSettings.auth.error.saveFailed',
            defaultMessage: '保存失败',
          }),
        ),
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card
      size="small"
      title={
        <Space size={6}>
          <GithubOutlined />
          <Text strong>
            <FormattedMessage
              id="pages.systemSiteSettings.auth.github.title"
              defaultMessage="GitHub OAuth"
            />
          </Text>
          {snapshot?.enabled ? (
            <Tag color="green">
              <FormattedMessage
                id="pages.systemSiteSettings.auth.provider.enabled"
                defaultMessage="已启用"
              />
            </Tag>
          ) : (
            <Tag>
              <FormattedMessage
                id="pages.systemSiteSettings.auth.provider.disabled"
                defaultMessage="未启用"
              />
            </Tag>
          )}
        </Space>
      }
      extra={
        <Text type="secondary">
          <FormattedMessage
            id="pages.systemSiteSettings.auth.hint.githubCard"
            defaultMessage="启用后登录页出现「GitHub 登录」入口，首次登录自动建号"
          />
        </Text>
      }
    >
      <Form form={form} name="auth-github" layout="vertical" size="small">
        <Row gutter={12}>
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
              <Input placeholder="Ov23liXXXXXXXXXXXXXX" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="clientSecret"
              label={
                <Space size={4}>
                  Client Secret
                  {snapshot?.secretSet ? (
                    <Tooltip
                      title={intl.formatMessage(
                        {
                          id: 'pages.systemSiteSettings.auth.secretSavedTooltip',
                          defaultMessage: `已保存：${snapshot.secretMasked}，留空保持不变`,
                        },
                        { masked: snapshot.secretMasked },
                      )}
                    >
                      <Tag color="purple" style={{ marginRight: 0 }}>
                        {snapshot.secretMasked}
                      </Tag>
                    </Tooltip>
                  ) : null}
                </Space>
              }
            >
              <Input.Password
                placeholder={
                  snapshot?.secretSet
                    ? intl.formatMessage({
                        id: 'pages.systemSiteSettings.auth.secretPlaceholderKeep',
                        defaultMessage: '留空保持不变',
                      })
                    : intl.formatMessage({
                        id: 'pages.systemSiteSettings.auth.secretPlaceholderUnset',
                        defaultMessage: '未设置',
                      })
                }
                autoComplete="new-password"
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="redirectUrl"
              label={
                <Space size={4}>
                  <FormattedMessage
                    id="pages.systemSiteSettings.auth.callbackUrlLabel"
                    defaultMessage="回调地址"
                  />
                  <SourceTag source={snapshot?.sources?.redirectUrl} />
                </Space>
              }
              tooltip={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.callbackUrlTooltip.github',
                defaultMessage:
                  'GitHub OAuth App 侧登记的回调：https://<host>/api/v1/auth/github/callback',
              })}
            >
              <Input placeholder="https://croupier.example.com/api/v1/auth/github/callback" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="defaultRoles"
              label={
                <Space size={4}>
                  <FormattedMessage
                    id="pages.systemSiteSettings.auth.jitRolesLabel"
                    defaultMessage="JIT 角色"
                  />
                  <SourceTag source={snapshot?.sources?.defaultRoles} />
                </Space>
              }
              tooltip={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.jitRolesTooltip',
                defaultMessage: '首次登录自动建号时赋予的角色（逗号分隔）',
              })}
            >
              <Input placeholder="viewer" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="successUrl"
              label={
                <Space size={4}>
                  <FormattedMessage
                    id="pages.systemSiteSettings.auth.successUrlLabel"
                    defaultMessage="登录成功跳转"
                  />
                  <SourceTag source={snapshot?.sources?.successUrl} />
                </Space>
              }
              tooltip={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.successUrlTooltip',
                defaultMessage: '回调签发 token 后跳转的前端地址，留空使用 /',
              })}
            >
              <Input placeholder="/" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="enabled"
              label={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.enableGitHubLabel',
                defaultMessage: '启用 GitHub 登录',
              })}
              valuePropName="checked"
            >
              <Switch
                checkedChildren={intl.formatMessage({
                  id: 'pages.systemSiteSettings.auth.switch.enable',
                  defaultMessage: '启用',
                })}
                unCheckedChildren={intl.formatMessage({
                  id: 'pages.systemSiteSettings.auth.switch.disable',
                  defaultMessage: '停用',
                })}
              />
            </Form.Item>
          </Col>
        </Row>
        <Button type="primary" size="small" loading={saving} onClick={() => void handleSave()}>
          <FormattedMessage id="pages.systemSiteSettings.auth.action.save" defaultMessage="保存" />
        </Button>
      </Form>
    </Card>
  );
}

type ExternalOAuthFormValues = {
  enabled: boolean;
  appId: string;
  appSecret: string;
  clientId: string;
  clientSecret: string;
  redirectUrl: string;
  authUrl: string;
  tokenUrl: string;
  userInfoUrl: string;
  scopes: string;
  usernameField: string;
  nicknameField: string;
  emailField: string;
  defaultRoles: string;
  successUrl: string;
};

/** 微信扫码 + 自定义 OAuth2（OPEN-ISSUES #51 第三批）：两卡同构，字段集按
 * provider 切换（微信 5 键 / 自定义 OAuth 13 键）。保存链路与 GitHub 相同：
 * 启用但凭证不全时后端拒绝并回滚（防锁死守卫在 RefreshIdentityProviders）。 */
function ExternalOAuthCard({
  provider,
  snapshot,
  onReload,
}: {
  provider: 'wechat' | 'genericoauth';
  snapshot: AuthProviderSnapshot | undefined;
  onReload: () => Promise<void>;
}) {
  const { message } = App.useApp();
  const intl = useIntl();
  const [form] = Form.useForm<ExternalOAuthFormValues>();
  const [saving, setSaving] = useState(false);
  const isWeChat = provider === 'wechat';
  const keyPrefix = isWeChat ? 'auth.wechat' : 'auth.genericoauth';

  useEffect(() => {
    const f = snapshot?.fields ?? {};
    form.setFieldsValue({
      enabled: snapshot?.enabled ?? false,
      appId: f.appId ?? '',
      appSecret: '',
      clientId: f.clientId ?? '',
      clientSecret: '',
      redirectUrl: f.redirectUrl ?? '',
      authUrl: f.authUrl ?? '',
      tokenUrl: f.tokenUrl ?? '',
      userInfoUrl: f.userInfoUrl ?? '',
      scopes: f.scopes ?? '',
      usernameField: f.usernameField ?? '',
      nicknameField: f.nicknameField ?? '',
      emailField: f.emailField ?? '',
      defaultRoles: f.defaultRoles ?? '',
      successUrl: f.successUrl ?? '',
    });
  }, [snapshot, form]);

  const handleSave = async () => {
    try {
      const v = await form.validateFields();
      setSaving(true);
      const credentialEntries: Array<{ key: string; value: unknown; isSecret?: boolean }> = isWeChat
        ? [
            { key: 'auth.wechat.appId', value: v.appId?.trim() ?? '' },
            { key: 'auth.wechat.appSecret', value: v.appSecret, isSecret: true },
          ]
        : [
            { key: 'auth.genericoauth.clientId', value: v.clientId?.trim() ?? '' },
            { key: 'auth.genericoauth.clientSecret', value: v.clientSecret, isSecret: true },
            { key: 'auth.genericoauth.authUrl', value: v.authUrl?.trim() ?? '' },
            { key: 'auth.genericoauth.tokenUrl', value: v.tokenUrl?.trim() ?? '' },
            { key: 'auth.genericoauth.userInfoUrl', value: v.userInfoUrl?.trim() ?? '' },
            { key: 'auth.genericoauth.scopes', value: v.scopes?.trim() ?? '' },
            { key: 'auth.genericoauth.usernameField', value: v.usernameField?.trim() ?? '' },
            { key: 'auth.genericoauth.nicknameField', value: v.nicknameField?.trim() ?? '' },
            { key: 'auth.genericoauth.emailField', value: v.emailField?.trim() ?? '' },
          ];
      await saveKeys([
        { key: `${keyPrefix}.enabled`, value: v.enabled },
        ...credentialEntries,
        { key: `${keyPrefix}.redirectUrl`, value: v.redirectUrl?.trim() ?? '' },
        { key: `${keyPrefix}.defaultRoles`, value: v.defaultRoles?.trim() ?? '' },
        { key: `${keyPrefix}.successUrl`, value: v.successUrl?.trim() ?? '' },
      ]);
      await onReload();
      // 启用但凭证不全时后端会拒绝并回滚本次保存（错误信息已说明缺哪项）
      message.success(
        intl.formatMessage({
          id: isWeChat
            ? 'pages.systemSiteSettings.auth.saved.wechat'
            : 'pages.systemSiteSettings.auth.saved.genericoauth',
          defaultMessage: isWeChat ? '微信配置已保存' : '自定义 OAuth 配置已保存',
        }),
      );
    } catch (error) {
      if ((error as { errorFields?: unknown }).errorFields) return;
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.systemSiteSettings.auth.error.saveFailed',
            defaultMessage: '保存失败',
          }),
        ),
      );
    } finally {
      setSaving(false);
    }
  };

  const secretSet = snapshot?.secretSet ?? false;
  const secretMasked = snapshot?.secretMasked ?? '';
  return (
    <Card
      size="small"
      title={
        <Space size={6}>
          {isWeChat ? <QrcodeOutlined /> : <KeyOutlined />}
          <Text strong>
            <FormattedMessage
              id={
                isWeChat
                  ? 'pages.systemSiteSettings.auth.wechat.title'
                  : 'pages.systemSiteSettings.auth.genericoauth.title'
              }
              defaultMessage={isWeChat ? '微信扫码登录' : '自定义 OAuth2'}
            />
          </Text>
          {snapshot?.enabled ? (
            <Tag color="green">
              <FormattedMessage
                id="pages.systemSiteSettings.auth.provider.enabled"
                defaultMessage="已启用"
              />
            </Tag>
          ) : (
            <Tag>
              <FormattedMessage
                id="pages.systemSiteSettings.auth.provider.disabled"
                defaultMessage="未启用"
              />
            </Tag>
          )}
        </Space>
      }
      extra={
        <Text type="secondary">
          <FormattedMessage
            id={
              isWeChat
                ? 'pages.systemSiteSettings.auth.hint.wechatCard'
                : 'pages.systemSiteSettings.auth.hint.genericoauthCard'
            }
            defaultMessage={
              isWeChat
                ? '启用后登录页出现「微信扫码登录」入口，首次登录自动建号（用户名=openid）'
                : '适配任意标准授权码流程身份源（Keycloak/Authentik/Auth0 等），首次登录自动建号'
            }
          />
        </Text>
      }
    >
      <Form form={form} name={`auth-${provider}`} layout="vertical" size="small">
        <Row gutter={12}>
          {isWeChat ? (
            <Col span={12}>
              <Form.Item
                name="appId"
                label={
                  <Space size={4}>
                    AppID
                    <SourceTag source={snapshot?.sources?.appId} />
                  </Space>
                }
                rules={[{ required: true }]}
              >
                <Input placeholder="wx1234567890abcdef" />
              </Form.Item>
            </Col>
          ) : (
            <>
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
                  <Input placeholder="croupier" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name="authUrl"
                  label={
                    <Space size={4}>
                      <FormattedMessage
                        id="pages.systemSiteSettings.auth.authUrlLabel"
                        defaultMessage="授权端点 AuthURL"
                      />
                      <SourceTag source={snapshot?.sources?.authUrl} />
                    </Space>
                  }
                  rules={[{ required: true }]}
                >
                  <Input placeholder="https://idp.example.com/authorize" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name="tokenUrl"
                  label={
                    <Space size={4}>
                      <FormattedMessage
                        id="pages.systemSiteSettings.auth.tokenUrlLabel"
                        defaultMessage="令牌端点 TokenURL"
                      />
                      <SourceTag source={snapshot?.sources?.tokenUrl} />
                    </Space>
                  }
                  rules={[{ required: true }]}
                >
                  <Input placeholder="https://idp.example.com/token" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name="userInfoUrl"
                  label={
                    <Space size={4}>
                      <FormattedMessage
                        id="pages.systemSiteSettings.auth.userInfoUrlLabel"
                        defaultMessage="UserInfo 端点"
                      />
                      <SourceTag source={snapshot?.sources?.userInfoUrl} />
                    </Space>
                  }
                  rules={[{ required: true }]}
                >
                  <Input placeholder="https://idp.example.com/userinfo" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name="scopes"
                  label={
                    <Space size={4}>
                      <FormattedMessage
                        id="pages.systemSiteSettings.auth.scopesLabel"
                        defaultMessage="授权范围 Scopes"
                      />
                      <SourceTag source={snapshot?.sources?.scopes} />
                    </Space>
                  }
                  tooltip={intl.formatMessage({
                    id: 'pages.systemSiteSettings.auth.scopesTooltip',
                    defaultMessage: '逗号分隔，如 openid,profile；留空不带 scope',
                  })}
                >
                  <Input placeholder="openid,profile" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name="usernameField"
                  label={
                    <Space size={4}>
                      <FormattedMessage
                        id="pages.systemSiteSettings.auth.usernameFieldLabel"
                        defaultMessage="用户名属性"
                      />
                      <SourceTag source={snapshot?.sources?.usernameField} />
                    </Space>
                  }
                  tooltip={intl.formatMessage({
                    id: 'pages.systemSiteSettings.auth.usernameFieldTooltip',
                    defaultMessage: 'UserInfo JSON 中的属性名，默认 username（仅支持顶层字段）',
                  })}
                >
                  <Input placeholder="username" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name="nicknameField"
                  label={
                    <Space size={4}>
                      <FormattedMessage
                        id="pages.systemSiteSettings.auth.nicknameFieldLabel"
                        defaultMessage="昵称属性"
                      />
                      <SourceTag source={snapshot?.sources?.nicknameField} />
                    </Space>
                  }
                  tooltip={intl.formatMessage({
                    id: 'pages.systemSiteSettings.auth.nicknameFieldTooltip',
                    defaultMessage: 'UserInfo JSON 中的属性名，默认 name',
                  })}
                >
                  <Input placeholder="name" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name="emailField"
                  label={
                    <Space size={4}>
                      <FormattedMessage
                        id="pages.systemSiteSettings.auth.emailFieldLabel"
                        defaultMessage="邮箱属性"
                      />
                      <SourceTag source={snapshot?.sources?.emailField} />
                    </Space>
                  }
                  tooltip={intl.formatMessage({
                    id: 'pages.systemSiteSettings.auth.emailFieldTooltip',
                    defaultMessage: 'UserInfo JSON 中的属性名，默认 email',
                  })}
                >
                  <Input placeholder="email" />
                </Form.Item>
              </Col>
            </>
          )}
          <Col span={12}>
            <Form.Item
              name={isWeChat ? 'appSecret' : 'clientSecret'}
              label={
                <Space size={4}>
                  {isWeChat ? 'AppSecret' : 'Client Secret'}
                  {secretSet ? (
                    <Tooltip
                      title={intl.formatMessage(
                        {
                          id: 'pages.systemSiteSettings.auth.secretSavedTooltip',
                          defaultMessage: `已保存：${secretMasked}，留空保持不变`,
                        },
                        { masked: secretMasked },
                      )}
                    >
                      <Tag color="purple" style={{ marginRight: 0 }}>
                        {secretMasked}
                      </Tag>
                    </Tooltip>
                  ) : null}
                </Space>
              }
            >
              <Input.Password
                placeholder={
                  secretSet
                    ? intl.formatMessage({
                        id: 'pages.systemSiteSettings.auth.secretPlaceholderKeep',
                        defaultMessage: '留空保持不变',
                      })
                    : intl.formatMessage({
                        id: 'pages.systemSiteSettings.auth.secretPlaceholderUnset',
                        defaultMessage: '未设置',
                      })
                }
                autoComplete="new-password"
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="redirectUrl"
              label={
                <Space size={4}>
                  <FormattedMessage
                    id="pages.systemSiteSettings.auth.callbackUrlLabel"
                    defaultMessage="回调地址"
                  />
                  <SourceTag source={snapshot?.sources?.redirectUrl} />
                </Space>
              }
              tooltip={intl.formatMessage({
                id: isWeChat
                  ? 'pages.systemSiteSettings.auth.callbackUrlTooltip.wechat'
                  : 'pages.systemSiteSettings.auth.callbackUrlTooltip.genericoauth',
                defaultMessage: isWeChat
                  ? '微信开放平台「授权回调域」登记：https://<host>/api/v1/auth/wechat/callback'
                  : '身份源侧登记：https://<host>/api/v1/auth/generic/callback',
              })}
            >
              <Input
                placeholder={
                  isWeChat
                    ? 'https://croupier.example.com/api/v1/auth/wechat/callback'
                    : 'https://croupier.example.com/api/v1/auth/generic/callback'
                }
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="defaultRoles"
              label={
                <Space size={4}>
                  <FormattedMessage
                    id="pages.systemSiteSettings.auth.jitRolesLabel"
                    defaultMessage="JIT 角色"
                  />
                  <SourceTag source={snapshot?.sources?.defaultRoles} />
                </Space>
              }
              tooltip={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.jitRolesTooltip',
                defaultMessage: '首次登录自动建号时赋予的角色（逗号分隔）',
              })}
            >
              <Input placeholder="viewer" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="successUrl"
              label={
                <Space size={4}>
                  <FormattedMessage
                    id="pages.systemSiteSettings.auth.successUrlLabel"
                    defaultMessage="登录成功跳转"
                  />
                  <SourceTag source={snapshot?.sources?.successUrl} />
                </Space>
              }
              tooltip={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.successUrlTooltip',
                defaultMessage: '回调签发 token 后跳转的前端地址，留空使用 /',
              })}
            >
              <Input placeholder="/" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="enabled"
              label={intl.formatMessage({
                id: isWeChat
                  ? 'pages.systemSiteSettings.auth.enableWeChatLabel'
                  : 'pages.systemSiteSettings.auth.enableGenericOAuthLabel',
                defaultMessage: isWeChat ? '启用微信扫码登录' : '启用自定义 OAuth 登录',
              })}
              valuePropName="checked"
            >
              <Switch
                checkedChildren={intl.formatMessage({
                  id: 'pages.systemSiteSettings.auth.switch.enable',
                  defaultMessage: '启用',
                })}
                unCheckedChildren={intl.formatMessage({
                  id: 'pages.systemSiteSettings.auth.switch.disable',
                  defaultMessage: '停用',
                })}
              />
            </Form.Item>
          </Col>
        </Row>
        <Button type="primary" size="small" loading={saving} onClick={() => void handleSave()}>
          <FormattedMessage id="pages.systemSiteSettings.auth.action.save" defaultMessage="保存" />
        </Button>
      </Form>
    </Card>
  );
}

type RegisterFormValues = {
  enabled: boolean;
  defaultRoles: string;
  domainWhitelist: string;
  aliasRestriction: boolean;
  verificationRequired: boolean;
};

/** 自助注册（OPEN-ISSUES #51b/#51c）：默认关闭；注册的是本地账密账号。
 * 邮箱策略两键（域白名单/别名限制）随卡保存，注册链路每次实时读 L3。 */
function RegisterCard({
  snapshot,
  emailPolicy,
  onReload,
}: {
  snapshot: AuthProviderSnapshot | undefined;
  emailPolicy: EmailPolicySnapshot | undefined;
  onReload: () => Promise<void>;
}) {
  const { message } = App.useApp();
  const intl = useIntl();
  const [form] = Form.useForm<RegisterFormValues>();
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const f = snapshot?.fields ?? {};
    form.setFieldsValue({
      enabled: snapshot?.enabled ?? false,
      defaultRoles: f.defaultRoles ?? '',
      domainWhitelist: emailPolicy?.domainWhitelist ?? '',
      aliasRestriction: emailPolicy?.aliasRestriction ?? false,
      verificationRequired: emailPolicy?.verificationRequired ?? false,
    });
  }, [snapshot, emailPolicy, form]);

  const handleSave = async () => {
    try {
      const v = await form.validateFields();
      setSaving(true);
      await saveKeys([
        { key: 'auth.register.enabled', value: v.enabled },
        { key: 'auth.register.defaultRoles', value: v.defaultRoles?.trim() ?? '' },
        { key: 'auth.email.domainWhitelist', value: v.domainWhitelist?.trim() ?? '' },
        { key: 'auth.email.aliasRestriction', value: v.aliasRestriction },
        { key: 'auth.email.verificationRequired', value: v.verificationRequired },
      ]);
      await onReload();
      message.success(
        intl.formatMessage({
          id: 'pages.systemSiteSettings.auth.saved.register',
          defaultMessage: '自助注册配置已保存',
        }),
      );
    } catch (error) {
      if ((error as { errorFields?: unknown }).errorFields) return;
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.systemSiteSettings.auth.error.saveFailed',
            defaultMessage: '保存失败',
          }),
        ),
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card
      size="small"
      title={
        <Space size={6}>
          <UserAddOutlined />
          <Text strong>
            <FormattedMessage
              id="pages.systemSiteSettings.auth.register.title"
              defaultMessage="自助注册"
            />
          </Text>
          {snapshot?.enabled ? (
            <Tag color="green">
              <FormattedMessage
                id="pages.systemSiteSettings.auth.provider.enabled"
                defaultMessage="已启用"
              />
            </Tag>
          ) : (
            <Tag>
              <FormattedMessage
                id="pages.systemSiteSettings.auth.provider.disabled"
                defaultMessage="未启用"
              />
            </Tag>
          )}
        </Space>
      }
      extra={
        <Text type="secondary">
          <FormattedMessage
            id="pages.systemSiteSettings.auth.hint.registerCard"
            defaultMessage="默认关闭；开启后登录页出现「注册账号」入口，注册的是本地账密账号"
          />
        </Text>
      }
    >
      <Form form={form} name="auth-register" layout="vertical" size="small">
        <Row gutter={12}>
          <Col span={12}>
            <Form.Item
              name="defaultRoles"
              label={
                <Space size={4}>
                  <FormattedMessage
                    id="pages.systemSiteSettings.auth.jitRolesLabel"
                    defaultMessage="JIT 角色"
                  />
                  <SourceTag source={snapshot?.sources?.defaultRoles} />
                </Space>
              }
              tooltip={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.register.rolesTooltip',
                defaultMessage: '注册账号被赋予的角色（逗号分隔）；留空则不赋角色',
              })}
            >
              <Input placeholder="viewer" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="enabled"
              label={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.register.enableLabel',
                defaultMessage: '允许自助注册',
              })}
              valuePropName="checked"
              tooltip={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.register.enableTooltip',
                defaultMessage: '开启后任何人可在登录页注册本地账号；密码走账号安全策略校验',
              })}
            >
              <Switch
                checkedChildren={intl.formatMessage({
                  id: 'pages.systemSiteSettings.auth.switch.enable',
                  defaultMessage: '启用',
                })}
                unCheckedChildren={intl.formatMessage({
                  id: 'pages.systemSiteSettings.auth.switch.disable',
                  defaultMessage: '停用',
                })}
              />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={12}>
          <Col span={12}>
            <Form.Item
              name="domainWhitelist"
              label={
                <Space size={4}>
                  <FormattedMessage
                    id="pages.systemSiteSettings.auth.register.domainWhitelistLabel"
                    defaultMessage="邮箱域白名单"
                  />
                  <SourceTag source={emailPolicy?.sources?.['auth.email.domainWhitelist']} />
                </Space>
              }
              tooltip={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.register.domainWhitelistTooltip',
                defaultMessage:
                  '逗号分隔域名后缀，子域自动放行（example.com 覆盖 api.example.com）；留空 = 不限',
              })}
            >
              <Input placeholder="example.com,foo.io" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="aliasRestriction"
              label={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.register.aliasRestrictionLabel',
                defaultMessage: '邮箱别名限制',
              })}
              valuePropName="checked"
              tooltip={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.register.aliasRestrictionTooltip',
                defaultMessage:
                  '开启后拒绝 + 别名（user+tag@）形态，且忽略点号归一查重（u.s@ 与 us@ 视为同一邮箱）',
              })}
            >
              <Switch
                checkedChildren={intl.formatMessage({
                  id: 'pages.systemSiteSettings.auth.switch.enable',
                  defaultMessage: '启用',
                })}
                unCheckedChildren={intl.formatMessage({
                  id: 'pages.systemSiteSettings.auth.switch.disable',
                  defaultMessage: '停用',
                })}
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="verificationRequired"
              label={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.register.verificationRequiredLabel',
                defaultMessage: '注册邮箱验证',
              })}
              valuePropName="checked"
              tooltip={intl.formatMessage({
                id: 'pages.systemSiteSettings.auth.register.verificationRequiredTooltip',
                defaultMessage:
                  '开启后注册须填写邮箱并完成邮件验证才能登录；SMTP 未配置时用户将收不到验证邮件（可凭用户名+邮箱重发）',
              })}
            >
              <Switch
                checkedChildren={intl.formatMessage({
                  id: 'pages.systemSiteSettings.auth.switch.enable',
                  defaultMessage: '启用',
                })}
                unCheckedChildren={intl.formatMessage({
                  id: 'pages.systemSiteSettings.auth.switch.disable',
                  defaultMessage: '停用',
                })}
              />
            </Form.Item>
          </Col>
        </Row>
        <Button type="primary" size="small" loading={saving} onClick={() => void handleSave()}>
          <FormattedMessage id="pages.systemSiteSettings.auth.action.save" defaultMessage="保存" />
        </Button>
      </Form>
    </Card>
  );
}

/** 登录方式 Tab：本地开关 + LDAP 直连级联 + OIDC/GitHub 重定向 SSO（Harbor 模式热配置）。 */
export default function AuthTab() {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让请求
  // effect 无限重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [, setLoading] = useState(false);
  const [snapshot, setSnapshot] = useState<AuthSnapshot | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setSnapshot(await fetchAuthSnapshot());
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.auth.error.loadFailed',
            defaultMessage: '加载登录方式配置失败',
          }),
        ),
      );
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
        <FormattedMessage
          id="pages.systemSiteSettings.auth.hint.tab"
          defaultMessage="配置文件仅作初始值，此处保存后热生效（无需重启）；停用账号密码登录前请先确保其他登录方式可用"
        />
      </Text>
    ),
    [],
  );

  return (
    <Space orientation="vertical" size={12} style={{ width: '100%' }}>
      {extra}
      <LocalCard snapshot={snapshot?.local} onReload={load} />
      <LDAPCard snapshot={snapshot?.ldap} onReload={load} />
      <OIDCCard snapshot={snapshot?.oidc} onReload={load} />
      <GitHubCard snapshot={snapshot?.github} onReload={load} />
      <ExternalOAuthCard provider="wechat" snapshot={snapshot?.wechat} onReload={load} />
      <ExternalOAuthCard
        provider="genericoauth"
        snapshot={snapshot?.genericoauth}
        onReload={load}
      />
      <RegisterCard snapshot={snapshot?.register} emailPolicy={snapshot?.email} onReload={load} />
    </Space>
  );
}
