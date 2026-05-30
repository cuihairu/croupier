import React from 'react';
import { Card, Typography, Divider, Tag, Alert, Space, Descriptions } from 'antd';
import { CheckCircleOutlined, InfoCircleOutlined, WarningOutlined } from '@ant-design/icons';

const { Title, Paragraph, Text } = Typography;

export default function PlatformConfig() {
  return (
    <div>
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        {/* 概述 */}
        <Card>
          <Title level={4}>第三方运营平台接入</Title>
          <Paragraph>
            本系统支持集成多个第三方运营平台，实现数据查询、广告管理、消息推送等功能。
            目前支持的平台包括 QuickSDK，未来可扩展至更多平台。
          </Paragraph>
          <Alert
            message="平台能力由官方扩展提供，需要先安装并启用 official.external-platform"
            type="info"
            showIcon
            icon={<InfoCircleOutlined />}
          />
        </Card>

        {/* QuickSDK 配置 */}
        <Card
          title={
            <Space>
              <CheckCircleOutlined /> QuickSDK 配置说明
            </Space>
          }
        >
          <Descriptions bordered column={1} size="small">
            <Descriptions.Item label="平台类型">QuickSDK</Descriptions.Item>
            <Descriptions.Item label="官方文档">
              <a href="https://www.quicksdk.com/" target="_blank" rel="noopener noreferrer">
                https://www.quicksdk.com/
              </a>
            </Descriptions.Item>
            <Descriptions.Item label="支持的API数量">
              <Tag color="blue">20+ 个接口</Tag>
            </Descriptions.Item>
          </Descriptions>

          <Divider orientation="left">配置参数</Divider>

          <Card size="small" type="inner" title="环境变量配置">
            <pre
              style={{
                background: '#f5f5f5',
                padding: 12,
                borderRadius: 4,
                overflow: 'auto',
              }}
            >
              {`# QuickSDK 配置
export QUICKSDK_OPEN_ID="your_open_id"
export QUICKSDK_OPEN_KEY="your_open_key"
export QUICKSDK_API_BASE_URL="https://www.quicksdk.com"

# 可选：扩展严格模式（建议）
export CROUPIER_PLATFORM_EXTENSION_ONLY=true`}
            </pre>
          </Card>

          <Card size="small" type="inner" title="扩展安装配置（示例）" style={{ marginTop: 16 }}>
            <pre
              style={{
                background: '#f5f5f5',
                padding: 12,
                borderRadius: 4,
                overflow: 'auto',
              }}
            >
              {`extension_id: official.external-platform
release_version: main
scope_type: system
scope_id: global
target_type: agent_group
target_id: default
config:
  providers:
    quicksdk:
      enabled: true
      open_id: "${QUICKSDK_OPEN_ID}"
      open_key: "${QUICKSDK_OPEN_KEY}"
      api_base_url: "https://www.quicksdk.com"
      timeout: "30s"
      retry_count: 3`}
            </pre>
          </Card>
        </Card>

        {/* 支持的 API */}
        <Card title="QuickSDK 支持的 API">
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <div>
              <Text strong>基础数据 (5 个 API)</Text>
              <div style={{ marginTop: 8 }}>
                {['channel_list', 'server_list', 'product_list', 'role_info', 'order_list'].map(
                  (m) => (
                    <Tag key={m} color="blue" style={{ margin: '4px' }}>
                      {m}
                    </Tag>
                  ),
                )}
              </div>
            </div>

            <div>
              <Text strong>运营报表 (5 个 API)</Text>
              <div style={{ marginTop: 8 }}>
                {[
                  'day_report',
                  'day_hour_report',
                  'user_live',
                  'channel_days_report',
                  'channel_report',
                ].map((m) => (
                  <Tag key={m} color="green" style={{ margin: '4px' }}>
                    {m}
                  </Tag>
                ))}
              </div>
            </div>

            <div>
              <Text strong>广告管理 (9 个 API)</Text>
              <div style={{ marginTop: 8 }}>
                {[
                  'ad_report',
                  'media_app_list',
                  'ad_plan_group_list',
                  'package_version_list',
                  'ad_pages_list',
                  'create_ad_plan',
                  'update_ad_plan',
                  'ad_plan_list',
                ].map((m) => (
                  <Tag key={m} color="purple" style={{ margin: '4px' }}>
                    {m}
                  </Tag>
                ))}
              </div>
            </div>

            <div>
              <Text strong>其他 (2 个 API)</Text>
              <div style={{ marginTop: 8 }}>
                {['user_lost_list', 'push_message'].map((m) => (
                  <Tag key={m} color="orange" style={{ margin: '4px' }}>
                    {m}
                  </Tag>
                ))}
              </div>
            </div>
          </Space>
        </Card>

        {/* 使用说明 */}
        <Card
          title={
            <Space>
              <WarningOutlined /> 使用说明
            </Space>
          }
        >
          <Space direction="vertical" size="small">
            <Alert
              message="API 调用需要正确的请求参数"
              description="不同 API 需要不同的请求参数，请参考 QuickSDK 官方文档或使用 API 测试功能查看参数模板。"
              type="warning"
              showIcon
            />

            <Alert
              message="速率限制"
              description="为保护第三方平台服务，系统对 API 调用实施了速率限制。默认配置为每分钟 1000 次请求，突发 100 次。"
              type="info"
              showIcon
            />

            <Alert
              message="缓存机制"
              description="部分不经常变化的数据（如渠道列表、区服列表）会启用缓存，默认缓存时间为 5 分钟，可减少对第三方平台的请求压力。"
              type="info"
              showIcon
            />
          </Space>
        </Card>

        {/* 故障排除 */}
        <Card title="故障排除">
          <Descriptions bordered column={1}>
            <Descriptions.Item label="503 错误">
              平台扩展未安装或未启用，请在扩展商店安装并启用 `official.external-platform`。
            </Descriptions.Item>
            <Descriptions.Item label="签名验证失败">
              请检查 `open_id` 和 `open_key` 配置是否正确。
            </Descriptions.Item>
            <Descriptions.Item label="超时">
              可在扩展安装配置中调整 `timeout` 参数，默认为 30 秒。
            </Descriptions.Item>
            <Descriptions.Item label="速率限制">
              如需更高请求速率，可在 provider 配置中调整速率限制参数。
            </Descriptions.Item>
          </Descriptions>
        </Card>
      </Space>
    </div>
  );
}
