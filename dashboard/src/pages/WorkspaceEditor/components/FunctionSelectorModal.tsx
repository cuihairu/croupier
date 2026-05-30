/**
 * 函数选择 Modal 组件
 *
 * 用于快速选择和添加函数，支持树形分组和多选。
 *
 * @module pages/WorkspaceEditor/components/FunctionSelectorModal
 */

import React, { useMemo, useState } from 'react';
import { Modal, Input, Tree, Checkbox, Tag, Space, Typography, Button, Empty, Alert } from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { TreeProps } from 'antd';

const { Text } = Typography;

export interface FunctionSelectorModalProps {
  open: boolean;
  functions: FunctionDescriptor[];
  selectedFunctionIds: string[];
  onOk: (functionIds: string[]) => void;
  onCancel: () => void;
  title?: string;
  multiple?: boolean;
}

/** 操作类型颜色映射 */
const OPERATION_TAG_COLOR: Record<string, string> = {
  list: 'blue',
  query: 'green',
  create: 'cyan',
  update: 'orange',
  delete: 'red',
  action: 'purple',
  read: 'green',
  custom: 'magenta',
};

/** 按实体和操作类型分组 */
function groupFunctions(functions: FunctionDescriptor[]) {
  const groups: Record<
    string,
    {
      label: string;
      functions: FunctionDescriptor[];
    }
  > = {};

  functions.forEach((func) => {
    // 按 entity 分组
    const groupKey = func.entity || func.category || '未分类';
    if (!groups[groupKey]) {
      groups[groupKey] = { label: groupKey, functions: [] };
    }
    groups[groupKey].functions.push(func);
  });

  return Object.entries(groups)
    .map(([key, value]) => ({ key, ...value }))
    .sort((a, b) => a.key.localeCompare(b.key));
}

function getFunctionPriority(func: FunctionDescriptor): number {
  const op = String(func.operation || '').toLowerCase();
  if (op.includes('list')) return 1;
  if (op.includes('query') || op.includes('read') || op.includes('detail')) return 2;
  if (op.includes('create') || op.includes('update') || op.includes('submit')) return 3;
  if (op.includes('delete')) return 5;
  if (op.includes('action')) return 4;
  return 6;
}

