import React, { useState, useEffect } from 'react';
import { Form, Select, Input, Button, Card, Space, Alert, Spin, Tag } from 'antd';
import { PlayCircleOutlined, LoadingOutlined } from '@ant-design/icons';
import type { PlatformInfo } from '@/services/api/platforms';
import { callPlatform, listPlatformMethods } from '@/services/api/platforms';
import type { JSONValue } from '@/types/dashboard';

const { TextArea } = Input;

interface APITesterProps {
  platforms: PlatformInfo[];
  selectedPlatform: string | null;
  onPlatformChange: (platform: string) => void;
}

// QuickSDK 方法参数模板
const methodTemplates: Record<string, JSONValue> = {
  channelList: {},
  serverList: { productCode: '' },
  productList: {},
  roleInfo: { productCode: '', userId: '', serverId: '' },
  orderList: {
    productCode: '',
    userId: '',
    startTime: '',
    endTime: '',
    page: 1,
    pageSize: 20,
  },
  dayReport: { productCode: '', date: '' },
  dayHourReport: { productCode: '', date: '' },
  userLive: { productCode: '', date: '', startDate: '', endDate: '' },
  channelDaysReport: { productCode: '', channelCode: '', startDate: '', endDate: '' },
  channelReport: { productCode: '', channelCode: '', date: '' },
  adReport: { productCode: '', startDate: '', endDate: '' },
  mediaAppList: {},
  adPlanGroupList: { productCode: '' },
  packageVersionList: { productCode: '' },
  adPagesList: { productCode: '' },
  createAdPlan: { productCode: '', planName: '', startTime: '', endTime: '', budget: 0 },
  updateAdPlan: { productCode: '', planId: '', planName: '', status: 1 },
  adPlanList: { productCode: '', page: 1, pageSize: 20 },
  userLostList: { productCode: '', serverId: '', startTime: '', endTime: '' },
  pushMessage: { productCode: '', userIds: [], title: '', content: '' },
};

const methodDescriptions: Record<string, string> = {
  channelList: '获取渠道列表 - 获取游戏配置的所有渠道信息',
  serverList: '获取区服列表 - 获取游戏的区服/服务器列表',
  productList: '获取产品列表 - 获取账号下的所有产品',
  roleInfo: '获取角色信息 - 查询指定玩家的角色详细信息',
  orderList: '获取订单列表 - 查询玩家的订单记录',
  dayReport: '单日报表 - 获取指定日期的游戏数据报表',
  dayHourReport: '每小时报表 - 获取指定日期每小时的详细数据',
  userLive: '玩家留存 - 查询玩家留存率数据',
  channelDaysReport: '渠道多日报表 - 获取渠道在指定时间段的数据',
  channelReport: '渠道日报 - 获取指定渠道的日报数据',
  adReport: '广告效果报表 - 获取广告投放效果数据',
  mediaAppList: '广告主列表 - 获取广告主应用列表',
  adPlanGroupList: '广告分组列表 - 获取广告计划的分组信息',
  packageVersionList: '分包列表 - 获取广告分包版本信息',
  adPagesList: '落地页列表 - 获取广告落地页列表',
  createAdPlan: '创建广告计划 - 创建新的广告投放计划',
  updateAdPlan: '更新广告计划 - 修改现有广告计划配置',
  adPlanList: '广告计划列表 - 查询所有广告计划',
  userLostList: '流失预警 - 获取可能流失的玩家列表',
  pushMessage: '消息推送 - 向指定玩家推送消息',
};

const methodsByCategory: Record<string, string[]> = {
  基础数据: ['channel_list', 'server_list', 'product_list', 'role_info', 'order_list'],
  运营报表: ['day_report', 'day_hour_report', 'user_live', 'channel_days_report', 'channel_report'],
  广告管理: [
    'ad_report',
    'media_app_list',
    'ad_plan_group_list',
    'package_version_list',
    'ad_pages_list',
    'create_ad_plan',
    'update_ad_plan',
    'ad_plan_list',
  ],
  其他: ['user_lost_list', 'push_message'],
};

