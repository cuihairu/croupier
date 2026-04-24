/**
 * 配置模板管理组件
 *
 * 支持保存、加载、导入、导出工作空间配置模板。
 *
 * @module pages/WorkspaceEditor/components/TemplateManager
 */

import React, { useState, useEffect } from 'react';
import {
  Modal,
  Card,
  List,
  Button,
  Input,
  Tag,
  Space,
  Typography,
  Empty,
  Spin,
  message,
  Popconfirm,
  Dropdown,
  Form,
  Select,
  Divider,
  Segmented,
  Row,
  Col,
} from 'antd';
import {
  SaveOutlined,
  DownloadOutlined,
  UploadOutlined,
  DeleteOutlined,
  CopyOutlined,
  EyeOutlined,
  MoreOutlined,
  FolderOutlined,
  FileOutlined,
  StarOutlined,
  StarFilled,
  SearchOutlined,
  FilterOutlined,
} from '@ant-design/icons';
import type { MenuProps } from 'antd';
import type { WorkspaceConfig } from '@/types/workspace';
import { DASHBOARD_PAGE_TOKENS } from '@/components';
import WorkspaceRenderer from '@/components/WorkspaceRenderer';
import { CodeEditor } from '@/components/MonacoDynamic';

const { Text, Title, Paragraph } = Typography;
const { Search } = Input;
const STABLE_TAB_LAYOUT_TYPES = new Set(['form-detail', 'list', 'form', 'detail']);
const BETA_TAB_LAYOUT_TYPES = new Set([
  'kanban',
  'timeline',
  'split',
  'wizard',
  'dashboard',
  'grid',
]);
const RENDERABLE_TAB_LAYOUT_TYPES = new Set([
  ...Array.from(STABLE_TAB_LAYOUT_TYPES),
  ...Array.from(BETA_TAB_LAYOUT_TYPES),
  'custom',
]);

// 模板类型
export type TemplateType = 'workspace' | 'layout' | 'function-set' | 'workflow';
export type TemplateScope = 'builtin' | 'team' | 'personal';

// 模板分类
export type TemplateCategory = 'standard' | 'gaming' | 'analytics' | 'admin' | 'custom';

// 模板元数据
export interface TemplateMeta {
  id: string;
  name: string;
  description: string;
  type: TemplateType;
  category: TemplateCategory;
  tags: string[];
  version: string;
  author: string;
  createdAt: string;
  updatedAt: string;
  isFavorite: boolean;
  isBuiltIn: boolean;
  usageCount: number;
  thumbnail?: string;
  scope?: TemplateScope;
  scenario?: string;
  requiredFunctions?: string[];
  riskNotes?: string[];
  applyChecklist?: string[];
  showcase?: boolean;
}

// 模板完整数据
export interface Template extends TemplateMeta {
  config: Record<string, any>;
}

function toPreviewWorkspaceConfig(template: Template): WorkspaceConfig {
  const cfg = (template?.config || {}) as Record<string, any>;
  return {
    objectKey: `template.${template.id}`,
    title: template.name || '模板预览',
    description: template.description || '',
    layout: (cfg.layout || { type: 'tabs', tabs: [] }) as any,
    status: 'draft',
    published: false,
  };
}

type TemplateCapabilityLevel = 'stable' | 'beta' | 'experimental';

function isRenderableTemplateConfig(config: Record<string, any> | undefined): boolean {
  const layout = config?.layout;
  if (!layout || layout.type !== 'tabs' || !Array.isArray(layout.tabs)) {
    return false;
  }
  return layout.tabs.every((tab: any) => RENDERABLE_TAB_LAYOUT_TYPES.has(tab?.layout?.type));
}

function getTemplateCapabilityLevel(
  config: Record<string, any> | undefined,
): TemplateCapabilityLevel {
  const layout = config?.layout;
  if (
    !layout ||
    layout.type !== 'tabs' ||
    !Array.isArray(layout.tabs) ||
    layout.tabs.length === 0
  ) {
    return 'experimental';
  }
  const tabTypes = layout.tabs.map((tab: any) => tab?.layout?.type);
  if (tabTypes.every((type: string) => STABLE_TAB_LAYOUT_TYPES.has(type))) {
    return 'stable';
  }
  if (
    tabTypes.every((type: string) => RENDERABLE_TAB_LAYOUT_TYPES.has(type) && type !== 'custom')
  ) {
    return 'beta';
  }
  return 'experimental';
}

function getCapabilityTag(level: TemplateCapabilityLevel): React.ReactNode {
  if (level === 'stable') {
    return <Tag color="green">正式</Tag>;
  }
  if (level === 'beta') {
    return <Tag color="blue">Beta</Tag>;
  }
  return <Tag color="orange">实验</Tag>;
}

function getTemplateLayoutSummary(template: Template): string {
  const tabs = template.config?.layout?.tabs;
  if (!Array.isArray(tabs) || tabs.length === 0) {
    return '暂无可用布局';
  }
  const layoutTypes = Array.from(
    new Set(tabs.map((tab: any) => String(tab?.layout?.type || '').trim()).filter(Boolean)),
  );
  return layoutTypes.join(' / ');
}

function getTemplateStructureFacts(template: Template): {
  tabCount: number;
  tabTitles: string[];
  layoutSummary: string;
  primaryBenefit: string;
} {
  const tabs = Array.isArray(template.config?.layout?.tabs) ? template.config.layout.tabs : [];
  const tabTitles = tabs
    .map((tab: any) => String(tab?.title || tab?.key || '').trim())
    .filter(Boolean)
    .slice(0, 4);
  const layoutSummary = getTemplateLayoutSummary(template);
  let primaryBenefit = '可直接作为工作台骨架起点';
  if (template.showcase) primaryBenefit = '适合直接演示产品能力和视觉标准';
  else if (template.type === 'layout') primaryBenefit = '适合快速起一个布局骨架';
  else if (template.type === 'workspace') primaryBenefit = '适合直接生成可继续细化的业务页面';
  return {
    tabCount: tabs.length,
    tabTitles,
    layoutSummary,
    primaryBenefit,
  };
}

