import { App, Button, Input, Modal, Space, Tag, Typography, theme } from 'antd';
import React, { useCallback, useState } from 'react';
import {
  confirmMfa,
  disableMfa,
  fetchMfaStatus,
  setupMfa,
  type MfaSetupResult,
  type MfaStatus,
} from '@/services/api/auth';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

/**
 * 账号中心「两步验证（TOTP）」区块：本地账号自助开启/关闭二次验证。
 * 外部身份源（LDAP/OIDC）账号的 MFA 由 IdP 管理，此处仅展示说明。
 */
const MfaSettings: React.FC = () => {
  const { message } = App.useApp();
  const { token } = theme.useToken();
  const [status, setStatus] = useState<MfaStatus | null>(null);
  const [setup, setSetup] = useState<MfaSetupResult | null>(null);
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    try {
      setStatus(await fetchMfaStatus());
    } catch (e) {
      message.error(extractErrorMessage(e, '加载两步验证状态失败'));
    }
  }, [message]);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  const startEnroll = useCallback(async () => {
    setBusy(true);
    try {
      const result = await setupMfa();
      if (result.alreadyEnabled) {
        await refresh();
        return;
      }
      setSetup(result);
    } catch (e) {
      message.error(extractErrorMessage(e, '生成密钥失败'));
    } finally {
      setBusy(false);
    }
  }, [message, refresh]);

  const doConfirm = useCallback(async () => {
    if (!code.trim()) {
      message.warning('请输入验证器 App 中的 6 位验证码');
      return;
    }
    setBusy(true);
    try {
      await confirmMfa(code.trim());
      message.success('两步验证已开启，下次登录需要输入验证码');
      setSetup(null);
      setCode('');
      await refresh();
    } catch (e) {
      message.error(extractErrorMessage(e, '确认失败，请核对验证码'));
    } finally {
      setBusy(false);
    }
  }, [code, message, refresh]);

  const doDisable = useCallback(async () => {
    if (!code.trim() || !password) {
      message.warning('请输入验证码与登录密码');
      return;
    }
    setBusy(true);
    try {
      await disableMfa(code.trim(), password);
      message.success('两步验证已关闭');
      setCode('');
      setPassword('');
      await refresh();
    } catch (e) {
      message.error(extractErrorMessage(e, '关闭失败，请核对验证码与密码'));
    } finally {
      setBusy(false);
    }
  }, [code, password, message, refresh]);

  if (status && !status.local) {
    return (
      <div className="security-item">
        <Space>
          <div>
            <Text strong>两步验证（TOTP）</Text>
            <br />
            <Text type="secondary">当前账号来自外部身份源，二次验证由身份提供方管理。</Text>
          </div>
          <Tag>由 IdP 管理</Tag>
        </Space>
      </div>
    );
  }

  return (
    <div className="security-item">
      <Space direction="vertical" size="small" style={{ width: '100%' }}>
        <Space>
          <div>
            <Text strong>两步验证（TOTP）</Text>
            <br />
            <Text type="secondary">
              开启后登录需要输入验证器 App（Google Authenticator 等）的 6 位动态码。
            </Text>
          </div>
          {status ? status.enabled ? <Tag color="success">已开启</Tag> : <Tag>未开启</Tag> : null}
        </Space>

        {status?.enabled ? (
          <Space wrap>
            <Input
              style={{ width: 160 }}
              placeholder="6 位验证码"
              maxLength={6}
              value={code}
              onChange={(e) => setCode(e.target.value)}
            />
            <Input.Password
              style={{ width: 180 }}
              placeholder="登录密码"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
            <Button danger loading={busy} onClick={() => void doDisable()}>
              关闭两步验证
            </Button>
          </Space>
        ) : setup ? (
          <Space direction="vertical" size="small" style={{ width: '100%' }}>
            <Text>
              1. 在验证器 App 中<strong>手动录入密钥</strong>，或使用
              <Text copyable>{setup.otpauthUrl}</Text>
            </Text>
            <Text code copyable={{ text: setup.secret }}>
              {setup.secret}
            </Text>
            <Space wrap>
              <Input
                style={{ width: 160 }}
                placeholder="6 位验证码"
                maxLength={6}
                value={code}
                onChange={(e) => setCode(e.target.value)}
                onPressEnter={() => void doConfirm()}
              />
              <Button type="primary" loading={busy} onClick={() => void doConfirm()}>
                确认开启
              </Button>
              <Button
                onClick={() => {
                  setSetup(null);
                  setCode('');
                }}
              >
                取消
              </Button>
            </Space>
            <Text type="secondary" style={{ color: token.colorWarning }}>
              密钥仅此次展示，请妥善保存；确认后登录必须携带动态码。
            </Text>
          </Space>
        ) : (
          <Button type="primary" loading={busy} onClick={() => void startEnroll()}>
            开启两步验证
          </Button>
        )}
      </Space>
    </div>
  );
};

export default MfaSettings;
