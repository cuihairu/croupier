import React from 'react';
import { Card, Select, Space, Button, Tooltip, Alert, Typography } from 'antd';
import { ThunderboltOutlined, QuestionCircleOutlined } from '@ant-design/icons';
import type { FunctionDescriptor } from '@/services/api/functions';
import { HelpTooltip, HelpModal } from '../HelpTooltip';

const LAYOUT_OPTIONS: Array<{ value: string; label: string }> = [
  { value: 'form-detail', label: '表单-详情（查询后展示）' },
  { value: 'list', label: '列表' },
  { value: 'form', label: '表单（提交操作）' },
  { value: 'detail', label: '详情（只读）' },
];

/** 预置布局模板库 */
const LAYOUT_TEMPLATES: Array<{
  id: string;
  label: string;
  description: string;
  layout: any;
}> = [
  {
    id: 'tpl_list_basic',
    label: '基础列表',
    description: '带分页的数据列表，含 ID、名称、状态、操作时间列',
    layout: {
      type: 'list',
      listFunction: '',
      columns: [
        { key: 'id', title: 'ID', width: 80, sortable: true },
        { key: 'name', title: '名称', ellipsis: true },
        { key: 'status', title: '状态', render: 'status', width: 100 },
        { key: 'updated_at', title: '更新时间', render: 'datetime', width: 180, sortable: true },
      ],
    },
  },
  {
    id: 'tpl_form_create',
    label: '创建表单',
    description: '基础创建表单，含名称、描述、状态字段',
    layout: {
      type: 'form',
      submitFunction: '',
      fields: [
        { key: 'name', label: '名称', type: 'input', required: true, placeholder: '请输入名称' },
        { key: 'description', label: '描述', type: 'textarea', placeholder: '请输入描述' },
        {
          key: 'status',
          label: '状态',
          type: 'select',
          options: [
            { label: '启用', value: 'active' },
            { label: '禁用', value: 'inactive' },
          ],
        },
      ],
    },
  },
  {
    id: 'tpl_detail_readonly',
    label: '只读详情',
    description: '只读详情页，含基本信息和扩展信息分区',
    layout: {
      type: 'detail',
      detailFunction: '',
      sections: [
        { title: '基本信息', fields: ['id', 'name', 'status'] },
        { title: '扩展信息', fields: ['created_at', 'updated_at', 'description'] },
      ],
    },
  },
  {
    id: 'tpl_form_detail_query',
    label: '查询-详情',
    description: '先查询再展示详情，适合按 ID 或条件查询单条记录',
    layout: {
      type: 'form-detail',
      queryFunction: '',
      queryFields: [
        { key: 'id', label: 'ID', type: 'input', required: true, placeholder: '请输入查询 ID' },
      ],
    },
  },
];

const TAB_SCENARIO_OPTIONS: Array<{ value: string; label: string }> = [
  { value: 'player_list_ops', label: '玩家运营列表' },
  { value: 'player_detail_profile', label: '玩家详情档案' },
];

type ScenarioId = 'player_list_ops' | 'player_detail_profile';

type ScenarioRecommendation = {
  id: ScenarioId;
  confidence: number;
  reasons: string[];
};

export interface LayoutTypeSelectorProps {
  layoutType: string;
  functions: string[];
  descriptors: FunctionDescriptor[];
  recommendedScenario: ScenarioRecommendation | null;
  showAdvancedActions?: boolean;
  undoStackLength: number;
  redoStackLength: number;
  onLayoutTypeChange: (type: string) => void;
  onApplyScenario: (scenarioId: string) => void;
  onApplyLayout?: (layout: any) => void;
  onAutoLayout: () => void;
  onHealLayout: () => void;
  onUndo: () => void;
  onRedo: () => void;
}

function getScenarioLabel(id: string): string {
  return TAB_SCENARIO_OPTIONS.find((x) => x.value === id)?.label || id;
}

