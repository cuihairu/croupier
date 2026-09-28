import { Footer } from '@/components';
import { changeCurrentUserPassword, createSession, fetchCurrentUserGames } from '@/services/api';
import { isMfaRequiredError } from '@/utils/errors';
import { fetchLoginProviders, type LoginProviders } from '@/services/api/sites';
import { setScope } from '@/stores/scope';
import { LockOutlined, SafetyCertificateOutlined, UserOutlined } from '@ant-design/icons';
import { LoginForm, ProFormCheckbox, ProFormText } from '@ant-design/pro-components';
import { LoginOutlined } from '@ant-design/icons';
import { FormattedMessage, history, SelectLang, useIntl, useModel, Helmet } from '@umijs/max';
import { Alert, Button, Divider, Form, Input, Modal, Typography } from 'antd';
import { getMessage } from '@/utils/antdApp';
import Settings from '../../../../config/defaultSettings';
import { BRAND } from '@/config/branding';
import { loadAuthedInitialState } from '@/services/initialState';
import React, { useEffect, useState } from 'react';
import { flushSync } from 'react-dom';
import { createStyles } from 'antd-style';

const useStyles = createStyles(({ token }) => {
  return {
    action: {
      marginLeft: '8px',
      color: 'rgba(0, 0, 0, 0.2)',
      fontSize: '24px',
      verticalAlign: 'middle',
      cursor: 'pointer',
      transition: 'color 0.3s',
      '&:hover': {
        color: token.colorPrimaryActive,
      },
    },
    lang: {
      width: 42,
      height: 42,
      lineHeight: '42px',
      position: 'fixed',
      right: 16,
      borderRadius: token.borderRadius,
      ':hover': {
        backgroundColor: token.colorBgTextHover,
      },
    },
    container: {
      display: 'flex',
      flexDirection: 'column',
      height: '100vh',
      overflow: 'auto',
      backgroundImage:
        "url('https://mdn.alipayobjects.com/yuyan_qk0oxh/afts/img/V-_oS6r-i7wAAAAAAAAAAAAAFl94AQBr')",
      backgroundSize: '100% 100%',
    },
  };
});

// No 3rd-party login methods

const Lang = () => {
  const { styles } = useStyles();

  return (
    <div className={styles.lang} data-lang>
      {SelectLang && <SelectLang />}
    </div>
  );
};

const LoginMessage: React.FC<{
  content: string;
  type?: 'error' | 'info' | 'warning' | 'success';
}> = ({ content, type = 'error' }) => {
  return (
    <Alert
      style={{
        marginBottom: 24,
      }}
      title={content}
      type={type}
      showIcon
    />
  );
};

/** 强制改密弹窗表单值（OPEN-ISSUES #20） */
type ForceChangePwdValues = { newPassword: string; confirm: string };

