import React, { useEffect, useMemo, useState } from 'react';
import { Button, Card, Select, Space, Steps, Typography, message } from 'antd';
import { RocketOutlined } from '@ant-design/icons';
import { listDescriptors, type FunctionDescriptor } from '@/services/api/functions';
import { schemaProperties } from './types';

const { Text, Title } = Typography;

/** 快速开始向导：选「列表函数 + 操作函数」一键生成玩家管理页骨架
 * （表格+行操作+弹窗+提交后刷新），把五步拖拽变成两步选择。 */
export default function QuickStart({
  onGenerate,
  onSkip,
}: {
  onGenerate: (tableFn: FunctionDescriptor, actionFn: FunctionDescriptor) => void;
  onSkip: () => void;
}) {
  const [descriptors, setDescriptors] = useState<FunctionDescriptor[]>([]);
  const [tableId, setTableId] = useState<string | undefined>();
  const [actionId, setActionId] = useState<string | undefined>();

  useEffect(() => {
    listDescriptors()
      .then((d) => setDescriptors(d))
      .catch(() => setDescriptors([]));
  }, []);

  const listFns = useMemo(
    () => descriptors.filter((f) => ['list', 'query', 'search'].includes(f.operation ?? '')),
    [descriptors],
  );
  const actionFns = useMemo(
    () => descriptors.filter((f) => !['list', 'query', 'search'].includes(f.operation ?? '')),
    [descriptors],
  );

  const tableFn = descriptors.find((f) => f.id === tableId);
  const actionFn = descriptors.find((f) => f.id === actionId);

  // 同名字段自动映射：操作函数输入 ← 表格行字段
  const autoMapping = useMemo(() => {
    if (!tableFn || !actionFn) return [];
    const rowFields = schemaProperties(tableFn.outputSchema);
    return schemaProperties(actionFn.inputSchema).filter((p) => rowFields.includes(p));
  }, [tableFn, actionFn]);

  if (descriptors.length === 0) return null;

  return (
    <Card
      size="small"
      style={{ maxWidth: 640, margin: '24px auto' }}
      title={
        <Space size={6}>
          <RocketOutlined style={{ color: '#1677ff' }} />
          <Title level={5} style={{ margin: 0 }}>
            快速开始：两步生成「列表 + 行操作弹窗」页面
          </Title>
        </Space>
      }
    >
      <Steps
        direction="vertical"
        size="small"
        current={actionId ? 2 : tableId ? 1 : 0}
        items={[
          {
            title: '选择列表函数（表格数据来源）',
            description: (
              <Select
                size="small"
                showSearch
                optionFilterProp="label"
                placeholder="如 player.list / inventory.list"
                style={{ width: 320, marginTop: 4 }}
                value={tableId}
                onChange={setTableId}
                options={listFns.map((f) => ({
                  value: f.id,
                  label: `${f.id}${f.summary?.['zh-CN'] ? `（${f.summary['zh-CN']}）` : ''}`,
                }))}
                notFoundContent={
                  <Text type="secondary">当前 scope 没有列表类函数（operation=list）</Text>
                }
              />
            ),
          },
          {
            title: '选择操作函数（弹窗表单，提交后自动刷新表格）',
            description: (
              <Select
                size="small"
                showSearch
                optionFilterProp="label"
                placeholder="如 mail.send / player.ban"
                style={{ width: 320, marginTop: 4 }}
                value={actionId}
                onChange={setActionId}
                options={actionFns.map((f) => ({
                  value: f.id,
                  label: `${f.id}${f.summary?.['zh-CN'] ? `（${f.summary['zh-CN']}）` : ''}`,
                }))}
              />
            ),
          },
          {
            title: '生成',
            description: (
              <Space direction="vertical" size={4} style={{ marginTop: 4 }}>
                {tableFn && actionFn && autoMapping.length > 0 && (
                  <Text type="success" style={{ fontSize: 12 }}>
                    自动行字段映射：{autoMapping.map((p) => `${p}←行.${p}`).join('、')}
                  </Text>
                )}
                <Space size={8}>
                  <Button
                    type="primary"
                    disabled={!tableFn || !actionFn}
                    onClick={() => {
                      if (tableFn && actionFn) {
                        onGenerate(tableFn, actionFn);
                        message.success(
                          '骨架已生成——继续从左栏添加任意组件（同一函数也可以拖多次）',
                        );
                      }
                    }}
                  >
                    生成页面（表格 + 行操作 + 弹窗 + 提交刷新）
                  </Button>
                  <Button onClick={onSkip}>跳过，自由搭建</Button>
                </Space>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  向导只是起点：生成后可从左侧继续添加任意多个组件、任意函数
                  （同一函数可拖多个实例分别配置）
                </Text>
              </Space>
            ),
          },
        ]}
      />
    </Card>
  );
}
