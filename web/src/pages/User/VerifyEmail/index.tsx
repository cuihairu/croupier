import { Alert, Button, Card, Spin, Typography } from 'antd';
import { createStyles } from 'antd-style';
import { FormattedMessage, useIntl, useSearchParams, history } from '@umijs/max';
import { useEffect, useState } from 'react';
import { verifyEmailToken } from '@/services/api/auth';
import { BRAND } from '@/config/branding';
import { useModel } from '@umijs/max';

const useStyles = createStyles(() => {
  return {
    container: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      height: '100vh',
    },
  };
});

type VerifyState = 'verifying' | 'success' | 'invalid';

/**
 * 邮箱验证结果页（OPEN-ISSUES #51c 第二批）：验证邮件里的链接指向
 * /user/verify-email?token=...，挂载后调 GET /auth/verify-email 消费令牌，
 * 展示成功/失败结果与回登录页入口。无 token 视同无效链接。
 */
const VerifyEmail: React.FC = () => {
  const { styles } = useStyles();
  const intl = useIntl();
  const { initialState } = useModel('@@initialState');
  const [searchParams] = useSearchParams();
  const token = searchParams.get('token') ?? '';
  const [state, setState] = useState<VerifyState>(token ? 'verifying' : 'invalid');

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    verifyEmailToken(token)
      .then(() => {
        if (!cancelled) setState('success');
      })
      .catch(() => {
        if (!cancelled) setState('invalid');
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  const backToLogin = (
    <Button type="primary" onClick={() => history.push('/user/login')}>
      <FormattedMessage id="pages.verifyEmail.backToLogin" defaultMessage="返回登录" />
    </Button>
  );

  return (
    <div className={styles.container}>
      <Card
        style={{ width: 'min(420px, calc(100vw - 32px))', textAlign: 'center' }}
        title={initialState?.siteConfig?.siteName || BRAND.title || 'Croupier'}
      >
        {state === 'verifying' && (
          <div style={{ padding: '24px 0' }}>
            <Spin />
            <Typography.Paragraph type="secondary" style={{ marginTop: 16 }}>
              <FormattedMessage id="pages.verifyEmail.verifying" defaultMessage="正在验证邮箱…" />
            </Typography.Paragraph>
          </div>
        )}
        {state === 'success' && (
          <Alert
            type="success"
            showIcon
            title={intl.formatMessage({
              id: 'pages.verifyEmail.success',
              defaultMessage: '邮箱验证成功，现在可以使用账号密码登录了',
            })}
            style={{ marginBottom: 16 }}
          />
        )}
        {state === 'invalid' && (
          <Alert
            type="error"
            showIcon
            title={intl.formatMessage({
              id: 'pages.verifyEmail.invalid',
              defaultMessage: '验证链接无效或已过期，请在登录页重新发送验证邮件',
            })}
            style={{ marginBottom: 16 }}
          />
        )}
        {state !== 'verifying' && backToLogin}
      </Card>
    </div>
  );
};

export default VerifyEmail;
