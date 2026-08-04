/**
 * PageEditor - 语义化页面编辑器
 *
 * 根据页面类型自动选择对应的编辑器面板：
 * - ResourcePage: 资源页面编辑器
 * - OperationPage: 操作页面编辑器
 * - TaskPage: 任务页面编辑器
 * - ReportPage: 报表页面编辑器
 */

import React from 'react';
import { Empty } from 'antd';
import type { PageSpec } from '@/types/dashboard';
import ResourcePageEditor from './ResourcePageEditor';
import OperationPageEditor from './OperationPageEditor';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface PageEditorProps {
  /** 当前 PageSpec */
  value: PageSpec;
  /** 值变化回调 */
  onChange: (value: PageSpec) => void;
  /** 是否只读 */
  readonly?: boolean;
}

// ---------------------------------------------------------------------------
// PageEditor Component
// ---------------------------------------------------------------------------

export default function PageEditor({
  value,
  onChange,
  readonly = false,
}: PageEditorProps) {
  // 根据页面类型选择编辑器
  switch (value.type) {
    case 'resource':
      return value.resource ? (
        <ResourcePageEditor
          value={value.resource}
          onChange={(resource) => onChange({ ...value, resource })}
          readonly={readonly}
        />
      ) : (
        <Empty description="无资源页面配置" />
      );

    case 'operation':
      return value.operation ? (
        <OperationPageEditor
          value={value.operation}
          onChange={(operation) => onChange({ ...value, operation })}
          readonly={readonly}
        />
      ) : (
        <Empty description="无操作页面配置" />
      );

    case 'task':
      // TODO: 实现 TaskPageEditor
      return <Empty description="任务页面编辑器开发中" />;

    case 'report':
      // TODO: 实现 ReportPageEditor
      return <Empty description="报表页面编辑器开发中" />;

    default:
      return <Empty description="未知页面类型" />;
  }
}

// 导出子编辑器
export { default as ResourcePageEditor } from './ResourcePageEditor';
export { default as OperationPageEditor } from './OperationPageEditor';