export default function LayoutTypeSelector({
  layoutType,
  functions,
  descriptors,
  recommendedScenario,
  showAdvancedActions = true,
  undoStackLength,
  redoStackLength,
  onLayoutTypeChange,
  onApplyScenario,
  onApplyLayout,
  onAutoLayout,
  onHealLayout,
  onUndo,
  onRedo,
}: LayoutTypeSelectorProps) {
  const [helpModalVisible, setHelpModalVisible] = React.useState(false);
  const canAutoLayout = functions.length > 0;
  const primaryFunction = descriptors.find((d) => d.id === functions[0]);

  return (
    <>
      <Card
        title="页面骨架选择"
        size="small"
        extra={
          <Space>
            <HelpTooltip
              title="布局怎么选"
              items={[
                '先绑定一个主函数，再使用“自动生成基础布局”。',
                '基础路径优先：列表、表单、详情、查询详情。',
                '当前默认推荐的是正式布局骨架；高级布局会按正式、Beta、实验分层开放。',
              ]}
              onOpenModal={() => setHelpModalVisible(true)}
            />
            <Button icon={<QuestionCircleOutlined />} onClick={() => setHelpModalVisible(true)} />
          </Space>
        }
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Card size="small">
            <Space direction="vertical" size={10} style={{ width: '100%' }}>
              <Space wrap size={[8, 8]}>
                <Typography.Text strong>
                  {canAutoLayout ? '建议先自动生成基础布局' : '请先绑定一个主函数'}
                </Typography.Text>
                <Typography.Text type="secondary">
                  {canAutoLayout
                    ? `当前主函数是 ${
                        primaryFunction?.displayName?.zh || primaryFunction?.id || functions[0]
                      }`
                    : '没有主函数时，系统无法可靠推断结构'}
                </Typography.Text>
              </Space>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                自动生成、推荐场景和快捷模板优先覆盖 list / form / detail /
                form-detail。看板、时间线、主从、向导等高级布局不再假装是默认主路径能力。
              </Typography.Text>
            </Space>
          </Card>
          <Space wrap>
            <Button
              type="primary"
              icon={<ThunderboltOutlined />}
              onClick={onAutoLayout}
              disabled={!canAutoLayout}
              title="根据第一个函数自动推导布局"
            >
              自动生成基础布局
            </Button>
            {recommendedScenario && (
              <Tooltip title={recommendedScenario.reasons.join('；')}>
                <Button onClick={() => onApplyScenario(recommendedScenario.id)}>
                  {`应用推荐场景: ${getScenarioLabel(recommendedScenario.id)}`}
                </Button>
              </Tooltip>
            )}
          </Space>
          <Card size="small">
            <div style={{ marginBottom: 8, fontWeight: 500 }}>手动选择页面骨架</div>
            <Select value={layoutType} onChange={onLayoutTypeChange} style={{ width: '100%' }}>
              {LAYOUT_OPTIONS.map((opt) => (
                <Select.Option key={opt.value} value={opt.value}>
                  {opt.label}
                </Select.Option>
              ))}
            </Select>
          </Card>
          <Card size="small">
            <Space direction="vertical" size={12} style={{ width: '100%' }}>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                推荐场景和快捷模板目前只覆盖正式布局骨架；高级布局会在模板中心或高级能力里单独分层开放。
              </Typography.Text>
              <Space wrap>
                <Select
                  size="small"
                  placeholder="推荐场景"
                  style={{ width: 180 }}
                  onChange={onApplyScenario}
                  options={TAB_SCENARIO_OPTIONS}
                  allowClear
                />
                <Select
                  size="small"
                  placeholder="骨架模板"
                  style={{ width: 220 }}
                  onChange={(tplId) => {
                    const tpl = LAYOUT_TEMPLATES.find((item) => item.id === tplId);
                    if (!tpl) return;
                    if (onApplyLayout) {
                      onApplyLayout(tpl.layout);
                    } else {
                      onLayoutTypeChange(tpl.layout.type);
                    }
                  }}
                  options={LAYOUT_TEMPLATES.map((tpl) => ({
                    value: tpl.id,
                    label: tpl.label,
                  }))}
                  allowClear
                />
                {showAdvancedActions ? (
                  <Button size="small" icon={<ThunderboltOutlined />} onClick={onHealLayout}>
                    一键补全
                  </Button>
                ) : null}
              </Space>
              {showAdvancedActions ? (
                <Space wrap>
                  <Button
                    size="small"
                    disabled={undoStackLength === 0}
                    onClick={onUndo}
                    title="Ctrl/Cmd + Alt + Z"
                  >
                    撤销编排
                  </Button>
                  <Button
                    size="small"
                    disabled={redoStackLength === 0}
                    onClick={onRedo}
                    title="Ctrl/Cmd + Alt + Y"
                  >
                    重做编排
                  </Button>
                </Space>
              ) : (
                <Alert
                  type="info"
                  showIcon
                  message="进阶修复和编排撤销已收进高级模式"
                />
              )}
            </Space>
          </Card>
        </Space>
      </Card>
      <HelpModal visible={helpModalVisible} onClose={() => setHelpModalVisible(false)} />
    </>
  );
}
