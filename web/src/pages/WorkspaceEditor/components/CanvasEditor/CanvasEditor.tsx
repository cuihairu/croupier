/**
 * 画布编辑器
 *
 * 可视化拖拽式布局编辑器，支持画布模式和表单模式切换。
 *
 * @module pages/WorkspaceEditor/components/CanvasEditor/CanvasEditor
 */

import React, { useState, useCallback, useEffect } from 'react';
import { Alert, Button, Space, Switch, Modal, message, Tooltip, Segmented, Tag } from 'antd';
import {
  SaveOutlined,
  UndoOutlined,
  RedoOutlined,
  ClearOutlined,
  EyeOutlined,
  EditOutlined,
  FullscreenOutlined,
} from '@ant-design/icons';
import CanvasRenderer from './CanvasRenderer';
import ComponentLibrary from './ComponentLibrary';
import PropertyPanel from './PropertyPanel';
import EditorEmptyState from '../EditorEmptyState';
import { useCanvasStore, CanvasProvider } from '../../utils/canvasStoreContext';
import { findComponent, type CanvasComponent } from '../../utils/canvasStore';
import './CanvasEditor.less';

export type EditorMode = 'design' | 'preview';
export type ViewMode = 'canvas' | 'form';

export interface CanvasEditorProps {
  /** 初始 Tab 配置 */
  tabConfig?: any;
  /** 保存回调 */
  onSave?: (config: any) => void;
  /** 取消回调 */
  onCancel?: () => void;
  /** 是否显示 */
  visible?: boolean;
  /** 是否只读 */
  readOnly?: boolean;
}

function countCanvasComponents(component: CanvasComponent | null | undefined): number {
  if (!component) return 0;
  return (
    1 + (component.children || []).reduce((sum, child) => sum + countCanvasComponents(child), 0)
  );
}

/**
 * 画布编辑器内容组件（在 CanvasProvider 内部使用）
 */
