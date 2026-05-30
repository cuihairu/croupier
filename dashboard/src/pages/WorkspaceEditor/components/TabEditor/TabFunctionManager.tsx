import React from 'react';
import {
  Alert,
  Card,
  Button,
  Divider,
  Empty,
  List,
  Space,
  Descriptions,
  Modal,
  Popover,
  Tag,
  Tooltip,
  Typography,
  message,
  Collapse,
} from 'antd';
import { DeleteOutlined, EyeOutlined, PlusOutlined, ThunderboltOutlined } from '@ant-design/icons';
import type { TabConfig } from '@/types/workspace';
import type { FunctionDescriptor } from '@/services/api/functions';
import { CodeEditor } from '@/components/MonacoDynamic';
import FunctionSelectorModal from '../FunctionSelectorModal';
import {
  recommendFunctions,
  getReasonText,
  getReasonColor,
  type FunctionRecommendation,
} from '../../utils/functionRecommender';

const { Text } = Typography;

/** 函数详情 Popover 内容 */
function FunctionPopoverContent({ descriptor }: { descriptor: FunctionDescriptor }) {
  const inputSchema = descriptor.inputSchema
    ? (() => {
        try {
          return JSON.parse(descriptor.inputSchema);
        } catch {
          return null;
        }
      })()
    : descriptor.params;
  const outputSchema = descriptor.outputSchema
    ? (() => {
        try {
          return JSON.parse(descriptor.outputSchema);
        } catch {
          return null;
        }
      })()
    : descriptor.outputs;

  const inputProps = inputSchema?.properties ? Object.keys(inputSchema.properties) : [];
  const outputProps = outputSchema?.properties ? Object.keys(outputSchema.properties) : [];

  return (
    <div style={{ maxWidth: 360 }}>
      <Descriptions column={1} size="small" bordered={false}>
        <Descriptions.Item label="ID">
          <Text code copyable={{ text: descriptor.id }} style={{ fontSize: 12 }}>
            {descriptor.id}
          </Text>
        </Descriptions.Item>
        {descriptor.description && (
          <Descriptions.Item label="描述">{descriptor.description}</Descriptions.Item>
        )}
        {descriptor.entity && (
          <Descriptions.Item label="实体">
            {descriptor.entityDisplay?.zh || descriptor.entity}
          </Descriptions.Item>
        )}
        {descriptor.operation && (
          <Descriptions.Item label="操作">
            {descriptor.operationDisplay?.zh || descriptor.operation}
          </Descriptions.Item>
        )}
        {descriptor.tags && descriptor.tags.length > 0 && (
          <Descriptions.Item label="标签">
            {descriptor.tags.map((t) => (
              <Tag key={t} style={{ marginBottom: 2 }}>
                {t}
              </Tag>
            ))}
          </Descriptions.Item>
        )}
        {inputProps.length > 0 && (
          <Descriptions.Item label="输入参数">
            <Space size={[4, 2]} wrap>
              {inputProps.slice(0, 8).map((p) => (
                <Tag key={p} color="blue">
                  {p}
                </Tag>
              ))}
              {inputProps.length > 8 && <Tag>+{inputProps.length - 8}</Tag>}
            </Space>
          </Descriptions.Item>
        )}
        {outputProps.length > 0 && (
          <Descriptions.Item label="输出字段">
            <Space size={[4, 2]} wrap>
              {outputProps.slice(0, 8).map((p) => (
                <Tag key={p} color="green">
                  {p}
                </Tag>
              ))}
              {outputProps.length > 8 && <Tag>+{outputProps.length - 8}</Tag>}
            </Space>
          </Descriptions.Item>
        )}
      </Descriptions>
    </div>
  );
}

export interface TabFunctionManagerProps {
  tab: TabConfig;
  descriptors: FunctionDescriptor[];
  onDrop: (e: React.DragEvent) => void;
  onRemoveFunction: (functionId: string) => void;
  onOpenLayoutWizard: (descriptor: FunctionDescriptor) => void;
  onAddFunctions?: (functionIds: string[]) => void;
}

