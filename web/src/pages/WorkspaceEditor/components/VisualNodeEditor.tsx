/**
 * 可视化节点编辑器组件
 *
 * 支持拖拽式节点编辑、节点连接、实时预览。
 *
 * @module pages/WorkspaceEditor/components/VisualNodeEditor
 */

import React, { useCallback, useRef, useState, useEffect } from 'react';
import {
  Alert,
  Button,
  Dropdown,
  Modal,
  Form,
  Input,
  Select,
  Space,
  Tag,
  Tooltip,
  message,
} from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  CopyOutlined,
  SettingOutlined,
  PlayCircleOutlined,
  ZoomInOutlined,
  ZoomOutOutlined,
  FullscreenOutlined,
  UndoOutlined,
  RedoOutlined,
  AppstoreAddOutlined,
  LinkOutlined,
  DisconnectOutlined,
} from '@ant-design/icons';
import type { MenuProps } from 'antd';

// 节点类型
export type NodeType =
  | 'function'
  | 'transform'
  | 'condition'
  | 'output'
  | 'input'
  | 'merge'
  | 'split';

// 节点位置
export interface NodePosition {
  x: number;
  y: number;
}

// 节点端口
export interface NodePort {
  id: string;
  type: 'input' | 'output';
  label: string;
  dataType: string;
  required: boolean;
}

// 节点数据
export interface VisualNode {
  id: string;
  type: NodeType;
  name: string;
  position: NodePosition;
  data: Record<string, any>;
  inputs: NodePort[];
  outputs: NodePort[];
  width: number;
  height: number;
  selected?: boolean;
  disabled?: boolean;
}

// 连接线
export interface NodeConnection {
  id: string;
  sourceNodeId: string;
  sourcePortId: string;
  targetNodeId: string;
  targetPortId: string;
  label?: string;
  animated?: boolean;
}

// 编辑器状态
export interface EditorState {
  nodes: VisualNode[];
  connections: NodeConnection[];
  selectedNodes: string[];
  selectedConnection: string | null;
  zoom: number;
  offset: { x: number; y: number };
  history: HistoryEntry[];
  historyIndex: number;
}

// 历史记录
export interface HistoryEntry {
  nodes: VisualNode[];
  connections: NodeConnection[];
  description: string;
}

// 节点模板
export interface NodeTemplate {
  type: NodeType;
  name: string;
  icon: React.ReactNode;
  color: string;
  defaultData: Record<string, any>;
  inputs: Omit<NodePort, 'id'>[];
  outputs: Omit<NodePort, 'id'>[];
}

// 节点模板定义
const NODE_TEMPLATES: NodeTemplate[] = [
  {
    type: 'function',
    name: '函数调用',
    icon: <PlayCircleOutlined />,
    color: '#1677ff',
    defaultData: { functionId: '', params: {} },
    inputs: [{ type: 'input', label: '输入', dataType: 'any', required: true }],
    outputs: [{ type: 'output', label: '输出', dataType: 'any', required: false }],
  },
  {
    type: 'transform',
    name: '数据转换',
    icon: <SettingOutlined />,
    color: '#52c41a',
    defaultData: { transform: '' },
    inputs: [{ type: 'input', label: '输入', dataType: 'any', required: true }],
    outputs: [{ type: 'output', label: '输出', dataType: 'any', required: false }],
  },
  {
    type: 'condition',
    name: '条件判断',
    icon: <LinkOutlined />,
    color: '#faad14',
    defaultData: { condition: '' },
    inputs: [{ type: 'input', label: '输入', dataType: 'any', required: true }],
    outputs: [
      { type: 'output', label: '是', dataType: 'any', required: false },
      { type: 'output', label: '否', dataType: 'any', required: false },
    ],
  },
  {
    type: 'input',
    name: '输入节点',
    icon: <AppstoreAddOutlined />,
    color: '#13c2c2',
    defaultData: { inputType: 'form' },
    inputs: [],
    outputs: [{ type: 'output', label: '输出', dataType: 'any', required: false }],
  },
  {
    type: 'output',
    name: '输出节点',
    icon: <PlayCircleOutlined />,
    color: '#722ed1',
    defaultData: { outputType: 'display' },
    inputs: [{ type: 'input', label: '输入', dataType: 'any', required: true }],
    outputs: [],
  },
  {
    type: 'merge',
    name: '合并节点',
    icon: <AppstoreAddOutlined />,
    color: '#eb2f96',
    defaultData: { mergeStrategy: 'all' },
    inputs: [
      { type: 'input', label: '输入1', dataType: 'any', required: false },
      { type: 'input', label: '输入2', dataType: 'any', required: false },
    ],
    outputs: [{ type: 'output', label: '输出', dataType: 'any', required: false }],
  },
];

