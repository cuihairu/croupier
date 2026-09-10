import React from 'react';
import { Descriptions, Form, Input, Modal, Select, Space, Tag } from 'antd';
import type { FormInstance } from 'antd';
import type { ResolveSemanticConflictRequest, SemanticConflictInfo } from '@/types/dashboard';
import { conflictSources, displaySemanticValue, sourceColors, sourceLabels } from './shared';

/** 解决语义冲突弹窗：展示候选值并选择采用来源（Form 实例与预填由主页持有）。 */
const ResolveConflictModal: React.FC<{
  open: boolean;
  form: FormInstance<ResolveSemanticConflictRequest>;
  conflict: SemanticConflictInfo | null;
  onOk: () => void;
  onCancel: () => void;
}> = ({ open, form, conflict, onOk, onCancel }) => (
  <Modal
    title="解决语义冲突"
    open={open}
    onOk={onOk}
    onCancel={onCancel}
    okText="确认选择"
    cancelText="取消"
  >
    {conflict && (
      <Form form={form} layout="vertical">
        <Descriptions column={1} bordered size="small" style={{ marginBottom: 16 }}>
          <Descriptions.Item label="字段">{conflict.field}</Descriptions.Item>
          <Descriptions.Item label="候选值">
            <Space wrap>
              {conflictSources(conflict).map((source) => (
                <Tag key={source} color={sourceColors[source]}>
                  {sourceLabels[source]}: {displaySemanticValue(conflict.values[source])}
                </Tag>
              ))}
            </Space>
          </Descriptions.Item>
        </Descriptions>
        <Form.Item
          label="采用来源"
          name="chosenSource"
          rules={[{ required: true, message: '请选择采用的语义来源' }]}
        >
          <Select
            options={conflictSources(conflict).map((source) => ({
              value: source,
              label: sourceLabels[source],
            }))}
          />
        </Form.Item>
        <Form.Item label="决议原因" name="reason">
          <Input.TextArea placeholder="说明为什么采用该来源" />
        </Form.Item>
      </Form>
    )}
  </Modal>
);

export default ResolveConflictModal;
