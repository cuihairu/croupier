import React, { useState, useEffect } from 'react';
import {
  Form,
  Select,
  Input,
  Button,
  Card,
  Space,
  Alert,
  Spin,
  Typography,
  Divider,
  Tag,
} from 'antd';
import { PlayCircleOutlined, LoadingOutlined } from '@ant-design/icons';
import type { PlatformInfo } from '@/services/api/platforms';
import { callPlatform, listPlatformMethods } from '@/services/api/platforms';

const { TextArea } = Input;
const { Text, Paragraph } = Typography;

interface APITesterProps {
  platforms: PlatformInfo[];
  selectedPlatform: string | null;
  onPlatformChange: (platform: string) => void;
}

interface QuickSDKMethod {
  name: string;
  category: string;
  description: string;
  params: { name: string; type: string; required: boolean; description: string }[];
}

// QuickSDK 方法参数模板
const methodTemplates: Record<string, any> = {
  channel_list: {},
  server_list: { product_code: '' },
  product_list: {},
  role_info: { product_code: '', user_id: '', server_id: '' },
  order_list: {
    product_code: '',
    user_id: '',
    start_time: '',
    end_time: '',
    page: 1,
    page_size: 20,
  },
  day_report: { product_code: '', date: '' },
  day_hour_report: { product_code: '', date: '' },
  user_live: { product_code: '', date: '', start_date: '', end_date: '' },
  channel_days_report: { product_code: '', channel_code: '', start_date: '', end_date: '' },
  channel_report: { product_code: '', channel_code: '', date: '' },
  ad_report: { product_code: '', start_date: '', end_date: '' },
  media_app_list: {},
  ad_plan_group_list: { product_code: '' },
  package_version_list: { product_code: '' },
  ad_pages_list: { product_code: '' },
  create_ad_plan: { product_code: '', plan_name: '', start_time: '', end_time: '', budget: 0 },
  update_ad_plan: { product_code: '', plan_id: '', plan_name: '', status: 1 },
  ad_plan_list: { product_code: '', page: 1, page_size: 20 },
  user_lost_list: { product_code: '', server_id: '', start_time: '', end_time: '' },
  push_message: { product_code: '', user_ids: [], title: '', content: '' },
};

const methodDescriptions: Record<string, string> = {
  channel_list: '获取渠道列表 - 获取游戏配置的所有渠道信息',
  server_list: '获取区服列表 - 获取游戏的区服/服务器列表',
  product_list: '获取产品列表 - 获取账号下的所有产品',
  role_info: '获取角色信息 - 查询指定玩家的角色详细信息',
  order_list: '获取订单列表 - 查询玩家的订单记录',
  day_report: '单日报表 - 获取指定日期的游戏数据报表',
  day_hour_report: '每小时报表 - 获取指定日期每小时的详细数据',
  user_live: '玩家留存 - 查询玩家留存率数据',
  channel_days_report: '渠道多日报表 - 获取渠道在指定时间段的数据',
  channel_report: '渠道日报 - 获取指定渠道的日报数据',
  ad_report: '广告效果报表 - 获取广告投放效果数据',
  media_app_list: '广告主列表 - 获取广告主应用列表',
  ad_plan_group_list: '广告分组列表 - 获取广告计划的分组信息',
  package_version_list: '分包列表 - 获取广告分包版本信息',
  ad_pages_list: '落地页列表 - 获取广告落地页列表',
  create_ad_plan: '创建广告计划 - 创建新的广告投放计划',
  update_ad_plan: '更新广告计划 - 修改现有广告计划配置',
  ad_plan_list: '广告计划列表 - 查询所有广告计划',
  user_lost_list: '流失预警 - 获取可能流失的玩家列表',
  push_message: '消息推送 - 向指定玩家推送消息',
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
  const [response, setResponse] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);

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
      setResponse(result.response);
    } catch (err: any) {
      setError(err?.response?.data?.message || err?.message || '请求失败');
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

                  {Form.useWatch('method', form) && (
                    <Alert
                      message={
                        <span>
                          {getMethodDescription(Form.useWatch('method', form))}
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
