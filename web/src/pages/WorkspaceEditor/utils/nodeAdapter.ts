/**
 * 数据适配层
 *
 * 负责在 WorkspaceConfig 和 VisualNode 格式之间进行转换。
 *
 * @module pages/WorkspaceEditor/utils/nodeAdapter
 */

import type { WorkspaceConfig, TabConfig, SectionConfig } from '@/types/workspace';
import type {
  VisualNode,
  NodeConnection,
  NodePort,
  NodeType,
} from '../components/VisualNodeEditor';

export const WORKSPACE_NODE_ADAPTER_EXPERIMENTAL = true;

// 生成唯一ID
const generateId = () => `node_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;

// 生成端口ID
const generatePortId = (nodeId: string, type: string, index: number) =>
  `${nodeId}_${type}_${index}`;

/**
 * 将 WorkspaceConfig 转换为 VisualNode 列表
 *
 * @param config - Workspace 配置
 * @returns VisualNode 列表和连接列表
 */
export function configToNodes(config: WorkspaceConfig | null): {
  nodes: VisualNode[];
  connections: NodeConnection[];
} {
  if (!config) {
    return { nodes: [], connections: [] };
  }

  const nodes: VisualNode[] = [];
  const connections: NodeConnection[] = [];

  // 创建输入节点
  const inputNode = createInputNode(config.objectKey);
  nodes.push(inputNode);

  // 根据布局类型处理
  if (config.layout.type === 'tabs' && config.layout.tabs) {
    config.layout.tabs.forEach((tab, index) => {
      const tabNodes = processTab(tab, index, inputNode);
      nodes.push(...tabNodes.nodes);
      connections.push(...tabNodes.connections);
    });
  } else if (config.layout.type === 'sections' && config.layout.sections) {
    config.layout.sections.forEach((section, index) => {
      const sectionNodes = processSection(section, index, inputNode);
      nodes.push(...sectionNodes.nodes);
      connections.push(...sectionNodes.connections);
    });
  }

  return { nodes, connections };
}

/**
 * 创建输入节点
 */
function createInputNode(objectKey: string): VisualNode {
  const nodeId = generateId();
  return {
    id: nodeId,
    type: 'input',
    name: '数据输入',
    position: { x: 50, y: 100 },
    data: { objectKey, inputType: 'form' },
    inputs: [],
    outputs: [
      {
        id: generatePortId(nodeId, 'output', 0),
        type: 'output',
        label: '输出',
        dataType: 'object',
        required: false,
      },
    ],
    width: 180,
    height: 100,
  };
}

/**
 * 处理 Tab 配置
 */
function processTab(
  tab: TabConfig,
  index: number,
  inputNode: VisualNode,
): { nodes: VisualNode[]; connections: NodeConnection[] } {
  const nodes: VisualNode[] = [];
  const connections: NodeConnection[] = [];

  const yOffset = index * 200;

  // 为每个函数创建节点
  tab.functions.forEach((funcId, funcIndex) => {
    const nodeId = generateId();
    const node: VisualNode = {
      id: nodeId,
      type: 'function',
      name: `函数: ${funcId}`,
      position: { x: 300 + funcIndex * 220, y: yOffset + 50 },
      data: { functionId: funcId, params: {} },
      inputs: [
        {
          id: generatePortId(nodeId, 'input', 0),
          type: 'input',
          label: '输入',
          dataType: 'any',
          required: true,
        },
      ],
      outputs: [
        {
          id: generatePortId(nodeId, 'output', 0),
          type: 'output',
          label: '输出',
          dataType: 'any',
          required: false,
        },
      ],
      width: 180,
      height: 100,
    };
    nodes.push(node);

    // 连接输入节点到函数节点
    connections.push({
      id: generateId(),
      sourceNodeId: inputNode.id,
      sourcePortId: inputNode.outputs[0].id,
      targetNodeId: nodeId,
      targetPortId: node.inputs[0].id,
      label: '',
    });
  });

  // 创建布局输出节点
  const layoutNode = createLayoutNode(tab.layout, tab.title, 600, yOffset + 100);
  nodes.push(layoutNode);

  // 连接最后一个函数到布局节点
  if (nodes.length > 1) {
    const lastFuncNode = nodes[nodes.length - 2];
    connections.push({
      id: generateId(),
      sourceNodeId: lastFuncNode.id,
      sourcePortId: lastFuncNode.outputs[0].id,
      targetNodeId: layoutNode.id,
      targetPortId: layoutNode.inputs[0].id,
    });
  }

  return { nodes, connections };
}

/**
 * 处理 Section 配置
 */
function processSection(
  section: SectionConfig,
  index: number,
  inputNode: VisualNode,
): { nodes: VisualNode[]; connections: NodeConnection[] } {
  const nodes: VisualNode[] = [];
  const connections: NodeConnection[] = [];

  const yOffset = index * 200;

  // 为每个函数创建节点
  section.functions.forEach((funcId, funcIndex) => {
    const nodeId = generateId();
    const node: VisualNode = {
      id: nodeId,
      type: 'function',
      name: `函数: ${funcId}`,
      position: { x: 300 + funcIndex * 220, y: yOffset + 50 },
      data: { functionId: funcId, params: {} },
      inputs: [
        {
          id: generatePortId(nodeId, 'input', 0),
          type: 'input',
          label: '输入',
          dataType: 'any',
          required: true,
        },
      ],
      outputs: [
        {
          id: generatePortId(nodeId, 'output', 0),
          type: 'output',
          label: '输出',
          dataType: 'any',
          required: false,
        },
      ],
      width: 180,
      height: 100,
    };
    nodes.push(node);

    // 连接输入节点到函数节点
    connections.push({
      id: generateId(),
      sourceNodeId: inputNode.id,
      sourcePortId: inputNode.outputs[0].id,
      targetNodeId: nodeId,
      targetPortId: node.inputs[0].id,
      label: '',
    });
  });

  // 创建布局输出节点
  const layoutNode = createLayoutNode(section.layout, section.title, 600, yOffset + 100);
  nodes.push(layoutNode);

  // 连接最后一个函数到布局节点
  if (nodes.length > 1) {
    const lastFuncNode = nodes[nodes.length - 2];
    connections.push({
      id: generateId(),
      sourceNodeId: lastFuncNode.id,
      sourcePortId: lastFuncNode.outputs[0].id,
      targetNodeId: layoutNode.id,
      targetPortId: layoutNode.inputs[0].id,
    });
  }

  return { nodes, connections };
}

/**
 * 创建布局节点
 */
function createLayoutNode(layout: any, title: string, x: number, y: number): VisualNode {
  const nodeId = generateId();
  const layoutType = layout?.type || 'list';

  return {
    id: nodeId,
    type: 'output',
    name: `布局: ${title}`,
    position: { x, y },
    data: { layoutType, layout },
    inputs: [
      {
        id: generatePortId(nodeId, 'input', 0),
        type: 'input',
        label: '输入',
        dataType: 'any',
        required: true,
      },
    ],
    outputs: [],
    width: 180,
    height: 100,
  };
}

/**
 * 将 VisualNode 列表转换回 WorkspaceConfig
 *
 * @param nodes - VisualNode 列表
 * @param connections - 连接列表
 * @param baseConfig - 基础配置
 * @returns WorkspaceConfig
 */
export function nodesToConfig(
  nodes: VisualNode[],
  connections: NodeConnection[],
  baseConfig: WorkspaceConfig,
): WorkspaceConfig {
  // 找出所有函数节点
  const functionNodes = nodes.filter((n) => n.type === 'function');

  // 找出所有输出/布局节点
  const outputNodes = nodes.filter((n) => n.type === 'output');

  // 构建 Tab 配置
  const tabs: TabConfig[] = outputNodes.map((outputNode, index) => {
    // 找到连接到此输出节点的函数节点
    const connectedFunctionIds = connections
      .filter((c) => c.targetNodeId === outputNode.id)
      .map((c) => {
        const sourceNode = nodes.find((n) => n.id === c.sourceNodeId);
        return sourceNode?.data?.functionId;
      })
      .filter(Boolean);

    return {
      key: `tab_${index}`,
      title: outputNode.name.replace('布局: ', ''),
      functions: connectedFunctionIds as string[],
      layout: outputNode.data?.layout || { type: 'list', listFunction: '', columns: [] },
    };
  });

  // 如果没有输出节点，使用函数节点创建默认 Tab
  if (tabs.length === 0 && functionNodes.length > 0) {
    tabs.push({
      key: 'tab_main',
      title: '主标签',
      functions: functionNodes.map((n) => n.data?.functionId).filter(Boolean) as string[],
      layout: { type: 'list', listFunction: '', columns: [] },
    });
  }

  return {
    ...baseConfig,
    layout: {
      type: 'tabs',
      tabs,
    },
  };
}

/**
 * 将函数描述符转换为节点模板
 *
 * @param desc - 函数描述符
 * @returns 节点模板
 */
export function descriptorToNodeTemplate(desc: any): {
  type: NodeType;
  name: string;
  data: Record<string, any>;
} {
  return {
    type: 'function',
    name: desc.displayName?.zh || desc.id,
    data: {
      functionId: desc.id,
      params: {},
    },
  };
}

/**
 * 从节点创建函数调用配置
 *
 * @param node - 函数节点
 * @returns 函数调用配置
 */
export function createFunctionConfig(node: VisualNode): {
  functionId: string;
  params: Record<string, any>;
} | null {
  if (node.type !== 'function' || !node.data?.functionId) {
    return null;
  }

  return {
    functionId: node.data.functionId,
    params: node.data.params || {},
  };
}

/**
 * 验证节点图是否有效
 *
 * @param nodes - 节点列表
 * @param connections - 连接列表
 * @returns 验证结果
 */
export function validateNodeGraph(
  nodes: VisualNode[],
  connections: NodeConnection[],
): { valid: boolean; errors: string[] } {
  const errors: string[] = [];

  // 检查是否有输入节点
  const inputNodes = nodes.filter((n) => n.type === 'input');
  if (inputNodes.length === 0) {
    errors.push('缺少输入节点');
  }

  // 检查是否有输出节点
  const outputNodes = nodes.filter((n) => n.type === 'output');
  if (outputNodes.length === 0) {
    errors.push('缺少输出节点');
  }

  // 检查函数节点是否有连接
  const functionNodes = nodes.filter((n) => n.type === 'function');
  functionNodes.forEach((node) => {
    const hasConnection = connections.some(
      (c) => c.sourceNodeId === node.id || c.targetNodeId === node.id,
    );
    if (!hasConnection) {
      errors.push(`函数节点 "${node.name}" 未连接`);
    }
  });

  // 检查是否有循环连接
  const visited = new Set<string>();
  const hasCycle = (nodeId: string, path: Set<string>): boolean => {
    if (path.has(nodeId)) return true;
    if (visited.has(nodeId)) return false;

    path.add(nodeId);
    visited.add(nodeId);

    const outgoingConnections = connections.filter((c) => c.sourceNodeId === nodeId);
    for (const conn of outgoingConnections) {
      if (hasCycle(conn.targetNodeId, path)) {
        return true;
      }
    }

    path.delete(nodeId);
    return false;
  };

  for (const node of nodes) {
    if (hasCycle(node.id, new Set())) {
      errors.push('存在循环连接');
      break;
    }
  }

  return {
    valid: errors.length === 0,
    errors,
  };
}
