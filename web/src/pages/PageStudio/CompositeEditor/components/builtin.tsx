import React from 'react';
import { Input } from 'antd';
import { registerComponent } from '../registry';
import type { CompositeView } from '../types';
import { fnTable } from './FnTable';
import { fnForm } from './FnForm';
import { fnFields } from './FnFields';
import { staticFormDef } from './StaticForm';
import { button } from './Button';
import { modal } from './Modal';
import { container } from './Container';
import { text } from './Text';

/** 内置组件注册入口（显式引导，无 import 副作用——registry 引导时序见 index.tsx）。
 * 各组件定义拆分在 ./FnTable ./FnForm ./FnFields ./StaticForm ./Button ./Modal
 * ./Container ./Text 八文件；共享 schema 片段在 ./shared。 */
export function registerBuiltinComponents(): void {
  registerComponent(fnTable);
  registerComponent(fnForm);
  registerComponent(fnFields);
  registerComponent(staticFormDef);
  registerComponent(button);
  registerComponent(modal);
  registerComponent(container);
  registerComponent(text);
}

/** 视图形态 → 组件类型（组件面板函数条目标注用）。 */
export function viewTypeToComponent(view: CompositeView): 'fnTable' | 'fnFields' | 'fnForm' {
  if (view === 'table') return 'fnTable';
  if (view === 'fields') return 'fnFields';
  return 'fnForm';
}

/** 文本输入占位（text Preview 内联使用，避免 antd Input 受控告警）。 */
export function TextPreviewInput({ value }: { value: string }) {
  return <Input size="small" value={value} readOnly />;
}