// 内置模板
const BUILTIN_TEMPLATES: Template[] = [
  {
    id: 'tpl-layout-list-empty',
    name: '列表布局-空模板',
    description: '最小列表布局，适合从零开始配置',
    type: 'layout',
    category: 'standard',
    tags: ['list', 'empty'],
    version: '1.0.0',
    author: 'System',
    createdAt: '2024-01-01',
    updatedAt: '2024-01-01',
    isFavorite: false,
    isBuiltIn: true,
    usageCount: 0,
    scope: 'builtin',
    config: {
      layout: {
        type: 'tabs',
        tabs: [
          {
            key: 'list',
            title: '列表',
            functions: [],
            layout: { type: 'list', listFunction: '', columns: [] },
          },
        ],
      },
    },
  },
  {
    id: 'tpl-layout-kanban-standard',
    name: '看板布局-标准模板',
    description: '待处理/处理中/已完成三列看板',
    type: 'layout',
    category: 'standard',
    tags: ['kanban', 'standard'],
    version: '1.0.0',
    author: 'System',
    createdAt: '2024-01-01',
    updatedAt: '2024-01-01',
    isFavorite: false,
    isBuiltIn: true,
    usageCount: 0,
    scope: 'builtin',
    config: {
      layout: {
        type: 'tabs',
        tabs: [
          {
            key: 'kanban',
            title: '看板',
            functions: [],
            layout: {
              type: 'kanban',
              dataFunction: '',
              columns: [
                { id: 'todo', title: '待处理', color: '#1677ff' },
                { id: 'processing', title: '处理中', color: '#faad14' },
                { id: 'done', title: '已完成', color: '#52c41a' },
              ],
            },
          },
        ],
      },
    },
  },
  {
    id: 'tpl-layout-timeline-standard',
    name: '时间线布局-标准模板',
    description: '事件流时间线，带筛选与逆序',
    type: 'layout',
    category: 'standard',
    tags: ['timeline', 'standard'],
    version: '1.0.0',
    author: 'System',
    createdAt: '2024-01-01',
    updatedAt: '2024-01-01',
    isFavorite: false,
    isBuiltIn: true,
    usageCount: 0,
    scope: 'builtin',
    config: {
      layout: {
        type: 'tabs',
        tabs: [
          {
            key: 'timeline',
            title: '时间线',
            functions: [],
            layout: { type: 'timeline', dataFunction: '', showFilter: true, reverse: true },
          },
        ],
      },
    },
  },
  {
    id: 'tpl-layout-split-master-detail',
    name: '主从布局-标准模板',
    description: '左列表右详情的主从分栏',
    type: 'layout',
    category: 'standard',
    tags: ['split', 'master-detail'],
    version: '1.0.0',
    author: 'System',
    createdAt: '2024-01-01',
    updatedAt: '2024-01-01',
    isFavorite: false,
    isBuiltIn: true,
    usageCount: 0,
    scope: 'builtin',
    config: {
      layout: {
        type: 'tabs',
        tabs: [
          {
            key: 'master-detail',
            title: '主从视图',
            functions: [],
            layout: {
              type: 'split',
              direction: 'horizontal',
              panels: [
                {
                  key: 'left',
                  title: '主列表',
                  span: 12,
                  component: { type: 'list', config: { listFunction: '', columns: [] } },
                },
                {
                  key: 'right',
                  title: '详情',
                  span: 12,
                  component: { type: 'detail', config: { detailFunction: '', sections: [] } },
                },
              ],
            },
          },
        ],
      },
    },
  },
  {
    id: 'tpl-layout-wizard-standard',
    name: '向导布局-标准模板',
    description: '两步向导流程模板',
    type: 'layout',
    category: 'standard',
    tags: ['wizard', 'standard'],
    version: '1.0.0',
    author: 'System',
    createdAt: '2024-01-01',
    updatedAt: '2024-01-01',
    isFavorite: false,
    isBuiltIn: true,
    usageCount: 0,
    scope: 'builtin',
    config: {
      layout: {
        type: 'tabs',
        tabs: [
          {
            key: 'wizard',
            title: '向导流程',
            functions: [],
            layout: {
              type: 'wizard',
              steps: [
                {
                  key: 'step1',
                  title: '信息填写',
                  component: { type: 'form', config: { submitFunction: '', fields: [] } },
                },
                {
                  key: 'step2',
                  title: '结果确认',
                  component: { type: 'detail', config: { detailFunction: '', sections: [] } },
                },
              ],
            },
          },
        ],
      },
    },
  },
  {
    id: 'tpl-layout-dashboard-standard',
    name: '仪表盘布局-标准模板',
    description: '指标卡 + 数据面板',
    type: 'layout',
    category: 'analytics',
    tags: ['dashboard', 'analytics'],
    version: '1.0.0',
    author: 'System',
    createdAt: '2024-01-01',
    updatedAt: '2024-01-01',
    isFavorite: false,
    isBuiltIn: true,
    usageCount: 0,
    scope: 'builtin',
    config: {
      layout: {
        type: 'tabs',
        tabs: [
          {
            key: 'dashboard',
            title: '仪表盘',
            functions: [],
            layout: {
              type: 'dashboard',
              stats: [
                { key: 'online', title: '在线人数', value: 0 },
                { key: 'dau', title: 'DAU', value: 0 },
              ],
              panels: [
                {
                  key: 'p1',
                  title: '列表',
                  span: 12,
                  component: { type: 'list', config: { listFunction: '', columns: [] } },
                },
                {
                  key: 'p2',
                  title: '详情',
                  span: 12,
                  component: { type: 'detail', config: { detailFunction: '', sections: [] } },
                },
              ],
            },
          },
        ],
      },
    },
  },
  {
    id: 'tpl-layout-grid-standard',
    name: '网格布局-标准模板',
    description: '双列网格，左列表右详情',
    type: 'layout',
    category: 'standard',
    tags: ['grid', 'standard'],
    version: '1.0.0',
    author: 'System',
    createdAt: '2024-01-01',
    updatedAt: '2024-01-01',
    isFavorite: false,
    isBuiltIn: true,
    usageCount: 0,
    scope: 'builtin',
    config: {
      layout: {
        type: 'tabs',
        tabs: [
          {
            key: 'grid',
            title: '网格',
            functions: [],
            layout: {
              type: 'grid',
              columns: 2,
              items: [
                {
                  key: 'item-list',
                  component: { type: 'list', config: { listFunction: '', columns: [] } },
                },
                {
                  key: 'item-detail',
                  component: { type: 'detail', config: { detailFunction: '', sections: [] } },
                },
              ],
            },
          },
        ],
      },
    },
  },
  {
    id: 'tpl-layout-custom-empty',
    name: '自定义布局-空模板',
    description: '自定义组件占位模板',
    type: 'layout',
    category: 'custom',
    tags: ['custom', 'empty'],
    version: '1.0.0',
    author: 'System',
    createdAt: '2024-01-01',
    updatedAt: '2024-01-01',
    isFavorite: false,
    isBuiltIn: true,
    usageCount: 0,
    scope: 'builtin',
    config: {
      layout: {
        type: 'tabs',
        tabs: [
          {
            key: 'custom',
            title: '自定义',
            functions: [],
            layout: { type: 'custom', component: 'CustomPanel', props: {} },
          },
        ],
      },
    },
  },
  {
    id: 'tpl-gaming-list-drawer-form',
    name: '游戏运营-列表+抽屉表单',
    description: '适合活动配置/补偿管理：列表 + 抽屉编辑 + 弹窗创建',
    type: 'workspace',
    category: 'gaming',
    tags: ['gaming', 'list', 'drawer', 'form'],
    version: '1.0.0',
    author: 'System',
    createdAt: '2024-01-01',
    updatedAt: '2024-01-01',
    isFavorite: false,
    isBuiltIn: true,
    usageCount: 0,
    scope: 'builtin',
    scenario: '运营列表台，适合活动、补偿、批量配置等高频处理场景',
    requiredFunctions: ['listFunction', 'createFunction', 'updateFunction'],
    riskNotes: ['抽屉与弹窗动作依赖真实函数绑定', '发布前应重点核对表单提交结果'],
    applyChecklist: ['确认列表函数已返回主字段', '检查新建/编辑动作是否绑定真实函数', '预览首屏是否能直接看懂主操作'],
    showcase: true,
    config: {
      layout: {
        type: 'tabs',
        tabs: [
          {
            key: 'ops-list',
            title: '运营列表',
            functions: [],
            layout: {
              type: 'list',
              listFunction: '',
              columns: [
                { key: 'id', title: 'ID' },
                { key: 'name', title: '名称' },
              ],
              toolbarActions: [
                {
                  key: 'create',
                  label: '新建',
                  type: 'modal',
                  function: '',
                  fields: [{ key: 'name', label: '名称', type: 'input', required: true }],
                },
              ],
              rowActions: [
                {
                  key: 'edit',
                  label: '编辑',
                  type: 'drawer',
                  function: '',
                  fields: [{ key: 'name', label: '名称', type: 'input', required: true }],
                },
              ],
            },
          },
        ],
      },
    },
  },
  {
    id: 'tpl-standard-crud',
    name: '标准 CRUD 管理页',
    description: '包含列表、创建、编辑、删除功能的标准管理页面模板',
    type: 'workspace',
    category: 'standard',
    tags: ['CRUD', '管理', '标准'],
    version: '1.0.0',
    author: 'System',
    createdAt: '2024-01-01',
    updatedAt: '2024-01-01',
    isFavorite: false,
    isBuiltIn: true,
    usageCount: 0,
    scope: 'builtin',
    scenario: '查询-处理工作台，适合标准后台对象的增删改查主路径',
    requiredFunctions: ['listFunction', 'submitFunction', 'deleteFunction'],
    riskNotes: ['删除动作需要绑定真实危险操作函数', '建议补充字段校验与空态检查'],
    applyChecklist: ['确认列表与创建页都已绑定核心函数', '检查删除动作是否配置风险确认', '发布前核对字段与空态提示'],
    showcase: true,
    config: {
      layout: {
        type: 'tabs',
        tabs: [
          {
            key: 'list',
            title: '列表',
            functions: [],
            layout: {
              type: 'list',
              listFunction: '',
              columns: [
                { key: 'id', title: 'ID' },
                { key: 'name', title: '名称' },
                { key: 'status', title: '状态' },
                { key: 'createdAt', title: '创建时间', render: 'datetime' },
              ],
              toolbarActions: [
                {
                  key: 'create',
                  label: '新建',
                  type: 'modal',
                  function: '',
                  fields: [
                    { key: 'name', label: '名称', type: 'input', required: true },
                    {
                      key: 'status',
                      label: '状态',
                      type: 'select',
                      options: [
                        { label: '启用', value: 'active' },
                        { label: '禁用', value: 'disabled' },
                      ],
                    },
                  ],
                },
              ],
              rowActions: [
                {
                  key: 'edit',
                  label: '编辑',
                  type: 'drawer',
                  function: '',
                  fields: [
                    { key: 'name', label: '名称', type: 'input', required: true },
                    {
                      key: 'status',
                      label: '状态',
                      type: 'select',
                      options: [
                        { label: '启用', value: 'active' },
                        { label: '禁用', value: 'disabled' },
                      ],
                    },
                  ],
                },
                {
                  key: 'delete',
                  label: '删除',
                  type: 'popconfirm',
                  function: '',
                  danger: true,
                  confirmMessage: '确认删除该记录？',
                },
              ],
            },
          },
          {
            key: 'create',
            title: '创建',
            functions: [],
            layout: {
              type: 'form',
              submitFunction: '',
              fields: [
                { key: 'name', label: '名称', type: 'input', required: true },
                { key: 'description', label: '描述', type: 'textarea' },
                {
                  key: 'status',
                  label: '状态',
                  type: 'select',
                  options: [
                    { label: '启用', value: 'active' },
                    { label: '禁用', value: 'disabled' },
                  ],
                },
              ],
            },
          },
        ],
      },
    },
  },
  {
    id: 'tpl-gaming-player',
    name: '玩家管理模板',
    description: '游戏玩家管理页面模板，包含玩家信息、背包、邮件等功能',
    type: 'workspace',
    category: 'gaming',
    tags: ['玩家', '游戏', 'GM'],
    version: '1.0.0',
    author: 'System',
    createdAt: '2024-01-01',
    updatedAt: '2024-01-01',
    isFavorite: false,
    isBuiltIn: true,
    usageCount: 0,
    scope: 'builtin',
    scenario: '对象详情台，适合多标签业务对象的全景查询与运营处理',
    requiredFunctions: ['detailFunction', 'inventoryListFunction', 'mailListFunction'],
    riskNotes: ['多标签页需要逐页检查函数绑定', '详情页发布前应重点核对权限与空态'],
    applyChecklist: ['逐页检查玩家信息/背包/邮件函数绑定', '确认详情页首屏字段是否足够完整', '验证多标签切换是否符合业务阅读顺序'],
    showcase: true,
    config: {
      layout: {
        type: 'tabs',
        tabs: [
          {
            key: 'info',
            title: '玩家信息',
            icon: 'UserOutlined',
            layout: {
              type: 'detail',
              detailFunction: '',
              sections: [
                {
                  title: '基础信息',
                  fields: [
                    { key: 'playerId', label: '玩家ID' },
                    { key: 'nickname', label: '昵称' },
                    { key: 'level', label: '等级' },
                    { key: 'vip', label: 'VIP' },
                  ],
                },
              ],
            },
          },
          {
            key: 'inventory',
            title: '背包',
            icon: 'InboxOutlined',
            layout: {
              type: 'list',
              listFunction: '',
              columns: [
                { key: 'itemId', title: '道具ID' },
                { key: 'itemName', title: '道具名' },
                { key: 'count', title: '数量' },
                { key: 'expireAt', title: '过期时间', render: 'datetime' },
              ],
            },
          },
          {
            key: 'mail',
            title: '邮件',
            icon: 'MailOutlined',
            layout: {
              type: 'list',
              listFunction: '',
              columns: [
                { key: 'mailId', title: '邮件ID' },
                { key: 'title', title: '标题' },
                { key: 'status', title: '状态' },
                { key: 'createdAt', title: '发送时间', render: 'datetime' },
              ],
            },
          },
        ],
      },
    },
  },
  {
    id: 'tpl-admin-users',
    name: '用户管理模板',
    description: '后台用户管理页面模板，包含用户列表、角色分配、权限管理',
    type: 'workspace',
    category: 'admin',
    tags: ['用户', '权限', '后台'],
    version: '1.0.0',
    author: 'System',
    createdAt: '2024-01-01',
    updatedAt: '2024-01-01',
    isFavorite: false,
    isBuiltIn: true,
    usageCount: 0,
    scope: 'builtin',
    scenario: '后台权限管理台，适合组织、角色、权限等管理对象',
    requiredFunctions: ['userListFunction', 'roleListFunction', 'permissionDetailFunction'],
    riskNotes: ['权限对象需要显式校验读写权限', '建议补充角色变更与审计链路'],
    config: {
      layout: {
        type: 'tabs',
        tabs: [
          {
            key: 'users',
            title: '用户列表',
            layout: {
              type: 'list',
              listFunction: '',
              columns: [
                { key: 'userId', title: '用户ID' },
                { key: 'username', title: '用户名' },
                { key: 'status', title: '状态' },
                { key: 'updatedAt', title: '更新时间', render: 'datetime' },
              ],
            },
          },
          {
            key: 'roles',
            title: '角色管理',
            layout: {
              type: 'list',
              listFunction: '',
              columns: [
                { key: 'roleId', title: '角色ID' },
                { key: 'roleName', title: '角色名' },
                { key: 'memberCount', title: '成员数' },
              ],
            },
          },
          {
            key: 'permissions',
            title: '权限设置',
            layout: {
              type: 'detail',
              detailFunction: '',
              sections: [
                {
                  title: '权限信息',
                  fields: [
                    { key: 'role', label: '角色' },
                    { key: 'scopes', label: '权限范围' },
                    { key: 'updatedBy', label: '最后修改人' },
                  ],
                },
              ],
            },
          },
        ],
      },
    },
  },
];