function CanvasEditorContent({
  tabConfig,
  onSave,
  onCancel,
  readOnly,
  fullscreen,
  onFullscreenChange,
}: {
  tabConfig?: any;
  onSave?: (config: any) => void;
  onCancel?: () => void;
  readOnly?: boolean;
  fullscreen: boolean;
  onFullscreenChange: (value: boolean) => void;
}) {
  const {
    rootComponent,
    selectedId,
    draggingComponent,
    history,
    historyIndex,
    setRootComponent,
    fromTabConfig,
    toTabConfig,
    addComponent,
    selectComponent,
    undo,
    redo,
    clearCanvas,
  } = useCanvasStore();

  const [editorMode, setEditorMode] = useState<EditorMode>('design');
  const [viewMode, setViewMode] = useState<ViewMode>('canvas');

  // 初始化：从 Tab 配置转换
  useEffect(() => {
    if (tabConfig) {
      const canvasComponent = fromTabConfig(tabConfig);
      setRootComponent(canvasComponent);
    }
  }, [tabConfig, fromTabConfig, setRootComponent]);

  // 处理拖拽放置
  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      if (!draggingComponent) return;
      addComponent(null, draggingComponent);
    },
    [draggingComponent, addComponent],
  );

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'copy';
  }, []);

  // 保存
  const handleSave = useCallback(() => {
    const config = toTabConfig(rootComponent);
    if (config && onSave) {
      onSave(config);
      message.success('保存成功');
    } else {
      message.warning('没有可保存的配置');
    }
  }, [toTabConfig, rootComponent, onSave]);

  // 取消
  const handleCancel = useCallback(() => {
    if (onCancel) {
      onCancel();
    }
    clearCanvas();
  }, [onCancel, clearCanvas]);

  // 清空画布
  const handleClear = useCallback(() => {
    Modal.confirm({
      title: '确认清空画布？',
      content: '此操作将清空所有组件，且无法撤销',
      onOk: clearCanvas,
    });
  }, [clearCanvas]);

  // 切换视图模式
  const handleViewModeChange = useCallback((mode: ViewMode) => {
    setViewMode(mode);
  }, []);

  // 切换编辑器模式
  const handleEditorModeChange = useCallback(
    (mode: EditorMode) => {
      setEditorMode(mode);
      if (mode === 'preview') {
        selectComponent(null);
      }
    },
    [selectComponent],
  );

  // 复制组件
  const handleCopyComponent = useCallback(() => {
    if (selectedId && rootComponent) {
      message.info('当前实验版暂不支持组件复制，请先手动复用或切回标准编辑流处理。');
    }
  }, [selectedId, rootComponent]);

  // 删除组件
  const handleDeleteComponent = useCallback(() => {
    if (selectedId) {
      message.success('已删除组件');
    }
  }, [selectedId]);

  // 计算是否可以撤销/重做
  const canUndo = historyIndex > 0;
  const canRedo = historyIndex < history.length - 1;
  const selectedComponent =
    rootComponent && selectedId ? findComponent(rootComponent, selectedId) : null;
  const componentCount = countCanvasComponents(rootComponent || null);
  const modeLabel =
    editorMode === 'preview' ? '预览模式' : viewMode === 'canvas' ? '画布编辑' : '表单编辑';
  const nextStepHint =
    editorMode === 'preview'
      ? '当前处于预览模式，确认结构无误后再切回编辑保存。'
      : viewMode === 'form'
      ? '表单模式适合做整体检查；需要精细调整组件时切回画布模式。'
      : selectedComponent
      ? `当前正在编辑 ${
          selectedComponent.label || selectedComponent.type
        }，建议先完成常用属性再处理样式。`
      : '先从左侧拖入一个组件，或在画布中选中已有组件后再调整属性。';

  const content = (
    <div className={`canvas-editor ${fullscreen ? 'fullscreen' : ''}`}>
      <Alert
        type="warning"
        showIcon
        style={{ margin: '0 0 12px' }}
        message={
          <Space size={8} wrap>
            <span>实验能力：画布编辑器</span>
            <Tag color="orange">Experimental</Tag>
          </Space>
        }
        description="当前画布模式仍用于探索复杂布局编辑，不属于默认正式编排路径。请优先在标准编辑流完成页面装配，再按需使用这里做实验性调整。"
      />
      {/* 工具栏 */}
      <div className="canvas-toolbar">
        <Space className="toolbar-left">
          <Segmented
            value={viewMode}
            onChange={(v) => handleViewModeChange(v as ViewMode)}
            options={[
              { label: '画布模式', value: 'canvas', icon: <EditOutlined /> },
              { label: '表单模式', value: 'form', icon: <SaveOutlined /> },
            ]}
          />
        </Space>

        <Space className="toolbar-center">
          <Tooltip title="撤销">
            <Button
              type="text"
              size="small"
              icon={<UndoOutlined />}
              disabled={!canUndo || readOnly}
              onClick={undo}
            />
          </Tooltip>
          <Tooltip title="重做">
            <Button
              type="text"
              size="small"
              icon={<RedoOutlined />}
              disabled={!canRedo || readOnly}
              onClick={redo}
            />
          </Tooltip>
        </Space>

        <Space className="toolbar-right">
          <Switch
            checked={editorMode === 'preview'}
            onChange={(checked) => handleEditorModeChange(checked ? 'preview' : 'design')}
            checkedChildren="预览"
            unCheckedChildren="编辑"
          />
          <Tooltip title={fullscreen ? '退出全屏' : '全屏'}>
            <Button
              type="text"
              size="small"
              icon={<FullscreenOutlined />}
              onClick={() => onFullscreenChange(!fullscreen)}
            />
          </Tooltip>
          <Tooltip title="清空">
            <Button
              type="text"
              size="small"
              icon={<ClearOutlined />}
              disabled={readOnly}
              onClick={handleClear}
              danger
            />
          </Tooltip>
          <Button
            type="primary"
            size="small"
            icon={<SaveOutlined />}
            disabled={readOnly}
            onClick={handleSave}
          >
            保存
          </Button>
        </Space>
      </div>

      <div className="canvas-statusbar">
        <Space wrap size={[8, 8]}>
          <Tag color="blue">{modeLabel}</Tag>
          <Tag>{`组件 ${componentCount}`}</Tag>
          <Tag color={selectedComponent ? 'processing' : 'default'}>
            {selectedComponent
              ? `选中 ${selectedComponent.label || selectedComponent.type}`
              : '未选中组件'}
          </Tag>
          {readOnly ? <Tag color="warning">只读</Tag> : null}
          {draggingComponent ? <Tag color="success">拖拽中</Tag> : null}
        </Space>
        <Alert
          type={selectedComponent || editorMode === 'preview' ? 'info' : 'warning'}
          showIcon
          className="canvas-statusbar-alert"
          message="当前阶段建议"
          description={nextStepHint}
        />
      </div>

      {/* 主内容区 */}
      <div className="canvas-content">
        {/* 左侧组件库 */}
        {viewMode === 'canvas' && editorMode === 'design' && (
          <div className="canvas-sidebar canvas-sidebar-left">
            <ComponentLibrary />
          </div>
        )}

        {/* 中间画布区 */}
        <div
          className={`canvas-main ${editorMode === 'preview' ? 'preview-mode' : ''}`}
          onDrop={handleDrop}
          onDragOver={handleDragOver}
        >
          {rootComponent ? (
            <CanvasRenderer
              component={rootComponent}
              editable={editorMode === 'design' && !readOnly}
              preview={editorMode === 'preview'}
            />
          ) : (
            <div className="canvas-empty">
              <EditorEmptyState
                title="画布里还没有组件"
                description="先从左侧组件库拖入一个基础组件，再到右侧属性面板调整标签、数据绑定和样式。"
              />
            </div>
          )}
        </div>

        {/* 右侧属性面板 */}
        {viewMode === 'canvas' && editorMode === 'design' && (
          <div className="canvas-sidebar canvas-sidebar-right">
            <PropertyPanel onDelete={handleDeleteComponent} onCopy={handleCopyComponent} />
          </div>
        )}
      </div>
    </div>
  );

  if (fullscreen) {
    return (
      <Modal
        open={fullscreen}
        onCancel={() => onFullscreenChange(false)}
        footer={null}
        width="95vw"
        style={{ top: 20 }}
        styles={{ body: { padding: 0, height: 'calc(100vh - 200px)' } }}
        closable={false}
      >
        {content}
      </Modal>
    );
  }

  return content;
}

/**
 * 画布编辑器主组件
 */
export default function CanvasEditor({
  tabConfig,
  onSave,
  onCancel,
  visible = true,
  readOnly = false,
}: CanvasEditorProps) {
  const [fullscreen, setFullscreen] = useState(false);

  if (!visible) return null;

  return (
    <CanvasProvider>
      <CanvasEditorContent
        tabConfig={tabConfig}
        onSave={onSave}
        onCancel={onCancel}
        readOnly={readOnly}
        fullscreen={fullscreen}
        onFullscreenChange={setFullscreen}
      />
    </CanvasProvider>
  );
}

/**
 * 画布模式开关按钮
 */
export function CanvasModeButton({
  tabConfig,
  onSave,
}: {
  tabConfig?: any;
  onSave?: (config: any) => void;
}) {
  const [visible, setVisible] = useState(false);

  return (
    <>
      <Button icon={<EditOutlined />} onClick={() => setVisible(true)}>
        画布编辑
      </Button>
      <CanvasEditor
        visible={visible}
        tabConfig={tabConfig}
        onSave={(config) => {
          onSave?.(config);
          setVisible(false);
        }}
        onCancel={() => setVisible(false)}
      />
    </>
  );
}
