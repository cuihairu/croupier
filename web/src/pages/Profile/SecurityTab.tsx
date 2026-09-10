import { useCallback } from 'react';
import { Button, Card, Col, Row, Space, Tag, Typography } from 'antd';
import { HistoryOutlined, LockOutlined, PhoneOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import MfaSettings from './MfaSettings';

const { Text } = Typography;

/** 安全中心 Tab：密码修改入口 + 登录通知开关态 + 审计可用性 + MFA 设置。 */
export default function SecurityTab({
  hasPhone,
  hasSessions,
  onShowPasswordModal,
}: {
  hasPhone: boolean;
  hasSessions: boolean;
  onShowPasswordModal: () => void;
}) {
  const intl = useIntl();
  const formatMessage = useCallback((id: string) => intl.formatMessage({ id }), [intl]);
  return (
    <Row gutter={[24, 24]}>
      <Col xs={24}>
        <Card title={formatMessage('profile.security.center')}>
          <Space orientation="vertical" size="large" style={{ width: '100%' }}>
            <div className="security-item">
              <Space>
                <LockOutlined />
                <div>
                  <Text strong>{formatMessage('profile.password.change.title')}</Text>
                  <br />
                  <Text type="secondary">{formatMessage('profile.password.description')}</Text>
                </div>
              </Space>
              <Button type="primary" onClick={onShowPasswordModal}>
                {formatMessage('profile.password.change.btn')}
              </Button>
            </div>
            <div className="security-item">
              <Space>
                <PhoneOutlined />
                <div>
                  <Text strong>{formatMessage('profile.login.notification')}</Text>
                  <br />
                  <Text type="secondary">{formatMessage('profile.security.phone.helper')}</Text>
                </div>
              </Space>
              {hasPhone ? (
                <Tag color="success">{formatMessage('profile.enabled')}</Tag>
              ) : (
                <Tag>{formatMessage('profile.not.enabled')}</Tag>
              )}
            </div>
            <div className="security-item">
              <Space>
                <HistoryOutlined />
                <div>
                  <Text strong>{formatMessage('profile.sessions.title')}</Text>
                  <br />
                  <Text type="secondary">{formatMessage('profile.security.audit.helper')}</Text>
                </div>
              </Space>
              <Tag color={hasSessions ? 'success' : 'default'}>
                {hasSessions
                  ? formatMessage('profile.security.audit.available')
                  : formatMessage('profile.security.audit.empty')}
              </Tag>
            </div>
            <MfaSettings />
          </Space>
        </Card>
      </Col>
    </Row>
  );
}