export default function FunctionSelectorModal({
  open,
  functions,
  selectedFunctionIds,
  onOk,
  onCancel,
  title = '选择函数',
  multiple = true,
}: FunctionSelectorModalProps) {
  const [searchText, setSearchText] = useState('');
  const [selectedKeys, setSelectedKeys] = useState<string[]>(selectedFunctionIds);
  const [expandedKeys, setExpandedKeys] = useState<string[]>([]);

  // 当打开 modal 时，同步已选函数
  React.useEffect(() => {
    if (open) {
      setSelectedKeys(selectedFunctionIds);
      // 默认展开所有分组
      const grouped = groupFunctions(functions);
      setExpandedKeys(grouped.map((g) => g.key));
    }
  }, [open, selectedFunctionIds, functions]);

  // 分组后的函数
  const groupedFunctions = useMemo(() => {
    return groupFunctions(functions);
  }, [functions]);

  // 过滤后的函数
  const filteredGroups = useMemo(() => {
    if (!searchText) return groupedFunctions;

    const text = searchText.toLowerCase();
    return groupedFunctions
      .map((group) => ({
        ...group,
        functions: group.functions.filter((func) => {
          return (
            func.id.toLowerCase().includes(text) ||
            func.displayName?.zh?.toLowerCase().includes(text) ||
            func.operation?.toLowerCase().includes(text) ||
            func.entity?.toLowerCase().includes(text) ||
            func.category?.toLowerCase().includes(text)
          );
        }),
      }))
      .map((group) => ({
        ...group,
        functions: [...group.functions].sort((a, b) => {
          const priorityDiff = getFunctionPriority(a) - getFunctionPriority(b);
          if (priorityDiff !== 0) return priorityDiff;
          const nameA = a.displayName?.zh || a.displayName?.en || a.id;
          const nameB = b.displayName?.zh || b.displayName?.en || b.id;
          return String(nameA).localeCompare(String(nameB));
        }),
      }))
      .filter((group) => group.functions.length > 0);
  }, [groupedFunctions, searchText]);

  // 树形数据结构
  const treeData = useMemo(() => {
    return filteredGroups.map((group) => ({
      title: (
        <Space size={8}>
          <span>{group.label}</span>
          <Tag>{group.functions.length}</Tag>
        </Space>
      ),
      key: group.key,
      selectable: false,
      children: group.functions.map((func) => {
        const isSelected = selectedKeys.includes(func.id);
        const op = func.operation || 'custom';
        return {
          title: (
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                padding: '4px 0',
              }}
            >
              <Space size={8}>
                {multiple && (
                  <Checkbox
                    checked={isSelected}
                    onChange={(e) => {
                      e.stopPropagation();
                      toggleFunctionSelection(func.id);
                    }}
                  />
                )}
                <div>
                  <div>{func.displayName?.zh || func.id}</div>
                  <Text type="secondary" style={{ fontSize: 11 }}>
                    {func.id}
                  </Text>
                </div>
              </Space>
              <Tag color={OPERATION_TAG_COLOR[op] || 'default'} style={{ fontSize: 11 }}>
                {op}
              </Tag>
            </div>
          ),
          key: func.id,
          selectable: !multiple,
          isLeaf: true,
          func,
        };
      }),
    }));
  }, [filteredGroups, selectedKeys, multiple]);

  const toggleFunctionSelection = (funcId: string) => {
    if (multiple) {
      setSelectedKeys((prev) => {
        if (prev.includes(funcId)) {
          return prev.filter((id) => id !== funcId);
        } else {
          return [...prev, funcId];
        }
      });
    } else {
      setSelectedKeys([funcId]);
    }
  };

  const handleOk = () => {
    onOk(selectedKeys);
  };

  const handleSelect: TreeProps['onSelect'] = (selectedKeys, info) => {
    if (!multiple && info.node?.isLeaf) {
      setSelectedKeys(selectedKeys as string[]);
    }
  };

  const handleTreeClick: TreeProps['onClick'] = (info) => {
    // 点击节点时处理
    if (info.node?.isLeaf && !multiple) {
      const funcId = info.node.key as string;
      setSelectedKeys([funcId]);
    }
  };

  return (
    <Modal
      title={
        <Space size={8}>
          <span>{title}</span>
          {selectedKeys.length > 0 && <Tag color="blue">已选 {selectedKeys.length}</Tag>}
        </Space>
      }
      open={open}
      onOk={handleOk}
      onCancel={onCancel}
      width={640}
      okText={multiple ? `添加 ${selectedKeys.length} 个函数` : '确定'}
      okButtonProps={{ disabled: selectedKeys.length === 0 }}
    >
      <Space direction="vertical" style={{ width: '100%' }} size={12}>
        <Alert
          type="info"
          showIcon
          message={multiple ? '先选主函数，再补辅助函数' : '选择一个核心函数'}
          description={
            multiple
              ? '注意顺序：你最先加入页面的函数会成为主函数。后续自动生成页面骨架、自动补字段和自动补列都会优先围绕它展开。'
              : '这里选中的函数会作为当前配置的核心函数。'
          }
        />
        <Input
          placeholder="搜索函数名称、ID、操作类型..."
          prefix={<SearchOutlined />}
          value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          allowClear
        />

        {!searchText && multiple && selectedKeys.length > 0 && (
          <div style={{ padding: '8px 12px', background: '#f6ffed', borderRadius: 4 }}>
            <Space direction="vertical" size={8} style={{ width: '100%' }}>
              <Text type="success" strong>
                已选 {selectedKeys.length} 个函数
              </Text>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                当前第一个函数会被当成主函数。请先确认它就是这个页面最核心的查询、列表或提交入口。
              </Typography.Text>
              <Space size={8} wrap>
                {selectedKeys.map((id, index) => {
                  const func = functions.find((f) => f.id === id);
                  return (
                    <Tag key={id} closable color="blue" onClose={() => toggleFunctionSelection(id)}>
                      {index === 0
                        ? `主: ${func?.displayName?.zh || id}`
                        : func?.displayName?.zh || id}
                    </Tag>
                  );
                })}
              </Space>
            </Space>
          </div>
        )}

        {filteredGroups.length === 0 ? (
          <Empty description="没有找到匹配的函数" />
        ) : (
          <div
            style={{
              maxHeight: 400,
              overflow: 'auto',
              border: '1px solid #f0f0f0',
              borderRadius: 4,
              padding: '8px 12px',
            }}
          >
            <Tree
              treeData={treeData}
              expandedKeys={expandedKeys}
              onExpand={setExpandedKeys}
              selectedKeys={multiple ? [] : selectedKeys}
              onSelect={handleSelect}
              onClick={handleTreeClick}
              showLine={{ showLeafIcon: false }}
              blockNode
            />
          </div>
        )}
      </Space>
    </Modal>
  );
}

/** 快速函数选择器（单选） */
export interface QuickFunctionPickerProps {
  functions: FunctionDescriptor[];
  value?: string;
  onChange?: (functionId: string) => void;
  placeholder?: string;
  style?: React.CSSProperties;
  allowClear?: boolean;
}

export function QuickFunctionPicker({
  functions,
  value,
  onChange,
  placeholder = '选择函数',
  style,
  allowClear = true,
}: QuickFunctionPickerProps) {
  const [open, setOpen] = useState(false);

  const selectedFunction = functions.find((f) => f.id === value);

  return (
    <>
      <Input
        value={selectedFunction?.displayName?.zh || selectedFunction?.id || value}
        placeholder={placeholder}
        readOnly
        onClick={() => setOpen(true)}
        style={{ cursor: 'pointer', ...style }}
        allowClear={allowClear}
        onClear={() => onChange?.('')}
      />

      <FunctionSelectorModal
        open={open}
        functions={functions}
        selectedFunctionIds={value ? [value] : []}
        onOk={(ids) => {
          if (ids.length > 0) {
            onChange?.(ids[0]);
          }
          setOpen(false);
        }}
        onCancel={() => setOpen(false)}
        title="选择函数"
        multiple={false}
      />
    </>
  );
}