export default function TabFunctionManager({
  tab,
  descriptors,
  onDrop,
  onRemoveFunction,
  onOpenLayoutWizard,
  onAddFunctions,
}: TabFunctionManagerProps) {
  const [descriptorPreviewOpen, setDescriptorPreviewOpen] = React.useState(false);
  const [previewDescriptor, setPreviewDescriptor] = React.useState<FunctionDescriptor | null>(null);
  const [functionSelectorOpen, setFunctionSelectorOpen] = React.useState(false);

  // 计算推荐函数
  const recommendations = React.useMemo<FunctionRecommendation[]>(() => {
    const existingFunctions = tab.functions || [];
    return recommendFunctions(existingFunctions, descriptors, {
      maxResults: 4,
      excludeExisting: true,
      minScore: 30,
    });
  }, [tab.functions, descriptors]);
  const primaryRecommendation = recommendations[0];
  const secondaryRecommendations = recommendations.slice(1);
  const primaryFunctionId = tab.functions?.[0];
  const primaryDescriptor = descriptors.find((d) => d.id === primaryFunctionId);
  const secondaryFunctionIds = (tab.functions || []).slice(1);

  const handlePreviewFunctionJson = (functionId: string) => {
    const descriptor = descriptors.find((d) => d.id === functionId);
    if (!descriptor) {
      message.warning('未找到函数描述符');
      return;
    }
    setPreviewDescriptor(descriptor);
    setDescriptorPreviewOpen(true);
  };

  const beforeMountJsonEditor = (monaco: any) => {
    if (!monaco?.editor || monaco.editor.getTheme?.() === 'sublime-monokai') return;
    monaco.editor.defineTheme('sublime-monokai', {
      base: 'vs-dark',
      inherit: true,
      rules: [
        { token: 'string.key.json', foreground: '66D9EF' },
        { token: 'string.value.json', foreground: 'A6E22E' },
        { token: 'number', foreground: 'E6DB74' },
        { token: 'keyword', foreground: 'F92672' },
      ],
      colors: {
        'editor.background': '#272822',
        'editorLineNumber.foreground': '#75715E',
        'editorLineNumber.activeForeground': '#F8F8F2',
      },
    });
  };

  return (
    <>
      <Card
        title={
          <Space size={8}>
            <span>使用的函数</span>
            <Tag>{tab.functions?.length || 0}</Tag>
          </Space>
        }
        size="small"
        extra={
          <Space size={8}>
            <Button
              type="primary"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => setFunctionSelectorOpen(true)}
            >
              选择函数
            </Button>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              推荐先选主函数，再补辅助函数
            </Typography.Text>
          </Space>
        }
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Alert
            type={(tab.functions?.length || 0) > 0 ? 'info' : 'warning'}
            showIcon
            message={
              (tab.functions?.length || 0) > 0
                ? '函数已绑定，可以继续生成页面骨架。'
                : '当前还没有绑定函数。'
            }
            description={
              (tab.functions?.length || 0) > 0
                ? '建议保留 1 到 3 个核心函数，并先明确谁是主函数。页面骨架默认优先围绕第一个函数生成，其它函数更适合作为辅助查询、补充提交或详情来源。'
                : '先用“选择函数”添加核心函数；如果你更熟悉工作流，也可以从左侧函数面板拖拽到下方区域。'
            }
          />

          {(tab.functions?.length || 0) > 0 && (
            <Card size="small" title="当前函数角色">
              <Space direction="vertical" size={10} style={{ width: '100%' }}>
                <Alert
                  type="info"
                  showIcon
                  message={
                    primaryDescriptor
                      ? `主函数: ${primaryDescriptor.displayName?.zh || primaryDescriptor.id}`
                      : `主函数: ${primaryFunctionId}`
                  }
                  description="当前列表里的第一个函数会被默认当成主函数。自动生成页面骨架、自动补全列/字段时，都会优先参考它。"
                />
                <Card
                  size="small"
                  style={{ background: '#f6ffed', borderColor: '#b7eb8f' }}
                  title="当前推荐先围绕这个主函数做页面"
                  extra={
                    primaryFunctionId ? (
                      <Button
                        size="small"
                        type="primary"
                        onClick={() => {
                          const d = descriptors.find((x) => x.id === primaryFunctionId);
                          if (!d) {
                            message.warning('未找到函数描述符');
                            return;
                          }
                          onOpenLayoutWizard(d);
                        }}
                      >
                        直接生成页面骨架
                      </Button>
                    ) : null
                  }
                >
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    <Space wrap size={[8, 8]}>
                      <Tag color="success">主函数</Tag>
                      <Typography.Text strong>
                        {primaryDescriptor?.displayName?.zh ||
                          primaryDescriptor?.displayName?.en ||
                          primaryFunctionId ||
                          '未设置'}
                      </Typography.Text>
                      {primaryDescriptor?.operation ? (
                        <Tag color="blue">
                          {primaryDescriptor.operationDisplay?.zh || primaryDescriptor.operation}
                        </Tag>
                      ) : null}
                    </Space>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      先用它把页面骨架跑通。只有主流程已经明确时，再补查询、提交或详情类辅助函数。
                    </Typography.Text>
                  </Space>
                </Card>
                {secondaryFunctionIds.length > 0 ? (
                  <Card size="small" title={`辅助函数 (${secondaryFunctionIds.length})`}>
                    <Space direction="vertical" size={8} style={{ width: '100%' }}>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        这些函数先作为补充能力保留，等主页面骨架跑通后，再决定是否让它们参与查询、提交或详情展示。
                      </Typography.Text>
                      <Space wrap size={[8, 8]}>
                        {secondaryFunctionIds.map((funcId) => {
                          const desc = descriptors.find((d) => d.id === funcId);
                          return (
                            <Tag key={funcId}>
                              {desc?.displayName?.zh || desc?.displayName?.en || funcId}
                            </Tag>
                          );
                        })}
                      </Space>
                    </Space>
                  </Card>
                ) : (
                  <Typography.Text type="secondary">
                    当前只有一个主函数。先把页面骨架跑通，再决定是否补充辅助函数。
                  </Typography.Text>
                )}
              </Space>
            </Card>
          )}

          {primaryRecommendation && (
            <div
              style={{
                padding: 12,
                border: '1px solid #f0f0f0',
                borderRadius: 8,
                background: '#fcfcfc',
              }}
            >
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <Space size={6}>
                  <ThunderboltOutlined style={{ color: '#faad14' }} />
                  <Typography.Text strong>推荐补充函数</Typography.Text>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    基于当前已绑定函数推断
                  </Typography.Text>
                </Space>
                <Alert
                  type="info"
                  showIcon
                  message={
                    primaryRecommendation.function.displayName?.zh ||
                    primaryRecommendation.function.id
                  }
                  description={
                    <Space wrap>
                      <Tag color={getReasonColor(primaryRecommendation.reason)}>
                        {getReasonText(primaryRecommendation.reason)}
                      </Tag>
                      <Typography.Text type="secondary">
                        {primaryRecommendation.description}
                      </Typography.Text>
                    </Space>
                  }
                  action={
                    <Button
                      size="small"
                      type="primary"
                      onClick={() => {
                        onAddFunctions?.([primaryRecommendation.function.id]);
                        message.success(
                          `已添加 ${
                            primaryRecommendation.function.displayName?.zh ||
                            primaryRecommendation.function.id
                          }`,
                        );
                      }}
                    >
                      添加推荐函数
                    </Button>
                  }
                />
                {secondaryRecommendations.length > 0 && (
                  <Collapse
                    size="small"
                    items={[
                      {
                        key: 'more-recommendations',
                        label: `更多推荐 (${secondaryRecommendations.length})`,
                        children: (
                          <Space wrap size={[8, 8]}>
                            {secondaryRecommendations.map((rec) => (
                              <Tooltip key={rec.function.id} title={rec.description}>
                                <Button
                                  size="small"
                                  onClick={() => {
                                    onAddFunctions?.([rec.function.id]);
                                    message.success(
                                      `已添加 ${rec.function.displayName?.zh || rec.function.id}`,
                                    );
                                  }}
                                >
                                  {`${
                                    rec.function.displayName?.zh || rec.function.id
                                  } · ${getReasonText(rec.reason)}`}
                                </Button>
                              </Tooltip>
                            ))}
                          </Space>
                        ),
                      },
                    ]}
                  />
                )}
              </Space>
            </div>
          )}

          <div
            onDrop={onDrop}
            onDragOver={(e) => e.preventDefault()}
            style={{
              minHeight: 96,
              border: '2px dashed #d9d9d9',
              borderRadius: 8,
              padding: 12,
              backgroundColor: '#fafafa',
            }}
          >
            {(tab.functions?.length || 0) > 0 ? (
              <List
                header={
                  <Space wrap size={[8, 8]}>
                    <Typography.Text strong>已绑定函数清单</Typography.Text>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      第一项就是主函数；如果主函数选错，建议先删掉重选，避免后面页面骨架围错对象。
                    </Typography.Text>
                  </Space>
                }
                size="small"
                dataSource={tab.functions}
                renderItem={(funcId) => {
                  const desc = descriptors.find((d) => d.id === funcId);
                  const isPrimary = funcId === primaryFunctionId;
                  return (
                    <List.Item
                      actions={[
                        <Button
                          key="json"
                          type="link"
                          size="small"
                          icon={<EyeOutlined />}
                          onClick={() => handlePreviewFunctionJson(funcId)}
                        >
                          查看 JSON
                        </Button>,
                        <Button
                          key="wizard"
                          type="link"
                          size="small"
                          onClick={() => {
                            const d = descriptors.find((x) => x.id === funcId);
                            if (!d) {
                              message.warning('未找到函数描述符');
                              return;
                            }
                            onOpenLayoutWizard(d);
                          }}
                        >
                          用它生成骨架
                        </Button>,
                        <Button
                          key="remove"
                          type="link"
                          danger
                          size="small"
                          icon={<DeleteOutlined />}
                          onClick={() => onRemoveFunction(funcId)}
                        />,
                      ]}
                    >
                      {desc ? (
                        <Popover
                          content={<FunctionPopoverContent descriptor={desc} />}
                          title={desc.displayName?.zh || desc.id}
                          trigger="hover"
                          placement="right"
                          mouseEnterDelay={0.3}
                        >
                          <Space size={8} wrap>
                            <Tag
                              color={isPrimary ? 'success' : 'blue'}
                              style={{ cursor: 'pointer', marginInlineEnd: 0 }}
                            >
                              {isPrimary ? '主函数' : '辅助函数'}
                            </Tag>
                            <Tag color="blue" style={{ cursor: 'pointer', marginInlineEnd: 0 }}>
                              {desc.displayName?.zh || funcId}
                            </Tag>
                            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                              {funcId}
                            </Typography.Text>
                          </Space>
                        </Popover>
                      ) : (
                        <Space size={8} wrap>
                          {isPrimary ? <Tag color="success">主函数</Tag> : <Tag>辅助函数</Tag>}
                          <Tag color="blue" style={{ marginInlineEnd: 0 }}>
                            {funcId}
                          </Tag>
                          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                            描述符缺失
                          </Typography.Text>
                        </Space>
                      )}
                    </List.Item>
                  );
                }}
              />
            ) : (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有绑定函数">
                <Space wrap>
                  <Button
                    type="primary"
                    icon={<PlusOutlined />}
                    onClick={() => setFunctionSelectorOpen(true)}
                  >
                    选择函数
                  </Button>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    推荐先从函数选择器明确主函数，拖拽只作为补充方式
                  </Typography.Text>
                </Space>
              </Empty>
            )}
          </div>

          {(tab.functions?.length || 0) > 0 && (
            <>
              <Divider style={{ margin: 0 }} />
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                建议：先围绕主函数生成页面骨架，再决定辅助函数是否要参与查询、提交或详情展示。
              </Typography.Text>
            </>
          )}
        </Space>
      </Card>

      <Modal
        title="函数 JSON 预览"
        open={descriptorPreviewOpen}
        footer={null}
        width={860}
        onCancel={() => {
          setDescriptorPreviewOpen(false);
          setPreviewDescriptor(null);
        }}
      >
        {previewDescriptor ? (
          <Space direction="vertical" style={{ width: '100%' }} size="small">
            <Text strong>{previewDescriptor.id}</Text>
            <Text type="secondary">
              {previewDescriptor.displayName?.zh ||
                previewDescriptor.displayName?.en ||
                previewDescriptor.id}
            </Text>
            <div style={{ border: '1px solid #f0f0f0', borderRadius: 6, overflow: 'hidden' }}>
              <CodeEditor
                value={JSON.stringify(previewDescriptor, null, 2)}
                language="json"
                height={500}
                readOnly
                theme="sublime-monokai"
                beforeMount={beforeMountJsonEditor}
                options={{
                  lineNumbers: 'on',
                  renderLineHighlight: 'line',
                  scrollBeyondLastLine: false,
                  automaticLayout: true,
                  minimap: { enabled: false },
                }}
              />
            </div>
          </Space>
        ) : null}
      </Modal>

      <FunctionSelectorModal
        open={functionSelectorOpen}
        functions={descriptors}
        selectedFunctionIds={tab.functions || []}
        onOk={(functionIds) => {
          // 过滤掉已选的函数
          const newFunctionIds = functionIds.filter((id) => !tab.functions?.includes(id));
          if (newFunctionIds.length > 0) {
            onAddFunctions?.(newFunctionIds);
            message.success(`已添加 ${newFunctionIds.length} 个函数`);
          } else {
            message.info('所选函数已存在');
          }
          setFunctionSelectorOpen(false);
        }}
        onCancel={() => setFunctionSelectorOpen(false)}
        title="批量添加函数"
        multiple={true}
      />
    </>
  );
}
