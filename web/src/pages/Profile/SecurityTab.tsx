import { useCallback, type ReactNode } from 'react';
import { Alert, Button, Card, Col, Row, Space, Switch, Tag, Tooltip, Typography } from 'antd';
import { HistoryOutlined, LockOutlined, PhoneOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import MfaSettings from './MfaSettings';
import type { NotificationChannelState } from './shared';

const { Text } = Typography;

/** 把后端的稳定 reason 短语翻译成可展示文案；未知短语按通道给兜底。 */
function reasonText(
  ch: NotificationChannelState,
  formatMessage: (id: string, fallback: string) => string,
): string {
  switch (ch.reason) {
    case 'smtp_not_configured':
      return formatMessage('profile.channel.reason.smtpMissing', '未配置 SMTP 服务器');
    case 'in_app_disabled':
      return formatMessage('profile.channel.reason.inAppOff', '站内信已被管理员关闭');
    default:
      // 'sms provider not configured' 及任何未来新增的未接入原因都落到通道级兜底
      return ch.key === 'sms'
        ? formatMessage('profile.channel.reason.smsMissing', '未接入短信服务')
        : formatMessage('profile.channel.reason.notConnected', '未接入该通知通道');
  }
}

/**
 * 依据后端上报的通道状态计算「通道能否送达通知」以及不可用时的原因文案。
 *
 * 判断顺序很关键：
 *   1. **先看 reason**：它承载的是「通道被关闭/未接入」这类平台侧事实。站内信
 *      的 `available` 恒为 true（消息表永远就绪），「被管理员关闭」只体现在
 *      reason 上——若先判 available，这条原因就永远不会展示。
 *   2. 再看 available：未接入服务商时为 false。
 *   3. 最后看用户是否填了接收目标：通道可用但没手机号/邮箱，同样收不到。
 */
export function channelAvailability(
  ch: NotificationChannelState,
  formatMessage: (id: string, fallback: string) => string,
): { enabled: boolean; reason: string | null } {
  if (ch.reason) {
    return { enabled: false, reason: reasonText(ch, formatMessage) };
  }
  if (!ch.available) {
    // available=false 却没带 reason：仍必须视为不可达（否则会出现无解释的灰开关），
    // 只是原因退化为通道级兜底。
    return { enabled: false, reason: reasonText(ch, formatMessage) };
  }
  if (ch.requiresTarget && !ch.hasTarget) {
    return {
      enabled: false,
      reason: ch.key === 'sms'
        ? formatMessage('profile.channel.reason.noPhone', '未填写手机号，填了才能接收')
        : formatMessage('profile.channel.reason.noEmail', '未填写邮箱，填了才能接收'),
    };
  }
  return { enabled: true, reason: null };
}

/** 单个通知通道行：真实状态 + 不可用时明确说明原因。 */
function ChannelRow({
  channel,
  formatMessage,
}: {
  channel: NotificationChannelState;
  formatMessage: (id: string, fallback: string) => string;
}) {
  const { enabled: operational, reason } = channelAvailability(channel, formatMessage);
  const label = {
    in_app: formatMessage('profile.channel.inApp', '站内消息'),
    email: formatMessage('profile.channel.email', '邮件通知'),
    sms: formatMessage('profile.channel.sms', '短信通知'),
  }[channel.key] ?? channel.key;

  // 开关只读呈现 `userEnabled`（当前来自平台级设置，没有用户侧写入口）。
  // 可点击却没有任何效果的开关同样是假状态——等出现「用户通知偏好」写接口
  // 时再把 disabled 去掉。
  const switchNode = (
    <Switch
      checked={channel.userEnabled}
      disabled
      data-testid={`channel-switch-${channel.key}`}
      aria-label={label}
    />
  );

  // 状态标签三分支：不可达（warning：未接入/已关闭）、送达且开启（success）、
  // 送达但未开启（default）。最后一支是关键——否则「SMTP 已配置但平台关了
  // 邮件通知」会渲染成绿色「已开启」+ 未勾选的开关，自相矛盾。
  let status: ReactNode;
  if (!operational) {
    const text =
      channel.reason === 'in_app_disabled'
        ? formatMessage('profile.channel.off', '已关闭')
        : formatMessage('profile.channel.notConnected', '未接入');
    status = (
      <Tooltip title={reason}>
        <span>
          <Tag color="warning" data-testid={`channel-status-${channel.key}`}>
            {text}
          </Tag>
          {switchNode}
        </span>
      </Tooltip>
    );
  } else if (channel.userEnabled) {
    status = (
      <Space>
        <Tag color="success" data-testid={`channel-status-${channel.key}`}>
          {formatMessage('profile.channel.on', '已开启')}
        </Tag>
        {switchNode}
      </Space>
    );
  } else {
    status = (
      <Space>
        <Tag data-testid={`channel-status-${channel.key}`}>
          {formatMessage('profile.channel.off', '已关闭')}
        </Tag>
        {switchNode}
      </Space>
    );
  }

  return (
    <div className="security-item" data-testid={`channel-row-${channel.key}`}>
      <Space>
        {channel.key === 'sms' ? <PhoneOutlined /> : <HistoryOutlined />}
        <div>
          <Text strong>{label}</Text>
          <br />
          <Text type="secondary">
            {reason ??
              formatMessage('profile.channel.ready', '已接入，可正常接收通知')}
          </Text>
        </div>
      </Space>
      {status}
    </div>
  );
}

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

            {notificationChannels.length === 0 ? (
              <Alert
                type="info"
                showIcon
                data-testid="channels-loading"
                title={formatMessage(
                  'profile.channel.loading',
                  '正在读取通知通道状态…',
                )}
              />
            ) : (
              <div data-testid="channel-list">
                <Text strong>
                  {formatMessage('profile.channel.section', '通知通道')}
                </Text>
                {notificationChannels.map((ch) => (
                  <ChannelRow key={ch.key} channel={ch} formatMessage={formatMessage} />
                ))}
              </div>
            )}

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
