/**
 * 配置预览组件
 *
 * 实时预览页面配置效果。
 *
 * @module pages/WorkspaceEditor/components/ConfigPreview
 */

import React, { useState } from 'react';
import { Alert, Card, Tabs, Button, Modal, Segmented, Space, Tag, Typography, Row, Col } from 'antd';
import { EyeOutlined, CodeOutlined } from '@ant-design/icons';
import type { WorkspaceConfig } from '@/types/workspace';
import WorkspaceRenderer from '@/components/WorkspaceRenderer';
import { CodeEditor } from '@/components/MonacoDynamic';
import { DASHBOARD_PAGE_TOKENS } from '@/components';
import EditorEmptyState from './EditorEmptyState';

export interface ConfigPreviewProps {
  /** 页面配置 */
  config: WorkspaceConfig | null;
  currentTabKey?: string;
  currentTabTitle?: string;
  appliedTemplateSummary?: {
    templateName: string;
    showcase: boolean;
    checklist: string[];
    headline: string;
    score: number;
    tabCount: number;
    tabTitles: string[];
    scenario?: string;
    requiredFunctions: string[];
    riskNotes: string[];
  } | null;
  onBackToEditor?: () => void;
}

/**
 * 配置预览组件
 */
export default function ConfigPreview({
  config,
  currentTabKey,
  currentTabTitle,
  appliedTemplateSummary,
  onBackToEditor,
}: ConfigPreviewProps) {
  const [viewMode, setViewMode] = useState<'preview' | 'code'>('preview');
  const [showFullPreview, setShowFullPreview] = useState(false);
  const [previewDataMode, setPreviewDataMode] = useState<'mock' | 'live'>('mock');
  const currentTab =
    config?.layout?.type === 'tabs'
      ? config.layout.tabs?.find((tab) => tab.key === currentTabKey) ||
        config.layout.tabs?.find((tab) => tab.defaultActive) ||
        config.layout.tabs?.[0]
      : undefined;
  const currentLayout = currentTab?.layout as any;
  const primaryFunctionId = currentTab?.functions?.[0];
  const currentBindingId =
    currentLayout?.listFunction ||
    currentLayout?.submitFunction ||
    currentLayout?.detailFunction ||
    currentLayout?.queryFunction ||
    currentLayout?.dataFunction ||
    '';

  if (!config) {
    return (
      <Card title="预览" style={{ height: 'calc(100vh - 200px)' }}>
        <EditorEmptyState
          title="还没有可预览的配置"
          description="先在中间区域完成至少一个 Tab 的基础配置，再回到这里检查界面结果。"
        />
      </Card>
    );
  }

  return (
    <>
      <Card
        title="预览"
        style={{ height: '100%', display: 'flex', flexDirection: 'column' }}
        styles={{ body: { padding: 0, flex: 1, overflow: 'hidden' } }}
        extra={
          <Space size={8}>
            {currentTabTitle ? (
              <Tag color="processing">{`当前聚焦: ${currentTabTitle}`}</Tag>
            ) : null}
            {appliedTemplateSummary?.templateName ? (
              <Tag color={appliedTemplateSummary.showcase ? 'magenta' : 'blue'}>
                {`${appliedTemplateSummary.showcase ? '官方样板' : '模板'}: ${
                  appliedTemplateSummary.templateName
                }`}
              </Tag>
            ) : null}
            {viewMode === 'preview' && (
              <Segmented
                size="small"
                value={previewDataMode}
                onChange={(v) => setPreviewDataMode(v as 'mock' | 'live')}
                options={[
                  { label: '示例', value: 'mock' },
                  { label: '真实', value: 'live' },
                ]}
              />
            )}
            <Tabs
              activeKey={viewMode}
              onChange={(key) => setViewMode(key as any)}
              size="small"
              items={[
                {
                  key: 'preview',
                  label: (
                    <>
                      <EyeOutlined /> 预览
                    </>
                  ),
                },
                {
                  key: 'code',
                  label: (
                    <>
                      <CodeOutlined /> 代码
                    </>
                  ),
                },
              ]}
            />
          </Space>
        }
      >
        {viewMode === 'preview' ? (
          <div style={{ height: '100%', overflow: 'auto', padding: 16 }}>
            {appliedTemplateSummary ? (
              <Card
                size="small"
                style={{ marginBottom: 12 }}
                styles={{
                  body: {
                    padding: DASHBOARD_PAGE_TOKENS.cardPadding,
                    background: appliedTemplateSummary.showcase
                      ? 'linear-gradient(135deg, rgba(240,101,149,0.1) 0%, rgba(22,119,255,0.05) 100%)'
                      : DASHBOARD_PAGE_TOKENS.infoGradient,
                  },
                }}
              >
                <Space direction="vertical" size={16} style={{ width: '100%' }}>
                  <Space wrap size={[8, 8]}>
                    <Tag color={appliedTemplateSummary.showcase ? 'magenta' : 'blue'}>
                      {appliedTemplateSummary.showcase ? '官方样板已接入' : '模板已接入'}
                    </Tag>
                    <Tag color="processing">{`质量评分 ${appliedTemplateSummary.score}/100`}</Tag>
                    <Tag color="green">{`页面骨架 ${appliedTemplateSummary.tabCount}`}</Tag>
                    {appliedTemplateSummary.tabTitles.map((item) => (
                      <Tag key={`summary-tab-${item}`}>{item}</Tag>
                    ))}
                  </Space>
                  <Space direction="vertical" size={4} style={{ width: '100%' }}>
                    <Typography.Title level={5} style={{ margin: 0 }}>
                      {appliedTemplateSummary.headline}
                    </Typography.Title>
                    <Typography.Text type="secondary">
                      {appliedTemplateSummary.scenario ||
                        '当前预览已经带入模板骨架，接下来重点核对字段、函数绑定和首屏动作是否能独立成立。'}
                    </Typography.Text>
                  </Space>
                  <Row gutter={[12, 12]}>
                    <Col xs={24} md={8}>
                      <Card
                        size="small"
                        style={{ height: '100%', background: 'rgba(255,255,255,0.82)' }}
                        styles={{ body: { padding: DASHBOARD_PAGE_TOKENS.compactCardPadding } }}
                      >
                        <Space direction="vertical" size={4}>
                          <Typography.Text type="secondary">当前最该先看</Typography.Text>
                          <Typography.Text strong>
                            {appliedTemplateSummary.showcase ? '首屏是否像成品页' : '主路径是否已经连通'}
                          </Typography.Text>
                          <Typography.Text type="secondary">
                            重点检查默认页面、主操作和首屏信息层级，不要只看组件有没有渲染出来。
                          </Typography.Text>
                        </Space>
                      </Card>
                    </Col>
                    <Col xs={24} md={8}>
                      <Card
                        size="small"
                        style={{ height: '100%', background: 'rgba(255,255,255,0.82)' }}
                        styles={{ body: { padding: DASHBOARD_PAGE_TOKENS.compactCardPadding } }}
                      >
                        <Space direction="vertical" size={4}>
                          <Typography.Text type="secondary">需优先绑定</Typography.Text>
                          <Typography.Text strong>
                            {appliedTemplateSummary.requiredFunctions[0] || '当前未声明额外函数'}
                          </Typography.Text>
                          <Typography.Text type="secondary">
                            {appliedTemplateSummary.requiredFunctions.length > 1
                              ? `还需核对 ${appliedTemplateSummary.requiredFunctions.slice(1).join(' / ')}`
                              : '当前样板已具备基础骨架，可直接补真实函数。'}
                          </Typography.Text>
                        </Space>
                      </Card>
                    </Col>
                    <Col xs={24} md={8}>
                      <Card
                        size="small"
                        style={{ height: '100%', background: 'rgba(255,255,255,0.82)' }}
                        styles={{ body: { padding: DASHBOARD_PAGE_TOKENS.compactCardPadding } }}
                      >
                        <Space direction="vertical" size={4}>
                          <Typography.Text type="secondary">发布前重点</Typography.Text>
                          <Typography.Text strong>
                            {appliedTemplateSummary.riskNotes[0] || '继续做发布前检查'}
                          </Typography.Text>
                          <Typography.Text type="secondary">
                            {appliedTemplateSummary.checklist[0] || '先走一轮预览，再进入发布前检查面板。'}
                          </Typography.Text>
                        </Space>
                      </Card>
                    </Col>
                  </Row>
                </Space>
              </Card>
            ) : null}
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 12 }}
              message={currentTabTitle ? `当前预览聚焦 ${currentTabTitle}` : '当前预览整个页面草稿'}
              description={
                <Space wrap>
                  {currentTabKey ? <Tag>{`Tab Key: ${currentTabKey}`}</Tag> : null}
                  {appliedTemplateSummary?.showcase ? <Tag color="magenta">官方演示样板运行预览</Tag> : null}
                  {primaryFunctionId ? (
                    <Tag color="blue">{`主函数: ${primaryFunctionId}`}</Tag>
                  ) : null}
                  {currentBindingId ? (
                    <Tag color="success">{`核心函数: ${currentBindingId}`}</Tag>
                  ) : null}
                  <Typography.Text type="secondary">
                    先检查当前页的层级、关键字段和操作链路；重点确认当前预览是否真的围绕这个主函数和核心函数在工作。
                  </Typography.Text>
                  {appliedTemplateSummary?.checklist?.map((item) => (
                    <Tag key={`preview-check-${item}`} color="processing">
                      {item}
                    </Tag>
                  ))}
                  {onBackToEditor ? (
                    <Button size="small" onClick={onBackToEditor}>
                      回到当前 Tab 编辑
                    </Button>
                  ) : null}
                </Space>
              }
            />
            <div
              style={{
                border: '1px solid #f0f0f0',
                padding: 16,
                minHeight: 300,
                backgroundColor: '#fff',
                borderRadius: 4,
              }}
            >
              <WorkspaceRenderer
                config={config}
                context={previewDataMode === 'mock' ? { templatePreview: true } : undefined}
              />
            </div>
            <div style={{ marginTop: 16, textAlign: 'center' }}>
              <Button type="link" onClick={() => setShowFullPreview(true)}>
                全屏预览
              </Button>
              <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
                适合做一次完整走查
              </Typography.Text>
            </div>
          </div>
        ) : (
          <div
            style={{
              height: '100%',
              border: '1px solid #f0f0f0',
              borderRadius: 4,
              overflow: 'hidden',
            }}
          >
            <CodeEditor
              value={JSON.stringify(config, null, 2)}
              language="json"
              height="100%"
              readOnly
              options={{
                lineNumbers: 'on',
                renderLineHighlight: 'line',
                scrollBeyondLastLine: false,
                automaticLayout: true,
                minimap: { enabled: false },
              }}
            />
          </div>
        )}
      </Card>

      {/* 全屏预览模态框 */}
      <Modal
        title="页面全屏预览"
        open={showFullPreview}
        onCancel={() => setShowFullPreview(false)}
        footer={null}
        width="90%"
        style={{ top: 20 }}
        styles={{ body: { height: 'calc(100vh - 150px)', overflow: 'auto' } }}
      >
        <WorkspaceRenderer
          config={config}
          context={previewDataMode === 'mock' ? { templatePreview: true } : undefined}
        />
      </Modal>
    </>
  );
}
