import React from 'react';
import type { JSONSchema } from '@/types/dashboard';
import type { ComponentType, PageNode } from './model';
import type { FunctionDescriptor } from '@/services/api/functions';

/** 组件定义（amis plugin × Appsmith widget 杂交）：
 * scaffold=拖入即实例化的默认 props；propSchema=属性面板声明（rjsf 渲染）；
 * Preview=画布内真实组件预览。 */
export interface ComponentDef {
  type: ComponentType;
  name: string;
  icon: React.ReactNode;
  /** 组件面板分组：函数组件由契约生成；基础组件直接拖入。 */
  category: 'function' | 'basic';
  /** 允许的父节点类型（空=任意非容器位置）。 */
  allowedParents?: ComponentType[];
  /** 允许的子节点类型（仅容器类配置）。 */
  allowedChildren?: ComponentType[];
  /** 属性面板 JSON Schema（字段约定：title/span/事件用 format:'action'）。 */
  propSchema: (ctx: PropSchemaCtx) => JSONSchema;
  /** 由函数契约生成默认 props（函数组件必填）。 */
  scaffold: (fn?: FunctionDescriptor) => Record<string, unknown>;
  /** 画布预览组件。 */
  Preview: React.FC<PreviewProps>;
}

export interface PropSchemaCtx {
  /** 全部节点（供事件目标下拉过滤）。 */
  nodes: PageNode[];
  /** 函数契约缓存。 */
  fnById: Map<string, FunctionDescriptor>;
}

export interface PreviewProps {
  node: PageNode;
  fn?: FunctionDescriptor;
  /** 预览态允许交互（真实执行）。 */
  interactive?: boolean;
}

const registry = new Map<ComponentType, ComponentDef>();

export function registerComponent(def: ComponentDef): void {
  if (registry.has(def.type)) {
    throw new Error(`component already registered: ${def.type}`);
  }
  registry.set(def.type, def);
}

export function getComponent(type: ComponentType): ComponentDef | undefined {
  return registry.get(type);
}

export function allComponents(): ComponentDef[] {
  return Array.from(registry.values());
}

/** 子节点约束校验：父类型是否接受该子类型。 */
export function acceptsChild(parent: PageNode, childType: ComponentType): boolean {
  const def = getComponent(parent.type);
  if (!def?.allowedChildren) return false;
  return def.allowedChildren.includes(childType);
}

/** 是否可放置到根级（非容器 children）。 */
export function allowedAtRoot(type: ComponentType): boolean {
  return type !== 'fnForm' || true; // V1：全部组件可入根级，modal 收纳区由画布处理
}

export function resetRegistryForTest(): void {
  registry.clear();
}