export const SHOWCASE_TEMPLATES: Template[] = BUILTIN_TEMPLATES.filter((template) => template.showcase);

// 分类颜色
const CATEGORY_COLORS: Record<TemplateCategory, string> = {
  standard: 'blue',
  gaming: 'purple',
  analytics: 'green',
  admin: 'orange',
  custom: 'cyan',
};

// 类型图标
const TYPE_ICONS: Record<TemplateType, React.ReactNode> = {
  workspace: <FolderOutlined />,
  layout: <FileOutlined />,
  'function-set': <StarOutlined />,
  workflow: <StarFilled />,
};

export interface TemplateManagerProps {
  visible: boolean;
  onClose: () => void;
  onSelect: (template: Template) => void;
  currentConfig?: Record<string, any>;
  onSaveAsTemplate?: (template: Omit<Template, 'id' | 'createdAt' | 'updatedAt'>) => Promise<void>;
  initialBrowseMode?: 'all' | 'showcase';
}

export default function TemplateManager({
  visible,
  onClose,
  onSelect,
  currentConfig,
  onSaveAsTemplate,
  initialBrowseMode = 'all',
}: TemplateManagerProps) {
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchText, setSearchText] = useState('');
  const [filterType, setFilterType] = useState<TemplateType | 'all'>('all');
  const [filterCategory, setFilterCategory] = useState<TemplateCategory | 'all'>('all');
  const [filterScope, setFilterScope] = useState<TemplateScope | 'all'>('all');
  const [browseMode, setBrowseMode] = useState<'all' | 'showcase'>(initialBrowseMode);
  const [showSaveModal, setShowSaveModal] = useState(false);
  const [previewTemplate, setPreviewTemplate] = useState<Template | null>(null);
  const [previewMode, setPreviewMode] = useState<'page' | 'json'>('page');
  const [saveForm] = Form.useForm();

  const persistTemplates = (allTemplates: Template[]) => {
    const personalTemplates = allTemplates.filter(
      (t) => !t.isBuiltIn && (t.scope || 'personal') === 'personal',
    );
    const teamTemplates = allTemplates.filter(
      (t) => !t.isBuiltIn && (t.scope || 'personal') === 'team',
    );
    localStorage.setItem('workspace_templates', JSON.stringify(personalTemplates));
    localStorage.setItem('workspace_team_templates', JSON.stringify(teamTemplates));
  };

  // 加载模板
  useEffect(() => {
    if (visible) {
      setBrowseMode(initialBrowseMode);
      loadTemplates();
    }
  }, [visible, initialBrowseMode]);

  const loadTemplates = async () => {
    setLoading(true);
    try {
      // 这里应该从 API 加载，现在使用内置模板
      // 同时加载本地存储的用户模板
      const storedTemplates = localStorage.getItem('workspace_templates');
      const storedTeamTemplates = localStorage.getItem('workspace_team_templates');
      let userTemplates: Template[] = [];
      let teamTemplates: Template[] = [];

      if (storedTemplates) {
        try {
          userTemplates = JSON.parse(storedTemplates);
        } catch (e) {
          console.error('Failed to parse stored templates', e);
        }
      }
      if (storedTeamTemplates) {
        try {
          teamTemplates = JSON.parse(storedTeamTemplates);
        } catch (e) {
          console.error('Failed to parse stored team templates', e);
        }
      }

      userTemplates = userTemplates.map((item) => ({ ...item, scope: item.scope || 'personal' }));
      teamTemplates = teamTemplates.map((item) => ({ ...item, scope: item.scope || 'team' }));

      setTemplates(
        [...BUILTIN_TEMPLATES, ...teamTemplates, ...userTemplates].filter((template) =>
          isRenderableTemplateConfig(template.config),
        ),
      );
    } catch (error: any) {
      message.error(error.message || '加载模板失败');
    } finally {
      setLoading(false);
    }
  };

  // 过滤模板
  const filteredTemplates = templates.filter((template) => {
    if (searchText) {
      const text = searchText.toLowerCase();
      const matchName = template.name.toLowerCase().includes(text);
      const matchDesc = template.description.toLowerCase().includes(text);
      const matchTags = template.tags.some((tag) => tag.toLowerCase().includes(text));
      if (!matchName && !matchDesc && !matchTags) return false;
    }

    if (filterType !== 'all' && template.type !== filterType) return false;
    if (filterCategory !== 'all' && template.category !== filterCategory) return false;
    if (filterScope !== 'all' && (template.scope || 'personal') !== filterScope) return false;
    if (browseMode === 'showcase' && !template.showcase) return false;

    return true;
  });

  const showcaseTemplates = filteredTemplates.filter((template) => template.showcase);

  // 收藏/取消收藏
  const toggleFavorite = (templateId: string) => {
    const updatedTemplates = templates.map((t) =>
      t.id === templateId ? { ...t, isFavorite: !t.isFavorite } : t,
    );
    setTemplates(updatedTemplates);
    persistTemplates(updatedTemplates);
  };

  // 删除模板
  const deleteTemplate = (templateId: string) => {
    const template = templates.find((t) => t.id === templateId);
    if (template?.isBuiltIn) {
      message.warning('内置模板不能删除');
      return;
    }

    const updatedTemplates = templates.filter((t) => t.id !== templateId);
    setTemplates(updatedTemplates);
    persistTemplates(updatedTemplates);
    message.success('模板已删除');
  };

  // 复制模板
  const duplicateTemplate = (template: Template) => {
    const newTemplate: Template = {
      ...template,
      id: `tpl_${Date.now()}`,
      name: `${template.name} (副本)`,
      isBuiltIn: false,
      scope: 'personal',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    const updatedTemplates = [...templates, newTemplate];
    setTemplates(updatedTemplates);
    persistTemplates(updatedTemplates);
    message.success('模板已复制');
  };

  // 导出模板
  const exportTemplate = (template: Template) => {
    const data = JSON.stringify(template, null, 2);
    const blob = new Blob([data], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${template.name}.json`;
    a.click();
    URL.revokeObjectURL(url);
    message.success('模板已导出');
  };

  // 导入模板
  const importTemplate = () => {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = '.json';
    input.onchange = async (e) => {
      const file = (e.target as HTMLInputElement).files?.[0];
      if (!file) return;

      try {
        const text = await file.text();
        const template: Template = JSON.parse(text);

        // 验证模板格式
        if (!template.name || !template.config) {
          throw new Error('无效的模板格式');
        }
        if (!isRenderableTemplateConfig(template.config)) {
          throw new Error('仅支持当前可渲染模板：tabs + 已接入运行时的布局类型');
        }

        // 生成新 ID
        template.id = `tpl_${Date.now()}`;
        template.isBuiltIn = false;
        template.scope = template.scope || 'personal';
        template.createdAt = new Date().toISOString();
        template.updatedAt = new Date().toISOString();

        const updatedTemplates = [...templates, template];
        setTemplates(updatedTemplates);
        persistTemplates(updatedTemplates);
        message.success('模板导入成功');
      } catch (error: any) {
        message.error(error.message || '导入失败');
      }
    };
    input.click();
  };

  // 保存当前配置为模板
  const handleSaveAsTemplate = async () => {
    if (!currentConfig) {
      message.warning('没有可保存的配置');
      return;
    }

    try {
      const values = await saveForm.validateFields();
      const template: Template = {
        id: `tpl_${Date.now()}`,
        name: values.name,
        description: values.description || '',
        type: 'workspace',
        category: values.category,
        tags: values.tags || [],
        version: '1.0.0',
        author: 'User',
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        isFavorite: false,
        isBuiltIn: false,
        usageCount: 0,
        scope: values.scope || 'personal',
        config: currentConfig,
      };

      const updatedTemplates = [...templates, template];
      setTemplates(updatedTemplates);
      persistTemplates(updatedTemplates);
      setShowSaveModal(false);
      saveForm.resetFields();
      message.success('模板保存成功');

      onSaveAsTemplate?.(template);
    } catch (error) {
      // 表单验证失败
    }
  };

  // 模板操作菜单
  const getTemplateMenuItems = (template: Template): MenuProps['items'] => [
    {
      key: 'duplicate',
      icon: <CopyOutlined />,
      label: '复制',
      onClick: () => duplicateTemplate(template),
    },
    {
      key: 'export',
      icon: <DownloadOutlined />,
      label: '导出',
      onClick: () => exportTemplate(template),
    },
    { type: 'divider' },
    {
      key: 'delete',
      icon: <DeleteOutlined />,
      label: '删除',
      danger: true,
      disabled: template.isBuiltIn,
      onClick: () => deleteTemplate(template.id),
    },
  ];

  return (
    <Modal
      title="页面配置模板"
      open={visible}
      onCancel={onClose}
      width={900}
      footer={null}
      styles={{ body: { padding: 0 } }}
    >
      <div style={{ display: 'flex', height: 600 }}>
        {/* 左侧过滤面板 */}
        <div
          style={{
            width: 200,
            borderRight: '1px solid #f0f0f0',
            padding: 16,
            background: '#fafafa',
          }}
        >
          <Title level={5} style={{ marginBottom: 16 }}>
            筛选
          </Title>
          <Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 16 }}>
            模板按正式、Beta、实验三层展示。默认推荐正式模板，高级布局模板会明确标注能力等级。
          </Paragraph>

          <div style={{ marginBottom: 16 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              浏览模式
            </Text>
            <Segmented
              value={browseMode}
              onChange={(value) => setBrowseMode(value as 'all' | 'showcase')}
              style={{ width: '100%', marginTop: 8 }}
              options={[
                { label: '全部模板', value: 'all' },
                { label: '演示样板', value: 'showcase' },
              ]}
            />
          </div>

          <div style={{ marginBottom: 16 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              按类型
            </Text>
            <Select
              value={filterType}
              onChange={setFilterType}
              style={{ width: '100%', marginTop: 8 }}
              options={[
                { value: 'all', label: '全部' },
                { value: 'workspace', label: '工作空间' },
                { value: 'layout', label: '布局模板' },
              ]}
            />
          </div>

          <div style={{ marginBottom: 16 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              按分类
            </Text>
            <Select
              value={filterCategory}
              onChange={setFilterCategory}
              style={{ width: '100%', marginTop: 8 }}
              options={[
                { value: 'all', label: '全部' },
                { value: 'standard', label: '标准' },
                { value: 'gaming', label: '游戏' },
                { value: 'analytics', label: '分析' },
                { value: 'admin', label: '管理' },
                { value: 'custom', label: '自定义' },
              ]}
            />
          </div>

          <div style={{ marginBottom: 16 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              按来源
            </Text>
            <Select
              value={filterScope}
              onChange={setFilterScope}
              style={{ width: '100%', marginTop: 8 }}
              options={[
                { value: 'all', label: '全部' },
                { value: 'builtin', label: '内置' },
                { value: 'team', label: '团队' },
                { value: 'personal', label: '个人' },
              ]}
            />
          </div>

          <Divider />

          <Space direction="vertical" style={{ width: '100%' }}>
            <Button icon={<UploadOutlined />} block onClick={importTemplate}>
              导入模板
            </Button>
            {currentConfig && (
              <Button
                type="primary"
                icon={<SaveOutlined />}
                block
                onClick={() => setShowSaveModal(true)}
              >
                保存为模板
              </Button>
            )}
          </Space>
        </div>

        {/* 右侧模板列表 */}
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
          {/* 搜索栏 */}
          <div style={{ padding: 16, borderBottom: '1px solid #f0f0f0' }}>
            <Space direction="vertical" size={12} style={{ width: '100%' }}>
              <Search
                placeholder="搜索模板名称、描述或标签"
                value={searchText}
                onChange={(e) => setSearchText(e.target.value)}
                allowClear
              />
              <Space wrap size={[8, 8]}>
                <Tag color="blue">{`当前结果 ${filteredTemplates.length}`}</Tag>
                <Tag color="magenta">{`官方样板 ${showcaseTemplates.length}`}</Tag>
                {browseMode === 'showcase' ? (
                  <Text type="secondary">当前只展示可直接用于演示和视觉校验的官方样板。</Text>
                ) : (
                  <Text type="secondary">优先从官方演示样板开始，再扩展到普通布局模板和个人模板。</Text>
                )}
              </Space>
            </Space>
          </div>

          {/* 模板列表 */}
          <div style={{ flex: 1, overflow: 'auto', padding: 16 }}>
            {loading ? (
              <div style={{ textAlign: 'center', padding: 40 }}>
                <Spin />
              </div>
            ) : filteredTemplates.length === 0 ? (
              <Empty description="没有找到匹配的模板" />
            ) : (
              <Space direction="vertical" size={16} style={{ width: '100%' }}>
                {browseMode === 'all' && showcaseTemplates.length > 0 ? (
                  <Card
                    size="small"
                    title="官方演示样板"
                    extra={<Text type="secondary">优先用于对外演示、模板基线和视觉校验</Text>}
                    styles={{
                      body: {
                        background:
                          'linear-gradient(135deg, rgba(240,101,149,0.08) 0%, rgba(22,119,255,0.03) 100%)',
                      },
                    }}
                  >
                    <List
                      grid={{ gutter: 12, column: 3 }}
                      dataSource={showcaseTemplates}
                      renderItem={(template) => (
                        <List.Item>
                          <Card
                            hoverable
                            size="small"
                            onClick={() => onSelect(template)}
                            styles={{ body: { padding: 14 } }}
                          >
                            <Space direction="vertical" size={8} style={{ width: '100%' }}>
                              <Space wrap size={[6, 6]}>
                                <Tag color="magenta">官方演示样板</Tag>
                                {getCapabilityTag(getTemplateCapabilityLevel(template.config))}
                                <Tag>{`${getTemplateStructureFacts(template).tabCount} 个页面`}</Tag>
                              </Space>
                              <Text strong>{template.name}</Text>
                              <Text type="secondary">{template.scenario || template.description}</Text>
                              <Space wrap size={[6, 6]}>
                                <Tag color="blue">{getTemplateLayoutSummary(template)}</Tag>
                                {(template.applyChecklist || []).slice(0, 1).map((item) => (
                                  <Tag key={`showcase-check-${template.id}-${item}`} color="processing">
                                    {item}
                                  </Tag>
                                ))}
                              </Space>
                              <Space wrap size={[6, 6]}>
                                <Button
                                  size="small"
                                  icon={<EyeOutlined />}
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    setPreviewMode('page');
                                    setPreviewTemplate(template);
                                  }}
                                >
                                  预览
                                </Button>
                                <Button
                                  size="small"
                                  type="primary"
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    onSelect(template);
                                  }}
                                >
                                  使用
                                </Button>
                              </Space>
                            </Space>
                          </Card>
                        </List.Item>
                      )}
                    />
                  </Card>
                ) : null}

                <List
                  grid={{ gutter: 16, column: 2 }}
                  dataSource={filteredTemplates}
                  renderItem={(template) => {
                    const capabilityLevel = getTemplateCapabilityLevel(template.config);
                    return (
                      <List.Item>
                        <Card
                          hoverable
                          size="small"
                          title={
                            <Space>
                              {TYPE_ICONS[template.type]}
                              <span>{template.name}</span>
                              {getCapabilityTag(capabilityLevel)}
                              {template.isBuiltIn && (
                                <Tag color="gold" style={{ marginLeft: 4 }}>
                                  内置
                                </Tag>
                              )}
                            </Space>
                          }
                          extra={
                            <Space>
                              <Button
                                type="text"
                                size="small"
                                icon={
                                  template.isFavorite ? (
                                    <StarFilled style={{ color: '#faad14' }} />
                                  ) : (
                                    <StarOutlined />
                                  )
                                }
                                onClick={() => toggleFavorite(template.id)}
                              />
                              <Dropdown
                                menu={{ items: getTemplateMenuItems(template) }}
                                trigger={['click']}
                              >
                                <Button type="text" size="small" icon={<MoreOutlined />} />
                              </Dropdown>
                            </Space>
                          }
                          onClick={() => onSelect(template)}
                          actions={[
                            <Button
                              key="preview"
                              type="link"
                              size="small"
                              icon={<EyeOutlined />}
                              onClick={(e) => {
                                e.stopPropagation();
                                setPreviewMode('page');
                                setPreviewTemplate(template);
                              }}
                            >
                              预览
                            </Button>,
                            <Button
                              key="use"
                              type="link"
                              size="small"
                              onClick={(e) => {
                                e.stopPropagation();
                                onSelect(template);
                              }}
                            >
                              使用
                            </Button>,
                          ]}
                        >
                          <Paragraph
                            ellipsis={{ rows: 2 }}
                            style={{ marginBottom: 8, minHeight: 44 }}
                          >
                            {template.description}
                          </Paragraph>
                          <Space direction="vertical" size={8} style={{ width: '100%' }}>
                            <div>
                              <Tag color={CATEGORY_COLORS[template.category]}>{template.category}</Tag>
                              <Tag>{getTemplateLayoutSummary(template)}</Tag>
                              <Tag>
                                {template.scope === 'builtin'
                                  ? '内置'
                                  : template.scope === 'team'
                                  ? '团队'
                                  : '个人'}
                              </Tag>
                              {template.showcase ? <Tag color="magenta">官方演示样板</Tag> : null}
                              {template.tags.slice(0, 2).map((tag) => (
                                <Tag key={tag}>{tag}</Tag>
                              ))}
                            </div>
                            {template.scenario ? (
                              <Text type="secondary">{`适用场景：${template.scenario}`}</Text>
                            ) : null}
                            {template.requiredFunctions?.length ? (
                              <Text type="secondary">
                                {`所需函数：${template.requiredFunctions.slice(0, 3).join(' / ')}`}
                              </Text>
                            ) : null}
                            {template.riskNotes?.[0] ? (
                              <Text type="secondary">{`风险提示：${template.riskNotes[0]}`}</Text>
                            ) : null}
                          </Space>
                        </Card>
                      </List.Item>
                    );
                  }}
                />
              </Space>
            )}
          </div>
        </div>
      </div>

      {/* 预览弹窗 */}
      <Modal
        title={`预览: ${previewTemplate?.name}`}
        open={!!previewTemplate}
        onCancel={() => {
          setPreviewTemplate(null);
          setPreviewMode('page');
        }}
        footer={[
          <Button
            key="cancel"
            onClick={() => {
              setPreviewTemplate(null);
              setPreviewMode('page');
            }}
          >
            取消
          </Button>,
          <Button
            key="use"
            type="primary"
            onClick={() => {
              if (previewTemplate) {
                onSelect(previewTemplate);
                setPreviewTemplate(null);
              }
            }}
          >
            使用此模板
          </Button>,
        ]}
        width="92vw"
        styles={{ body: { paddingTop: 12 } }}
      >
        {previewTemplate && (
          <div>
            {(() => {
              const facts = getTemplateStructureFacts(previewTemplate);
              return (
                <Card
                  size="small"
                  style={{ marginBottom: 16 }}
                  styles={{
                    body: {
                      background:
                        'linear-gradient(135deg, rgba(240,101,149,0.08) 0%, rgba(22,119,255,0.03) 100%)',
                    },
                  }}
                >
                  <Space direction="vertical" size={14} style={{ width: '100%' }}>
                    <Space wrap size={[8, 8]}>
                      {previewTemplate.showcase ? <Tag color="magenta">官方演示样板</Tag> : null}
                      {getCapabilityTag(getTemplateCapabilityLevel(previewTemplate.config))}
                      <Tag>{facts.layoutSummary}</Tag>
                    </Space>
                    <Space direction="vertical" size={4} style={{ width: '100%' }}>
                      <Text strong>应用后会生成什么</Text>
                      <Text type="secondary">
                        {facts.primaryBenefit}
                        {facts.tabCount > 0 ? `，并生成 ${facts.tabCount} 个页面骨架。` : '。'}
                      </Text>
                    </Space>
                    <Row gutter={[12, 12]}>
                      <Col xs={24} md={8}>
                        <Card size="small" style={{ height: '100%', background: 'rgba(255,255,255,0.82)' }}>
                          <Space direction="vertical" size={4}>
                            <Text type="secondary">页面数量</Text>
                            <Text strong>{`${facts.tabCount} 个 Tab`}</Text>
                            <Text type="secondary">可作为首个工作台骨架直接继续细化。</Text>
                          </Space>
                        </Card>
                      </Col>
                      <Col xs={24} md={8}>
                        <Card size="small" style={{ height: '100%', background: 'rgba(255,255,255,0.82)' }}>
                          <Space direction="vertical" size={4}>
                            <Text type="secondary">默认页面</Text>
                            <Text strong>{facts.tabTitles[0] || '待生成'}</Text>
                            <Text type="secondary">
                              {facts.tabTitles.length > 1
                                ? `还会同时带出 ${facts.tabTitles.slice(1).join(' / ')}`
                                : '首个页面生成后可继续扩展更多 Tab。'}
                            </Text>
                          </Space>
                        </Card>
                      </Col>
                      <Col xs={24} md={8}>
                        <Card size="small" style={{ height: '100%', background: 'rgba(255,255,255,0.82)' }}>
                          <Space direction="vertical" size={4}>
                            <Text type="secondary">应用收益</Text>
                            <Text strong>{previewTemplate.scenario || facts.primaryBenefit}</Text>
                            <Text type="secondary">适合直接进入预览、字段补齐和函数绑定阶段。</Text>
                          </Space>
                        </Card>
                      </Col>
                    </Row>
                    <Row gutter={[12, 12]}>
                      <Col xs={24} md={8}>
                        <Card
                          size="small"
                          style={{ height: '100%', background: 'rgba(255,255,255,0.82)' }}
                          styles={{ body: { padding: DASHBOARD_PAGE_TOKENS.compactCardPadding } }}
                        >
                          <Space direction="vertical" size={4}>
                            <Text type="secondary">适用场景</Text>
                            <Text strong>{previewTemplate.scenario || '通用页面搭建起点'}</Text>
                            <Text type="secondary">优先判断它是不是你当前对象工作台最接近的成品骨架。</Text>
                          </Space>
                        </Card>
                      </Col>
                      <Col xs={24} md={8}>
                        <Card
                          size="small"
                          style={{ height: '100%', background: 'rgba(255,255,255,0.82)' }}
                          styles={{ body: { padding: DASHBOARD_PAGE_TOKENS.compactCardPadding } }}
                        >
                          <Space direction="vertical" size={4}>
                            <Text type="secondary">需补函数</Text>
                            <Text strong>
                              {previewTemplate.requiredFunctions?.[0] || '暂无额外声明'}
                            </Text>
                            <Text type="secondary">
                              {previewTemplate.requiredFunctions?.length
                                ? `还需核对 ${previewTemplate.requiredFunctions.slice(1).join(' / ') || '其余关键函数绑定'}`
                                : '主要补真实字段与动作绑定即可。'}
                            </Text>
                          </Space>
                        </Card>
                      </Col>
                      <Col xs={24} md={8}>
                        <Card
                          size="small"
                          style={{ height: '100%', background: 'rgba(255,255,255,0.82)' }}
                          styles={{ body: { padding: DASHBOARD_PAGE_TOKENS.compactCardPadding } }}
                        >
                          <Space direction="vertical" size={4}>
                            <Text type="secondary">发布前重点</Text>
                            <Text strong>{previewTemplate.riskNotes?.[0] || '继续做发布前检查'}</Text>
                            <Text type="secondary">
                              {previewTemplate.applyChecklist?.[0] || '应用后先走一轮完整预览与空态核对。'}
                            </Text>
                          </Space>
                        </Card>
                      </Col>
                    </Row>
                  </Space>
                </Card>
              );
            })()}
            <Paragraph>
              <Text strong>描述：</Text>
              {previewTemplate.description}
            </Paragraph>
            <Paragraph>
              <Text strong>类型：</Text>
              {previewTemplate.type}
            </Paragraph>
            <Paragraph>
              <Text strong>分类：</Text>
              <Tag color={CATEGORY_COLORS[previewTemplate.category]}>
                {previewTemplate.category}
              </Tag>
            </Paragraph>
            <Paragraph>
              <Text strong>能力等级：</Text>
              {getCapabilityTag(getTemplateCapabilityLevel(previewTemplate.config))}
            </Paragraph>
            <Paragraph>
              <Text strong>布局等级：</Text>
              <Tag>{getTemplateLayoutSummary(previewTemplate)}</Tag>
              {previewTemplate.showcase ? <Tag color="magenta">官方演示样板</Tag> : null}
            </Paragraph>
            {previewTemplate.scenario ? (
              <Paragraph>
                <Text strong>适用场景：</Text>
                {previewTemplate.scenario}
              </Paragraph>
            ) : null}
            {previewTemplate.requiredFunctions?.length ? (
              <Paragraph>
                <Text strong>所需函数：</Text>
                {previewTemplate.requiredFunctions.map((item) => (
                  <Tag key={item}>{item}</Tag>
                ))}
              </Paragraph>
            ) : null}
            {previewTemplate.riskNotes?.length ? (
              <Paragraph>
                <Text strong>风险提示：</Text>
                {previewTemplate.riskNotes.map((item) => (
                  <Tag key={item} color="warning">
                    {item}
                  </Tag>
                ))}
              </Paragraph>
            ) : null}
            {previewTemplate.applyChecklist?.length ? (
              <Paragraph>
                <Text strong>应用后检查：</Text>
                {previewTemplate.applyChecklist.map((item) => (
                  <Tag key={item} color="processing">
                    {item}
                  </Tag>
                ))}
              </Paragraph>
            ) : null}
            <Paragraph>
              <Text strong>标签：</Text>
              {previewTemplate.tags.map((tag) => (
                <Tag key={tag}>{tag}</Tag>
              ))}
            </Paragraph>
            <Divider>预览模式</Divider>
            <Segmented
              style={{ marginBottom: 12 }}
              value={previewMode}
              onChange={(v) => setPreviewMode(v as 'page' | 'json')}
              options={[
                { label: '页面预览', value: 'page' },
                { label: 'JSON 预览', value: 'json' },
              ]}
            />
            {previewMode === 'page' ? (
              <div
                style={{
                  border: '1px solid #f0f0f0',
                  borderRadius: 6,
                  minHeight: '62vh',
                  maxHeight: '72vh',
                  overflow: 'auto',
                  padding: 12,
                }}
              >
                <WorkspaceRenderer
                  key={`tpl-preview-${previewTemplate.id}-${previewMode}`}
                  config={toPreviewWorkspaceConfig(previewTemplate)}
                  loading={false}
                  context={{ templatePreview: true }}
                />
              </div>
            ) : (
              <div style={{ border: '1px solid #f0f0f0', borderRadius: 6, overflow: 'hidden' }}>
                <CodeEditor
                  value={JSON.stringify(previewTemplate.config, null, 2)}
                  language="json"
                  height={420}
                  readOnly
                  options={{
                    lineNumbers: 'on',
                    renderLineHighlight: 'line',
                    scrollBeyondLastLine: false,
                    automaticLayout: true,
                    minimap: { enabled: false },
                  }}
                />
              </div>
            )}
          </div>
        )}
      </Modal>

      {/* 保存为模板弹窗 */}
      <Modal
        title="保存为模板"
        open={showSaveModal}
        onOk={handleSaveAsTemplate}
        onCancel={() => {
          setShowSaveModal(false);
          saveForm.resetFields();
        }}
      >
        <Form form={saveForm} layout="vertical">
          <Form.Item
            name="name"
            label="模板名称"
            rules={[{ required: true, message: '请输入模板名称' }]}
          >
            <Input placeholder="输入模板名称" />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="输入模板描述" />
          </Form.Item>

          <Form.Item
            name="type"
            label="类型"
            rules={[{ required: true, message: '请选择类型' }]}
            initialValue="workspace"
          >
            <Select options={[{ value: 'workspace', label: '工作空间' }]} />
          </Form.Item>

          <Form.Item
            name="category"
            label="分类"
            rules={[{ required: true, message: '请选择分类' }]}
            initialValue="custom"
          >
            <Select
              options={[
                { value: 'standard', label: '标准' },
                { value: 'gaming', label: '游戏' },
                { value: 'analytics', label: '分析' },
                { value: 'admin', label: '管理' },
                { value: 'custom', label: '自定义' },
              ]}
            />
          </Form.Item>

          <Form.Item name="scope" label="保存到" initialValue="personal">
            <Select
              options={[
                { value: 'personal', label: '个人模板' },
                { value: 'team', label: '团队模板' },
              ]}
            />
          </Form.Item>

          <Form.Item name="tags" label="标签">
            <Select mode="tags" placeholder="输入标签后按回车" tokenSeparators={[',']} />
          </Form.Item>
        </Form>
      </Modal>
    </Modal>
  );
}
