import { Footer } from '@/components';
import { createSession, fetchCurrentUserGames } from '@/services/api';
import { extractErrorCode, isMfaRequiredError } from '@/utils/errors';
import { fetchLoginProviders, type LoginProviders } from '@/services/api/sites';
import { setScope } from '@/stores/scope';
import { LockOutlined, SafetyCertificateOutlined, UserOutlined } from '@ant-design/icons';
import { LoginForm, ProFormCheckbox, ProFormText } from '@ant-design/pro-components';
import { LoginOutlined } from '@ant-design/icons';
import { FormattedMessage, history, SelectLang, useIntl, useModel, Helmet } from '@umijs/max';
import { Alert, Button, Divider, Modal, Typography } from 'antd';
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
      message={content}
      type={type}
      showIcon
    />
  );
};

const Login: React.FC = () => {
  // siteCfg 在下方 useModel 声明后取用
  const [userLoginState] = useState<{ status?: string; type?: string }>({});
  // Only account/password login is supported
  const { initialState, setInitialState } = useModel('@@initialState');
  const siteCfg = initialState?.siteConfig;
  const [forgotOpen, setForgotOpen] = useState(false);
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
            defaultMessage: '该账号已启用两步验证，请输入动态验证码',
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
  const { status } = userLoginState;

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
                      其他登录方式
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
                    SSO 登录
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
                defaultMessage: '两步验证已开启，请输入认证器 App 中的 6 位动态验证码',
              })}
            />
          )}
          {status === 'error' && (
            <LoginMessage
              content={intl.formatMessage({
                id: 'pages.login.accountLogin.errorMessage',
                defaultMessage: '账户或密码错误(admin/ant.design)',
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
                    maxLength: 6,
                    autoComplete: 'one-time-code',
                  }}
                  placeholder={intl.formatMessage({
                    id: 'pages.login.mfa.placeholder',
                    defaultMessage: '动态验证码（6 位）',
                  })}
                  rules={[
                    {
                      required: true,
                      message: (
                        <FormattedMessage
                          id="pages.login.mfa.required"
                          defaultMessage="请输入动态验证码！"
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
              message="支持域账号：直接输入 LDAP 用户名和密码登录（本地账号校验失败时自动尝试目录服务）"
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
            <p>请联系管理员为你的账号重置密码。</p>
            <p>如果你是管理员：在「权限 → 用户」中选择用户，点击「设置密码」即可重置。</p>
          </div>
        </Modal>
      </div>
      <Footer />
    </div>
  );
};

export default Login;
