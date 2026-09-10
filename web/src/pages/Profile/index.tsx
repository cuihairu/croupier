import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Alert,
  Avatar,
  Badge,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Row,
  Space,
  Statistic,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import {
  BellOutlined,
  HistoryOutlined,
  RocketOutlined,
  SafetyOutlined,
  SettingOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { useIntl, useLocation, useModel, useNavigate } from '@umijs/max';
import { updateMyProfile } from '@/services/api/me';
import { formatDateTime } from '@/utils/format';
import { TAB_KEYS, type ProfileData } from './shared';
import { useProfileData } from './useProfileData';
import InfoTab from './InfoTab';
import SecurityTab from './SecurityTab';
import GamesTab from './GamesTab';
import PermissionsTab from './PermissionsTab';
import ActivityTab from './ActivityTab';
import SessionsTab from './SessionsTab';
import NotificationsTab from './NotificationsTab';
import PasswordModal from './PasswordModal';
import AvatarModal from './AvatarModal';
import BroadcastModal from './BroadcastModal';
import './index.less';

const { Title, Text } = Typography;

/** 个人中心：hero 概览 + 七 Tab（资料/安全/项目/权限/活动/会话/通知）。
 * 数据层在 useProfileData，各 Tab 与弹窗为独立组件；主页只做编排与 URL 同步。 */
export default function Profile() {
  const intl = useIntl();
  const location = useLocation();
  const navigate = useNavigate();
  const formatMessage = useCallback((id: string) => intl.formatMessage({ id }), [intl]);
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [profileEditing, setProfileEditing] = useState(false);
  const [passwordModalVisible, setPasswordModalVisible] = useState(false);
  const [avatarModalVisible, setAvatarModalVisible] = useState(false);
  const [sendOpen, setSendOpen] = useState(false);

  const {
    profile,
    games,
    permissions,
    permissionCatalogAvailable,
    activities,
    notifications,
    detailMessage,
    extrasLoading,
    permissionGroups,
    applyPermissionCandidates,
    loginSessionRows,
    latestLoginIP,
    loadProfile,
    loadExtras,
    openMessage,
    markAllRead,
    setDetailMessage,
  } = useProfileData(form);

  const infoSectionRef = useRef<HTMLDivElement>(null);
  const initialTab = useMemo(
    () => new URLSearchParams(location.search).get('tab') || TAB_KEYS.PROFILE,
    [location.search],
  );
  const [activeTab, setActiveTab] = useState(initialTab);

  useEffect(() => {
    setActiveTab(initialTab);
  }, [initialTab]);

  useEffect(() => {
    loadProfile();
  }, [loadProfile]);

  const handleProfileSubmit = async (values: ProfileData) => {
    setLoading(true);
    try {
      await updateMyProfile({
        displayName: values.displayName,
        email: values.email,
        phone: values.phone,
      });
      message.success(formatMessage('profile.update.success'));
      setProfileEditing(false);
      loadProfile();
    } catch {
      message.error(formatMessage('profile.update.error'));
    } finally {
      setLoading(false);
    }
  };

  const { initialState } = useModel('@@initialState');
  const isAdminUser = (initialState?.currentUser?.roles || []).includes('admin');

  const getStatusBadge = (status?: boolean) => (
    <Badge
      status={status ? 'success' : 'default'}
      text={
        status ? formatMessage('profile.status.active') : formatMessage('profile.status.inactive')
      }
    />
  );

  const stats = [
    { title: formatMessage('profile.stats.games'), value: games.length, icon: <RocketOutlined /> },
    {
      title: formatMessage('profile.stats.roles'),
      value: profile?.roles?.length || 0,
      icon: <UserOutlined />,
    },
    {
      title: formatMessage('profile.stats.permissions'),
      value: permissions.length,
      icon: <SafetyOutlined />,
    },
    {
      title: formatMessage('profile.stats.activities'),
      value: activities.length,
      icon: <HistoryOutlined />,
    },
  ];

  const passwordModalEl = (
    <PasswordModal open={passwordModalVisible} onClose={() => setPasswordModalVisible(false)} />
  );
  // 广播弹窗在两个分支（加载中/已加载）都渲染：打开入口在 NotificationsTab，
  // 仅 profile 加载完成后可见，若只渲染在 !profile 分支则弹窗不可达。
  const broadcastModalEl = (
    <BroadcastModal
      open={sendOpen}
      onClose={() => setSendOpen(false)}
      onSent={() => loadExtras(String(profile?.username || ''))}
    />
  );

  if (!profile) {
    return (
      <>
        <PageContainer>
          <Card>
            <Space size="large" orientation="vertical" align="center" style={{ width: '100%' }}>
              <Avatar size={96} icon={<UserOutlined />} />
              <Title level={4}>{formatMessage('profile.loading')}</Title>
            </Space>
          </Card>
        </PageContainer>
        {passwordModalEl}
        {broadcastModalEl}
      </>
    );
  }

  const username = String(profile?.username || '');

  return (
    <>
      <PageContainer className="profile-page">
        <Space orientation="vertical" size="large" style={{ width: '100%' }}>
          <Card className="profile-hero" styles={{ body: { padding: 24 } }}>
            <Row gutter={[32, 24]} align="middle">
              <Col xs={24} md={10}>
                <Space align="center">
                  <Avatar
                    size={96}
                    src={profile?.avatar}
                    icon={!profile?.avatar ? <UserOutlined /> : undefined}
                    style={{
                      border: '3px solid #1890ff',
                      boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
                    }}
                  />
                  <div>
                    <Space align="center">
                      <Title level={3} style={{ margin: 0 }}>
                        {profile?.displayName || profile?.username}
                      </Title>
                      {typeof profile?.active === 'boolean' ? (
                        <span style={{ marginLeft: 8 }}>{getStatusBadge(profile?.active)}</span>
                      ) : null}
                    </Space>
                    <Space wrap style={{ marginTop: 8 }}>
                      {(profile?.roles || []).map((role: string) => (
                        <Tag color="blue" key={role}>
                          {role}
                        </Tag>
                      ))}
                    </Space>
                    <Descriptions column={1} size="small" style={{ marginTop: 12 }}>
                      <Descriptions.Item label={formatMessage('profile.info.username')}>
                        <Text strong>{profile?.username}</Text>
                      </Descriptions.Item>
                      <Descriptions.Item label={formatMessage('profile.info.email')}>
                        {profile?.email ? (
                          <Text>{profile.email}</Text>
                        ) : (
                          <Text type="secondary">{formatMessage('profile.info.notSet')}</Text>
                        )}
                      </Descriptions.Item>
                      <Descriptions.Item label={formatMessage('profile.info.phone')}>
                        {profile?.phone ? (
                          <Text>{profile.phone}</Text>
                        ) : (
                          <Text type="secondary">{formatMessage('profile.info.notSet')}</Text>
                        )}
                      </Descriptions.Item>
                      <Descriptions.Item label={formatMessage('profile.info.joined')}>
                        {profile?.createdAt ? (
                          <Text>{formatDateTime(String(profile.createdAt))}</Text>
                        ) : (
                          <Text type="secondary">{formatMessage('profile.info.notSet')}</Text>
                        )}
                      </Descriptions.Item>
                      <Descriptions.Item label={formatMessage('profile.info.last.login')}>
                        {profile?.lastLoginAt ? (
                          <Text>{formatDateTime(String(profile.lastLoginAt))}</Text>
                        ) : (
                          <Text type="secondary">{formatMessage('profile.info.notSet')}</Text>
                        )}
                      </Descriptions.Item>
                    </Descriptions>
                    <Space style={{ marginTop: 12 }}>
                      <Button
                        type="primary"
                        icon={<SettingOutlined />}
                        onClick={() => {
                          setActiveTab(TAB_KEYS.PROFILE);
                          setProfileEditing(true);
                          const search = new URLSearchParams(location.search);
                          search.set('tab', TAB_KEYS.PROFILE);
                          navigate(`${location.pathname}?${search.toString()}`, { replace: true });
                          setTimeout(() => {
                            infoSectionRef.current?.scrollIntoView({ behavior: 'smooth' });
                          }, 0);
                        }}
                      >
                        {formatMessage('profile.hero.edit')}
                      </Button>
                      <Button onClick={() => setAvatarModalVisible(true)}>
                        {formatMessage('profile.avatar.change')}
                      </Button>
                    </Space>
                  </div>
                </Space>
              </Col>
              <Col xs={24} md={14}>
                <Row gutter={[16, 16]} className="profile-stats">
                  {stats.map((stat) => (
                    <Col xs={12} md={6} key={stat.title}>
                      <Card variant="borderless" className="profile-stats__card">
                        <Statistic title={stat.title} value={stat.value} prefix={stat.icon} />
                      </Card>
                    </Col>
                  ))}
                </Row>
              </Col>
            </Row>
          </Card>

          <Tabs
            activeKey={activeTab}
            onChange={(key) => {
              setActiveTab(key);
              const search = new URLSearchParams(location.search);
              search.set('tab', key);
              navigate(`${location.pathname}?${search.toString()}`, { replace: true });
            }}
            items={[
              {
                key: TAB_KEYS.PROFILE,
                label: (
                  <Space>
                    <UserOutlined />
                    {formatMessage('profile.tab.info')}
                  </Space>
                ),
                children: (
                  <div ref={infoSectionRef}>
                    <InfoTab
                      profile={profile}
                      editing={profileEditing}
                      form={form}
                      loading={loading}
                      latestLoginIP={latestLoginIP}
                      onEdit={() => setProfileEditing(true)}
                      onCancelEdit={() => {
                        form.setFieldsValue({
                          displayName: profile?.displayName || profile?.nickname,
                          email: profile?.email,
                          phone: profile?.phone,
                        });
                        setProfileEditing(false);
                      }}
                      onSubmit={handleProfileSubmit}
                    />
                  </div>
                ),
              },
              {
                key: TAB_KEYS.SECURITY,
                label: (
                  <Space>
                    <SafetyOutlined />
                    {formatMessage('profile.security.center')}
                  </Space>
                ),
                children: (
                  <SecurityTab
                    hasPhone={!!profile?.phone}
                    hasSessions={loginSessionRows.length > 0}
                    onShowPasswordModal={() => setPasswordModalVisible(true)}
                  />
                ),
              },
              {
                key: TAB_KEYS.GAMES,
                label: (
                  <Space>
                    <RocketOutlined />
                    {formatMessage('profile.games.title')}
                  </Space>
                ),
                children: <GamesTab games={games} loading={extrasLoading} />,
              },
              {
                key: TAB_KEYS.PERMISSIONS,
                label: (
                  <Space>
                    <SafetyOutlined />
                    {formatMessage('profile.permissions.summary.title')}
                  </Space>
                ),
                children: (
                  <PermissionsTab
                    groups={permissionGroups}
                    candidates={applyPermissionCandidates}
                    catalogAvailable={permissionCatalogAvailable}
                    username={profile?.username ? String(profile.username) : undefined}
                  />
                ),
              },
              {
                key: TAB_KEYS.ACTIVITY,
                label: (
                  <Space>
                    <HistoryOutlined />
                    {formatMessage('profile.activities.title')}
                  </Space>
                ),
                children: (
                  <ActivityTab
                    activities={activities}
                    loading={extrasLoading}
                    username={username}
                    onRefresh={loadExtras}
                  />
                ),
              },
              {
                key: TAB_KEYS.SESSIONS,
                label: (
                  <Space>
                    <HistoryOutlined />
                    {formatMessage('profile.sessions.title')}
                  </Space>
                ),
                children: (
                  <Space orientation="vertical" size={16} style={{ width: '100%' }}>
                    {loginSessionRows.length === 0 && !extrasLoading ? (
                      <Alert
                        showIcon
                        type="info"
                        message={formatMessage('profile.sessions.unavailable')}
                        style={{ marginBottom: 16 }}
                      />
                    ) : null}
                    <SessionsTab
                      rows={loginSessionRows}
                      loading={extrasLoading}
                      username={username}
                      onRefresh={loadExtras}
                    />
                  </Space>
                ),
              },
              {
                key: TAB_KEYS.NOTIFICATIONS,
                label: (
                  <Space>
                    <BellOutlined />
                    {formatMessage('profile.notifications.title')}
                  </Space>
                ),
                children: (
                  <Space orientation="vertical" size={16} style={{ width: '100%' }}>
                    {notifications.length === 0 && !extrasLoading ? (
                      <Alert
                        showIcon
                        type="info"
                        message={formatMessage('profile.notifications.unavailable')}
                        style={{ marginBottom: 16 }}
                      />
                    ) : null}
                    <NotificationsTab
                      items={notifications}
                      loading={extrasLoading}
                      detailMessage={detailMessage}
                      isAdminUser={isAdminUser}
                      onOpenMessage={openMessage}
                      onMarkAllRead={markAllRead}
                      onSendClick={() => setSendOpen(true)}
                      onDetailClose={() => setDetailMessage(null)}
                    />
                  </Space>
                ),
              },
            ]}
          />
        </Space>
      </PageContainer>
      {passwordModalEl}
      {broadcastModalEl}
      <AvatarModal
        open={avatarModalVisible}
        avatar={profile?.avatar}
        onClose={() => setAvatarModalVisible(false)}
        onPersisted={loadProfile}
      />
    </>
  );
}
