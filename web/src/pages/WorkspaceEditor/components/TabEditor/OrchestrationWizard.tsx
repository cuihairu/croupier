import React from 'react';
import { Modal, Space, Segmented, Select, Button, Tooltip, Tag } from 'antd';
import type { FunctionDescriptor } from '@/services/api/functions';

type OrchestratorMode = 'list' | 'form-detail';
type OrchestratorRole = 'list' | 'detail' | 'submit' | 'query' | 'data';
type OrchestratorBindings = Record<OrchestratorRole, string>;
type OrchestratorApplyMode = 'overwrite' | 'merge';

export interface OrchestrationWizardProps {
  open: boolean;
  mode: OrchestratorMode;
  applyMode: OrchestratorApplyMode;
  bindings: OrchestratorBindings | null;
  functions: string[];
  descriptors: FunctionDescriptor[];
  orchestrationPlan: { layout: any; assignments: string[] } | null;
  activeRoles: OrchestratorRole[];
  invalidRoles: OrchestratorRole[];
  defaultBindings: OrchestratorBindings;
  displayedAssignments: string[];
  riskTips: string[];
  diffPreview: string[];
  onClose: () => void;
  onApply: () => void;
  onModeChange: (mode: OrchestratorMode) => void;
  onApplyModeChange: (mode: OrchestratorApplyMode) => void;
  onBindingsChange: (bindings: OrchestratorBindings) => void;
  onResetBindings: () => void;
}

function getFunctionShortName(descriptors: FunctionDescriptor[], functionId: string): string {
  const d = descriptors.find((x) => x.id === functionId);
  return d?.displayName?.zh || d?.displayName?.en || functionId || '-';
}

function getFunctionMetaLine(descriptors: FunctionDescriptor[], functionId: string): string {
  const d = descriptors.find((x) => x.id === functionId);
  if (!d) return functionId || '-';
  const op = d.operation || 'unknown';
  const tags = (d.tags || []).slice(0, 3).join(', ') || '-';
  return `${d.id} | operation: ${op} | tags: ${tags}`;
}

function getFunctionOptionLabel(
  descriptor: FunctionDescriptor | undefined,
  functionId: string,
): string {
  if (!descriptor) return functionId;
  const name = descriptor.displayName?.zh || descriptor.displayName?.en || descriptor.id;
  const op = descriptor.operation || 'unknown';
  return `${name} (${op})`;
}

export default function OrchestrationWizard({
  open,
  mode,
  applyMode,
  bindings,
  functions,
  descriptors,
  orchestrationPlan,
  activeRoles,
  invalidRoles,
  defaultBindings,
  displayedAssignments,
  riskTips,
  diffPreview,
  onClose,
  onApply,
  onModeChange,
  onApplyModeChange,
  onBindingsChange,
  onResetBindings,
}: OrchestrationWizardProps) {
  const functionOptions = React.useMemo(
    () =>
      functions.map((fid) => {
        const d = descriptors.find((x) => x.id === fid);
        return { value: fid, label: getFunctionOptionLabel(d, fid) };
      }),
    [functions, descriptors],
  );

  return (
    <Modal
      title="多函数编排向导"
      open={open}
      onCancel={onClose}
      onOk={onApply}
      okText="应用方案"
      okButtonProps={{ disabled: !orchestrationPlan || invalidRoles.length > 0 }}
      width={720}
    >
      <Space direction="vertical" style={{ width: '100%' }} size="middle">
        <Segmented
          block
          value={applyMode}
          onChange={(v) => onApplyModeChange(v as OrchestratorApplyMode)}
          options={[
            { label: '覆盖当前布局', value: 'overwrite' },
            { label: '仅补空字段', value: 'merge' },
          ]}
        />
        <Segmented
          block
          value={mode}
          onChange={(v) => onModeChange(v as OrchestratorMode)}
          options={[
            { label: '标准列表', value: 'list' },
            { label: '查询详情', value: 'form-detail' },
          ]}
        />
        {orchestrationPlan ? (
          <>
            {invalidRoles.length > 0 && (
              <div style={{ fontSize: 12, color: '#cf1322' }}>
                检测到失效角色绑定: {invalidRoles.join(', ')}，请重新选择或重置为推荐
              </div>
            )}
            <div style={{ fontSize: 12, color: '#999' }}>角色绑定（可手动调整）</div>
            <Space direction="vertical" style={{ width: '100%' }} size="small">
              {activeRoles.map((role) => (
                <div key={role} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <div style={{ width: 64 }}>{role}</div>
                  <Select
                    value={bindings?.[role]}
                    style={{ flex: 1 }}
                    options={functionOptions}
                    onChange={(v) =>
                      onBindingsChange({
                        ...(bindings || defaultBindings),
                        [role]: v,
                      })
                    }
                  />
                  <Tooltip title={getFunctionMetaLine(descriptors, bindings?.[role] || '')}>
                    <Tag color="blue">
                      {getFunctionShortName(descriptors, bindings?.[role] || '')}
                    </Tag>
                  </Tooltip>
                </div>
              ))}
            </Space>
            <Button size="small" onClick={onResetBindings} title="将角色绑定恢复为系统推荐">
              重置为推荐
            </Button>
            <div style={{ fontSize: 12, color: '#999' }}>函数角色分配</div>
            <div style={{ fontSize: 13 }}>
              {displayedAssignments.map((line) => (
                <div key={line}>{line}</div>
              ))}
            </div>
            <div style={{ fontSize: 12, color: '#999' }}>
              方案摘要: 类型 {orchestrationPlan.layout.type}，
              {applyMode === 'merge' ? '仅补全当前空字段' : '将覆盖当前 Tab 布局配置'}
            </div>
            {riskTips.length > 0 && (
              <>
                <div style={{ fontSize: 12, color: '#cf1322' }}>风险提示</div>
                <div style={{ fontSize: 13, color: '#cf1322' }}>
                  {riskTips.map((line) => (
                    <div key={line}>{line}</div>
                  ))}
                </div>
              </>
            )}
            <div style={{ fontSize: 12, color: '#999' }}>变更预览</div>
            <div style={{ fontSize: 13 }}>
              {(diffPreview.length > 0 ? diffPreview : ['无结构变化']).map((line) => (
                <div key={line}>{line}</div>
              ))}
            </div>
          </>
        ) : (
          <div style={{ color: '#999' }}>请先在当前 Tab 添加至少一个函数</div>
        )}
      </Space>
    </Modal>
  );
}