const Login: React.FC = () => {
  // siteCfg 在下方 useModel 声明后取用
  // Only account/password login is supported
  const { initialState, setInitialState } = useModel('@@initialState');
  const siteCfg = initialState?.siteConfig;
  const [forgotOpen, setForgotOpen] = useState(false);
  // 强制改密（OPEN-ISSUES #20）：登录响应 mustChangePassword=true 时弹窗。
  // oldPassword 复用刚验证通过的登录密码（改密接口要求携带旧密码）。
  const [forceChangeOpen, setForceChangeOpen] = useState(false);
  const [forceChangeOldPwd, setForceChangeOldPwd] = useState('');
  const [forceChangeForm] = Form.useForm<ForceChangePwdValues>();
  // MFA 二次验证：401+mfa_required 后置 true，展示动态验证码输入（凭据由
  // 表单 values 持续携带，重试时一并提供）
  const [mfaRequired, setMfaRequired] = useState(false);
  // 已启用登录方式（LDAP 级联提示 / OIDC SSO 入口）；拉取失败静默回落本地登录
  const [providers, setProviders] = useState<LoginProviders | null>(null);
  useEffect(() => {
    fetchLoginProviders()
      .then(setProviders)
      .catch(() => setProviders(null));
  }, []);
  const { styles } = useStyles();
  const intl = useIntl();

  const fetchUserInfo = async () => {
    const fetcher = initialState?.fetchUserInfo;
    if (!fetcher) return;
    const authedState = await loadAuthedInitialState(fetcher);
    if (authedState.currentUser) {
      flushSync(() => {
        setInitialState((s) => ({
          ...s,
          ...authedState,
        }));
      });
    }
  };

  const handleSubmit = async (values: {
    username: string;
    password: string;
    totpCode?: string;
  }) => {
    try {
      // RESTful: 创建会话
      const res = await createSession({
        username: values.username,
        password: values.password,
        totpCode: values.totpCode,
      });
      localStorage.setItem('token', res.token);
      if (res.mustChangePassword) {
        // 被标记「登录后必须修改密码」或密码已过有效期（OPEN-ISSUES #20）：
        // 不进入应用，强制先改密。token 仍保留——改密接口需要鉴权；改密成功后
        // 后端吊销该 token（BumpTokenVersion），前端清除并引导用新密码重登。
        setForceChangeOldPwd(values.password);
        setForceChangeOpen(true);
        return;
      }
      if (res.mfaSetupRequired) {
        // 账号安全策略强制 TOTP（security.mfaRequired）：token 有效（绑定
        // 接口需要鉴权），鉴权中间件已把其余 API 拦为 403 mfa_required，
        // 直接引导到个人安全页绑定，与 mustChangePassword 同构。
        getMessage()?.warning(
          intl.formatMessage({
            id: 'pages.login.mfaSetupRequired',
            defaultMessage: '管理员已开启强制二次验证，请先绑定 TOTP',
          }),
        );
        await fetchUserInfo();
        history.push('/profile?tab=security');
        return;
      }
      try {
        // Restore last-selected scope from server, or fall back to first authorized game
        let gameId = res.lastGameId;
        let env = res.lastEnv;
        if (!gameId) {
          const gamesResp = await fetchCurrentUserGames();
          const games = Array.isArray(gamesResp?.games) ? gamesResp.games : [];
          const firstGame = games[0];
          gameId = firstGame?.gameId;
          env = env || firstGame?.envs?.[0];
        }
        if (gameId || env) {
          setScope(
            { gameId: gameId || undefined, env: env || undefined },
            { persist: true, emit: true },
          );
        }
      } catch {}
      getMessage()?.success(
        intl.formatMessage({ id: 'pages.login.success', defaultMessage: '登录成功！' }),
      );
      await fetchUserInfo();
      const urlParams = new URL(window.location.href).searchParams;
      history.push(urlParams.get('redirect') || '/');
      return;
    } catch (error) {
      // MFA 已启用账号：401 + error=mfa_required → 展示动态验证码输入，
      // 凭据由表单 values 保留，重试时一并提供 totpCode。
      if (isMfaRequiredError(error)) {
        setMfaRequired(true);
        getMessage()?.info(
          intl.formatMessage({
            id: 'pages.login.mfa.required.info',
            defaultMessage: '该账号已启用两步验证，请输入动态验证码或备用恢复码',
          }),
        );
        return;
      }
      const defaultLoginFailureMessage = intl.formatMessage({
        id: 'pages.login.failure',
        defaultMessage: '登录失败，请重试！',
      });
      getMessage()?.error(defaultLoginFailureMessage);
    }
  };

  // 强制改密提交（OPEN-ISSUES #20）：旧密码复用刚登录成功的密码；成功后后端
  // 吊销当前 token，清除本地凭据并提示用新密码重新登录（不进入应用）。
  const submitForceChange = async () => {
    const values = await forceChangeForm.validateFields().catch(() => null);
    if (!values) return;
    try {
      await changeCurrentUserPassword({
        oldPassword: forceChangeOldPwd,
        newPassword: values.newPassword,
      });
      localStorage.removeItem('token');
      setForceChangeOpen(false);
      forceChangeForm.resetFields();
      getMessage()?.success(
        intl.formatMessage({
          id: 'pages.login.mustChange.success',
          defaultMessage: '密码已修改，请使用新密码重新登录',
        }),
      );
    } catch {
      getMessage()?.error(
        intl.formatMessage({
          id: 'pages.login.mustChange.error',
          defaultMessage: '修改密码失败，请重新登录后重试',
        }),
      );
    }
  };

  // 放弃本次登录：清除刚签发的 token，回到登录表单（不给绕过改密的入口）
  const cancelForceChange = () => {
    localStorage.removeItem('token');
    setForceChangeOpen(false);
    forceChangeForm.resetFields();
  };

  return (
    <div className={styles.container}>
      <Helmet>
        <title>
          {intl.formatMessage({
            id: 'menu.login',
            defaultMessage: '登录页',
          })}
          - {Settings.title}
        </title>
      </Helmet>
      <Lang />
      <div
        style={{
          flex: '1',
          padding: '32px 0',
        }}
      >
        <LoginForm
          contentStyle={{
            minWidth: 280,
            width: 'min(420px, calc(100vw - 32px))',
            maxWidth: 'calc(100vw - 32px)',
          }}
          logo={<img alt="logo" src={siteCfg?.logoUrl || BRAND.logo || '/logo.svg'} />}
          title={siteCfg?.siteName || BRAND.title || 'Croupier'}
          subTitle={
            siteCfg?.description ||
            BRAND.subTitle ||
            intl.formatMessage({ id: 'pages.layouts.userLayout.title' })
          }
          initialValues={{
            autoLogin: true,
          }}
          actions={
            providers?.oidc
              ? [
                  <Divider plain key="sso-divider" style={{ margin: '8px 0' }}>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      <FormattedMessage
                        id="pages.login.sso.divider"
                        defaultMessage="其他登录方式"
                      />
                    </Typography.Text>
                  </Divider>,
                  <Button
                    key="sso"
                    block
                    size="large"
                    icon={<LoginOutlined />}
                    onClick={() => {
                      window.location.href = '/api/v1/auth/oidc/login';
                    }}
                  >
                    <FormattedMessage id="pages.login.sso.button" defaultMessage="SSO 登录" />
                  </Button>,
                ]
              : []
          }
          onFinish={async (values) => {
            await handleSubmit(values as { username: string; password: string });
          }}
        >
          {/* Only account/password login */}

          {mfaRequired && (
            <LoginMessage
              type="info"
              content={intl.formatMessage({
                id: 'pages.login.mfa.hint',
                defaultMessage:
                  '两步验证已开启，请输入认证器 App 中的 6 位动态验证码，或绑定时的备用恢复码',
              })}
            />
          )}
          {
            <>
              <ProFormText
                name="username"
                fieldProps={{
                  size: 'large',
                  prefix: <UserOutlined />,
                }}
                placeholder={intl.formatMessage({
                  id: 'pages.login.username.placeholder',
                  defaultMessage: '用户名: admin or user',
                })}
                rules={[
                  {
                    required: true,
                    message: (
                      <FormattedMessage
                        id="pages.login.username.required"
                        defaultMessage="请输入用户名!"
                      />
                    ),
                  },
                ]}
              />
              <ProFormText.Password
                name="password"
                fieldProps={{
                  size: 'large',
                  prefix: <LockOutlined />,
                }}
                placeholder={intl.formatMessage({
                  id: 'pages.login.password.placeholder',
                  defaultMessage: '密码: admin',
                })}
                rules={[
                  {
                    required: true,
                    message: (
                      <FormattedMessage
                        id="pages.login.password.required"
                        defaultMessage="请输入密码！"
                      />
                    ),
                  },
                ]}
              />
              {mfaRequired && (
                <ProFormText.Password
                  name="totpCode"
                  fieldProps={{
                    size: 'large',
                    prefix: <SafetyCertificateOutlined />,
                    // 兼容 10 位备用恢复码（后端 verifySecondFactor 二选一），
                    // 6 位动态码与 10 位恢复码都能通过此输入框提交
                    maxLength: 12,
                    autoComplete: 'one-time-code',
                  }}
                  placeholder={intl.formatMessage({
                    id: 'pages.login.mfa.placeholder',
                    defaultMessage: '动态验证码或备用恢复码',
                  })}
                  rules={[
                    {
                      required: true,
                      message: (
                        <FormattedMessage
                          id="pages.login.mfa.required"
                          defaultMessage="请输入动态验证码或备用恢复码！"
                        />
                      ),
                    },
                  ]}
                />
              )}
            </>
          }
          {providers?.ldap && (
            <Alert
              style={{ marginBottom: 16 }}
              type="info"
              showIcon
              title={intl.formatMessage({
                id: 'pages.login.ldap.notice',
                defaultMessage:
                  '支持域账号：直接输入 LDAP 用户名和密码登录（本地账号校验失败时自动尝试目录服务）',
              })}
            />
          )}
          <div
            style={{
              marginBottom: 24,
            }}
          >
            <ProFormCheckbox noStyle name="autoLogin">
              <FormattedMessage id="pages.login.rememberMe" defaultMessage="自动登录" />
            </ProFormCheckbox>
            <a style={{ float: 'right' }} onClick={() => setForgotOpen(true)}>
              <FormattedMessage id="pages.login.forgotPassword" defaultMessage="忘记密码" />
            </a>
          </div>
        </LoginForm>
        <Modal
          title={intl.formatMessage({
            id: 'pages.login.forgotPassword',
            defaultMessage: '忘记密码',
          })}
          open={forgotOpen}
          onCancel={() => setForgotOpen(false)}
          onOk={() => setForgotOpen(false)}
        >
          <div>
            <p>
              <FormattedMessage
                id="pages.login.forgotPassword.contactAdmin"
                defaultMessage="请联系管理员为你的账号重置密码。"
              />
            </p>
            <p>
              <FormattedMessage
                id="pages.login.forgotPassword.adminHint"
                defaultMessage="如果你是管理员：在「权限 → 用户」中选择用户，点击「设置密码」即可重置。"
              />
            </p>
          </div>
        </Modal>
        {/* 强制改密（OPEN-ISSUES #20）：账号被标记或密码过期时登录不进入应用，
            必须改密（或放弃本次登录清除 token） */}
        <Modal
          title={intl.formatMessage({
            id: 'pages.login.mustChange.title',
            defaultMessage: '请先修改密码',
          })}
          open={forceChangeOpen}
          onOk={submitForceChange}
          onCancel={cancelForceChange}
          okText={intl.formatMessage({
            id: 'pages.login.mustChange.submit',
            defaultMessage: '修改密码',
          })}
          cancelText={intl.formatMessage({
            id: 'pages.login.mustChange.cancel',
            defaultMessage: '放弃本次登录',
          })}
          closable={false}
          mask={{ closable: false }}
          keyboard={false}
        >
          <Alert
            style={{ marginBottom: 16 }}
            type="warning"
            showIcon
            title={intl.formatMessage({
              id: 'pages.login.mustChange.alert',
              defaultMessage:
                '该账号要求登录后立即修改密码（管理员标记或密码已过有效期），修改成功后需使用新密码重新登录。',
            })}
          />
          <Form form={forceChangeForm} layout="vertical">
            <Form.Item
              label={intl.formatMessage({
                id: 'pages.login.mustChange.new',
                defaultMessage: '新密码',
              })}
              name="newPassword"
              rules={[
                {
                  required: true,
                  message: intl.formatMessage({
                    id: 'pages.login.mustChange.required',
                    defaultMessage: '请输入新密码！',
                  }),
                },
                {
                  min: 6,
                  message: intl.formatMessage({
                    id: 'pages.login.mustChange.min',
                    defaultMessage: '至少 6 位',
                  }),
                },
              ]}
            >
              <Input.Password
                placeholder={intl.formatMessage({
                  id: 'pages.login.mustChange.new.placeholder',
                  defaultMessage: '请输入新密码',
                })}
              />
            </Form.Item>
            <Form.Item
              label={intl.formatMessage({
                id: 'pages.login.mustChange.confirm',
                defaultMessage: '确认新密码',
              })}
              name="confirm"
              dependencies={['newPassword']}
              rules={[
                {
                  required: true,
                  message: intl.formatMessage({
                    id: 'pages.login.mustChange.confirmRequired',
                    defaultMessage: '请再次输入新密码！',
                  }),
                },
                ({ getFieldValue }) => ({
                  validator(_, value: string) {
                    if (!value || getFieldValue('newPassword') === value) {
                      return Promise.resolve();
                    }
                    return Promise.reject(
                      new Error(
                        intl.formatMessage({
                          id: 'pages.login.mustChange.mismatch',
                          defaultMessage: '两次输入的新密码不一致',
                        }),
                      ),
                    );
                  },
                }),
              ]}
            >
              <Input.Password
                placeholder={intl.formatMessage({
                  id: 'pages.login.mustChange.confirm.placeholder',
                  defaultMessage: '请再次输入新密码',
                })}
              />
            </Form.Item>
          </Form>
        </Modal>
      </div>
      <Footer />
    </div>
  );
};

export default Login;