// 生成唯一ID
const generateId = () => `node_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;

// 生成端口ID
const generatePortId = (nodeId: string, type: string, index: number) =>
  `${nodeId}_${type}_${index}`;

export interface VisualNodeEditorProps {
  initialState?: Partial<EditorState>;
  onChange?: (state: EditorState) => void;
  onNodeSelect?: (node: VisualNode | null) => void;
  onConnectionSelect?: (connection: NodeConnection | null) => void;
  availableFunctions?: any[];
  readOnly?: boolean;
}

export default function VisualNodeEditor({
  initialState,
  onChange,
  onNodeSelect,
  onConnectionSelect,
  availableFunctions = [],
  readOnly = false,
}: VisualNodeEditorProps) {
  const canvasRef = useRef<HTMLDivElement>(null);
  const svgRef = useRef<SVGSVGElement>(null);

  const [state, setState] = useState<EditorState>({
    nodes: initialState?.nodes || [],
    connections: initialState?.connections || [],
    selectedNodes: [],
    selectedConnection: null,
    zoom: initialState?.zoom || 1,
    offset: initialState?.offset || { x: 0, y: 0 },
    history: [],
    historyIndex: -1,
  });

  const [dragging, setDragging] = useState<{
    type: 'node' | 'canvas' | 'connection';
    nodeId?: string;
    startX: number;
    startY: number;
    portId?: string;
    portType?: 'input' | 'output';
  } | null>(null);

  const [connectingFrom, setConnectingFrom] = useState<{
    nodeId: string;
    portId: string;
    portType: 'output';
  } | null>(null);

  const [tempConnection, setTempConnection] = useState<{
    startX: number;
    startY: number;
    endX: number;
    endY: number;
  } | null>(null);

  const [showNodeConfig, setShowNodeConfig] = useState(false);
  const [configuringNode, setConfiguringNode] = useState<VisualNode | null>(null);
  const [configForm] = Form.useForm();

  // 保存历史
  const saveHistory = useCallback(
    (description: string) => {
      if (readOnly) return;
      setState((prev) => {
        const newHistory = prev.history.slice(0, prev.historyIndex + 1);
        newHistory.push({
          nodes: JSON.parse(JSON.stringify(prev.nodes)),
          connections: JSON.parse(JSON.stringify(prev.connections)),
          description,
        });
        return {
          ...prev,
          history: newHistory.slice(-50), // 保留最近50条历史
          historyIndex: newHistory.length - 1,
        };
      });
    },
    [readOnly],
  );

  // 通知外部变化
  useEffect(() => {
    onChange?.(state);
  }, [state, onChange]);

  // 添加节点
  const addNode = useCallback(
    (type: NodeType, position: NodePosition) => {
      if (readOnly) return;

      const template = NODE_TEMPLATES.find((t) => t.type === type);
      if (!template) return;

      const nodeId = generateId();
      const node: VisualNode = {
        id: nodeId,
        type,
        name: template.name,
        position,
        data: { ...template.defaultData },
        inputs: template.inputs.map((input, index) => ({
          ...input,
          id: generatePortId(nodeId, 'input', index),
        })),
        outputs: template.outputs.map((output, index) => ({
          ...output,
          id: generatePortId(nodeId, 'output', index),
        })),
        width: 180,
        height: 100 + Math.max(template.inputs.length, template.outputs.length) * 24,
      };

      saveHistory('添加节点');
      setState((prev) => ({
        ...prev,
        nodes: [...prev.nodes, node],
        selectedNodes: [nodeId],
      }));
    },
    [readOnly, saveHistory],
  );

  // 删除选中节点
  const deleteSelectedNodes = useCallback(() => {
    if (readOnly || state.selectedNodes.length === 0) return;

    saveHistory('删除节点');
    setState((prev) => ({
      ...prev,
      nodes: prev.nodes.filter((n) => !prev.selectedNodes.includes(n.id)),
      connections: prev.connections.filter(
        (c) =>
          !prev.selectedNodes.includes(c.sourceNodeId) &&
          !prev.selectedNodes.includes(c.targetNodeId),
      ),
      selectedNodes: [],
    }));
    onNodeSelect?.(null);
  }, [readOnly, state.selectedNodes, saveHistory, onNodeSelect]);

  // 复制节点
  const copyNodes = useCallback(() => {
    if (readOnly || state.selectedNodes.length === 0) return;

    const newNodes: VisualNode[] = [];
    const newConnections: NodeConnection[] = [];
    const nodeIdMap: Record<string, string> = {};

    // 复制节点
    state.nodes.forEach((node) => {
      if (state.selectedNodes.includes(node.id)) {
        const newId = generateId();
        nodeIdMap[node.id] = newId;

        const newNode: VisualNode = {
          ...JSON.parse(JSON.stringify(node)),
          id: newId,
          position: {
            x: node.position.x + 20,
            y: node.position.y + 20,
          },
          inputs: node.inputs.map((input, index) => ({
            ...input,
            id: generatePortId(newId, 'input', index),
          })),
          outputs: node.outputs.map((output, index) => ({
            ...output,
            id: generatePortId(newId, 'output', index),
          })),
        };
        newNodes.push(newNode);
      }
    });

    // 复制连接
    state.connections.forEach((conn) => {
      if (nodeIdMap[conn.sourceNodeId] && nodeIdMap[conn.targetNodeId]) {
        newConnections.push({
          ...conn,
          id: generateId(),
          sourceNodeId: nodeIdMap[conn.sourceNodeId],
          targetNodeId: nodeIdMap[conn.targetNodeId],
        });
      }
    });

    saveHistory('复制节点');
    setState((prev) => ({
      ...prev,
      nodes: [...prev.nodes, ...newNodes],
      connections: [...prev.connections, ...newConnections],
      selectedNodes: newNodes.map((n) => n.id),
    }));
  }, [readOnly, state.nodes, state.connections, state.selectedNodes, saveHistory]);

  // 撤销
  const undo = useCallback(() => {
    if (readOnly || state.historyIndex <= 0) return;

    const prevEntry = state.history[state.historyIndex - 1];
    setState((prev) => ({
      ...prev,
      nodes: JSON.parse(JSON.stringify(prevEntry.nodes)),
      connections: JSON.parse(JSON.stringify(prevEntry.connections)),
      historyIndex: prev.historyIndex - 1,
    }));
  }, [readOnly, state.historyIndex, state.history]);

  // 重做
  const redo = useCallback(() => {
    if (readOnly || state.historyIndex >= state.history.length - 1) return;

    const nextEntry = state.history[state.historyIndex + 1];
    setState((prev) => ({
      ...prev,
      nodes: JSON.parse(JSON.stringify(nextEntry.nodes)),
      connections: JSON.parse(JSON.stringify(nextEntry.connections)),
      historyIndex: prev.historyIndex + 1,
    }));
  }, [readOnly, state.historyIndex, state.history]);

  // 缩放
  const handleZoom = useCallback((delta: number) => {
    setState((prev) => ({
      ...prev,
      zoom: Math.max(0.25, Math.min(2, prev.zoom + delta)),
    }));
  }, []);

  // 适应画布
  const fitCanvas = useCallback(() => {
    if (state.nodes.length === 0) return;

    const bounds = state.nodes.reduce(
      (acc, node) => ({
        minX: Math.min(acc.minX, node.position.x),
        minY: Math.min(acc.minY, node.position.y),
        maxX: Math.max(acc.maxX, node.position.x + node.width),
        maxY: Math.max(acc.maxY, node.position.y + node.height),
      }),
      { minX: Infinity, minY: Infinity, maxX: -Infinity, maxY: -Infinity },
    );

    const canvas = canvasRef.current;
    if (!canvas) return;

    const padding = 50;
    const contentWidth = bounds.maxX - bounds.minX + padding * 2;
    const contentHeight = bounds.maxY - bounds.minY + padding * 2;

    const scaleX = canvas.clientWidth / contentWidth;
    const scaleY = canvas.clientHeight / contentHeight;
    const newZoom = Math.min(1, Math.min(scaleX, scaleY));

    setState((prev) => ({
      ...prev,
      zoom: newZoom,
      offset: {
        x: -bounds.minX * newZoom + padding,
        y: -bounds.minY * newZoom + padding,
      },
    }));
  }, [state.nodes]);

  // 处理鼠标事件
  const handleMouseDown = useCallback(
    (e: React.MouseEvent, nodeId?: string, portId?: string, portType?: 'input' | 'output') => {
      if (readOnly) return;

      // 如果点击的是端口，开始连接
      if (portId && portType) {
        if (portType === 'output') {
          setConnectingFrom({ nodeId: nodeId!, portId, portType });
          setTempConnection({
            startX: e.clientX,
            startY: e.clientY,
            endX: e.clientX,
            endY: e.clientY,
          });
        }
        return;
      }

      // 如果点击的是节点
      if (nodeId) {
        const node = state.nodes.find((n) => n.id === nodeId);
        if (!node) return;

        // 更新选中状态
        if (e.shiftKey) {
          // 多选
          setState((prev) => ({
            ...prev,
            selectedNodes: prev.selectedNodes.includes(nodeId)
              ? prev.selectedNodes.filter((id) => id !== nodeId)
              : [...prev.selectedNodes, nodeId],
            selectedConnection: null,
          }));
        } else {
          // 单选
          setState((prev) => ({
            ...prev,
            selectedNodes: [nodeId],
            selectedConnection: null,
          }));
        }

        onNodeSelect?.(node);
        onConnectionSelect?.(null);

        // 开始拖拽
        setDragging({
          type: 'node',
          nodeId,
          startX: e.clientX,
          startY: e.clientY,
        });
      } else {
        // 点击画布，取消选择
        setState((prev) => ({
          ...prev,
          selectedNodes: [],
          selectedConnection: null,
        }));
        onNodeSelect?.(null);
        onConnectionSelect?.(null);

        // 开始拖拽画布
        setDragging({
          type: 'canvas',
          startX: e.clientX,
          startY: e.clientY,
        });
      }
    },
    [readOnly, state.nodes, onNodeSelect, onConnectionSelect],
  );

  const handleMouseMove = useCallback(
    (e: React.MouseEvent) => {
      if (connectingFrom && tempConnection) {
        setTempConnection({
          ...tempConnection,
          endX: e.clientX,
          endY: e.clientY,
        });
        return;
      }

      if (!dragging) return;

      const dx = e.clientX - dragging.startX;
      const dy = e.clientY - dragging.startY;

      if (dragging.type === 'node' && dragging.nodeId) {
        // 移动节点
        setState((prev) => ({
          ...prev,
          nodes: prev.nodes.map((node) => {
            if (prev.selectedNodes.includes(node.id)) {
              return {
                ...node,
                position: {
                  x: node.position.x + dx / prev.zoom,
                  y: node.position.y + dy / prev.zoom,
                },
              };
            }
            return node;
          }),
        }));
        setDragging({ ...dragging, startX: e.clientX, startY: e.clientY });
      } else if (dragging.type === 'canvas') {
        // 移动画布
        setState((prev) => ({
          ...prev,
          offset: {
            x: prev.offset.x + dx,
            y: prev.offset.y + dy,
          },
        }));
        setDragging({ ...dragging, startX: e.clientX, startY: e.clientY });
      }
    },
    [dragging, connectingFrom, tempConnection],
  );

  const handleMouseUp = useCallback(
    (e: React.MouseEvent) => {
      // 完成连接
      if (connectingFrom && tempConnection) {
        const target = (e.target as HTMLElement).closest('[data-port-id]');
        if (target) {
          const targetNodeId = target.getAttribute('data-node-id');
          const targetPortId = target.getAttribute('data-port-id');
          const targetPortType = target.getAttribute('data-port-type') as 'input' | 'output';

          if (
            targetNodeId &&
            targetPortId &&
            targetPortType === 'input' &&
            targetNodeId !== connectingFrom.nodeId
          ) {
            // 检查是否已存在连接
            const exists = state.connections.some(
              (c) =>
                c.sourceNodeId === connectingFrom.nodeId &&
                c.sourcePortId === connectingFrom.portId &&
                c.targetNodeId === targetNodeId &&
                c.targetPortId === targetPortId,
            );

            if (!exists) {
              saveHistory('创建连接');
              setState((prev) => ({
                ...prev,
                connections: [
                  ...prev.connections,
                  {
                    id: generateId(),
                    sourceNodeId: connectingFrom.nodeId,
                    sourcePortId: connectingFrom.portId,
                    targetNodeId,
                    targetPortId,
                    animated: true,
                  },
                ],
              }));
            }
          }
        }
      }

      // 结束拖拽时保存历史
      if (dragging?.type === 'node') {
        saveHistory('移动节点');
      }

      setDragging(null);
      setConnectingFrom(null);
      setTempConnection(null);
    },
    [dragging, connectingFrom, tempConnection, state.connections, saveHistory],
  );

  // 处理键盘事件
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (readOnly) return;

      if (e.key === 'Delete' || e.key === 'Backspace') {
        deleteSelectedNodes();
      } else if (e.key === 'c' && (e.ctrlKey || e.metaKey)) {
        copyNodes();
      } else if (e.key === 'z' && (e.ctrlKey || e.metaKey)) {
        if (e.shiftKey) {
          redo();
        } else {
          undo();
        }
      } else if (e.key === 'y' && (e.ctrlKey || e.metaKey)) {
        redo();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [readOnly, deleteSelectedNodes, copyNodes, undo, redo]);

  // 计算连接线路径
  const getConnectionPath = (connection: NodeConnection) => {
    const sourceNode = state.nodes.find((n) => n.id === connection.sourceNodeId);
    const targetNode = state.nodes.find((n) => n.id === connection.targetNodeId);

    if (!sourceNode || !targetNode) return '';

    const sourcePortIndex = sourceNode.outputs.findIndex((p) => p.id === connection.sourcePortId);
    const targetPortIndex = targetNode.inputs.findIndex((p) => p.id === connection.targetPortId);

    const startX = (sourceNode.position.x + sourceNode.width) * state.zoom + state.offset.x;
    const startY =
      (sourceNode.position.y + 40 + sourcePortIndex * 24) * state.zoom + state.offset.y;
    const endX = targetNode.position.x * state.zoom + state.offset.x;
    const endY = (targetNode.position.y + 40 + targetPortIndex * 24) * state.zoom + state.offset.y;

    const controlOffset = Math.abs(endX - startX) * 0.5;

    return `M ${startX} ${startY} C ${startX + controlOffset} ${startY}, ${
      endX - controlOffset
    } ${endY}, ${endX} ${endY}`;
  };

  // 添加节点菜单
  const addNodeMenuItems: MenuProps['items'] = NODE_TEMPLATES.map((template) => ({
    key: template.type,
    label: (
      <Space>
        {template.icon}
        <span>{template.name}</span>
      </Space>
    ),
    onClick: () => {
      addNode(template.type, { x: 100, y: 100 });
    },
  }));

  // 打开节点配置
  const openNodeConfig = (node: VisualNode) => {
    setConfiguringNode(node);
    configForm.setFieldsValue(node.data);
    setShowNodeConfig(true);
  };

  // 保存节点配置
  const handleSaveNodeConfig = async () => {
    if (!configuringNode) return;

    const values = await configForm.validateFields();
    saveHistory('更新节点配置');
    setState((prev) => ({
      ...prev,
      nodes: prev.nodes.map((n) => (n.id === configuringNode.id ? { ...n, data: values } : n)),
    }));
    setShowNodeConfig(false);
    setConfiguringNode(null);
  };

  // 获取节点颜色
  const getNodeColor = (type: NodeType) => {
    const template = NODE_TEMPLATES.find((t) => t.type === type);
    return template?.color || '#d9d9d9';
  };

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Alert
        type="warning"
        showIcon
        style={{ margin: '8px 12px 0' }}
        message="内部实验能力：节点编辑器"
        description="该能力不属于当前正式交付范围，不提供生产可执行的条件分支、数据转换与流程执行能力。"
      />

      {/* 工具栏 */}
      <div
        style={{
          padding: '8px 12px',
          borderBottom: '1px solid #f0f0f0',
          display: 'flex',
          alignItems: 'center',
          gap: 8,
        }}
      >
        <Dropdown menu={{ items: addNodeMenuItems }} trigger={['click']}>
          <Button icon={<PlusOutlined />}>添加节点</Button>
        </Dropdown>

        <Button
          icon={<DeleteOutlined />}
          danger
          disabled={state.selectedNodes.length === 0}
          onClick={deleteSelectedNodes}
        >
          删除
        </Button>

        <Button
          icon={<CopyOutlined />}
          disabled={state.selectedNodes.length === 0}
          onClick={copyNodes}
        >
          复制
        </Button>

        <div style={{ flex: 1 }} />

        <Space.Compact>
          <Tooltip title="撤销 (Ctrl+Z)">
            <Button icon={<UndoOutlined />} disabled={state.historyIndex <= 0} onClick={undo} />
          </Tooltip>
          <Tooltip title="重做 (Ctrl+Y)">
            <Button
              icon={<RedoOutlined />}
              disabled={state.historyIndex >= state.history.length - 1}
              onClick={redo}
            />
          </Tooltip>
        </Space.Compact>

        <Space.Compact>
          <Tooltip title="缩小">
            <Button icon={<ZoomOutOutlined />} onClick={() => handleZoom(-0.1)} />
          </Tooltip>
          <Button style={{ width: 60 }}>{Math.round(state.zoom * 100)}%</Button>
          <Tooltip title="放大">
            <Button icon={<ZoomInOutlined />} onClick={() => handleZoom(0.1)} />
          </Tooltip>
        </Space.Compact>

        <Tooltip title="适应画布">
          <Button icon={<FullscreenOutlined />} onClick={fitCanvas} />
        </Tooltip>
      </div>

      {/* 画布 */}
      <div
        ref={canvasRef}
        style={{
          flex: 1,
          overflow: 'hidden',
          position: 'relative',
          background: '#fafafa',
          cursor: dragging?.type === 'canvas' ? 'grabbing' : 'grab',
        }}
        onMouseDown={(e) => handleMouseDown(e)}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onMouseLeave={handleMouseUp}
      >
        {/* SVG 连接线层 */}
        <svg
          ref={svgRef}
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            width: '100%',
            height: '100%',
            pointerEvents: 'none',
          }}
        >
          {state.connections.map((conn) => (
            <g key={conn.id}>
              <path
                d={getConnectionPath(conn)}
                fill="none"
                stroke={state.selectedConnection === conn.id ? '#1677ff' : '#999'}
                strokeWidth={2}
                strokeDasharray={conn.animated ? '5,5' : undefined}
                style={{ pointerEvents: 'stroke' }}
              />
            </g>
          ))}
          {/* 临时连接线 */}
          {tempConnection && (
            <line
              x1={tempConnection.startX}
              y1={tempConnection.startY}
              x2={tempConnection.endX}
              y2={tempConnection.endY}
              stroke="#1677ff"
              strokeWidth={2}
              strokeDasharray="5,5"
            />
          )}
        </svg>

        {/* 节点层 */}
        <div
          style={{
            position: 'absolute',
            transform: `translate(${state.offset.x}px, ${state.offset.y}px) scale(${state.zoom})`,
            transformOrigin: '0 0',
          }}
        >
          {state.nodes.map((node) => (
            <div
              key={node.id}
              style={{
                position: 'absolute',
                left: node.position.x,
                top: node.position.y,
                width: node.width,
                minHeight: node.height,
                background: '#fff',
                border: `2px solid ${
                  state.selectedNodes.includes(node.id) ? '#1677ff' : getNodeColor(node.type)
                }`,
                borderRadius: 8,
                boxShadow: state.selectedNodes.includes(node.id)
                  ? '0 0 0 2px rgba(22, 119, 255, 0.2)'
                  : '0 2px 8px rgba(0,0,0,0.1)',
                cursor: 'move',
                userSelect: 'none',
              }}
              onMouseDown={(e) => handleMouseDown(e, node.id)}
              onDoubleClick={() => openNodeConfig(node)}
            >
              {/* 节点标题 */}
              <div
                style={{
                  padding: '8px 12px',
                  borderBottom: '1px solid #f0f0f0',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                  background: getNodeColor(node.type),
                  color: '#fff',
                  borderRadius: '6px 6px 0 0',
                }}
              >
                {NODE_TEMPLATES.find((t) => t.type === node.type)?.icon}
                <span style={{ flex: 1, fontWeight: 500 }}>{node.name}</span>
                <Button
                  type="text"
                  size="small"
                  icon={<SettingOutlined />}
                  style={{ color: '#fff' }}
                  onClick={(e) => {
                    e.stopPropagation();
                    openNodeConfig(node);
                  }}
                />
              </div>

              {/* 节点内容 */}
              <div style={{ padding: 8, position: 'relative' }}>
                {/* 输入端口 */}
                <div style={{ position: 'absolute', left: -8, top: 0 }}>
                  {node.inputs.map((port, index) => (
                    <div
                      key={port.id}
                      data-port-id={port.id}
                      data-node-id={node.id}
                      data-port-type="input"
                      style={{
                        width: 16,
                        height: 16,
                        borderRadius: '50%',
                        background: '#fff',
                        border: '2px solid #52c41a',
                        marginBottom: 8,
                        cursor: 'crosshair',
                      }}
                      title={port.label}
                    />
                  ))}
                </div>

                {/* 输出端口 */}
                <div style={{ position: 'absolute', right: -8, top: 0 }}>
                  {node.outputs.map((port, index) => (
                    <div
                      key={port.id}
                      data-port-id={port.id}
                      data-node-id={node.id}
                      data-port-type="output"
                      style={{
                        width: 16,
                        height: 16,
                        borderRadius: '50%',
                        background: '#fff',
                        border: '2px solid #1677ff',
                        marginBottom: 8,
                        cursor: 'crosshair',
                      }}
                      title={port.label}
                    />
                  ))}
                </div>

                {/* 节点数据预览 */}
                <div style={{ fontSize: 12, color: '#666' }}>
                  {node.type === 'function' && node.data.functionId && (
                    <Tag color="blue">{node.data.functionId}</Tag>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* 节点配置弹窗 */}
      <Modal
        title={`配置节点: ${configuringNode?.name}`}
        open={showNodeConfig}
        onOk={handleSaveNodeConfig}
        onCancel={() => {
          setShowNodeConfig(false);
          setConfiguringNode(null);
        }}
        width={600}
      >
        {configuringNode && (
          <Form form={configForm} layout="vertical">
            {configuringNode.type === 'function' && (
              <>
                <Form.Item name="functionId" label="函数" rules={[{ required: true }]}>
                  <Select
                    placeholder="选择函数"
                    showSearch
                    optionFilterProp="label"
                    options={availableFunctions.map((f) => ({
                      value: f.id,
                      label: f.displayName?.zh || f.id,
                    }))}
                  />
                </Form.Item>
              </>
            )}

            {configuringNode.type === 'transform' && (
              <Form.Item name="transform" label="转换表达式" rules={[{ required: true }]}>
                <Input.TextArea rows={4} placeholder="输入数据转换表达式" />
              </Form.Item>
            )}

            {configuringNode.type === 'condition' && (
              <Form.Item name="condition" label="条件表达式" rules={[{ required: true }]}>
                <Input placeholder="输入条件表达式" />
              </Form.Item>
            )}

            {configuringNode.type === 'input' && (
              <Form.Item name="inputType" label="输入类型" rules={[{ required: true }]}>
                <Select
                  options={[
                    { value: 'form', label: '表单输入' },
                    { value: 'parameter', label: '参数输入' },
                    { value: 'trigger', label: '触发器' },
                  ]}
                />
              </Form.Item>
            )}

            {configuringNode.type === 'output' && (
              <Form.Item name="outputType" label="输出类型" rules={[{ required: true }]}>
                <Select
                  options={[
                    { value: 'display', label: '显示结果' },
                    { value: 'notification', label: '通知' },
                    { value: 'callback', label: '回调' },
                  ]}
                />
              </Form.Item>
            )}
          </Form>
        )}
      </Modal>
    </div>
  );
}