export default function APITester({
  platforms,
  selectedPlatform,
  onPlatformChange,
}: APITesterProps) {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [calling, setCalling] = useState(false);
  const [availableMethods, setAvailableMethods] = useState<string[]>([]);
  const [methodsSource, setMethodsSource] = useState<string>('');
  const [callSource, setCallSource] = useState<string>('');
  const [response, setResponse] = useState<JSONValue>(null);
  const [error, setError] = useState<string | null>(null);
  const currentMethod = Form.useWatch('method', form);

  const currentPlatform = platforms.find((p) => p.name === selectedPlatform);

  // 加载平台方法列表
  useEffect(() => {
    if (selectedPlatform) {
      setLoading(true);
      listPlatformMethods(selectedPlatform)
        .then((r) => {
          setAvailableMethods(r?.methods || []);
          setMethodsSource(r?.source || '');
        })
        .catch(() => {
          setAvailableMethods([]);
          setMethodsSource('');
        })
        .finally(() => {
          setLoading(false);
        });
    }
  }, [selectedPlatform]);

  // 平台选择变化时重置表单
  useEffect(() => {
    if (selectedPlatform) {
      form.setFieldsValue({ platform: selectedPlatform });
    }
  }, [selectedPlatform, form]);

  // 方法选择变化时更新请求参数模板
  const handleMethodChange = (method: string) => {
    const template = methodTemplates[method] || {};
    form.setFieldsValue({ request: JSON.stringify(template, null, 2) });
    setResponse(null);
    setError(null);
  };

  // 调用 API
  const handleCall = async () => {
    try {
      const values = await form.validateFields();
      setCalling(true);
      setError(null);
      setResponse(null);

      const result = await callPlatform({
        platform: values.platform,
        method: values.method,
        request: values.request,
      });
      setCallSource(result?.source || '');
      setResponse(result.response ?? null);
    } catch (err) {
      const errMsg = err instanceof Error ? err.message : '请求失败';
      setError(errMsg);
    } finally {
      setCalling(false);
    }
  };

  // 格式化请求参数
  const formatRequest = (value: string) => {
    try {
      const parsed = JSON.parse(value);
      return JSON.stringify(parsed, null, 2);
    } catch {
      return value;
    }
  };

  // 获取方法描述
  const getMethodDescription = (method: string) => {
    return methodDescriptions[method] || '调用第三方平台 API';
  };

  return (
    <div>
      <Form form={form} layout="vertical" initialValues={{ request: '{}' }}>
        <Space direction="vertical" style={{ width: '100%' }} size="large">
          {/* 平台选择 */}
          <Card size="small" title="选择平台">
            <Form.Item
              name="platform"
              label="平台"
              rules={[{ required: true, message: '请选择平台' }]}
            >
              <Select
                placeholder="请选择平台"
                value={selectedPlatform}
                onChange={onPlatformChange}
                options={platforms.map((p) => ({
                  label: p.name === 'quicksdk' ? 'QuickSDK' : p.name,
                  value: p.name,
                }))}
              />
            </Form.Item>

            {currentPlatform && (
              <Alert
                message={
                  <span>
                    <Tag color={currentPlatform.enabled ? 'green' : 'default'}>
                      {currentPlatform.enabled ? '已启用' : '未启用'}
                    </Tag>{' '}
                    支持 {currentPlatform.methods?.length || 0} 个 API 方法
                    {currentPlatform.source && (
                      <>
                        {' '}
                        <Tag color={currentPlatform.source === 'extension' ? 'green' : 'default'}>
                          {currentPlatform.source}
                        </Tag>
                      </>
                    )}
                  </span>
                }
                type={currentPlatform.enabled ? 'info' : 'warning'}
                showIcon
              />
            )}
          </Card>

          {/* 方法选择 */}
          {selectedPlatform && (
            <Card size="small" title="选择方法">
              {loading ? (
                <Spin />
              ) : (
                <>
                  <Form.Item
                    name="method"
                    label="方法"
                    rules={[{ required: true, message: '请选择方法' }]}
                  >
                    <Select
                      placeholder="请选择要调用的方法"
                      onChange={handleMethodChange}
                      showSearch
                      optionFilterProp="label"
                    >
                      {Object.entries(methodsByCategory).map(([category, methods]) => {
                        const categoryMethods = methods.filter((m) => availableMethods.includes(m));
                        if (categoryMethods.length === 0) return null;
                        return (
                          <Select.OptGroup key={category} label={category}>
                            {categoryMethods.map((method) => (
                              <Select.Option key={method} value={method} label={method}>
                                <Space>
                                  <span>{method}</span>
                                  <Tag
                                    color={availableMethods.includes(method) ? 'green' : 'default'}
                                  >
                                    API
                                  </Tag>
                                </Space>
                              </Select.Option>
                            ))}
                          </Select.OptGroup>
                        );
                      })}
                    </Select>
                  </Form.Item>

                  {currentMethod && (
                    <Alert
                      message={
                        <span>
                          {getMethodDescription(currentMethod)}
                          {methodsSource && (
                            <>
                              {' '}
                              <Tag color={methodsSource === 'extension' ? 'green' : 'default'}>
                                methods: {methodsSource}
                              </Tag>
                            </>
                          )}
                        </span>
                      }
                      type="info"
                      showIcon
                      style={{ marginTop: 8 }}
                    />
                  )}
                </>
              )}
            </Card>
          )}

          {/* 请求参数 */}
          <Card size="small" title="请求参数 (JSON)">
            <Form.Item
              name="request"
              rules={[
                { required: true, message: '请输入请求参数' },
                {
                  validator: (_, value) => {
                    try {
                      JSON.parse(value);
                      return Promise.resolve();
                    } catch {
                      return Promise.reject(new Error('请输入有效的 JSON 格式'));
                    }
                  },
                },
              ]}
            >
              <TextArea
                rows={10}
                placeholder='请输入 JSON 格式的请求参数，例如：{"product_code": "xxx", "date": "2024-01-01"}'
                onBlur={(e) => {
                  form.setFieldsValue({ request: formatRequest(e.target.value) });
                }}
                style={{ fontFamily: 'Monaco, Menlo, "Ubuntu Mono", Consolas, monospace' }}
              />
            </Form.Item>

            <Button
              type="primary"
              icon={calling ? <LoadingOutlined /> : <PlayCircleOutlined />}
              onClick={handleCall}
              loading={calling}
              disabled={!selectedPlatform || !availableMethods.length}
              block
            >
              {calling ? '调用中...' : '调用 API'}
            </Button>
          </Card>

          {/* 响应结果 */}
          {(response || error) && (
            <Card
              size="small"
              title={error ? '错误信息' : '响应结果'}
              extra={error && <Tag color="error">失败</Tag>}
            >
              {error ? (
                <Alert type="error" message={error} showIcon />
              ) : (
                <div>
                  <Alert
                    message={
                      <Tag color="success" icon="✓">
                        调用成功
                      </Tag>
                    }
                    type="success"
                    showIcon={false}
                    style={{ marginBottom: 12 }}
                  />
                  {callSource && (
                    <div style={{ marginBottom: 12 }}>
                      <Tag color={callSource === 'extension' ? 'green' : 'default'}>
                        call: {callSource}
                      </Tag>
                    </div>
                  )}
                  <pre
                    style={{
                      background: '#f5f5f5',
                      padding: 12,
                      borderRadius: 4,
                      overflow: 'auto',
                      maxHeight: 400,
                      fontSize: 12,
                    }}
                  >
                    {JSON.stringify(response, null, 2)}
                  </pre>
                </div>
              )}
            </Card>
          )}
        </Space>
      </Form>
    </div>
  );
}
