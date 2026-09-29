import { Footer } from '@/components';
import {
  changeCurrentUserPassword,
  createSession,
  fetchCurrentUserGames,
  registerAccount,
} from '@/services/api';
import { extractErrorMessage, isMfaRequiredError, isEmailNotVerifiedError } from '@/utils/errors';
import { resendVerification } from '@/services/api/auth';
import { fetchLoginProviders, type LoginProviders } from '@/services/api/sites';
import { setScope } from '@/stores/scope';
import {
  GithubOutlined,
  LockOutlined,
  SafetyCertificateOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { LoginForm, ProFormCheckbox, ProFormText } from '@ant-design/pro-components';
import { LoginOutlined, QrcodeOutlined } from '@ant-design/icons';
import { FormattedMessage, history, SelectLang, useIntl, useModel, Helmet } from '@umijs/max';
import { Alert, Button, Divider, Form, Input, Modal, Space, Typography } from 'antd';
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

/** 自助注册弹窗表单值（OPEN-ISSUES #51b） */
type RegisterFormValues = {
  username: string;
  password: string;
  confirm: string;
  nickname?: string;
  email?: string;
};

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
  // 系统信息协议弹窗（OPEN-ISSUES #49）：'user' | 'privacy' | null
  const [agreementView, setAgreementView] = useState<null | 'user' | 'privacy'>(null);
  // 自助注册（OPEN-ISSUES #51b）：providers.register=true 时展示入口；
  // 仅账密表单可见时注册才有意义（注册的是本地账密账号）
  const [registerOpen, setRegisterOpen] = useState(false);
  // #51c 第二批：403 email_not_verified → 展示重发验证邮件块；用户名在
  // 登录提交被拦时一并捕获，邮箱由用户补填——后端防枚举，不匹配静默成功
  const [emailNotVerified, setEmailNotVerified] = useState(false);
  const [resendUsername, setResendUsername] = useState('');
  const [resendTo, setResendTo] = useState('');
  const [resending, setResending] = useState(false);
  const [registerSubmitting, setRegisterSubmitting] = useState(false);
  const [registerForm] = Form.useForm<RegisterFormValues>();
  useEffect(() => {
    fetchLoginProviders()
      .then(setProviders)
      .catch(() => setProviders(null));
  }, []);
  const { styles } = useStyles();
  const intl = useIntl();
  // 账密表单可见性（OPEN-ISSUES #51）：local 关但 LDAP 开时仍要显示
  //（LDAP 用户走同一表单级联认证）；拉取失败默认显示（fail-open 同现状）
  const passwordFormVisible = providers ? providers.local || providers.ldap : true;

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
      // #51c 第二批：403 + email_not_verified → 展示重发验证邮件块。
      if (isEmailNotVerifiedError(error)) {
        setResendUsername(values.username);
        setEmailNotVerified(true);
        return;
      }
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

  // 自助注册提交（OPEN-ISSUES #51b）：成功不自动登录（MFA/强制改密门控
  // 在登录链上照常生效），提示后回登录表单。
  const submitRegister = async () => {
    const values = await registerForm.validateFields().catch(() => null);
    if (!values) return;
    setRegisterSubmitting(true);
    try {
      const res = await registerAccount({
        username: values.username,
        password: values.password,
        nickname: values.nickname,
        email: values.email,
      });
      setRegisterOpen(false);
      registerForm.resetFields();
      getMessage()?.success(
        intl.formatMessage(
          {
            id: 'pages.login.register.success',
            defaultMessage: '账号 {username} 注册成功，请登录',
          },
          { username: res.username },
        ),
      );
    } catch (error) {
      getMessage()?.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.login.register.error',
            defaultMessage: '注册失败，请检查输入后重试',
          }),
        ),
      );
    } finally {
      setRegisterSubmitting(false);
    }
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
            providers?.oidc || providers?.github || providers?.wechat || providers?.genericoauth
              ? [
                  <Divider plain key="sso-divider" style={{ margin: '8px 0' }}>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      <FormattedMessage
                        id="pages.login.sso.divider"
                        defaultMessage="其他登录方式"
                      />
                    </Typography.Text>
                  </Divider>,
                  ...(providers?.oidc
                    ? [
                        <Button
                          key="sso-oidc"
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
                    : []),
                  ...(providers?.github
                    ? [
                        <Button
                          key="sso-github"
                          block
                          size="large"
                          icon={<GithubOutlined />}
                          onClick={() => {
                            window.location.href = '/api/v1/auth/github/login';
                          }}
                        >
                          <FormattedMessage
                            id="pages.login.github.button"
                            defaultMessage="GitHub 登录"
                          />
                        </Button>,
                      ]
                    : []),
                  ...(providers?.wechat
                    ? [
                        <Button
                          key="sso-wechat"
                          block
                          size="large"
                          icon={<QrcodeOutlined />}
                          onClick={() => {
                            window.location.href = '/api/v1/auth/wechat/login';
                          }}
                        >
                          <FormattedMessage
                            id="pages.login.wechat.button"
                            defaultMessage="微信扫码登录"
                          />
                        </Button>,
                      ]
                    : []),
                  ...(providers?.genericoauth
                    ? [
                        <Button
                          key="sso-generic"
                          block
                          size="large"
                          icon={<LoginOutlined />}
                          onClick={() => {
                            window.location.href = '/api/v1/auth/generic/login';
                          }}
                        >
                          <FormattedMessage
                            id="pages.login.generic.button"
                            defaultMessage="自定义 OAuth 登录"
                          />
                        </Button>,
                      ]
                    : []),
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

          {emailNotVerified && (
            <Alert
              style={{ marginBottom: 16 }}
              type="warning"
              showIcon
              title={intl.formatMessage({
                id: 'pages.login.emailNotVerified.hint',
                defaultMessage: '邮箱尚未验证，请查收验证邮件；未收到可凭用户名和邮箱重发',
              })}
              description={
                <Space.Compact style={{ width: '100%', marginTop: 8 }}>
                  <Input
                    value={resendTo}
                    onChange={(e) => setResendTo(e.target.value)}
                    placeholder={intl.formatMessage({
                      id: 'pages.login.emailNotVerified.placeholder',
                      defaultMessage: '注册时填写的邮箱',
                    })}
                  />
                  <Button
                    loading={resending}
                    onClick={async () => {
                      const username = resendUsername;
                      const email = resendTo.trim();
                      if (!email) return;
                      setResending(true);
                      try {
                        await resendVerification(username, email);
                        getMessage()?.success(
                          intl.formatMessage({
                            id: 'pages.login.emailNotVerified.sent',
                            defaultMessage: '若信息匹配，验证邮件已重新发送，请查收',
                          }),
                        );
                      } catch (err) {
                        getMessage()?.error(
                          extractErrorMessage(
                            err,
                            intl.formatMessage({
                              id: 'pages.login.emailNotVerified.failed',
                              defaultMessage: '重发失败，请稍后重试',
                            }),
                          ),
                        );
                      } finally {
                        setResending(false);
                      }
                    }}
                  >
                    <FormattedMessage
                      id="pages.login.emailNotVerified.resend"
                      defaultMessage="重发验证邮件"
                    />
                  </Button>
                </Space.Compact>
              }
            />
          )}
          {passwordFormVisible ? (
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
          ) : (
            <Alert
              style={{ marginBottom: 16 }}
              type="info"
              showIcon
              title={intl.formatMessage({
                id: 'pages.login.passwordDisabled',
                defaultMessage: '账号密码登录已停用，请使用其他登录方式',
              })}
            />
          )}
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
          {passwordFormVisible && (
            <div
              style={{
                marginBottom: 24,
              }}
            >
              <ProFormCheckbox noStyle name="autoLogin">
                <FormattedMessage id="pages.login.rememberMe" defaultMessage="自动登录" />
              </ProFormCheckbox>
              <span style={{ float: 'right' }}>
                {providers?.register && (
                  <>
                    <a onClick={() => setRegisterOpen(true)}>
                      <FormattedMessage id="pages.login.register.entry" defaultMessage="注册账号" />
                    </a>
                    <span style={{ margin: '0 8px', color: 'rgba(0,0,0,0.15)' }}>|</span>
                  </>
                )}
                <a onClick={() => setForgotOpen(true)}>
                  <FormattedMessage id="pages.login.forgotPassword" defaultMessage="忘记密码" />
                </a>
              </span>
            </div>
          )}
        </LoginForm>
        {/* 系统信息消费面（OPEN-ISSUES #49）：首页内容欢迎区 + 文档/协议入口，
            均按配置存在才渲染，未配置零占位 */}
        {(siteCfg?.homeContent ||
          siteCfg?.docsUrl ||
          siteCfg?.userAgreement ||
          siteCfg?.privacyPolicy) && (
          <div
            style={{
              width: 'min(420px, calc(100vw - 32px))',
              margin: '0 auto 24px',
              textAlign: 'center',
            }}
          >
            {siteCfg?.homeContent && (
              <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
                {siteCfg.homeContent}
              </Typography.Paragraph>
            )}
            <Space size={16} wrap>
              {siteCfg?.docsUrl && (
                <a href={siteCfg.docsUrl} target="_blank" rel="noreferrer">
                  <FormattedMessage id="pages.login.docsLink" defaultMessage="文档" />
                </a>
              )}
              {siteCfg?.userAgreement && (
                <a onClick={() => setAgreementView('user')}>
                  <FormattedMessage id="pages.login.userAgreement" defaultMessage="用户协议" />
                </a>
              )}
              {siteCfg?.privacyPolicy && (
                <a onClick={() => setAgreementView('privacy')}>
                  <FormattedMessage id="pages.login.privacyPolicy" defaultMessage="隐私政策" />
                </a>
              )}
            </Space>
          </div>
        )}
        <Modal
          title={
            agreementView === 'user'
              ? intl.formatMessage({
                  id: 'pages.login.userAgreement',
                  defaultMessage: '用户协议',
                })
              : intl.formatMessage({
                  id: 'pages.login.privacyPolicy',
                  defaultMessage: '隐私政策',
                })
          }
          open={agreementView !== null}
          footer={null}
          width={640}
          onCancel={() => setAgreementView(null)}
        >
          <div style={{ whiteSpace: 'pre-wrap', maxHeight: '60vh', overflowY: 'auto' }}>
            {agreementView === 'user' ? siteCfg?.userAgreement : siteCfg?.privacyPolicy}
          </div>
        </Modal>
        {/* 自助注册（OPEN-ISSUES #51b）：开关默认关闭，providers.register 才有入口；
            成功不自动登录，提示后走正常登录 */}
        <Modal
          title={intl.formatMessage({
            id: 'pages.login.register.title',
            defaultMessage: '注册账号',
          })}
          open={registerOpen}
          onCancel={() => setRegisterOpen(false)}
          onOk={() => void submitRegister()}
          okText={intl.formatMessage({
            id: 'pages.login.register.submit',
            defaultMessage: '注册',
          })}
          confirmLoading={registerSubmitting}
          destroyOnHidden
        >
          <Form form={registerForm} layout="vertical">
            <Form.Item
              label={intl.formatMessage({
                id: 'pages.login.username.placeholder',
                defaultMessage: '用户名',
              })}
              name="username"
              rules={[
                { required: true },
                {
                  pattern: /^[a-zA-Z0-9_-]{3,32}$/,
                  message: intl.formatMessage({
                    id: 'pages.login.register.usernameRule',
                    defaultMessage: '3-32 位字母、数字、下划线或连字符',
                  }),
                },
              ]}
            >
              <Input placeholder="username" autoComplete="username" />
            </Form.Item>
            <Form.Item
              label={intl.formatMessage({
                id: 'pages.login.register.nickname',
                defaultMessage: '昵称（可选）',
              })}
              name="nickname"
            >
              <Input autoComplete="nickname" />
            </Form.Item>
            <Form.Item
              label={intl.formatMessage({
                id: 'pages.login.register.email',
                defaultMessage: '邮箱（可选）',
              })}
              name="email"
              rules={[
                {
                  type: 'email',
                  message: intl.formatMessage({
                    id: 'pages.login.register.emailRule',
                    defaultMessage: '邮箱格式不正确',
                  }),
                },
              ]}
            >
              <Input autoComplete="email" />
            </Form.Item>
            <Form.Item
              label={intl.formatMessage({
                id: 'pages.login.mustChange.new',
                defaultMessage: '新密码',
              })}
              name="password"
              rules={[
                {
                  required: true,
                  message: intl.formatMessage({
                    id: 'pages.login.password.required',
                    defaultMessage: '请输入密码！',
                  }),
                },
              ]}
            >
              <Input.Password
                placeholder={intl.formatMessage({
                  id: 'pages.login.register.passwordPlaceholder',
                  defaultMessage: '8 位以上，建议混合字符类',
                })}
                autoComplete="new-password"
              />
            </Form.Item>
            <Form.Item
              label={intl.formatMessage({
                id: 'pages.login.mustChange.confirm',
                defaultMessage: '确认新密码',
              })}
              name="confirm"
              dependencies={['password']}
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
                    if (!value || getFieldValue('password') === value) {
                      return Promise.resolve();
                    }
                    return Promise.reject(
                      new Error(
                        intl.formatMessage({
                          id: 'pages.login.register.mismatch',
                          defaultMessage: '两次输入的密码不一致',
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
                autoComplete="new-password"
              />
            </Form.Item>
          </Form>
        </Modal>
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
