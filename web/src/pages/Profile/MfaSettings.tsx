import { FormattedMessage, useIntl } from '@umijs/max';
import { App, Button, Input, QRCode, Space, Tag, Typography, theme } from 'antd';
import React, { useCallback, useRef, useState } from 'react';
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
 * 两步验证（TOTP）绑定面板。
 *
 * 之前只有「手动抄密钥」一条路径——把 base32 密钥和 otpauth:// 串当纯文本
 * 展示，用户必须自行粘贴到 App 里，微软/谷歌 Authenticator 并不提供「粘贴
 * otpauth 链接」的入口，实际体验等同于没有绑定功能。现在：
 *
 *   1. 直接渲染 otpauth:// 的二维码，扫码即绑；
 *   2. 保留手动录入作为兜底（无摄像头时仍可用）；
 *   3. 确认成功后**一次性**展示备用恢复码（丢失验证器时用它登录）；
 *   4. 已启用时展示剩余恢复码数量，并对数量偏低给出提示。
 */
const MfaSettings: React.FC = () => {
  const intl = useIntl();
  const { message } = App.useApp();
  const { token } = theme.useToken();
  // 测试 mock 每次渲染返回新的 intl 引用；经 ref 转发后回调依赖稳定
  const intlRef = useRef(intl);
  intlRef.current = intl;

  const [status, setStatus] = useState<MfaStatus | null>(null);
  const [setup, setSetup] = useState<MfaSetupResult | null>(null);
  const [recoveryCodes, setRecoveryCodes] = useState<string[] | null>(null);
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    try {
      setStatus(await fetchMfaStatus());
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.profileMfa.error.loadStatus',
            defaultMessage: '加载两步验证状态失败',
          }),
        ),
      );
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
        // 密钥此前已下发过但前端没拿到（例如刷新前点过「开启」）：直接回到已启用态
        setSetup(null);
        await refresh();
        message.warning(
          intlRef.current.formatMessage({
            id: 'pages.profileMfa.alreadyEnabled',
            defaultMessage: '两步验证已处于开启状态',
          }),
        );
        return;
      }
      setSetup(result);
      setCode('');
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.profileMfa.error.generateSecret',
            defaultMessage: '生成密钥失败',
          }),
        ),
      );
    } finally {
      setBusy(false);
    }
  }, [message, refresh]);

  const doConfirm = useCallback(async () => {
    const trimmed = code.trim();
    if (!trimmed) {
      message.warning(
        intlRef.current.formatMessage({
          id: 'pages.profileMfa.warning.codeRequired',
          defaultMessage: '请输入验证器 App 中的 6 位验证码',
        }),
      );
      return;
    }
    setBusy(true);
    try {
      const resp = await confirmMfa(trimmed);
      // 恢复码只在这一刻返回一次（库里仅存哈希），必须立刻展示给用户保存
      setRecoveryCodes(resp.recoveryCodes ?? []);
      setSetup(null);
      setCode('');
      await refresh();
      message.success(
        intlRef.current.formatMessage({
          id: 'pages.profileMfa.success.enabled',
          defaultMessage: '两步验证已开启，下次登录需要输入验证码',
        }),
      );
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.profileMfa.error.confirm',
            defaultMessage: '确认失败，请核对验证码',
          }),
        ),
      );
    } finally {
      setBusy(false);
    }
  }, [code, message, refresh]);

  const doDisable = useCallback(async () => {
    if (!code.trim() || !password) {
      message.warning(
        intlRef.current.formatMessage({
          id: 'pages.profileMfa.warning.codeAndPasswordRequired',
          defaultMessage: '请输入验证码与登录密码',
        }),
      );
      return;
    }
    setBusy(true);
    try {
      await disableMfa(code.trim(), password);
      setCode('');
      setPassword('');
      setRecoveryCodes(null);
      await refresh();
      message.success(
        intlRef.current.formatMessage({
          id: 'pages.profileMfa.success.disabled',
          defaultMessage: '两步验证已关闭',
        }),
      );
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.profileMfa.error.disable',
            defaultMessage: '关闭失败，请核对验证码与密码',
          }),
        ),
      );
    } finally {
      setBusy(false);
    }
  }, [code, password, message, refresh]);

  // 外部 IdP 账号：MFA 由身份提供方管理，平台侧不可配置
  if (status && !status.local) {
    return (
      <div data-testid="mfa-external">
        <Space orientation="vertical" size={4} style={{ width: '100%' }}>
          <Space>
            <Text strong>
              <FormattedMessage id="pages.profileMfa.title" defaultMessage="两步验证（TOTP）" />
            </Text>
            <Tag>
              <FormattedMessage id="pages.profileMfa.external.tag" defaultMessage="由 IdP 管理" />
            </Tag>
          </Space>
          <Text type="secondary">
            <FormattedMessage
              id="pages.profileMfa.external.description"
              defaultMessage="当前账号来自外部身份源，二次验证由身份提供方管理。"
            />
          </Text>
        </Space>
      </div>
    );
  }

  const remaining = status?.recoveryCodesRemaining ?? 0;
  const total = status?.recoveryCodeTotal ?? 0;
  const lowOnCodes = status?.enabled === true && total > 0 && remaining > 0 && remaining <= 2;

  return (
    <div data-testid="mfa-settings">
      <Space orientation="vertical" size="small" style={{ width: '100%' }}>
        <Space>
          <Text strong>
            <FormattedMessage id="pages.profileMfa.title" defaultMessage="两步验证（TOTP）" />
          </Text>
          {status?.enabled ? (
            <Tag color="success" data-testid="mfa-status">
              <FormattedMessage id="pages.profileMfa.status.enabled" defaultMessage="已开启" />
            </Tag>
          ) : (
            <Tag data-testid="mfa-status">
              <FormattedMessage id="pages.profileMfa.status.disabled" defaultMessage="未开启" />
            </Tag>
          )}
          {status?.enabled && total > 0 && (
            <Tag
              color={lowOnCodes ? 'warning' : 'default'}
              data-testid="mfa-recovery-remaining"
            >
              {/* 用 formatMessage 而非 FormattedMessage：需要 values 插值，
                  与仓内其它带占位符的文案一致 */}
              {intl.formatMessage(
                {
                  id: 'pages.profileMfa.recovery.remaining',
                  defaultMessage: '剩余恢复码 {remaining}/{total}',
                },
                { remaining, total },
              )}
            </Tag>
          )}
        </Space>

        {/* 恢复码一次性展示：确认成功后弹出，关闭即不再可见 */}
        {recoveryCodes && recoveryCodes.length > 0 && (
          <div
            data-testid="mfa-recovery-codes"
            style={{
              border: `1px solid ${token.colorWarningBorder}`,
              background: token.colorWarningBg,
              borderRadius: 6,
              padding: 12,
            }}
          >
            <Space orientation="vertical" size={6} style={{ width: '100%' }}>
              <Text strong>
                <FormattedMessage
                  id="pages.profileMfa.recovery.title"
                  defaultMessage="请立即保存备用恢复码"
                />
              </Text>
              <Text type="secondary">
                <FormattedMessage
                  id="pages.profileMfa.recovery.hint"
                  defaultMessage="每个恢复码只能使用一次。丢失验证器 App 时，可在登录页用恢复码代替动态验证码。关闭本提示后无法再次查看。"
                />
              </Text>
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
                  gap: 4,
                  fontFamily: 'monospace',
                }}
              >
                {recoveryCodes.map((c) => (
                  <Text key={c} copyable={{ text: c }} data-testid="mfa-recovery-code">
                    {c}
                  </Text>
                ))}
              </div>
              <Space>
                <Button
                  size="small"
                  type="primary"
                  onClick={() => {
                    const blob = new Blob([`${recoveryCodes.join('\n')}\n`], {
                      type: 'text/plain',
                    });
                    const url = URL.createObjectURL(blob);
                    const a = document.createElement('a');
                    a.href = url;
                    a.download = 'croupier-mfa-recovery-codes.txt';
                    a.click();
                    URL.revokeObjectURL(url);
                  }}
                >
                  <FormattedMessage
                    id="pages.profileMfa.recovery.download"
                    defaultMessage="下载全部恢复码"
                  />
                </Button>
                <Text
                  type="secondary"
                  style={{ cursor: 'pointer' }}
                  onClick={() => setRecoveryCodes(null)}
                  data-testid="mfa-recovery-dismiss"
                >
                  <FormattedMessage
                    id="pages.profileMfa.recovery.saved"
                    defaultMessage="我已保存，关闭"
                  />
                </Text>
              </Space>
            </Space>
          </div>
        )}

        {status?.enabled ? (
          <Space wrap>
            <Input
              style={{ width: 160 }}
              placeholder={intl.formatMessage({
                id: 'pages.profileMfa.code.placeholder',
                defaultMessage: '6 位验证码',
              })}
              maxLength={12}
              value={code}
              onChange={(e) => setCode(e.target.value)}
              onPressEnter={() => void doDisable()}
              data-testid="mfa-disable-code"
            />
            <Input.Password
              style={{ width: 200 }}
              placeholder={intl.formatMessage({
                id: 'pages.profileMfa.password.placeholder',
                defaultMessage: '登录密码',
              })}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              onPressEnter={() => void doDisable()}
              data-testid="mfa-disable-password"
            />
            <Button danger loading={busy} onClick={() => void doDisable()} data-testid="mfa-disable">
              <FormattedMessage
                id="pages.profileMfa.disable.button"
                defaultMessage="关闭两步验证"
              />
            </Button>
          </Space>
        ) : setup ? (
          <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
            <Text>
              <FormattedMessage
                id="pages.profileMfa.setup.step1Prefix"
                defaultMessage="1. 在验证器 App 中"
              />
              <strong>
                <FormattedMessage
                  id="pages.profileMfa.setup.step1Scan"
                  defaultMessage="扫描二维码"
                />
              </strong>
              <FormattedMessage
                id="pages.profileMfa.setup.step1Suffix"
                defaultMessage="（微软/谷歌 Authenticator、1Password 等均支持），"
              />
            </Text>
            <div>
              <QRCode value={setup.otpauthUrl} size={192} data-testid="mfa-qrcode" />
              <div style={{ marginTop: 8, maxWidth: 320 }}>
                <Text type="secondary">
                  <FormattedMessage
                    id="pages.profileMfa.setup.qrIssuer"
                    defaultMessage="签发方：{issuer}"
                    values={{ issuer: setup.issuer || 'Croupier' }}
                  />
                </Text>
              </div>
            </div>
            <Text>
              <FormattedMessage id="pages.profileMfa.setup.step2Prefix" defaultMessage="2. 无法扫码时" />
              <strong>
                <FormattedMessage
                  id="pages.profileMfa.setup.step1Bold"
                  defaultMessage="手动录入密钥"
                />
              </strong>
            </Text>
            <Space orientation="vertical" size={4} style={{ width: '100%' }}>
              <Text copyable={{ text: setup.secret }} data-testid="mfa-secret">
                <Text code>{setup.secret}</Text>
              </Text>
              <Text copyable style={{ wordBreak: 'break-all', fontSize: 12 }} type="secondary">
                {setup.otpauthUrl}
              </Text>
            </Space>
            <Text>
              <FormattedMessage id="pages.profileMfa.setup.step3Prefix" defaultMessage="3. 输入 App 显示的" />
              <strong>
                <FormattedMessage
                  id="pages.profileMfa.setup.step3Bold"
                  defaultMessage="6 位验证码"
                />
              </strong>
              <FormattedMessage id="pages.profileMfa.setup.step3Suffix" defaultMessage="完成绑定" />
            </Text>
            <Space wrap>
              <Input
                style={{ width: 160 }}
                placeholder={intl.formatMessage({
                  id: 'pages.profileMfa.code.placeholder',
                  defaultMessage: '6 位验证码',
                })}
                maxLength={6}
                value={code}
                onChange={(e) => setCode(e.target.value)}
                onPressEnter={() => void doConfirm()}
                data-testid="mfa-confirm-code"
              />
              <Button
                type="primary"
                loading={busy}
                onClick={() => void doConfirm()}
                data-testid="mfa-confirm"
              >
                <FormattedMessage id="pages.profileMfa.confirm.button" defaultMessage="确认开启" />
              </Button>
              <Button
                onClick={() => {
                  setSetup(null);
                  setCode('');
                }}
              >
                <FormattedMessage id="pages.profileMfa.cancel.button" defaultMessage="取消" />
              </Button>
            </Space>
            <Text type="secondary" style={{ color: token.colorWarning }}>
              <FormattedMessage
                id="pages.profileMfa.setup.secretWarning"
                defaultMessage="密钥仅此次展示，请妥善保存；确认后登录必须携带动态码。"
              />
            </Text>
          </Space>
        ) : (
          <>
            <Text type="secondary">
              <FormattedMessage
                id="pages.profileMfa.description"
                defaultMessage="开启后登录需要输入验证器 App（Google Authenticator 等）的 6 位动态码。"
              />
            </Text>
            <Button
              type="primary"
              loading={busy}
              onClick={() => void startEnroll()}
              data-testid="mfa-enable"
            >
              <FormattedMessage id="pages.profileMfa.enable.button" defaultMessage="开启两步验证" />
            </Button>
          </>
        )}
      </Space>
    </div>
  );
};

export default MfaSettings;
