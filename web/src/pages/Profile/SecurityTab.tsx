import { useCallback, type ReactNode } from 'react';
import { Button, Card, Col, Row, Space, Tag, Typography } from 'antd';
import { HistoryOutlined, LockOutlined, PhoneOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import MfaSettings from './MfaSettings';
import NotificationChannels from './NotificationChannels';
import type { NotificationChannelState } from './shared';

const { Text } = Typography;

/**
 * 安全中心 Tab：密码修改入口 + 通知通道真实状态 + 审计可用性 + MFA 设置。
 *
 * 「登录通知」此前是一个假开关：只要用户填了手机号就显示「已开启」，而仓内
 * 根本没有短信服务商。现在改为完全由后端上报的通道状态驱动——未接入的通道
 * 明确标注「未接入」并禁用开关（docs/BUGS.md BUG-016）。
 */
export default function SecurityTab({
  hasSessions,
  notificationChannels,
  onShowPasswordModal,
}: {
  hasSessions: boolean;
  /** 后端 GET /api/v1/profile/notification-channels 的 channels；未加载时为空数组 */
  notificationChannels: NotificationChannelState[];
  onShowPasswordModal: () => void;
}) {
  const intl = useIntl();
  const formatMessage = useCallback(
    (id: string, fallback: string) => intl.formatMessage({ id, defaultMessage: fallback }),
    [intl],
  );
  return (
    <Row gutter={[24, 24]}>
      <Col xs={24}>
        <Card title={formatMessage('profile.security.center', '安全中心')}>
          <Space orientation="vertical" size="large" style={{ width: '100%' }}>
            <div className="security-item">
              <Space>
                <LockOutlined />
                <div>
                  <Text strong>
                    {formatMessage('profile.password.change.title', '修改密码')}
                  </Text>
                  <br />
                  <Text type="secondary">
                    {formatMessage('profile.password.description', '定期更换密码可提升账号安全。')}
                  </Text>
                </div>
              </Space>
              <Button type="primary" onClick={onShowPasswordModal}>
                {formatMessage('profile.password.change.btn', '修改')}
              </Button>
            </div>

            <NotificationChannels channels={notificationChannels} />

            <div className="security-item">
              <Space>
                <HistoryOutlined />
                <div>
                  <Text strong>{formatMessage('profile.sessions.title', '登录记录')}</Text>
                  <br />
                  <Text type="secondary">
                    {formatMessage('profile.security.audit.helper', '可追溯账号的历史登录行为。')}
                  </Text>
                </div>
              </Space>
              <Tag color={hasSessions ? 'success' : 'default'}>
                {hasSessions
                  ? formatMessage('profile.security.audit.available', '有记录')
                  : formatMessage('profile.security.audit.empty', '暂无记录')}
              </Tag>
            </div>
            <MfaSettings />
          </Space>
        </Card>
      </Col>
    </Row>
  );
}
